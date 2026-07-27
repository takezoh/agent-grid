import { describe, expect, it, vi } from "vitest";
import type { ServerConfig } from "../src/main/desktop-config.js";
import { FleetHealthMonitor, probeServerFleet } from "../src/main/server-health.js";

const servers: ServerConfig[] = [
  {
    id: "healthy",
    display_name: "Healthy",
    enabled: true,
    url: "http://healthy.test",
    auth: { mode: "bearer", token: "healthy-token" },
  },
  {
    id: "down",
    display_name: "Down",
    enabled: true,
    url: "http://down.test",
    auth: { mode: "bearer", token: "down-token" },
  },
  {
    id: "disabled",
    display_name: "Disabled",
    enabled: false,
    url: "http://disabled.test",
    auth: { mode: "none" },
  },
];

describe("probeServerFleet", () => {
  it("reports every configured server and never probes disabled entries", async () => {
    const fetchFn = vi.fn(async (input: string | URL | Request) => ({
      ok: String(input).includes("healthy.test"),
      status: 503,
    })) as unknown as typeof fetch;
    const result = await probeServerFleet(servers, { fetchFn });

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
    expect(fetchFn).toHaveBeenNthCalledWith(
      1,
      expect.any(URL),
      expect.objectContaining({
        headers: { Authorization: "Bearer healthy-token" },
      }),
    );
  });

  it("omits Authorization for explicit none auth", async () => {
    const fetchFn = vi.fn(async () => ({ ok: true, status: 200 })) as unknown as typeof fetch;
    await probeServerFleet([{ ...servers[0]!, auth: { mode: "none" } }], { fetchFn });

    expect(fetchFn).toHaveBeenCalledWith(
      expect.any(URL),
      expect.objectContaining({ headers: undefined }),
    );
  });
});

describe("FleetHealthMonitor", () => {
  it("coalesces renderer requests and invalidates after config changes", async () => {
    const fetchFn = vi.fn(async () => ({ ok: true, status: 200 })) as unknown as typeof fetch;
    const monitor = new FleetHealthMonitor(() => servers.slice(0, 1), 60_000, {
      fetchFn,
    });

    await Promise.all([monitor.list(), monitor.list()]);
    await monitor.list();
    expect(fetchFn).toHaveBeenCalledTimes(1);

    monitor.invalidate();
    await monitor.list();
    expect(fetchFn).toHaveBeenCalledTimes(2);
  });
});
