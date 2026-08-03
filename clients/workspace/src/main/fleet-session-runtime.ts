import { randomUUID } from "node:crypto";
import type { ServerConfig } from "./desktop-config.js";
import {
  PROTOCOL_VERSION,
  PROTOCOL_VERSION_HEADER,
  protocolMismatchReason,
} from "./protocol-version.js";
import {
  classifyHealthFailure,
  type FleetHealthSnapshotV1,
  type HealthReason,
  type PriorHealthKind,
  type QualifiedHealthObservation,
} from "./server-health.js";

export interface FleetSessionRef {
  serverId: string;
  sessionId: string;
}

export interface FleetMember {
  serverId: string;
  serverLabel: string;
  generation: number;
  state: "connecting" | "connected" | "reconnecting" | "unavailable";
  stale?: boolean;
  reason?: string;
  sessions: Array<Record<string, unknown>>;
}

export interface FleetSnapshot {
  revision: number;
  members: FleetMember[];
  sessions: Array<{
    ref: FleetSessionRef;
    serverLabel: string;
    session: Record<string, unknown>;
    stale: boolean;
  }>;
}

type SessionListener = (snapshot: FleetSnapshot) => void;
type HealthListener = (snapshot: FleetHealthSnapshotV1) => void;
type FetchLike = typeof fetch;
type WebSocketLike = Pick<WebSocket, "close" | "addEventListener" | "readyState"> & {
  removeEventListener?: WebSocket["removeEventListener"];
};

export const HEALTH_PHASE_TIMEOUT_MS = 3_000;
export const HEALTH_CONNECTED_FRESHNESS_MS = 5_000;
export const HEALTH_RETRY_BASE_MS = 250;
export const HEALTH_RETRY_CAP_MS = 2_000;
export const HEALTH_MAX_AUTO_RETRIES = 16;

export interface FleetRuntimeClock {
  now(): number;
  sleep(ms: number): Promise<void>;
}

export interface FleetSessionRuntimeOptions {
  fetchFn?: FetchLike;
  webSocketFactory?: (url: string) => WebSocketLike;
  clock?: FleetRuntimeClock;
  hostInstanceId?: string;
  random?: () => number;
}

interface HealthRecord {
  observation: QualifiedHealthObservation;
  generation: number;
  terminalIncompatible: boolean;
}

interface LiveHandles {
  timers: number;
  fetches: number;
  sockets: number;
}

const defaultClock: FleetRuntimeClock = {
  now: () => Date.now(),
  sleep: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
};

export class FleetSessionRuntime {
  private snapshotValue: FleetSnapshot;
  private readonly sessionListeners = new Set<SessionListener>();
  private readonly healthListeners = new Set<HealthListener>();
  private readonly sessionGenerations = new Map<string, number>();
  private readonly healthByServer = new Map<string, HealthRecord>();
  private readonly sockets = new Set<WebSocketLike>();
  private readonly socketsByServer = new Map<string, Set<WebSocketLike>>();
  private readonly abortControllers = new Set<AbortController>();
  private readonly abortsByServer = new Map<string, Set<AbortController>>();
  private stopped = false;
  private healthRevision = 0;
  private readonly hostInstanceId: string;
  private readonly fetchFn: FetchLike;
  private readonly webSocketFactory: (url: string) => WebSocketLike;
  private readonly clock: FleetRuntimeClock;
  private readonly random: () => number;
  private live: LiveHandles = { timers: 0, fetches: 0, sockets: 0 };
  private workersStarted = 0;
  /**
   * True while baseline workers from start() are owned. start() is idempotent
   * while this is set; stop() clears it so a later start() may re-arm.
   */
  private baselineRunning = false;
  /** Per-server worker epoch: reconnect bumps it so the prior follow loop exits. */
  private readonly workerEpochs = new Map<string, number>();
  /** Resolvers for in-flight interruptible sleeps (retry backoff / phase polls). */
  private readonly sleepWaiters = new Set<() => void>();
  private readonly sleepWaitersByServer = new Map<string, Set<() => void>>();

