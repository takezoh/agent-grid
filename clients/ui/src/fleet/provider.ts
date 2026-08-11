import type { CreateRepositorySessionPayload } from "../api/sessions";
import { desktopBaseUrl } from "../auth";
import { PROTOCOL_VERSION, protocolMismatchReason } from "../protocol-version";
import type {
  GitHubAction,
  GitHubActionResult,
  HostPreparation,
  HostRepositoryProjection,
} from "../store/repository_picker";
import {
  type FleetHealthSnapshotV1,
  type FleetHealthView,
  type FleetSnapshot,
  applyHealthDeliveryLoss,
  decodeFleetHealthSnapshotV1,
  deliveryLostWithoutSnapshot,
  healthViewFromSnapshot,
  initialHealthView,
} from "./session-protocol";

export interface FleetConfigOutcome {
  serverId: string;
  serverLabel: string;
  isDefaultExecution?: boolean;
  state: "connected" | "unavailable" | "incompatible_protocol";
  reason?: string;
  config?: Record<string, unknown>;
}

export type { GitHubAction, GitHubActionResult, HostRepositoryProjection };

export interface FleetProvider {
  initialServerId(): string | null;
  subscribe(listener: (snapshot: FleetSnapshot) => void): () => void;
  /** Host health subscription with consumer delivery-liveness overlay. */
  subscribeHealth(listener: (view: FleetHealthView) => void): () => void;
  /** Credential-free host-reported servers.json path when available. */
  configPath?(): Promise<string | null>;
  listConfigs(): Promise<FleetConfigOutcome[]>;
  create(serverId: string, payload: unknown): Promise<unknown>;
  /** Credential-free host projection of projects + GitHub catalog + preparations. */
  repositoryProjection(): Promise<HostRepositoryProjection>;
  /** Settings/account actions; host-only — never returns credentials. */
  githubAction(action: GitHubAction): Promise<GitHubActionResult>;
  /** Workspace-only navigation to the host-owned GitHub account settings. */
  openGitHubSettings?(): Promise<void>;
  /** Workspace-only navigation to Connections settings. */
  openConnections?(): Promise<void>;
  /** Start repository preparation on an execution server (host-mediated). */
  createRepositorySession(
    serverId: string,
    payload: CreateRepositorySessionPayload & { intent_id?: string },
  ): Promise<unknown>;
  /**
   * Host-mediated manual reconnect for one configured server. Resolves when the
   * host accepts the request (fresh worker started); does not wait for connected.
   * Renderer never handles credentials or host-specific transport details.
   */
  reconnect(serverId: string): Promise<void>;
  apiUrl(serverId: string, pathname: string): string;
  wsUrl(serverId: string, ticket: string): string;
  activeConnection(
    serverId: string,
    bearerToken: string,
  ): {
    ticketEndpoint: string;
    wsUrl(ticket: string): string;
    fetchFn?: typeof fetch;
    wsFactory?: (url: string) => WebSocket;
  };
  request(serverId: string, pathname: string, init?: RequestInit): Promise<Response>;
}

/** Bounded automatic reconnect window for host health delivery. */
export const HEALTH_DELIVERY_MAX_AUTO_RETRIES = 16;
export const HEALTH_DELIVERY_RETRY_BASE_MS = 250;
export const HEALTH_DELIVERY_RETRY_CAP_MS = 2_000;
/** Cooldown between capped retry windows so long outages remain recoverable. */
export const HEALTH_DELIVERY_COOLDOWN_MS = HEALTH_DELIVERY_RETRY_CAP_MS;

export type ConfiguredHealthServer = {
  serverId: string;
  serverLabel: string;
  enabled: boolean;
};

