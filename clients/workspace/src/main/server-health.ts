import type { ServerConfig } from "./desktop-config.js";
import {
  PROTOCOL_VERSION,
  PROTOCOL_VERSION_HEADER,
  protocolMismatchReason,
} from "./protocol-version.js";

export type ServerHealthState =
  | "connected"
  | "unavailable"
  | "incompatible_protocol"
  | "disabled";

export interface ServerHealth {
  serverId: string;
  displayName: string;
  state: ServerHealthState;
  reason?: string;
}

export interface FleetHealth {
  checkedAt: string;
  servers: ServerHealth[];
}

export interface ServerHealthDependencies {
  fetchFn?: typeof fetch;
}

/** Runtime-only fleet observation. It never enters DesktopConfig persistence. */
export async function probeServerFleet(
  servers: readonly ServerConfig[],
  deps: ServerHealthDependencies = {},
): Promise<FleetHealth> {
  const fetchFn = deps.fetchFn ?? fetch;
  const observations = await Promise.all(
    servers.map(async (server): Promise<ServerHealth> => {
      if (!server.enabled) {
        return {
          serverId: server.id,
          displayName: server.display_name,
          state: "disabled",
        };
      }
      try {
        const headers = server.auth.mode === "bearer"
          ? { Authorization: `Bearer ${server.auth.token}` }
          : undefined;
        const capabilitiesResponse = await fetchFn(
          new URL("/api/capabilities", `${server.url}/`),
          {
            headers,
            signal: AbortSignal.timeout(3000),
          },
        );
        if (!capabilitiesResponse.ok) throw new Error(`HTTP ${capabilitiesResponse.status}`);
        const capabilities = (await capabilitiesResponse.json()) as {
          protocolVersion?: unknown;
        };
        if (capabilities.protocolVersion !== PROTOCOL_VERSION) {
          return {
            serverId: server.id,
            displayName: server.display_name,
            state: "incompatible_protocol",
            reason: protocolMismatchReason(capabilities.protocolVersion),
          };
        }
        const versionedHeaders = new Headers(headers);
        versionedHeaders.set(PROTOCOL_VERSION_HEADER, PROTOCOL_VERSION);
        const response = await fetchFn(new URL("/api/sessions", `${server.url}/`), {
          headers: versionedHeaders,
          signal: AbortSignal.timeout(3000),
        });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        return {
          serverId: server.id,
          displayName: server.display_name,
          state: "connected",
        };
      } catch (error) {
        return {
          serverId: server.id,
          displayName: server.display_name,
          state: "unavailable",
          reason: error instanceof Error ? error.message : String(error),
        };
      }
    }),
  );
  return { checkedAt: new Date().toISOString(), servers: observations };
}

export class FleetHealthMonitor {
  private cached: FleetHealth | null = null;
  private inFlight: Promise<FleetHealth> | null = null;

  constructor(
    private readonly servers: () => readonly ServerConfig[],
    private readonly minimumIntervalMs = 5000,
    private readonly deps: ServerHealthDependencies = {},
  ) {}

  list(): Promise<FleetHealth> {
    if (
      this.cached &&
      Date.now() - new Date(this.cached.checkedAt).getTime() < this.minimumIntervalMs
    ) {
      return Promise.resolve(this.cached);
    }
    if (this.inFlight) return this.inFlight;
    this.inFlight = probeServerFleet(this.servers(), this.deps)
      .then((next) => {
        this.cached = next;
        return next;
      })
      .finally(() => {
        this.inFlight = null;
      });
    return this.inFlight;
  }

  invalidate(): void {
    this.cached = null;
  }
}
