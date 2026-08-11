import { parseSessionKey, sessionKey } from "../fleet/session-identity";
import type { FleetSessionRow } from "../fleet/session-identity";
import { PROTOCOL_VERSION, PROTOCOL_VERSION_HEADER } from "../protocol-version";
import { useDaemonStore } from "../store/daemon";
import { useFrameMessagingStore } from "../store/frameMessaging";
import { useNotificationsStore } from "../store/notifications";
import { useSubscriptionStore } from "../store/subscriptions";
import { useTranscriptStore } from "../store/transcripts";
import { useWorkspaceActivityStore } from "../store/workspaceActivity";
import type { ClientFrame } from "../wire/client";
import { parseServerFrame, serializeClientFrame } from "../wire/codec";
import type {
  ControlFrame,
  LifecycleOutcomeFrame,
  OutputFrame,
  RespErrFrame,
  RespOKFrame,
  SessionInfo,
} from "../wire/server";
import { backoffDelay, exceededAttempts } from "./backoff";
import { type RetryDeps, type SubscribeOutcome, subscribeWithRetry } from "./retry";
import {
  TerminalSubscriptionController,
  type TerminalSubscriptionLease,
} from "./terminalSubscription";
import type { PublicCorrelation } from "./transportObservation";

export type ConnectionConfig = {
  ticketEndpoint: string; // POST /api/ws-ticket
  wsUrl: (ticket: string) => string; // build ws://host/...?ticket=
  bearerToken: string;
  // factories injectable for tests
  wsFactory?: (url: string) => WebSocket;
  sleep?: (ms: number) => Promise<void>;
  fetchFn?: typeof fetch;
  /** FleetRuntime owns membership in hosted clients. */
  publishSessions?: boolean;
  serverId?: string;
};

type Pending = {
  resolve: (resp: RespOKFrame | RespErrFrame) => void;
};

/** Private dual-fence authority for one ticket/socket attempt. */
/**
 * Why a data-plane frame was dropped instead of sent.
 *
 * `server-mismatch` is a benign transition (the active server changed while a
 * keystroke was in flight); `decode-failed` is an internal contract violation
 * (a value reached the data plane that is neither a fleet key nor a session id).
 * The split exists so the second can reach the user without the first crying
 * wolf during a normal server switch.
 */
export type DataPlaneDropReason = "server-mismatch" | "decode-failed";

/** Gateway session ids are opaque but always drawn from this alphabet. */
const BARE_SESSION_ID = /^[A-Za-z0-9_-]+$/;

type AttemptAuth = {
  lifecycle: number;
  attempt: number;
};

export class Connection {
  private cfg: ConnectionConfig;
  private ws: WebSocket | null = null;
  private pending = new Map<string, Pending>();
  /** Automatic retries completed or in-flight within the current lifecycle (0..MAX_ATTEMPTS). */
  private reconnectAttempt = 0;
  private closedByUser = false;
  /**
   * True while a lifecycle owns automatic recovery or an in-flight attempt.
   * Exhaustion and user close clear it so Retry can start a fresh lifecycle.
   */
  private lifecycleActive = false;
  /** Monotonic lifecycle generation — bumped on start/Retry/close/server-switch. */
  private lifecycle = 0;
  /** Monotonic attempt sequence within the Connection instance. */
  private attempt = 0;
  /** True while a reconnect delay is scheduled or a close is being handled once. */
  private reconnecting = false;
  /** Cancels a pending injected-sleep reconnect without depending on timer abort APIs. */
  private cancelScheduledRetry: (() => void) | null = null;
  private reqIdCounter = 0;
  /** upstream session id -> fleet key, for the kHz output path (see fleetSessionID). */
  private readonly fleetIdMemo = new Map<string, string>();
  private drops: Record<DataPlaneDropReason, number> = { "server-mismatch": 0, "decode-failed": 0 };
  private notifiedDrops = new Set<string>();
  private terminalSubscriptions: TerminalSubscriptionController;