function createHealthSubscription(options: {
  open: (onMessage: (raw: unknown) => void, onClose: () => void) => () => void;
  now?: () => string;
  sleep?: (ms: number) => Promise<void>;
  /** When false, delivery close is terminal (Workspace IPC). Default true for Web. */
  reconnect?: boolean;
  random?: () => number;
  /**
   * Configured fleet authority (credential-free ids/labels/enabled only).
   * Seeds checking/lost rows before a host snapshot is accepted.
   */
  configuredServers?: () => Promise<ConfiguredHealthServer[]>;
}): (listener: (view: FleetHealthView) => void) => () => void {
  return (listener) => {
    let lastAccepted: FleetHealthSnapshotV1 | null = null;
    let seededServers: ConfiguredHealthServer[] = [];
    let deliveryLostWithoutAccept = false;
    let unsubscribed = false;
    let generation = 0;
    let closeCurrent: (() => void) | null = null;
    let reconnecting = false;
    let reconnectAttempt = 0;
    let cancelScheduledRetry: (() => void) | null = null;
    const now = options.now ?? (() => new Date().toISOString());
    const sleep = options.sleep ?? ((ms) => new Promise((resolve) => setTimeout(resolve, ms)));
    const reconnect = options.reconnect !== false;
    const random = options.random ?? Math.random;

    const publishLost = () => {
      if (lastAccepted) {
        listener(applyHealthDeliveryLoss(lastAccepted, now()));
        return;
      }
      deliveryLostWithoutAccept = true;
      if (seededServers.length > 0) {
        listener(deliveryLostWithoutSnapshot(seededServers, now()));
        return;
      }
      listener({
        hostInstanceId: null,
        revision: null,
        delivery: "lost",
        servers: [],
      });
    };

    const cancelRetry = () => {
      cancelScheduledRetry?.();
      cancelScheduledRetry = null;
      reconnecting = false;
    };

    const scheduleReconnect = (scheduledGeneration: number) => {
      if (!reconnect || unsubscribed || reconnecting) return;
      reconnecting = true;
      let delay: number;
      if (reconnectAttempt >= HEALTH_DELIVERY_MAX_AUTO_RETRIES) {
        // Cap the exponential window, cool down, then open a fresh attempt cycle.
        delay = HEALTH_DELIVERY_COOLDOWN_MS;
        reconnectAttempt = 0;
      } else {
        delay = Math.floor(
          random() *
            Math.min(
              HEALTH_DELIVERY_RETRY_CAP_MS,
              HEALTH_DELIVERY_RETRY_BASE_MS * 2 ** reconnectAttempt,
            ),
        );
        reconnectAttempt += 1;
      }
      let canceled = false;
      cancelScheduledRetry = () => {
        canceled = true;
      };
      void sleep(delay).then(() => {
        cancelScheduledRetry = null;
        reconnecting = false;
        if (canceled || unsubscribed) return;
        if (scheduledGeneration !== generation) return;
        openSocket();
      });
    };

    const openSocket = () => {
      if (unsubscribed) return;
      const auth = ++generation;
      closeCurrent?.();
      closeCurrent = options.open(
        (raw) => {
          // Late callbacks from a retired socket generation are ignored.
          if (unsubscribed || auth !== generation) return;
          try {
            const decoded = decodeFleetHealthSnapshotV1(raw);
            if (
              lastAccepted &&
              lastAccepted.hostInstanceId === decoded.hostInstanceId &&
              decoded.revision < lastAccepted.revision
            ) {
              return;
            }
            lastAccepted = decoded;
            deliveryLostWithoutAccept = false;
            reconnectAttempt = 0;
            listener(healthViewFromSnapshot(decoded));
          } catch {
            // Invalid wire payloads are ignored; delivery remains on last accepted view.
          }
        },
        () => {
          if (unsubscribed || auth !== generation) return;
          publishLost();
          scheduleReconnect(auth);
        },
      );
    };

    if (options.configuredServers) {
      void options.configuredServers().then((servers) => {
        if (unsubscribed) return;
        seededServers = servers;
        if (lastAccepted || servers.length === 0) return;
        if (deliveryLostWithoutAccept) {
          listener(deliveryLostWithoutSnapshot(servers, now()));
        } else {
          listener(initialHealthView(servers, now()));
        }
      });
    }

    openSocket();

    return () => {
      unsubscribed = true;
      cancelRetry();
      closeCurrent?.();
      closeCurrent = null;
      generation += 1;
    };
  };
}

type HostProjection = {
  servers?: Array<{
    serverId?: unknown;
    displayName?: unknown;
    enabled?: unknown;
    defaultExecution?: unknown;
  }>;
};

function explicitServerId(): string | null {
  const hosted =
    window.hostedModeInfo?.serverId ?? window.agentGridWorkspace?.hostedModeInfo?.serverId;
  if (hosted) return hosted;
  return new URLSearchParams(window.location.search).get("server");
}