  constructor(
    private servers: readonly ServerConfig[],
    fetchFnOrOptions: FetchLike | FleetSessionRuntimeOptions = {},
    webSocketFactory?: (url: string) => WebSocketLike,
  ) {
    const options: FleetSessionRuntimeOptions =
      typeof fetchFnOrOptions === "function"
        ? { fetchFn: fetchFnOrOptions, webSocketFactory }
        : fetchFnOrOptions;
    this.fetchFn = options.fetchFn ?? fetch;
    this.webSocketFactory =
      options.webSocketFactory ?? webSocketFactory ?? ((url) => new WebSocket(url));
    this.clock = options.clock ?? defaultClock;
    this.random = options.random ?? Math.random;
    this.hostInstanceId = options.hostInstanceId ?? randomUUID();
    const enabled = servers.filter((server) => server.enabled);
    this.snapshotValue = project(
      0,
      enabled.map((server) => ({
        serverId: server.id,
        serverLabel: server.display_name,
        generation: 0,
        state: "connecting" as const,
        sessions: [],
      })),
    );
    for (const server of servers) {
      this.healthByServer.set(server.id, {
        generation: 0,
        terminalIncompatible: false,
        observation: server.enabled
          ? { kind: "checking", observedAt: this.isoNow() }
          : { kind: "disabled" },
      });
    }
  }

  start(): void {
    this.stopped = false;
    // Idempotent: a second start while baseline is running must not duplicate workers.
    if (this.baselineRunning) return;
    this.baselineRunning = true;
    for (const server of this.servers) {
      if (!server.enabled) continue;
      const epoch = this.workerEpochs.get(server.id) ?? 0;
      this.workerEpochs.set(server.id, epoch);
      this.workersStarted += 1;
      void this.follow(server, epoch);
    }
  }

  stop(): void {
    this.stopped = true;
    this.baselineRunning = false;
    for (const wake of [...this.sleepWaiters]) wake();
    this.sleepWaiters.clear();
    this.sleepWaitersByServer.clear();
    for (const controller of [...this.abortControllers]) controller.abort();
    this.abortControllers.clear();
    this.abortsByServer.clear();
    for (const socket of [...this.sockets]) socket.close();
    this.sockets.clear();
    this.socketsByServer.clear();
    this.sessionListeners.clear();
    this.healthListeners.clear();
    // After synchronously aborting/closing every tracked handle, live counts are
    // zero by contract. In-flight finally blocks use Math.max(0, n-1) so a late
    // retire cannot go negative. Tests must still establish pending work and
    // observe real abort/close — not only this zeroing.
    this.live = { timers: 0, fetches: 0, sockets: 0 };
  }

  /**
   * Manual per-server reconnect: retire the selected server's current attempt,
   * fence late callbacks by generation, reset capped retry / terminal protocol
   * state, publish checking evidence, and start exactly one fresh worker.
   * Does not mutate servers.json or restart other servers.
   *
   * Safe before start(): ensures the normal baseline once, then fences/replaces
   * only the selected server so a later start() cannot duplicate workers.
   */
  async reconnect(serverId: string): Promise<void> {
    if (this.stopped) {
      throw new Error("fleet runtime is not ready");
    }
    const server = this.servers.find((candidate) => candidate.id === serverId);
    if (!server || !server.enabled) {
      throw new Error(`unknown or disabled server '${serverId}'`);
    }

    // Ensure baseline ownership first so reconnect-before-start leaves one worker
    // per enabled server and start() remains a no-op afterward.
    if (!this.baselineRunning) {
      this.start();
    }

    const record = this.healthByServer.get(serverId);
    if (record) {
      record.terminalIncompatible = false;
    }

    // Retire in-flight I/O for this server only, then fence by generation.
    this.retireServerAttempt(serverId);
    const generation = (this.sessionGenerations.get(serverId) ?? 0) + 1;
    this.sessionGenerations.set(serverId, generation);
    const epoch = (this.workerEpochs.get(serverId) ?? 0) + 1;
    this.workerEpochs.set(serverId, epoch);

    this.publishHealth(serverId, generation, {
      kind: "checking",
      observedAt: this.isoNow(),
    });
    this.replace(server, generation, {
      state: "reconnecting",
      reason: "manual reconnect",
      sessions: [],
    });

    this.workersStarted += 1;
    void this.follow(server, epoch);
  }

