import type {
  TerminalSubscriptionPhase,
  TerminalSubscriptionSnapshot,
} from "../store/subscriptions";
import { CAP_MS } from "./backoff";
import type { SubscribeOutcome } from "./retry";
import {
  type PublicCorrelation,
  type TransportObservation,
  observeTransport,
  reduceTransportObservation,
} from "./transportObservation";

export interface TerminalSubscriptionTransport {
  subscribe(sessionId: string, cols: number, rows: number): Promise<SubscribeOutcome>;
  unsubscribe(sessionId: string): Promise<void>;
  publishDesired?: (
    sessionId: string,
    cols: number,
    rows: number,
    correlation: PublicCorrelation,
    desired: boolean,
  ) => Promise<SubscribeOutcome>;
}

export type TerminalSubscriptionLease = {
  release: () => void;
};

type ControllerOptions = {
  cooldown?: () => Promise<void>;
  onSnapshot?: (snapshot: TerminalSubscriptionSnapshot) => void;
  onDeliveryTimeout?: () => void;
  clientInstanceID?: string;
  onAuthoritativeTerminal?: (correlation: PublicCorrelation) => void;
  // Interval between lease-renewal `ld` re-sends. Daemon-side lease expiry is
  // 12s (ADR terminal-lifecycle-bounds); 4s gives three heartbeats of margin.
  renewalIntervalMs?: number;
};

const defaultCooldown = () => new Promise<void>((resolve) => setTimeout(resolve, CAP_MS));
const RENEWAL_INTERVAL_MS = 4000;

export class TerminalSubscriptionController {
  private desired: {
    sessionId: string;
    token: number;
    geometry: { cols: number; rows: number } | null;
  } | null = null;
  private wireSessionId: string | null = null;
  private wireGeometry: { cols: number; rows: number } | null = null;
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
  private readonly onDeliveryTimeout?: () => void;
  private clientInstanceID: string;
  private readonly onAuthoritativeTerminal?: (correlation: PublicCorrelation) => void;
  private connectionGeneration = 0;
  private clientRevision = 0;
  private observation: TransportObservation | null = null;
  private observationTimer: ReturnType<typeof setTimeout> | null = null;
  // Lease renewal: daemon expires our binding after 12s of no fresh `ld`. We
  // re-send `ld(desired=true)` with a fresh revision every 4s while phase is
  // "confirmed" so the subscription outlives idle periods (ADR
  // terminal-lifecycle-bounds: "renewal 4s, expiry 12s").
  private renewalTimer: ReturnType<typeof setTimeout> | null = null;
  private renewalPending = false;
  private readonly renewalIntervalMs: number;

  constructor(
    private readonly transport: TerminalSubscriptionTransport,
    options: ControllerOptions = {},
  ) {
    this.cooldown = options.cooldown ?? defaultCooldown;
    this.onSnapshot = options.onSnapshot;
    this.onDeliveryTimeout = options.onDeliveryTimeout;
    this.clientInstanceID = options.clientInstanceID ?? makeClientInstanceID();
    this.onAuthoritativeTerminal = options.onAuthoritativeTerminal;
    this.renewalIntervalMs = options.renewalIntervalMs ?? RENEWAL_INTERVAL_MS;
  }