  constructor(cfg: ConnectionConfig) {
    this.cfg = cfg;
    this.terminalSubscriptions = new TerminalSubscriptionController(
      {
        subscribe: (sessionId, cols, rows) => this.subscribeOnce(sessionId, cols, rows),
        unsubscribe: (sessionId) => this.unsubscribeOnce(sessionId),
        publishDesired: (sessionId, cols, rows, correlation, desired) =>
          this.publishDesired(sessionId, cols, rows, correlation, desired),
      },
      {
        onSnapshot: (snapshot) => useSubscriptionStore.getState().replace(snapshot),
        onDeliveryTimeout: () => this.ws?.close(),
        onAuthoritativeTerminal: () => {
          // The controller owns the observation transition; this callback is
          // reserved for future stage watermark consumers.
        },
      },
    );
  }

  async start(): Promise<void> {
    // React StrictMode mounts → close()s → remounts the parent effect, which
    // can leave closedByUser=true on a Connection reused across remounts
    // (the useMemo instance is preserved). Reset it so the next disconnect
    // triggers handleClose() reconnect logic instead of permanently halting.
    this.beginLifecycle("connecting");
    try {
      await this.connect();
    } catch {
      // Initial ticket/socket failure follows the same bounded recovery path
      // as an established socket close; desired state is retained.
      this.handleClose({ lifecycle: this.lifecycle, attempt: this.attempt });
    }
  }

  /** Restarts a connection whose bounded automatic recovery has stopped. */
  retry(): void {
    // Single-flight: while a lifecycle is active (connecting/reconnecting/open),
    // additional Retry activations are no-ops.
    if (this.lifecycleActive) return;
    void this.start();
  }

  close(): void {
    this.closedByUser = true;
    this.lifecycleActive = false;
    this.lifecycle += 1;
    this.cancelRetryDelay();
    this.reconnecting = false;
    this.drainPending();
    this.retireSocket();
    this.terminalSubscriptions.onClose();
    useDaemonStore.getState().setStatus("closed");
  }

  /**
   * Send terminal input for a session addressed by its fleet key.
   *
   * The data plane takes an intent, not a frame. Assembling `{k:"i"}` outside
   * this class is what put a compound fleet key on the wire: the gateway could
   * not resolve it, dropped it with a log line and no reply, and the keystroke
   * vanished. Callers name the session and the bytes; the wire format and the
   * id space stay in here.
   */
  sendTerminalInput(fleetSessionId: string, data: string): void {
    const upstream = this.dataPlaneUpstreamID(fleetSessionId);
    if (upstream === null) return;
    this.sendFrame({ k: "i", d: data, sessionId: upstream });
  }

  /** Send a terminal resize for a session addressed by its fleet key. */
  sendTerminalResize(fleetSessionId: string, cols: number, rows: number): void {
    const upstream = this.dataPlaneUpstreamID(fleetSessionId);
    if (upstream === null) return;
    this.sendFrame({ k: "r", cols, rows, sessionId: upstream });
  }

  /**
   * Frames dropped by the data plane, by reason, for this lifecycle.
   *
   * Read-only and per-connection on purpose: it is a diagnostic, not app state.
   * `server-mismatch` is expected to be non-zero during a server switch and
   * carries no user-facing signal; `decode-failed` means something handed the
   * data plane a value that is neither a fleet key nor a session id.
   */
  get dataPlaneDrops(): Readonly<Record<DataPlaneDropReason, number>> {
    return { ...this.drops };
  }

  private sendFrame(frame: ClientFrame): void {
    const ws = this.ws;
    // A socket that exists but is still CONNECTING throws on send, and this is
    // reachable: xterm fits and emits a resize as soon as the terminal mounts,
    // which can land before the handshake completes. Dropping is right for both
    // planes — the control plane retries after the socket opens, and a resize
    // for an unopened socket has nothing to resize.
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(serializeClientFrame(frame));
  }

  acquireTerminal(sessionId: string): TerminalSubscriptionLease {
    return this.terminalSubscriptions.acquire(sessionId);
  }

  updateTerminalGeometry(sessionId: string, cols: number, rows: number): void {
    this.terminalSubscriptions.updateGeometry(sessionId, cols, rows);
  }

  /** Force an immediate terminal lease renewal. Wired to `visibilitychange`
   *  so a page that returns to visibility after being throttled recovers a
   *  subscription silently expired by the daemon (ADR
   *  terminal-lifecycle-bounds: 4s renewal / 12s expiry). No-op unless a
   *  confirmed terminal subscription exists. */
  forceTerminalRenewal(): void {
    this.terminalSubscriptions.forceRenewal();
  }