/**
 * Statuses that WHATWG Fetch forbids from carrying a body. `new Response("",
 * {status: 204})` throws in Chromium and in Node; only happy-dom is permissive.
 */
const NULL_BODY_STATUSES: ReadonlySet<number> = new Set([101, 204, 205, 304]);

/**
 * Pick the body to hand the `Response` constructor for an IPC envelope.
 *
 * Electron main owns the invariant and sends `body: null` for these statuses,
 * so on a matched pair this only forwards what main already decided. The
 * status check exists for the mismatched pair: the Electron app copies the SPA
 * dist in at build time, so a dev run or a partial rebuild can put a new
 * renderer in front of an old main that still sends `""`. Dropping the body
 * here keeps that combination working instead of throwing a TypeError that
 * doFetch would report as a network error. This branch is deliberate
 * duplication of the main-side invariant — not a second owner of it — and it
 * stays after the producer change.
 */
function bodyForStatus(status: number, body: string | null): string | null {
  return NULL_BODY_STATUSES.has(status) ? null : body;
}

function qualifiedPath(serverId: string, pathname: string): string {
  const suffix = pathname.startsWith("/") ? pathname : `/${pathname}`;
  return `/host/servers/${encodeURIComponent(serverId)}${suffix}`;
}

function websocketURL(pathname: string, ticket: string): string {
  const url = new URL(pathname, window.location.origin);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("ticket", ticket);
  url.searchParams.set("protocolVersion", PROTOCOL_VERSION);
  return url.toString();
}

