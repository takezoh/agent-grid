import { describe, expect, it, vi } from "vitest";
import type { ServerConfig } from "../src/main/desktop-config.js";
import { FleetHealthMonitor, probeServerFleet } from "../src/main/server-health.js";

const servers: ServerConfig[] = [
  {
    id: "healthy",
    display_name: "Healthy",
    enabled: true,
    url: "http://healthy.test",
    token_path: "healthy.token",
  },
  {
    id: "down",
    display_name: "Down",
    enabled: true,
    url: "http://down.test",
    token_path: "down.token",
  },
  {
    id: "disabled",
    display_name: "Disabled",
    enabled: false,
    url: "http://disabled.test",
    token_path: "disabled.token",
  },
];

describe("probeServerFleet", () => {
  it("reports every configured server and never probes disabled entries", async () => {
    const fetchFn = vi.fn(async (input: string | URL | Request) => ({
      ok: String(input).includes("healthy.test"),
      status: 503,
    })) as unknown as typeof fetch;
    const readToken = vi.fn(async () => "token");

    const result = await probeServerFleet(servers, { fetchFn, readToken });

    expect(result.servers).toEqual([
      { serverId: "healthy", displayName: "Healthy", state: "connected" },
      {
        serverId: "down",
        displayName: "Down",
        state: "unavailable",
        reason: "HTTP 503",
      },
      { serverId: "disabled", displayName: "Disabled", state: "disabled" },
    ]);
    expect(fetchFn).toHaveBeenCalledTimes(2);
    expect(readToken).toHaveBeenCalledTimes(2);
  });
});

describe("FleetHealthMonitor", () => {
  it("coalesces renderer requests and invalidates after config changes", async () => {
    const fetchFn = vi.fn(async () => ({ ok: true, status: 200 })) as unknown as typeof fetch;
    const monitor = new FleetHealthMonitor(() => servers.slice(0, 1), 60_000, {
      fetchFn,
      readToken: async () => "token",
    });

    await Promise.all([monitor.list(), monitor.list()]);
    await monitor.list();
    expect(fetchFn).toHaveBeenCalledTimes(1);

    monitor.invalidate();
    await monitor.list();
    expect(fetchFn).toHaveBeenCalledTimes(2);
  });
});