  private beginLifecycle(status: "connecting"): void {
    // Drop records are scoped to an explicitly started lifecycle — start() and
    // retry(). Bounded automatic reconnects deliberately keep the counters and
    // the dedupe set: they are the same lifecycle from the user's point of
    // view, and re-toasting the same unroutable session on every reconnect
    // would be noise.
    this.drops = { "server-mismatch": 0, "decode-failed": 0 };
    this.notifiedDrops = new Set();
    this.cancelRetryDelay();
    this.reconnecting = false;
    this.lifecycle += 1;
    this.reconnectAttempt = 0;
    this.closedByUser = false;
    this.lifecycleActive = true;
    this.drainPending();
    this.retireSocket();
    useDaemonStore.getState().setStatus(status);
  }

  private isCurrent(auth: AttemptAuth): boolean {
    return auth.lifecycle === this.lifecycle && auth.attempt === this.attempt;
  }

  private cancelRetryDelay(): void {
    this.cancelScheduledRetry?.();
    this.cancelScheduledRetry = null;
  }

  private retireSocket(): void {
    const ws = this.ws;
    this.ws = null;
    if (!ws) return;
    ws.onopen = null;
    ws.onmessage = null;
    ws.onclose = null;
    ws.onerror = null;
    try {
      ws.close();
    } catch {
      // ignore
    }
  }

