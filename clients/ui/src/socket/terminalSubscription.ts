import type {
  TerminalSubscriptionPhase,
  TerminalSubscriptionSnapshot,
} from "../store/subscriptions";
import { CAP_MS } from "./backoff";
import type { SubscribeOutcome } from "./retry";

export interface TerminalSubscriptionTransport {
  subscribe(sessionId: string, cols: number, rows: number): Promise<SubscribeOutcome>;
  unsubscribe(sessionId: string): Promise<void>;
}

export type TerminalSubscriptionLease = {
  release: () => void;
};

type ControllerOptions = {
  cooldown?: () => Promise<void>;
  onSnapshot?: (snapshot: TerminalSubscriptionSnapshot) => void;
};

const defaultCooldown = () => new Promise<void>((resolve) => setTimeout(resolve, CAP_MS));

export class TerminalSubscriptionController {
  private desired: {
    sessionId: string;
    token: number;
    geometry: { cols: number; rows: number } | null;
  } | null = null;
  private wireSessionId: string | null = null;
  private connected = false;
  private connectionEpoch = 0;
  private ownershipEpoch = 0;
  private phase: TerminalSubscriptionPhase = "idle";
  private attempt = 0;
  private lastError: string | null = null;
  private running = false;
  private rerun = false;
  private wakeReconcile: (() => void) | null = null;
  private readonly cooldown: () => Promise<void>;
  private readonly onSnapshot?: (snapshot: TerminalSubscriptionSnapshot) => void;

  constructor(
    private readonly transport: TerminalSubscriptionTransport,
    options: ControllerOptions = {},
  ) {
    this.cooldown = options.cooldown ?? defaultCooldown;
    this.onSnapshot = options.onSnapshot;
  }

  acquire(sessionId: string): TerminalSubscriptionLease {
    this.ownershipEpoch += 1;
    const token = this.ownershipEpoch;
    const sameSession = this.desired?.sessionId === sessionId;
    this.desired = { sessionId, token, geometry: null };
    this.lastError = null;
    this.attempt = 0;
    if (this.phase === "blocked") {
      this.phase = this.connected ? "idle" : "disconnected";
    } else if (!sameSession && this.phase !== "disconnected") {
      this.phase = "idle";
    }
    this.publish();
    this.schedule();
    return {
      release: () => {
        queueMicrotask(() => {
          if (this.desired?.token !== token) return;
          this.desired = null;
          this.ownershipEpoch += 1;
          this.lastError = null;
          this.attempt = 0;
          this.publish();
          this.schedule();
        });
      },
    };
  }

  updateGeometry(sessionId: string, cols: number, rows: number): void {
    if (this.desired?.sessionId !== sessionId || cols <= 0 || rows <= 0) return;
    this.desired = { ...this.desired, geometry: { cols, rows } };
    this.schedule();
  }

  onOpen(): void {
    this.connected = true;
    this.connectionEpoch += 1;
    this.wireSessionId = null;
    this.phase = "idle";
    this.lastError = null;
    this.publish();
    this.schedule();
  }

  onClose(): void {
    this.connected = false;
    this.connectionEpoch += 1;
    this.wireSessionId = null;
    this.phase = this.desired ? "disconnected" : "idle";
    this.lastError = this.desired ? "connection-closed" : null;
    this.publish();
  }

  onSurfaceSevered(sessionId: string): void {
    if (!this.connected || this.wireSessionId !== sessionId) return;
    this.wireSessionId = null;
    this.publish();
    this.schedule();
  }

  snapshot(): TerminalSubscriptionSnapshot {
    return {
      sessionId: this.desired?.sessionId ?? null,
      phase: this.phase,
      attempt: this.attempt,
      lastError: this.lastError,
      ownershipEpoch: this.ownershipEpoch,
    };
  }

  private publish(): void {
    this.onSnapshot?.(this.snapshot());
  }

  private schedule(): void {
    if (this.running) {
      this.rerun = true;
      this.wakeReconcile?.();
      return;
    }
    this.running = true;
    void this.reconcile().finally(() => {
      this.running = false;
      if (this.rerun) {
        this.rerun = false;
        this.schedule();
      }
    });
  }

  private async reconcile(): Promise<void> {
    while (this.connected) {
      const desiredId = this.desired?.sessionId ?? null;
      if (this.wireSessionId && this.wireSessionId !== desiredId) {
        const oldSessionId = this.wireSessionId;
        const epoch = this.connectionEpoch;
        await this.transport.unsubscribe(oldSessionId);
        if (epoch !== this.connectionEpoch) continue;
        if (this.wireSessionId === oldSessionId) this.wireSessionId = null;
        continue;
      }
      if (!desiredId) {
        this.phase = "idle";
        this.lastError = null;
        this.publish();
        return;
      }
      const geometry = this.desired?.geometry ?? null;
      if (!geometry) {
        this.phase = "idle";
        this.publish();
        return;
      }
      if (this.wireSessionId === desiredId) {
        this.phase = "confirmed";
        this.lastError = null;
        this.publish();
        return;
      }
      if (this.phase === "blocked") return;

      const epoch = this.connectionEpoch;
      this.phase = "subscribing";
      this.publish();
      const outcome = await this.transport.subscribe(desiredId, geometry.cols, geometry.rows);
      if (epoch !== this.connectionEpoch) continue;
      if (outcome.status === "confirmed") {
        const latest = this.desired;
        if (
          latest?.sessionId !== desiredId ||
          latest.geometry?.cols !== geometry.cols ||
          latest.geometry?.rows !== geometry.rows
        ) {
          await this.transport.unsubscribe(desiredId);
          if (epoch !== this.connectionEpoch) continue;
          continue;
        }
        this.wireSessionId = desiredId;
        this.phase = "confirmed";
        this.attempt = 0;
        this.lastError = null;
        this.publish();
        continue;
      }
      if (outcome.lastError === "connection-closed") {
        this.phase = "disconnected";
        this.lastError = outcome.lastError;
        this.publish();
        return;
      }
      if (outcome.lastError !== "frame-not-ready") {
        this.phase = "blocked";
        this.lastError = outcome.lastError;
        this.publish();
        return;
      }

      this.phase = "waiting";
      this.attempt += 1;
      this.lastError = outcome.lastError;
      this.publish();
      await this.waitForCooldownOrChange();
    }
  }

  private async waitForCooldownOrChange(): Promise<void> {
    let wake!: () => void;
    const changed = new Promise<void>((resolve) => {
      wake = resolve;
    });
    this.wakeReconcile = wake;
    try {
      await Promise.race([this.cooldown(), changed]);
    } finally {
      if (this.wakeReconcile === wake) this.wakeReconcile = null;
    }
  }
}