  private retireServerAttempt(serverId: string): void {
    const aborts = this.abortsByServer.get(serverId);
    if (aborts) {
      for (const controller of [...aborts]) controller.abort();
      aborts.clear();
    }
    const sockets = this.socketsByServer.get(serverId);
    if (sockets) {
      for (const socket of [...sockets]) socket.close();
      sockets.clear();
    }
    const sleeps = this.sleepWaitersByServer.get(serverId);
    if (sleeps) {
      for (const wake of [...sleeps]) wake();
      sleeps.clear();
    }
  }

  liveHandleCounts(): LiveHandles {
    return { ...this.live };
  }

  enabledWorkerCount(): number {
    return this.servers.filter((server) => server.enabled).length;
  }

  startedWorkerCount(): number {
    return this.workersStarted;
  }

  subscribe(listener: SessionListener): () => void {
    this.sessionListeners.add(listener);
    listener(this.snapshotValue);
    return () => this.sessionListeners.delete(listener);
  }

  subscribeHealth(listener: HealthListener): () => void {
    this.healthListeners.add(listener);
    listener(this.healthSnapshot());
    return () => this.healthListeners.delete(listener);
  }

  healthSnapshot(): FleetHealthSnapshotV1 {
    return {
      schemaVersion: 1,
      hostInstanceId: this.hostInstanceId,
      revision: this.healthRevision,
      servers: this.servers.map((server) => {
        const record = this.healthByServer.get(server.id);
        return {
          serverId: server.id,
          serverLabel: server.display_name,
          enabled: server.enabled,
          observation: record?.observation ?? { kind: "disabled" },
        };
      }),
    };
  }

