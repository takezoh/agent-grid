import { initialServerHint } from "../auth";
import { parseSessionKey, sessionKey } from "../fleet/session-protocol";
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

  send(frame: ClientFrame): void {
    this.ws?.send(serializeClientFrame(frame));
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
      send: (s) => this.ws?.send(s),
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
    this.send({ k: "u", reqId, sessionId: this.upstreamSessionID(sessionId) });
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
    this.send({
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
      if (this.cfg.serverId)
        output[3] = sessionKey({
          serverId: this.cfg.serverId,
          sessionId: output[3],
        });
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
        frame.sessions = tagSessions(frame.sessions);
        if (this.cfg.publishSessions !== false) {
          useDaemonStore.getState().seedHello(frame);
          useFrameMessagingStore.getState().replaceFromSessions(frame.sessions);
        } else {
          // FleetRuntime owns membership, so hello must not rebind the session
          // list. Every authoritative hello — including the first — is still
          // the one-shot status-signal boundary: seed signal maps, overlay
          // gateway status onto existing membership, and suppress replayed
          // bells while durable human_input stays.
          useDaemonStore
            .getState()
            .seedReconnectStatusSignals(this.qualifyHelloSessions(frame.sessions));
        }
        break;
      case "v":
        if (frame.sessions !== undefined) frame.sessions = tagSessions(frame.sessions);
        if (this.cfg.publishSessions !== false) {
          useDaemonStore.getState().applyViewUpdate(frame);
          if (frame.sessions !== undefined) {
            useFrameMessagingStore.getState().replaceFromSessions(frame.sessions);
          }
        } else {
          if (frame.sessions !== undefined) {
            // Fleet owns membership keys; still fold gateway status into the
            // existing fleet-qualified rows so Idle; Notification is live.
            useDaemonStore
              .getState()
              .applyGatewaySessionStatus(this.qualifyHelloSessions(frame.sessions));
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

  private fleetSessionID(value: string): string {
    return this.cfg.serverId
      ? sessionKey({ serverId: this.cfg.serverId, sessionId: value })
      : value;
  }

  /** Align hello session ids with fleet-qualified membership keys. */
  private qualifyHelloSessions(sessions: SessionInfo[]): SessionInfo[] {
    const serverId = this.cfg.serverId;
    if (!serverId) return sessions;
    return sessions.map((session) => {
      const bareId =
        session.upstreamSessionId ?? parseSessionKey(session.id)?.sessionId ?? session.id;
      return {
        ...session,
        id: sessionKey({ serverId, sessionId: bareId }),
        upstreamSessionId: bareId,
        // Force Connection.serverId — do not keep tagSessions' initialServerHint.
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

function tagSessions<T extends { serverId?: string; serverLabel?: string }>(
  sessions: T[] | null | undefined,
): T[] {
  const server = initialServerHint();
  return (sessions ?? []).map((session) => ({
    ...session,
    serverId: session.serverId ?? server.id,
    serverLabel: session.serverLabel ?? server.label,
  }));
}