function webProvider(fetchImpl: typeof fetch): FleetProvider {
  return {
    initialServerId: explicitServerId,
    subscribe(listener) {
      const url = new URL("/host/fleet", window.location.origin);
      url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
      const socket = new WebSocket(url);
      socket.onmessage = (event) => {
        const snapshot = JSON.parse(String(event.data)) as FleetSnapshot;
        listener(snapshot);
      };
      return () => socket.close();
    },
    subscribeHealth: createHealthSubscription({
      open(onMessage, onClose) {
        const url = new URL("/host/fleet/health", window.location.origin);
        url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
        const socket = new WebSocket(url);
        socket.onmessage = (event) => {
          onMessage(JSON.parse(String(event.data)));
        };
        socket.onclose = () => onClose();
        socket.onerror = () => onClose();
        return () => socket.close();
      },
      async configuredServers() {
        try {
          const response = await fetchImpl("/host/config");
          if (!response.ok) return [];
          const projection = (await response.json()) as HostProjection;
          const servers = Array.isArray(projection.servers) ? projection.servers : [];
          return servers
            .filter((server) => typeof server.serverId === "string" && server.serverId)
            .map((server) => ({
              serverId: server.serverId as string,
              serverLabel:
                typeof server.displayName === "string" && server.displayName
                  ? server.displayName
                  : (server.serverId as string),
              enabled: server.enabled === true,
            }));
        } catch {
          return [];
        }
      },
    }),
    async configPath() {
      try {
        const response = await fetchImpl("/host/config");
        if (!response.ok) return null;
        const projection = (await response.json()) as {
          configPath?: unknown;
          config_path?: unknown;
        };
        if (typeof projection.configPath === "string") return projection.configPath;
        if (typeof projection.config_path === "string") return projection.config_path;
        return null;
      } catch {
        return null;
      }
    },
    async listConfigs() {
      const projectionResponse = await fetchImpl("/host/config");
      if (!projectionResponse.ok) throw new Error(`HTTP ${projectionResponse.status}`);
      const projection = (await projectionResponse.json()) as HostProjection;
      const servers = Array.isArray(projection.servers) ? projection.servers : [];
      return Promise.all(
        servers
          .filter((server) => server.enabled === true && typeof server.serverId === "string")
          .map(async (server): Promise<FleetConfigOutcome> => {
            const serverId = server.serverId as string;
            const serverLabel =
              typeof server.displayName === "string" ? server.displayName : serverId;
            try {
              const capabilitiesResponse = await fetchImpl(
                qualifiedPath(serverId, "/api/capabilities"),
              );
              if (!capabilitiesResponse.ok) {
                throw new Error(`HTTP ${capabilitiesResponse.status}`);
              }
              const capabilities = (await capabilitiesResponse.json()) as {
                protocolVersion?: unknown;
              };
              if (capabilities.protocolVersion !== PROTOCOL_VERSION) {
                return {
                  serverId,
                  serverLabel,
                  isDefaultExecution: server.defaultExecution === true,
                  state: "incompatible_protocol",
                  reason: protocolMismatchReason(capabilities.protocolVersion),
                };
              }
              const response = await fetchImpl(qualifiedPath(serverId, "/api/session-config"));
              if (!response.ok) {
                return {
                  serverId,
                  serverLabel,
                  isDefaultExecution: server.defaultExecution === true,
                  state: "unavailable",
                  reason: `HTTP ${response.status}`,
                };
              }
              return {
                serverId,
                serverLabel,
                isDefaultExecution: server.defaultExecution === true,
                state: "connected",
                config: (await response.json()) as Record<string, unknown>,
              };
            } catch (error) {
              return {
                serverId,
                serverLabel,
                isDefaultExecution: server.defaultExecution === true,
                state: "unavailable",
                reason: error instanceof Error ? error.message : String(error),
              };
            }
          }),
      );
    },
    async create(serverId, payload) {
      const response = await fetchImpl(
        `/host/fleet/servers/${encodeURIComponent(serverId)}/sessions`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      return response.json();
    },
    async repositoryProjection() {
      const response = await fetchImpl("/host/repository");
      // 404 is a transient host/route failure — not an empty catalog. Throwing
      // preserves TargetPicker retention of last complete projection, selection,
      // and in-flight preparation instead of wiping to unauthenticated empty.
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      return (await response.json()) as HostRepositoryProjection;
    },
    async githubAction(action) {
      const response = await fetchImpl("/host/github", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action }),
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      return (await response.json()) as GitHubActionResult;
    },
    async createRepositorySession(serverId, payload) {
      const response = await fetchImpl(
        `/host/fleet/servers/${encodeURIComponent(serverId)}/repository-sessions`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      return response.json();
    },
    async reconnect(serverId) {
      const response = await fetchImpl(
        `/host/fleet/servers/${encodeURIComponent(serverId)}/reconnect`,
        { method: "POST" },
      );
      if (!response.ok) {
        // Drain body without projecting it — host text may contain secrets.
        try {
          await response.text();
        } catch {
          // ignore drain failures
        }
        if (response.status === 404) {
          throw new Error("unknown or disabled server");
        }
        throw new Error(`reconnect failed (HTTP ${response.status})`);
      }
    },
    apiUrl: qualifiedPath,
    wsUrl(serverId, ticket) {
      return websocketURL(qualifiedPath(serverId, "/ws"), ticket);
    },
    activeConnection(serverId) {
      if (serverId === "local") {
        return {
          ticketEndpoint: "/api/ws-ticket",
          wsUrl: (ticket: string) => websocketURL("/ws", ticket),
        };
      }
      return {
        ticketEndpoint: qualifiedPath(serverId, "/api/ws-ticket"),
        wsUrl: (ticket) => websocketURL(qualifiedPath(serverId, "/ws"), ticket),
      };
    },
    request(serverId, pathname, init) {
      return fetchImpl(qualifiedPath(serverId, pathname), init);
    },
  };
}

function workspaceProvider(bridge: NonNullable<Window["agentGridWorkspace"]>): FleetProvider {
  const fleet = bridge.sessionFleet;
  if (!fleet) throw new Error("Workspace FleetProvider is unavailable");
  const trackedPreparations = new Map<string, { serverId: string; preparation: HostPreparation }>();
  return {
    initialServerId: explicitServerId,
    subscribe(listener) {
      if (!fleet.subscribe) throw new Error("Workspace fleet subscription is unavailable");
      return fleet.subscribe((snapshot) => listener(snapshot as FleetSnapshot));
    },
    subscribeHealth: createHealthSubscription({
      // Workspace IPC remains open for the renderer lifetime; delivery loss is
      // only reported when the host subscription cannot be established.
      reconnect: false,
      open(onMessage, onClose) {
        if (!fleet.subscribeHealth) {
          queueMicrotask(onClose);
          return () => undefined;
        }
        return fleet.subscribeHealth((snapshot) => onMessage(snapshot));
      },
    }),
    async configPath() {
      if (fleet.configPath) return fleet.configPath();
      return null;
    },
    async openConnections() {
      if (!bridge.openSettings) throw new Error("Workspace Settings bridge is unavailable");
      await bridge.openSettings("connections");
    },
    listConfigs: () => fleet.listConfigs() as Promise<FleetConfigOutcome[]>,
    create: (serverId, payload) => fleet.create(serverId, payload),
    async repositoryProjection() {
      if (fleet.repositoryProjection) {
        return fleet.repositoryProjection() as Promise<HostRepositoryProjection>;
      }
      // Compose credential-free status + projects when host only exposes github actions.
      const github = bridge.github;
      const status = github
        ? await github.action("status")
        : {
            outcome: "unauthenticated" as const,
            account: { state: "unauthenticated" },
            catalog: { outcome: "empty", generation: 0, repositories: [] },
          };
      const configs = (await fleet.listConfigs()) as FleetConfigOutcome[];
      const targets: HostRepositoryProjection["targets"] = [];
      for (const cfg of configs) {
        if (cfg.state !== "connected" || !cfg.config) continue;
        const projects = (cfg.config as { projects?: Array<{ path?: string }> }).projects;
        if (!Array.isArray(projects)) continue;
        for (const p of projects) {
          if (typeof p?.path === "string" && p.path) {
            targets.push({
              source: "project",
              project: { server_id: cfg.serverId, project: p.path, server_label: cfg.serverLabel },
            });
          }
        }
      }
      for (const r of status.catalog.repositories ?? []) {
        if (r.repository_id && r.owner && r.name) {
          targets.push({ source: "repository", repository: { ...r } });
        }
      }
      const ghAvailable =
        status.account.state === "authenticated" ||
        status.account.state === "authorizing" ||
        (status.catalog.repositories?.length ?? 0) > 0;
      const preparations = await Promise.all(
        [...trackedPreparations.values()].map(async (tracked) => {
          if (!fleet.request) return tracked.preparation;
          try {
            const result = await fleet.request(
              tracked.serverId,
              `/api/repository-preparations/${encodeURIComponent(tracked.preparation.preparation_id)}`,
              { method: "GET" },
            );
            if (result.status < 200 || result.status >= 300) return tracked.preparation;
            // A 2xx with no body (204) means the host had nothing to report, not
            // that the preparation became null. `JSON.parse(null)` does not
            // throw — it returns null — so without this branch a missing body
            // would quietly overwrite a live preparation with null and the
            // failure would read as a success.
            if (result.body === null) return tracked.preparation;
            const preparation = JSON.parse(result.body) as HostPreparation;
            tracked.preparation = preparation;
            return preparation;
          } catch {
            return tracked.preparation;
          }
        }),
      );
      return {
        sources: [
          { id: "projects", available: true },
          {
            id: "github",
            available: ghAvailable,
            status: String(status.catalog.outcome || status.account.state),
          },
        ],
        account: status.account,
        catalog: {
          outcome: status.catalog.outcome,
          generation: status.catalog.generation ?? 0,
          stale: status.catalog.stale,
          failure_code: status.catalog.failure_code,
          repositories: status.catalog.repositories ?? [],
        },
        targets,
        preparations,
      };
    },
    async githubAction(action) {
      if (!bridge.github) throw new Error("Workspace GitHub bridge is unavailable");
      return bridge.github.action(action);
    },
    async openGitHubSettings() {
      if (!bridge.openSettings) throw new Error("Workspace Settings bridge is unavailable");
      await bridge.openSettings("github");
    },
    async createRepositorySession(serverId, payload) {
      let result: unknown;
      if (fleet.createRepositorySession) {
        result = await fleet.createRepositorySession(serverId, payload);
      } else {
        result = await fleet.create(serverId, { kind: "repository", ...payload });
      }
      const preparation = result as Partial<HostPreparation>;
      if (typeof preparation.preparation_id === "string" && preparation.preparation_id) {
        trackedPreparations.set(preparation.preparation_id, {
          serverId,
          preparation: {
            preparation_id: preparation.preparation_id,
            intent_id: String(preparation.intent_id ?? ""),
            server_id: serverId,
            repository: preparation.repository ?? {
              repository_id: payload.repository_id,
              owner: payload.owner,
              name: payload.name,
            },
            stage: String(preparation.stage ?? "confirmed"),
            revision: Number(preparation.revision ?? 1),
            failure_code: preparation.failure_code,
            retry_origin: preparation.retry_origin,
            environment_disposition: preparation.environment_disposition,
            session_id: preparation.session_id,
          },
        });
      }
      return result;
    },
    async reconnect(serverId) {
      if (!fleet.reconnect) throw new Error("Workspace reconnect is unavailable");
      await fleet.reconnect(serverId);
    },
    apiUrl(_serverId, pathname) {
      const baseUrl = desktopBaseUrl();
      if (!baseUrl) throw new Error("Workspace server base URL is unavailable");
      return new URL(pathname, `${baseUrl}/`).toString();
    },
    wsUrl(_serverId, ticket) {
      const baseUrl = desktopBaseUrl();
      if (!baseUrl) throw new Error("Workspace server base URL is unavailable");
      const url = new URL("/ws", `${baseUrl}/`);
      url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
      url.searchParams.set("ticket", ticket);
      url.searchParams.set("protocolVersion", PROTOCOL_VERSION);
      return url.toString();
    },
    activeConnection(serverId) {
      const openChannel = fleet.openChannel;
      if (!openChannel) {
        throw new Error("Workspace active session channel is unavailable");
      }
      return {
        ticketEndpoint: "workspace://session-channel",
        wsUrl: () => "workspace://session-channel",
        fetchFn: async () => Response.json({ ticket: "ipc" }),
        wsFactory: () => new WorkspaceSocket(serverId, openChannel) as unknown as WebSocket,
      };
    },
    async request(serverId, pathname, init = {}) {
      if (!fleet.request) throw new Error("Workspace routed request is unavailable");
      const headers = Object.fromEntries(new Headers(init.headers).entries());
      const result = await fleet.request(serverId, pathname, {
        method: init.method,
        headers,
        body: typeof init.body === "string" ? init.body : undefined,
      });
      return new Response(bodyForStatus(result.status, result.body), {
        status: result.status,
        headers: result.headers,
      });
    },
  };
}

class WorkspaceSocket {
  readyState: number = WebSocket.CONNECTING;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  private channel: { send(data: string): void; close(): void } | null = null;
  private pending: string[] = [];
  private closing = false;

  constructor(
    serverId: string,
    open: NonNullable<
      NonNullable<NonNullable<Window["agentGridWorkspace"]>["sessionFleet"]>["openChannel"]
    >,
  ) {
    void open(serverId, (event) => {
      if (event.kind === "open") {
        this.readyState = WebSocket.OPEN;
        for (const data of this.pending) this.channel?.send(data);
        this.pending = [];
        this.onopen?.(new Event("open"));
      } else if (event.kind === "message") {
        this.onmessage?.(new MessageEvent("message", { data: String(event.data) }));
      } else if (event.kind === "error") {
        this.onerror?.(new Event("error"));
      } else if (event.kind === "close") {
        this.readyState = WebSocket.CLOSED;
        this.onclose?.(new CloseEvent("close"));
      }
    })
      .then((channel) => {
        this.channel = channel;
        if (this.closing) channel.close();
      })
      .catch(() => {
        this.readyState = WebSocket.CLOSED;
        this.onerror?.(new Event("error"));
        this.onclose?.(new CloseEvent("close"));
      });
  }

  send(data: string): void {
    if (this.readyState === WebSocket.OPEN) this.channel?.send(data);
    else this.pending.push(data);
  }

  close(): void {
    this.closing = true;
    this.readyState = WebSocket.CLOSING;
    this.channel?.close();
  }
}

/**
 * The only host-delivery selection point. Consumers use one server-qualified
 * contract; browser and Electron never imply different fleet cardinality.
 */
export function makeFleetProvider(fetchImpl?: typeof fetch): FleetProvider {
  const bridge = window.agentGridWorkspace;
  return bridge?.sessionFleet
    ? workspaceProvider(bridge)
    : webProvider(fetchImpl ?? fetch.bind(globalThis));
}
