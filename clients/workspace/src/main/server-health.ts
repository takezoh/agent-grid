import * as fs from "node:fs/promises";
import type { ServerConfig } from "./desktop-config.js";

export type ServerHealthState = "connected" | "unavailable" | "disabled";

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
  readToken?: (path: string) => Promise<string>;
}

/** Runtime-only fleet observation. It never enters DesktopConfig persistence. */
export async function probeServerFleet(
  servers: readonly ServerConfig[],
  deps: ServerHealthDependencies = {},
): Promise<FleetHealth> {
  const fetchFn = deps.fetchFn ?? fetch;
  const readToken = deps.readToken ?? ((path: string) => fs.readFile(path, "utf8"));
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
        const token = (await readToken(server.token_path)).trim();
        if (!token) throw new Error("token is empty");
        const response = await fetchFn(new URL("/api/sessions", `${server.url}/`), {
          headers: { Authorization: `Bearer ${token}` },
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