  waitFor(ref: FleetSessionRef, timeoutMs = 10_000): Promise<boolean> {
    return new Promise((resolve) => {
      let settled = false;
      let unsubscribe = () => {};
      const finish = (value: boolean) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        unsubscribe();
        resolve(value);
      };
      const timer = setTimeout(() => finish(false), timeoutMs);
      unsubscribe = this.subscribe((snapshot) => {
        if (
          snapshot.sessions.some(
            (session) =>
              session.ref.serverId === ref.serverId && session.ref.sessionId === ref.sessionId,
          )
        ) {
          finish(true);
        }
      });
      if (settled) unsubscribe();
    });
  }

  replace(
    server: ServerConfig,
    generation: number,
    patch: Omit<FleetMember, "serverId" | "serverLabel" | "generation">,
  ): void {
    const index = this.snapshotValue.members.findIndex((member) => member.serverId === server.id);
    if (index < 0) return;
    const current = this.snapshotValue.members[index];
    if (generation < current.generation) return;
    const failed = patch.state !== "connected";
    const members = this.snapshotValue.members.slice();
    members[index] = {
      serverId: server.id,
      serverLabel: server.display_name,
      generation,
      ...patch,
      stale: failed && current.sessions.length > 0,
      sessions: failed ? current.sessions : patch.sessions,
    };
    this.snapshotValue = project(this.snapshotValue.revision + 1, members);
    for (const listener of this.sessionListeners) listener(this.snapshotValue);
  }

  /** Test seam: publish a generation-fenced health observation. */
  publishHealth(
    serverId: string,
    generation: number,
    observation: QualifiedHealthObservation,
    options: { terminalIncompatible?: boolean } = {},
  ): boolean {
    const record = this.healthByServer.get(serverId);
    if (!record) return false;
    if (generation < record.generation) return false;
    if (this.stopped) return false;
    record.generation = generation;
    record.observation = observation;
    if (options.terminalIncompatible) record.terminalIncompatible = true;
    this.healthRevision += 1;
    const snapshot = this.healthSnapshot();
    for (const listener of this.healthListeners) listener(snapshot);
    return true;
  }

  private isoNow(): string {
    return new Date(this.clock.now()).toISOString();
  }

  private async follow(server: ServerConfig, epoch: number): Promise<void> {
    let attempt = 0;
    while (!this.stopped && (this.workerEpochs.get(server.id) ?? 0) === epoch) {
      const generation = (this.sessionGenerations.get(server.id) ?? 0) + 1;
      this.sessionGenerations.set(server.id, generation);
      const healthRecord = this.healthByServer.get(server.id);
      if (healthRecord?.terminalIncompatible) return;
      this.publishHealth(server.id, generation, {
        kind: "checking",
        observedAt: this.isoNow(),
      });
      try {
        await this.followGeneration(server, generation);
        attempt = 0;
      } catch (error) {
        if (this.stopped) return;
        if ((this.workerEpochs.get(server.id) ?? 0) !== epoch) return;
        const record = this.healthByServer.get(server.id);
        if (record?.terminalIncompatible) return;
        const classified = classifyHealthFailure(error);
        if (classified.reason === "protocol_mismatch") {
          this.publishHealth(
            server.id,
            generation,
            {
              kind: "incompatible_protocol",
              observedAt: this.isoNow(),
              reason: "protocol_mismatch",
              ...(classified.detail ? { detail: classified.detail } : {}),
            },
            { terminalIncompatible: true },
          );
          this.replace(server, generation, {
            state: "unavailable",
            reason: classified.detail ?? "Incompatible protocol",
            sessions: [],
          });
          return;
        }
        const prior = nestablePrior(record?.observation);
        const isRefresh =
          record?.observation.kind === "connected" ||
          record?.observation.kind === "health_unavailable";
        if (isRefresh) {
          this.publishHealth(server.id, generation, {
            kind: "health_unavailable",
            observedAt: this.isoNow(),
            reason: "refresh_failed",
            ...(prior ? { prior } : {}),
          });
        } else {
          this.publishHealth(server.id, generation, {
            kind: "unavailable",
            observedAt: this.isoNow(),
            reason: classified.reason,
            ...(classified.detail ? { detail: classified.detail } : {}),
          });
        }
        this.replace(server, generation, {
          state: "reconnecting",
          reason: classified.detail ?? classified.reason,
          sessions: [],
        });
        if ((this.workerEpochs.get(server.id) ?? 0) !== epoch) return;
        if (attempt >= HEALTH_MAX_AUTO_RETRIES) {
          await this.interruptibleSleep(HEALTH_RETRY_CAP_MS, server.id).promise;
          attempt = 0;
        } else {
          const delay = Math.max(
            1,
            Math.min(
              HEALTH_RETRY_CAP_MS,
              Math.floor(this.random() * HEALTH_RETRY_BASE_MS * 2 ** attempt),
            ),
          );
          attempt += 1;
          await this.interruptibleSleep(delay, server.id).promise;
        }
      }
    }
  }

  private async followGeneration(server: ServerConfig, generation: number): Promise<void> {
    const protocol = await this.probeCapabilities(server, generation);
    if (protocol !== PROTOCOL_VERSION) {
      // Closed protocol-mismatch detail — never interpolates untrusted raw.
      const detail = protocolMismatchReason(protocol);
      throw Object.assign(new Error(detail), { healthReason: "protocol_mismatch" as HealthReason });
    }

    const ticket = await this.requestTicket(server, generation);
    const url = new URL("/ws", `${server.url.replace(/\/+$/, "")}/`);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.searchParams.set("ticket", ticket);
    url.searchParams.set("protocolVersion", PROTOCOL_VERSION);
    const socket = this.webSocketFactory(url.toString());
    this.sockets.add(socket);
    let serverSockets = this.socketsByServer.get(server.id);
    if (!serverSockets) {
      serverSockets = new Set();
      this.socketsByServer.set(server.id, serverSockets);
    }
    serverSockets.add(socket);
    this.live.sockets += 1;

    let sawHello = false;
    let freshnessEpoch = 0;
    const freshnessControl: { cancel: (() => void) | undefined } = { cancel: undefined };
    let resolveClose: (() => void) | null = null;
    let rejectClose: ((error: Error) => void) | null = null;
    const closed = new Promise<void>((resolve, reject) => {
      resolveClose = resolve;
      rejectClose = reject;
    });

    const armFreshness = () => {
      freshnessControl.cancel?.();
      const epoch = ++freshnessEpoch;
      this.live.timers += 1;
      const sleep = this.interruptibleSleep(HEALTH_CONNECTED_FRESHNESS_MS, server.id);
      freshnessControl.cancel = () => {
        sleep.cancel();
      };
      void sleep.promise.then(() => {
        this.live.timers = Math.max(0, this.live.timers - 1);
        if (epoch !== freshnessEpoch) return;
        freshnessControl.cancel = undefined;
        if (this.stopped || this.sessionGenerations.get(server.id) !== generation) return;
        socket.close();
      });
    };

    const onMessage = (event: Event) => {
      if (this.stopped || this.sessionGenerations.get(server.id) !== generation) return;
      const data = (event as MessageEvent).data;
      if (typeof data !== "string") return;
      let frame: { k?: unknown; sessions?: unknown };
      try {
        frame = JSON.parse(data) as typeof frame;
      } catch {
        return;
      }
      if ((frame.k === "h" || frame.k === "v") && Array.isArray(frame.sessions)) {
        sawHello = true;
        const sessions = frame.sessions.filter(
          (entry): entry is Record<string, unknown> =>
            Boolean(entry) && typeof entry === "object",
        );
        this.replace(server, generation, { state: "connected", sessions });
        this.publishHealth(server.id, generation, {
          kind: "connected",
          observedAt: this.isoNow(),
        });
        armFreshness();
      }
    };

    const onClose = () => {
      resolveClose?.();
    };
    const onError = () => {
      rejectClose?.(new Error("websocket failed"));
    };

    socket.addEventListener("message", onMessage);
    socket.addEventListener("close", onClose);
    socket.addEventListener("error", onError);

    try {
      await this.withPhaseTimeout(
        (async () => {
          const deadline = this.clock.now() + HEALTH_PHASE_TIMEOUT_MS;
          while (!sawHello) {
            if (this.stopped || this.sessionGenerations.get(server.id) !== generation) {
              throw new Error("generation fenced");
            }
            if (this.clock.now() >= deadline) throw new Error("hello timed out");
            await Promise.race([
              closed.then(() => {
                if (!sawHello) throw new Error("websocket closed");
              }),
              this.interruptibleSleep(25, server.id).promise,
            ]);
          }
        })(),
        HEALTH_PHASE_TIMEOUT_MS,
        "hello timed out",
        () => {
          socket.close();
        },
        server.id,
      );

      await closed;
      if (!this.stopped && this.sessionGenerations.get(server.id) === generation) {
        throw new Error("websocket closed");
      }
    } finally {
      freshnessEpoch += 1;
      freshnessControl.cancel?.();
      freshnessControl.cancel = undefined;
      socket.close();
      this.sockets.delete(socket);
      this.socketsByServer.get(server.id)?.delete(socket);
      this.live.sockets = Math.max(0, this.live.sockets - 1);
    }
  }

  private async probeCapabilities(server: ServerConfig, generation: number): Promise<unknown> {
    const response = await this.timedFetch(
      new URL("/api/capabilities", `${server.url.replace(/\/+$/, "")}/`),
      {
        method: "GET",
        headers: fleetHeaders(server, false),
        redirect: "manual",
      },
      generation,
      server.id,
    );
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const body = (await response.json()) as { protocolVersion?: unknown };
    return body.protocolVersion;
  }

  private async requestTicket(server: ServerConfig, generation: number): Promise<string> {
    const response = await this.timedFetch(
      new URL("/api/ws-ticket", `${server.url.replace(/\/+$/, "")}/`),
      {
        method: "POST",
        headers: fleetHeaders(server, true),
        redirect: "manual",
      },
      generation,
      server.id,
    );
    if (!response.ok) throw new Error(`ws-ticket HTTP ${response.status}`);
    const body = (await response.json()) as { ticket?: unknown };
    if (typeof body.ticket !== "string" || !body.ticket) throw new Error("empty websocket ticket");
    return body.ticket;
  }

  private async timedFetch(
    input: URL,
    init: RequestInit,
    generation: number,
    serverId: string,
  ): Promise<Response> {
    if (this.stopped || this.sessionGenerations.get(serverId) !== generation) {
      throw new Error("generation fenced");
    }
    const controller = new AbortController();
    this.abortControllers.add(controller);
    let serverAborts = this.abortsByServer.get(serverId);
    if (!serverAborts) {
      serverAborts = new Set();
      this.abortsByServer.set(serverId, serverAborts);
    }
    serverAborts.add(controller);
    this.live.fetches += 1;
    let retired = false;
    const retire = () => {
      if (retired) return;
      retired = true;
      this.abortControllers.delete(controller);
      this.abortsByServer.get(serverId)?.delete(controller);
      this.live.fetches = Math.max(0, this.live.fetches - 1);
    };
    try {
      const response = await this.withPhaseTimeout(
        this.fetchFn(input, { ...init, signal: controller.signal }),
        HEALTH_PHASE_TIMEOUT_MS,
        "Connection timed out",
        () => {
          // Timeout must abort the real request exactly once so retries do not
          // accumulate orphaned fetch work.
          controller.abort();
          retire();
        },
        serverId,
      );
      if (this.stopped || this.sessionGenerations.get(serverId) !== generation) {
        throw new Error("generation fenced");
      }
      return response;
    } finally {
      retire();
    }
  }

  private async withPhaseTimeout<T>(
    work: Promise<T>,
    timeoutMs: number,
    timeoutMessage: string,
    onTimeout?: () => void,
    serverId?: string,
  ): Promise<T> {
    this.live.timers += 1;
    let settled = false;
    try {
      return await new Promise<T>((resolve, reject) => {
        const timeoutSleep = this.interruptibleSleep(timeoutMs, serverId);
        void timeoutSleep.promise.then(() => {
          if (settled) return;
          settled = true;
          onTimeout?.();
          reject(new Error(timeoutMessage));
        });
        void work.then(
          (value) => {
            if (settled) return;
            settled = true;
            timeoutSleep.cancel();
            resolve(value);
          },
          (error) => {
            if (settled) return;
            settled = true;
            timeoutSleep.cancel();
            reject(error);
          },
        );
      });
    } finally {
      this.live.timers = Math.max(0, this.live.timers - 1);
    }
  }

  /**
   * Sleep that resolves immediately when canceled or when the runtime is stopped.
   * cancel() is idempotent and does not invoke timeout side effects by itself.
   * When serverId is set, reconnect for that server can wake this sleep alone.
   */
  private interruptibleSleep(
    ms: number,
    serverId?: string,
  ): { promise: Promise<void>; cancel: () => void } {
    if (this.stopped) {
      return { promise: Promise.resolve(), cancel: () => undefined };
    }
    let done = false;
    let resolve!: () => void;
    const promise = new Promise<void>((r) => {
      resolve = r;
    });
    const finish = () => {
      if (done) return;
      done = true;
      this.sleepWaiters.delete(finish);
      if (serverId) this.sleepWaitersByServer.get(serverId)?.delete(finish);
      resolve();
    };
    this.sleepWaiters.add(finish);
    if (serverId) {
      let serverSleeps = this.sleepWaitersByServer.get(serverId);
      if (!serverSleeps) {
        serverSleeps = new Set();
        this.sleepWaitersByServer.set(serverId, serverSleeps);
      }
      serverSleeps.add(finish);
    }
    void this.clock.sleep(ms).then(finish, finish);
    return { promise, cancel: finish };
  }
}

