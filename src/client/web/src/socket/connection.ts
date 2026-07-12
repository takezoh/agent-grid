import { useDaemonStore } from "../store/daemon";
import { useFrameMessagingStore } from "../store/frameMessaging";
import { useNotificationsStore } from "../store/notifications";
import { useSubscriptionStore } from "../store/subscriptions";
import { useTranscriptStore } from "../store/transcripts";
import type { ClientFrame } from "../wire/client";
import { parseServerFrame, serializeClientFrame } from "../wire/codec";
import type { ControlFrame, OutputFrame, RespErrFrame, RespOKFrame } from "../wire/server";
import { backoffDelay, exceededAttempts } from "./backoff";
import { type RetryDeps, type SubscribeOutcome, subscribeWithRetry } from "./retry";
import {
  TerminalSubscriptionController,
  type TerminalSubscriptionLease,
} from "./terminalSubscription";

export type ConnectionConfig = {
  ticketEndpoint: string; // POST /api/ws-ticket
  wsUrl: (ticket: string) => string; // build ws://host/...?ticket=
  bearerToken: string;
  // factories injectable for tests
  wsFactory?: (url: string) => WebSocket;
  sleep?: (ms: number) => Promise<void>;
  fetchFn?: typeof fetch;
};

type Pending = {
  resolve: (resp: RespOKFrame | RespErrFrame) => void;
};

export class Connection {
  private cfg: ConnectionConfig;
  private ws: WebSocket | null = null;
  private pending = new Map<string, Pending>();
  private reconnectAttempt = 0;
  private closedByUser = false;
  private reconnecting = false;
  private reqIdCounter = 0;
  private terminalSubscriptions: TerminalSubscriptionController;

  constructor(cfg: ConnectionConfig) {
    this.cfg = cfg;
    this.terminalSubscriptions = new TerminalSubscriptionController(
      {
        subscribe: (sessionId) => this.subscribeOnce(sessionId),
        unsubscribe: (sessionId) => this.unsubscribeOnce(sessionId),
      },
      {
        onSnapshot: (snapshot) => useSubscriptionStore.getState().replace(snapshot),
      },
    );
  }

  async start(): Promise<void> {
    // React StrictMode mounts → close()s → remounts the parent effect, which
    // can leave closedByUser=true on a Connection reused across remounts
    // (the useMemo instance is preserved). Reset it so the next disconnect
    // triggers handleClose() reconnect logic instead of permanently halting.
    this.closedByUser = false;
    useDaemonStore.getState().setStatus("connecting");
    await this.connect();
  }

  close(): void {
    this.closedByUser = true;
    this.drainPending();
    this.ws?.close();
    this.ws = null;
    this.terminalSubscriptions.onClose();
    useDaemonStore.getState().setStatus("closed");
  }

  send(frame: ClientFrame): void {
    this.ws?.send(serializeClientFrame(frame));
  }

  acquireTerminal(sessionId: string): TerminalSubscriptionLease {
    return this.terminalSubscriptions.acquire(sessionId);
  }

  private async subscribeOnce(sessionId: string): Promise<SubscribeOutcome> {
    const deps: RetryDeps = {
      send: (s) => this.ws?.send(s),
      awaitResponse: (reqId) =>
        new Promise<RespOKFrame | RespErrFrame>((resolve) => {
          this.pending.set(reqId, { resolve });
        }),
      newReqId: () => this.nextReqId(),
      sleep: this.cfg.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms))),
    };
    return subscribeWithRetry(sessionId, deps);
  }

  private async unsubscribeOnce(sessionId: string): Promise<void> {
    const reqId = this.nextReqId();
    const response = new Promise<RespOKFrame | RespErrFrame>((resolve) => {
      this.pending.set(reqId, { resolve });
    });
    this.send({ k: "u", reqId, sessionId });
    await response;
  }

  private nextReqId(): string {
    this.reqIdCounter += 1;
    return `r${this.reqIdCounter}`;
  }

  private async connect(): Promise<void> {
    const fetchFn = this.cfg.fetchFn ?? fetch;
    const resp = await fetchFn(this.cfg.ticketEndpoint, {
      method: "POST",
      headers: { Authorization: `Bearer ${this.cfg.bearerToken}` },
    });
    if (!resp.ok) {
      throw new Error(`ws-ticket failed: ${resp.status}`);
    }
    const body = (await resp.json()) as { ticket: string };
    // close() may have fired between the fetch await and here; if so, the
    // user has explicitly torn the connection down — do not create a fresh
    // WebSocket that nobody references (would leak the live socket and
    // keep emitting onopen/onclose handlers that touch a torn-down store).
    if (this.closedByUser) return;
    const wsFactory = this.cfg.wsFactory ?? ((u) => new WebSocket(u));
    const ws = wsFactory(this.cfg.wsUrl(body.ticket));
    // Symmetric guard: close() between the previous check and now races
    // with the wsFactory call. Belt-and-braces.
    if (this.closedByUser) {
      try {
        ws.close();
      } catch {
        // ignore
      }
      return;
    }
    this.ws = ws;
    this.ws.onopen = () => this.handleOpen();
    this.ws.onmessage = (ev) => this.handleMessage(String(ev.data));
    this.ws.onclose = () => this.handleClose();
    // onerror is intentionally a noop: browsers always fire onclose after onerror,
    // so letting onerror also call handleClose would trigger reconnect twice.
    this.ws.onerror = () => {};
  }

  private handleOpen(): void {
    this.reconnectAttempt = 0;
    useDaemonStore.getState().setStatus("open");
    this.terminalSubscriptions.onOpen();
  }

  private handleMessage(raw: string): void {
    const frame = parseServerFrame(raw);
    if (!frame) return;
    if (Array.isArray(frame)) {
      // OutputFrame — direct callback per FR-β07 (kHz output, UI must not block)
      this.onOutput?.(frame as OutputFrame);
      return;
    }
    switch (frame.k) {
      case "h":
        useDaemonStore.getState().seedHello(frame);
        useFrameMessagingStore.getState().replaceFromSessions(frame.sessions);
        break;
      case "v":
        useDaemonStore.getState().applyViewUpdate(frame);
        useFrameMessagingStore.getState().replaceFromSessions(frame.sessions);
        break;
      case "tt":
        useTranscriptStore.getState().appendLine(frame.sessionId, "transcript", frame.line);
        break;
      case "et":
        useTranscriptStore.getState().appendLine(frame.sessionId, "event-log", frame.line);
        break;
      case "n":
        useNotificationsStore.getState().addFromFrame(frame);
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

  private handleClose(): void {
    if (this.closedByUser) return;
    // Guard: onerror + onclose both fire in real browsers. Only run once.
    if (this.reconnecting) return;
    this.reconnecting = true;
    this.drainPending();
    this.terminalSubscriptions.onClose();
    useDaemonStore.getState().setStatus("reconnecting");
    if (exceededAttempts(this.reconnectAttempt)) {
      useDaemonStore.getState().setStatus("closed");
      this.reconnecting = false;
      return;
    }
    const delay = backoffDelay(this.reconnectAttempt);
    this.reconnectAttempt += 1;
    const sleep = this.cfg.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms)));
    void sleep(delay).then(() => {
      this.reconnecting = false;
      if (!this.closedByUser) {
        this.connect().catch(() => {
          this.handleClose();
        });
      }
    });
  }

  // hook for TerminalPane: called on output frames (FR-β07: kHz output, not via store)
  onOutput?: (frame: OutputFrame) => void;
}