  private async subscribeOnce(
    sessionId: string,
    cols: number,
    rows: number,
  ): Promise<SubscribeOutcome> {
    const deps: RetryDeps = {
      send: (frame) => this.sendFrame(frame),
      awaitResponse: (reqId) =>
        new Promise<RespOKFrame | RespErrFrame>((resolve) => {
          this.pending.set(reqId, { resolve });
        }),
      newReqId: () => this.nextReqId(),
      sleep: this.cfg.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms))),
    };
    return subscribeWithRetry(this.upstreamSessionID(sessionId), cols, rows, deps);
  }

  private async unsubscribeOnce(sessionId: string): Promise<void> {
    const reqId = this.nextReqId();
    const response = new Promise<RespOKFrame | RespErrFrame>((resolve) => {
      this.pending.set(reqId, { resolve });
    });
    this.sendFrame({ k: "u", reqId, sessionId: this.upstreamSessionID(sessionId) });
    await response;
  }

  private async publishDesired(
    sessionId: string,
    cols: number,
    rows: number,
    correlation: PublicCorrelation,
    desired: boolean,
  ): Promise<SubscribeOutcome> {
    const reqId = this.nextReqId();
    const response = new Promise<RespOKFrame | RespErrFrame>((resolve) => {
      this.pending.set(reqId, { resolve });
    });
    this.sendFrame({
      k: "ld",
      reqId,
      sessionId: this.upstreamSessionID(sessionId),
      cols,
      rows,
      correlation,
      desired,
    });
    const resp = await response;
    if (resp.k === "r") return { status: "confirmed", reqId };
    return { status: "exhausted", lastError: resp.code };
  }

  private nextReqId(): string {
    this.reqIdCounter += 1;
    return `r${this.reqIdCounter}`;
  }

  /**
   * Resolve a data-plane session id to the upstream id, or drop it.
   *
   * This never throws. It runs inside xterm's synchronous `onData` emitter, so
   * a throw would unwind into xterm itself; the control plane keeps its throw
   * contract because it runs from async request paths. Dropping silently is
   * equally wrong — that is exactly what the gateway does with an unresolvable
   * id, and it is why this defect survived two repairs — so every drop is
   * counted, and the one that means "internal contract violation" also reaches
   * the user.
   */
  private dataPlaneUpstreamID(sessionId: string): string | null {
    // A fleet key is JSON, so it starts with "["; a gateway session id is
    // [A-Za-z0-9_-] and never can. Checking the first character keeps
    // JSON.parse off the keystroke path without changing which values are
    // classified as which.
    if (sessionId.startsWith("[")) {
      const ref = parseSessionKey(sessionId);
      if (!ref) return this.dropDataPlaneFrame(sessionId, "decode-failed");
      if (this.cfg.serverId && ref.serverId !== this.cfg.serverId) {
        // Benign and expected: App.tsx rebuilds the Connection when the active
        // server changes, so in-flight keystrokes can arrive addressed to the
        // previous server. Counted, never surfaced.
        return this.dropDataPlaneFrame(sessionId, "server-mismatch");
      }
      return ref.sessionId;
    }
    if (BARE_SESSION_ID.test(sessionId)) return sessionId;
    return this.dropDataPlaneFrame(sessionId, "decode-failed");
  }

  private dropDataPlaneFrame(sessionId: string, reason: DataPlaneDropReason): null {
    this.drops[reason] += 1;
    if (reason === "decode-failed") {
      // Keystrokes repeat, so the notification is deduped per (session, reason)
      // for the lifetime of this connection while the counter keeps rising —
      // the count is the diagnostic, the toast is the alarm.
      const dedupeKey = `${reason}:${sessionId}`;
      if (!this.notifiedDrops.has(dedupeKey)) {
        this.notifiedDrops.add(dedupeKey);
        useNotificationsStore.getState().add({
          level: "error",
          message: `A terminal frame could not be routed to its session (${reason}).`,
        });
      }
    }
    return null;
  }

  private upstreamSessionID(value: string): string {
    const ref = parseSessionKey(value);
    if (!ref) return value;
    if (this.cfg.serverId && ref.serverId !== this.cfg.serverId) {
      throw new Error(`session belongs to ${ref.serverId}, not ${this.cfg.serverId}`);
    }
    return ref.sessionId;
  }

  private async connect(): Promise<void> {
    this.attempt += 1;
    const auth: AttemptAuth = { lifecycle: this.lifecycle, attempt: this.attempt };
    const fetchFn = this.cfg.fetchFn ?? fetch;
    const resp = await fetchFn(this.cfg.ticketEndpoint, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${this.cfg.bearerToken}`,
        [PROTOCOL_VERSION_HEADER]: PROTOCOL_VERSION,
      },
    });
    // close()/Retry/server-switch may have advanced authority while the ticket
    // was in flight — fence the result rather than depending on fetch abort.
    if (!this.isCurrent(auth) || this.closedByUser) return;
    if (!resp.ok) {
      throw new Error(`ws-ticket failed: ${resp.status}`);
    }
    const body = (await resp.json()) as { ticket: string };
    if (!this.isCurrent(auth) || this.closedByUser) return;
    const wsFactory = this.cfg.wsFactory ?? ((u) => new WebSocket(u));
    const ws = wsFactory(this.cfg.wsUrl(body.ticket));
    if (!this.isCurrent(auth) || this.closedByUser) {
      try {
        ws.close();
      } catch {
        // ignore
      }
      return;
    }
    // At most one live active socket: detach any prior handle before install.
    this.retireSocket();
    this.ws = ws;
    this.ws.onopen = () => this.handleOpen(auth);
    this.ws.onmessage = (ev) => this.handleMessage(String(ev.data), auth);
    this.ws.onclose = () => this.handleClose(auth);
    // onerror is intentionally a noop: browsers always fire onclose after onerror,
    // so letting onerror also call handleClose would trigger reconnect twice.
    this.ws.onerror = () => {};
  }

  private handleOpen(auth: AttemptAuth): void {
    if (!this.isCurrent(auth)) return;
    // WebSocket open is internal transport state only (FR-REC-002 / UAC-002).
    // Terminal subscription reopen still keys off transport open.
    this.terminalSubscriptions.onOpen();
  }

  private handleMessage(raw: string, auth: AttemptAuth): void {
    if (!this.isCurrent(auth)) return;
    const frame = parseServerFrame(raw);
    if (!frame) return;
    if (Array.isArray(frame)) {
      // OutputFrame — direct callback per FR-β07 (kHz output, UI must not block)
      const output = frame as OutputFrame;
      output[3] = this.fleetSessionID(output[3]);
      this.onOutput?.(output);
      return;
    }
    switch (frame.k) {
      case "h":
        // A socket handshake only proves that the gateway is reachable.
        // Hello is the first daemon-derived lifecycle frame and therefore the
        // authoritative recovery boundary (contract-daemon-hello-proof).
        if (!this.isCurrent(auth)) return;
        this.reconnectAttempt = 0;
        useDaemonStore.getState().setDaemonDisconnected(false);
        useDaemonStore.getState().setStatus("open");
        useWorkspaceActivityStore.getState().setTransportDegraded(false);
        if (frame.clientInstanceID) {
          this.terminalSubscriptions.setClientInstanceID(frame.clientInstanceID);
        }
        {
          const helloRows = this.qualifyHelloSessions(frame.sessions ?? []);
          if (this.cfg.publishSessions !== false) {
            useDaemonStore.getState().seedHello({ ...frame, sessions: helloRows });
            useFrameMessagingStore.getState().replaceFromSessions(helloRows);
          } else {
            // FleetRuntime owns membership, so hello must not rebind the session
            // list. Every authoritative hello — including the first — is still
            // the one-shot status-signal boundary: seed signal maps, overlay
            // gateway status onto existing membership, and suppress replayed
            // bells while durable human_input stays.
            useDaemonStore.getState().seedReconnectStatusSignals(helloRows);
          }
        }
        break;
      case "v": {
        const viewRows =
          frame.sessions === undefined ? undefined : this.qualifyHelloSessions(frame.sessions);
        if (frame.activity_session_id !== undefined) {
          frame.activity_session_id = this.fleetSessionID(frame.activity_session_id);
        }
        if (frame.activity_events?.length) {
          // The per-event session_id is a second bare id on the same frame. It
          // never called the identity helpers, which is why the earlier
          // symptom-keyed scan missed it and why the store had to brute-force
          // every id form downstream.
          frame.activity_events = frame.activity_events.map((event) => ({
            ...event,
            session_id: this.fleetSessionID(event.session_id),
          }));
        }
        if (this.cfg.publishSessions !== false) {
          useDaemonStore.getState().applyViewUpdate({ ...frame, sessions: viewRows });
          if (viewRows !== undefined) {
            useFrameMessagingStore.getState().replaceFromSessions(viewRows);
          }
        } else {
          if (viewRows !== undefined) {
            // Fleet owns membership keys; still fold gateway status into the
            // existing fleet-qualified rows so Idle; Notification is live.
            useDaemonStore.getState().applyGatewaySessionStatus(viewRows);
          }
          if (frame.activity_events?.length) {
            // Workspace activity remains independent of membership rebind.
            useDaemonStore.getState().applyViewUpdate({
              k: "v",
              activity_session_id: frame.activity_session_id,
              activity_events: frame.activity_events,
            });
          }
        }
        break;
      }
      case "tt":
        useTranscriptStore
          .getState()
          .appendLine(this.fleetSessionID(frame.sessionId), "transcript", frame.line);
        break;
      case "et":
        useTranscriptStore
          .getState()
          .appendLine(this.fleetSessionID(frame.sessionId), "event-log", frame.line);
        break;
      case "n":
        useNotificationsStore.getState().addFromFrame(frame);
        break;
      case "lo":
        this.handleLifecycleOutcome(frame);
        break;
      case "c":
        this.handleControl(frame);
        break;
      case "r":
      case "e": {
        const p = this.pending.get(frame.reqId);
        if (p) {
          p.resolve(frame);
          this.pending.delete(frame.reqId);
        }
        break;
      }
    }
  }

  /**
   * The single inbound requalification path: every session id arriving from
   * this gateway becomes the fleet key the store and the UI address it by.
   *
   * Every inbound element goes through here — output frames, transcript and
   * event-log lines, workspace-activity ids, and the session lists on hello and
   * viewUpdate. Leaving one of them bare is what forced the downstream alias
   * sweep in store/workspaceActivity.ts, which then hid the next gap.
   *
   * Output frames arrive at kHz, so the mapping is memoized per upstream id.
   * The memo is safe to keep for the connection's lifetime because
   * `cfg.serverId` is fixed at construction: a different server means a
   * different Connection.
   */
  private fleetSessionID(value: string): string {
    const serverId = this.cfg.serverId;
    if (!serverId) return value;
    const memoized = this.fleetIdMemo.get(value);
    if (memoized !== undefined) return memoized;
    const key = sessionKey({ serverId, sessionId: value });
    this.fleetIdMemo.set(value, key);
    return key;
  }

  /**
   * Turn the gateway's session list into store rows.
   *
   * This is the ingress half of the identity boundary: what arrives is a wire
   * mirror keyed by the gateway's own id, what the store holds is a row that
   * knows which server it came from. The server is always this connection's —
   * never re-derived from the id — so a row cannot end up tagged with one
   * server and keyed by another.
   *
   * A connection with no configured server is a non-fleet client: ids pass
   * through unchanged and the row's server is empty, which is exactly what
   * "this client does not address sessions by server" means. Production always
   * configures one.
   */
  private qualifyHelloSessions(sessions: SessionInfo[]): FleetSessionRow[] {
    const serverId = this.cfg.serverId;
    if (!serverId) {
      return sessions.map((session) => ({
        ...session,
        serverId: "",
        upstreamSessionId: session.id,
      }));
    }
    return sessions.map((session) => {
      const bareId = parseSessionKey(session.id)?.sessionId ?? session.id;
      return {
        ...session,
        id: sessionKey({ serverId, sessionId: bareId }),
        upstreamSessionId: bareId,
        serverId,
      };
    });
  }

  private handleLifecycleOutcome(frame: LifecycleOutcomeFrame): void {
    if (frame.status === "accepted" || frame.status === "waiting") {
      return;
    }
    this.terminalSubscriptions.observeAuthoritativeTerminal(frame.correlation);
  }

  private handleControl(frame: ControlFrame): void {
    // ControlFrame: code is int (omitted when 0), data carries event payload string
    if (frame.data === "daemon-disconnected") {
      useDaemonStore.getState().setDaemonDisconnected(true);
      return;
    }
    if (frame.data === "surface-unsubscribed" && frame.sessionId) {
      this.terminalSubscriptions.onSurfaceSevered(frame.sessionId);
    }
  }

  private drainPending(): void {
    // Resolve all in-flight pending promises with a synthetic non-retryable error so
    // that awaiters (subscribeWithRetry) return immediately instead of hanging forever.
    for (const [reqId, p] of this.pending) {
      p.resolve({ k: "e", reqId, code: "connection-closed", message: "WebSocket closed" });
    }
    this.pending.clear();
  }

  private handleClose(auth: AttemptAuth): void {
    if (this.closedByUser) return;
    // Stale attempt/generation: late close must not schedule competing work.
    if (!this.isCurrent(auth)) return;
    // Guard: onerror + onclose both fire in real browsers. Only run once per attempt.
    if (this.reconnecting) return;
    this.reconnecting = true;
    this.drainPending();
    // Detach the closed socket so its late callbacks cannot re-enter.
    if (this.ws) {
      const ws = this.ws;
      this.ws = null;
      ws.onopen = null;
      ws.onmessage = null;
      ws.onclose = null;
      ws.onerror = null;
    }
    this.terminalSubscriptions.onClose();
    useDaemonStore.getState().setStatus("reconnecting");
    useWorkspaceActivityStore.getState().setTransportDegraded(true);
    if (exceededAttempts(this.reconnectAttempt)) {
      useDaemonStore.getState().setStatus("closed");
      this.lifecycleActive = false;
      this.reconnecting = false;
      return;
    }
    const delay = backoffDelay(this.reconnectAttempt);
    this.reconnectAttempt += 1;
    const scheduledLifecycle = this.lifecycle;
    const sleep = this.cfg.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms)));
    this.cancelRetryDelay();
    // Pair injected sleep with a cancel flag (discretion-cancelable-scheduler):
    // correctness fences on lifecycle even if the sleep promise still settles.
    let canceled = false;
    this.cancelScheduledRetry = () => {
      canceled = true;
    };
    void sleep(delay).then(() => {
      this.cancelScheduledRetry = null;
      this.reconnecting = false;
      if (canceled) return;
      if (this.closedByUser) return;
      if (scheduledLifecycle !== this.lifecycle) return;
      if (!this.lifecycleActive) return;
      this.connect().catch(() => {
        this.handleClose({ lifecycle: this.lifecycle, attempt: this.attempt });
      });
    });
  }

  // hook for TerminalPane: called on output frames (FR-β07: kHz output, not via store)
  onOutput?: (frame: OutputFrame) => void;
}

// tagSessions used to guess a session's server from initialServerHint() and
// apply it independently of qualifyHelloSessions. Two taggings for the same
// fact is how the id spaces drifted apart in the first place, so both hello and
// viewUpdate now go through qualifyHelloSessions on either side of the
// publishSessions branch. The branch itself stays — it decides who owns
// membership, which is a different question.