function nestablePrior(
  observation: QualifiedHealthObservation | undefined,
): { kind: PriorHealthKind; observedAt: string; reason?: HealthReason; detail?: string } | undefined {
  if (!observation) return undefined;
  if (observation.kind === "connected") {
    return { kind: "connected", observedAt: observation.observedAt };
  }
  if (observation.kind === "unavailable" || observation.kind === "incompatible_protocol") {
    return {
      kind: observation.kind,
      observedAt: observation.observedAt,
      ...(observation.reason ? { reason: observation.reason } : {}),
      ...(observation.detail ? { detail: observation.detail } : {}),
    };
  }
  if (observation.kind === "health_unavailable") return observation.prior;
  return undefined;
}

function fleetHeaders(server: ServerConfig, includeProtocol: boolean): Headers {
  const headers = new Headers();
  if (includeProtocol) headers.set(PROTOCOL_VERSION_HEADER, PROTOCOL_VERSION);
  if (server.auth.mode === "bearer") {
    headers.set("Authorization", `Bearer ${server.auth.token}`);
  }
  return headers;
}

function project(revision: number, members: FleetMember[]): FleetSnapshot {
  const labelCounts = new Map<string, number>();
  for (const member of members) {
    labelCounts.set(member.serverLabel, (labelCounts.get(member.serverLabel) ?? 0) + 1);
  }
  return {
    revision,
    members,
    sessions: members.flatMap((member) =>
      member.sessions.flatMap((session) =>
        typeof session.id === "string" && session.id
          ? [
              {
                ref: { serverId: member.serverId, sessionId: session.id },
                serverLabel:
                  labelCounts.get(member.serverLabel) === 1
                    ? member.serverLabel
                    : `${member.serverLabel} · ${member.serverId}`,
                session,
                stale: member.stale === true,
              },
            ]
          : [],
      ),
    ),
  };
}
