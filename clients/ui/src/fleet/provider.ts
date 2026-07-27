import { desktopBaseUrl } from "../auth";

export interface FleetConfigOutcome {
  serverId: string;
  serverLabel: string;
  state: "connected" | "unavailable";
  reason?: string;
  config?: Record<string, unknown>;
}

export interface FleetProvider {
  currentServerId(): string | null;
  listConfigs(): Promise<FleetConfigOutcome[]>;
  create(serverId: string, payload: unknown): Promise<unknown>;
  apiUrl(serverId: string, pathname: string): string;
  wsUrl(serverId: string, ticket: string): string;
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
  return url.toString();
}

function webProvider(fetchImpl: typeof fetch): FleetProvider {
  return {
    currentServerId: explicitServerId,
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
      const response = await fetchImpl(qualifiedPath(serverId, "/api/sessions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      return response.json();
    },
    apiUrl: qualifiedPath,
    wsUrl(serverId, ticket) {
      return websocketURL(qualifiedPath(serverId, "/ws"), ticket);
    },
  };
}

function workspaceProvider(bridge: NonNullable<Window["agentGridWorkspace"]>): FleetProvider {
  const fleet = bridge.sessionFleet;
  if (!fleet) throw new Error("Workspace FleetProvider is unavailable");
  return {
    currentServerId: explicitServerId,
    listConfigs: () => fleet.listConfigs() as Promise<FleetConfigOutcome[]>,
    create: (serverId, payload) => fleet.create(serverId, payload),
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
      return url.toString();
    },
  };
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
