import { desktopBaseUrl } from "../auth";
import { PROTOCOL_VERSION, protocolMismatchReason } from "../protocol-version";
import type {
  GitHubAction,
  GitHubActionResult,
  HostRepositoryProjection,
} from "../store/repository_picker";
import type { FleetSnapshot } from "./session-protocol";

export interface FleetConfigOutcome {
  serverId: string;
  serverLabel: string;
  state: "connected" | "unavailable" | "incompatible_protocol";
  reason?: string;
  config?: Record<string, unknown>;
}

export type { GitHubAction, GitHubActionResult, HostRepositoryProjection };

export interface FleetProvider {
  initialServerId(): string | null;
  subscribe(listener: (snapshot: FleetSnapshot) => void): () => void;
  listConfigs(): Promise<FleetConfigOutcome[]>;
  create(serverId: string, payload: unknown): Promise<unknown>;
  /** Credential-free host projection of projects + GitHub catalog + preparations. */
  repositoryProjection(): Promise<HostRepositoryProjection>;
  /** Settings/account actions; host-only — never returns credentials. */
  githubAction(action: GitHubAction): Promise<GitHubActionResult>;
  /** Start repository preparation on an execution server (host-mediated). */
  createRepositorySession(
    serverId: string,
    payload: {
      repository_id: string;
      owner: string;
      name: string;
      intent_id?: string;
    },
  ): Promise<unknown>;
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

type HostProjection = {
  servers?: Array<{ serverId?: unknown; displayName?: unknown; enabled?: unknown }>;
};

function explicitServerId(): string | null {
  const hosted =
    window.hostedModeInfo?.serverId ?? window.agentGridWorkspace?.hostedModeInfo?.serverId;
  if (hosted) return hosted;
  return new URLSearchParams(window.location.search).get("server");
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

function emptyRepositoryProjection(): HostRepositoryProjection {
  return {
    sources: [
      { id: "projects", available: true },
      { id: "github", available: false, status: "unauthenticated" },
    ],
    account: { state: "unauthenticated" },
    catalog: { outcome: "empty", generation: 0, repositories: [] },
    targets: [],
    preparations: [],
  };
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
                  state: "incompatible_protocol",
                  reason: protocolMismatchReason(capabilities.protocolVersion),
                };
              }
              const response = await fetchImpl(qualifiedPath(serverId, "/api/session-config"));
              if (!response.ok) {
                return {
                  serverId,
                  serverLabel,
                  state: "unavailable",
                  reason: `HTTP ${response.status}`,
                };
              }
              return {
                serverId,
                serverLabel,
                state: "connected",
                config: (await response.json()) as Record<string, unknown>,
              };
            } catch (error) {
              return {
                serverId,
                serverLabel,
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
      if (response.status === 404) return emptyRepositoryProjection();
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
  return {
    initialServerId: explicitServerId,
    subscribe(listener) {
      if (!fleet.subscribe) throw new Error("Workspace fleet subscription is unavailable");
      return fleet.subscribe((snapshot) => listener(snapshot as FleetSnapshot));
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
        preparations: [],
      };
    },
    async githubAction(action) {
      if (!bridge.github) throw new Error("Workspace GitHub bridge is unavailable");
      return bridge.github.action(action);
    },
    async createRepositorySession(serverId, payload) {
      if (fleet.createRepositorySession) {
        return fleet.createRepositorySession(serverId, payload);
      }
      return fleet.create(serverId, { kind: "repository", ...payload });
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
      return new Response(result.body, {
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