  acquire(sessionId: string): TerminalSubscriptionLease {
    this.replaceObservationIfNeeded(sessionId, this.desired?.geometry ?? null);
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
          this.clearRenewalTimer();
          this.publish();
          this.schedule();
        });
      },
    };
  }

  updateGeometry(sessionId: string, cols: number, rows: number): void {
    if (this.desired?.sessionId !== sessionId || cols <= 0 || rows <= 0) return;
    const oldGeometry = this.desired.geometry;
    if (oldGeometry && (oldGeometry.cols !== cols || oldGeometry.rows !== rows)) {
      this.replaceObservationIfNeeded(sessionId, oldGeometry);
    }
    this.desired = { ...this.desired, geometry: { cols, rows } };
    this.schedule();
  }

  onOpen(): void {
    this.connected = true;
    this.connectionEpoch += 1;
    this.connectionGeneration += 1;
    this.clientRevision += 1;
    this.clearObservationTimer();
    this.clearRenewalTimer();
    this.observation = null;
    this.wireSessionId = null;
    this.wireGeometry = null;
    this.phase = "idle";
    this.lastError = null;
    this.publish();
    this.schedule();
  }

  setClientInstanceID(clientInstanceID: string): void {
    if (!clientInstanceID || clientInstanceID === this.clientInstanceID) return;
    this.clientInstanceID = clientInstanceID;
  }

  onClose(): void {
    this.connected = false;
    this.connectionEpoch += 1;
    this.clearObservationTimer();
    this.clearRenewalTimer();
    if (this.observation?.kind === "observing") {
      this.observation = reduceTransportObservation(this.observation, { type: "socket_close" });
    }
    this.wireSessionId = null;
    this.wireGeometry = null;
    this.phase = this.desired ? "disconnected" : "idle";
    this.lastError = this.desired ? "connection-closed" : null;
    this.publish();
  }

  onSurfaceSevered(sessionId: string): void {
    if (!this.connected || this.wireSessionId !== sessionId) return;
    this.wireSessionId = null;
    this.wireGeometry = null;
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

  /** Called by the wire layer when a daemon terminal outcome is received. */
  observeAuthoritativeTerminal(correlation: PublicCorrelation): void {
    if (!this.observation) return;
    const next = reduceTransportObservation(this.observation, {
      type: "authoritative_terminal",
      correlation,
    });
    if (next.kind === "observed_remote") this.clearObservationTimer();
    this.observation = next;
    if (next.kind === "observed_remote") this.onAuthoritativeTerminal?.(correlation);
  }

  /** Force an immediate lease renewal — used as the `visibilitychange`
   *  safety net so a page that returns to visibility after being throttled
   *  can recover a subscription silently expired by the daemon. No-op unless
   *  a confirmed subscription exists. */
  forceRenewal(): void {
    if (this.phase !== "confirmed") return;
    this.clearRenewalTimer();
    this.renewalPending = true;
    this.schedule();
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
        if (this.transport.publishDesired && this.observation) {
          await this.transport.publishDesired(
            oldSessionId,
            geometryFor(this.desired),
            geometryForRows(this.desired),
            this.observation.correlation,
            false,
          );
        } else {
          await this.transport.unsubscribe(oldSessionId);
        }
        if (epoch !== this.connectionEpoch) continue;
        if (this.wireSessionId === oldSessionId) {
          this.wireSessionId = null;
          this.wireGeometry = null;
        }
        continue;
      }
      if (!desiredId) {
        this.phase = "idle";
        this.lastError = null;
        this.publish();
        this.clearRenewalTimer();
        return;
      }
      const geometry = this.desired?.geometry ?? null;
      if (!geometry) {
        this.phase = "idle";
        this.publish();
        this.clearRenewalTimer();
        return;
      }
      if (
        !this.renewalPending &&
        this.wireSessionId === desiredId &&
        this.wireGeometry?.cols === geometry.cols &&
        this.wireGeometry?.rows === geometry.rows
      ) {
        this.phase = "confirmed";
        this.lastError = null;
        this.publish();
        this.scheduleRenewal();
        return;
      }
      if (this.phase === "blocked") return;

      const epoch = this.connectionEpoch;
      this.renewalPending = false;
      this.phase = "subscribing";
      this.publish();
      this.beginObservation();
      const outcome =
        this.transport.publishDesired && this.observation?.kind === "observing"
          ? await this.transport.publishDesired(
              desiredId,
              geometry.cols,
              geometry.rows,
              this.observation.correlation,
              true,
            )
          : await this.transport.subscribe(desiredId, geometry.cols, geometry.rows);
      if (epoch !== this.connectionEpoch) continue;
      if (outcome.status === "confirmed") {
        // A v2 publish acknowledgement only proves gateway admission. The
        // watchdog remains active until a matching daemon terminal outcome
        // arrives. Legacy subscribe retains its historical immediate ACK
        // semantics.
        if (!this.transport.publishDesired) this.completeObservation();
        const latest = this.desired;
        if (
          latest?.sessionId !== desiredId ||
          latest.geometry?.cols !== geometry.cols ||
          latest.geometry?.rows !== geometry.rows
        ) {
          if (this.transport.publishDesired && this.observation) {
            await this.transport.publishDesired(
              desiredId,
              geometry.cols,
              geometry.rows,
              this.observation.correlation,
              false,
            );
          } else {
            await this.transport.unsubscribe(desiredId);
          }
          if (epoch !== this.connectionEpoch) continue;
          continue;
        }
        this.wireSessionId = desiredId;
        this.wireGeometry = { ...geometry };
        this.phase = "confirmed";
        this.attempt = 0;
        this.lastError = null;
        this.publish();
        this.scheduleRenewal();
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

  private replaceObservationIfNeeded(
    _sessionId: string,
    _geometry: { cols: number; rows: number } | null,
  ): void {
    if (this.observation?.kind !== "observing") return;
    this.clientRevision += 1;
    this.observation = reduceTransportObservation(this.observation, {
      type: "publication_replace",
      nextRevision: this.clientRevision,
    });
    this.clearObservationTimer();
  }

  private beginObservation(): void {
    this.clearObservationTimer();
    const correlation: PublicCorrelation = {
      clientInstanceID: this.clientInstanceID,
      connectionGeneration: this.connectionGeneration,
      clientRevision: ++this.clientRevision,
    };
    const deadlineMs = Date.now() + 4000;
    this.observation = observeTransport(correlation, deadlineMs);
    this.observationTimer = setTimeout(() => {
      if (this.observation?.kind !== "observing") return;
      this.observation = reduceTransportObservation(this.observation, {
        type: "deadline",
        nowMs: Date.now(),
      });
      if (this.observation.kind !== "delivery_timeout") return;
      this.phase = "disconnected";
      this.lastError = "delivery-timeout";
      this.publish();
      this.onDeliveryTimeout?.();
    }, 4000);
  }

  private completeObservation(): void {
    if (this.observation?.kind === "observing") {
      this.observation = reduceTransportObservation(this.observation, {
        type: "authoritative_terminal",
        correlation: this.observation.correlation,
      });
    }
    this.clearObservationTimer();
  }

  private clearObservationTimer(): void {
    if (this.observationTimer !== null) clearTimeout(this.observationTimer);
    this.observationTimer = null;
  }

  private scheduleRenewal(): void {
    this.clearRenewalTimer();
    if (!this.connected || !this.desired) return;
    this.renewalTimer = setTimeout(() => {
      this.renewalTimer = null;
      // Only renew if still confirmed on the same session; a raced
      // release/switch/close would have cleared the timer already, but the
      // check keeps this defensive.
      if (this.phase !== "confirmed" || !this.desired) return;
      this.renewalPending = true;
      this.schedule();
    }, this.renewalIntervalMs);
  }

  private clearRenewalTimer(): void {
    if (this.renewalTimer !== null) clearTimeout(this.renewalTimer);
    this.renewalTimer = null;
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

function geometryFor(desired: { geometry: { cols: number; rows: number } | null } | null): number {
  return desired?.geometry?.cols ?? 80;
}

function geometryForRows(
  desired: { geometry: { cols: number; rows: number } | null } | null,
): number {
  return desired?.geometry?.rows ?? 24;
}

function makeClientInstanceID(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return `client-${Math.random().toString(36).slice(2)}`;
}
