import { describe, expect, it, vi } from "vitest";
import type { ServerConfig } from "../src/main/desktop-config.js";
import {
  FleetHealthMonitor,
  connectionFailureReason,
  probeServerFleet,
} from "../src/main/server-health.js";

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
  it("normalizes runtime transport failures without exposing fetch internals", () => {
    expect(connectionFailureReason(new TypeError("fetch failed"))).toBe(
      "Could not connect to server",
    );
    expect(connectionFailureReason(new Error("The operation was aborted due to timeout"))).toBe(
      "Connection timed out",
    );
    expect(connectionFailureReason(new Error("HTTP 503"))).toBe("HTTP 503");
  });

  it("reports every configured server and never probes disabled entries", async () => {
    const fetchFn = vi.fn(async (input: string | URL | Request) => {
      const url = String(input);
      if (url.includes("healthy.test") && url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("healthy.test")) {
        return Response.json({ sessions: [] });
      }
      return new Response("", { status: 503 });
    }) as unknown as typeof fetch;
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
    expect(fetchFn).toHaveBeenCalledTimes(3);
    expect(fetchFn).toHaveBeenNthCalledWith(
      1,
      expect.any(URL),
      expect.objectContaining({
        headers: { Authorization: "Bearer healthy-token" },
      }),
    );
  });

  it("omits Authorization for explicit none auth", async () => {
    const fetchFn = vi.fn(async (input: string | URL | Request) =>
      String(input).includes("/api/capabilities")
        ? Response.json({ protocolVersion: "protocol-3" })
        : Response.json({ sessions: [] })) as unknown as typeof fetch;
    await probeServerFleet([{ ...servers[0]!, auth: { mode: "none" } }], { fetchFn });

    expect(fetchFn).toHaveBeenCalledWith(
      expect.any(URL),
      expect.objectContaining({ headers: undefined }),
    );
  });

  it("reports version mismatch without probing sessions", async () => {
    const fetchFn = vi.fn(async () => Response.json({ protocolVersion: "old" })) as typeof fetch;
    const result = await probeServerFleet(servers.slice(0, 1), { fetchFn });

    expect(result.servers[0]).toEqual({
      serverId: "healthy",
      displayName: "Healthy",
      state: "incompatible_protocol",
      reason: "incompatible protocol: client protocol-3, server old",
    });
    expect(fetchFn).toHaveBeenCalledTimes(1);
  });
});

describe("FleetHealthMonitor", () => {
  it("coalesces renderer requests and invalidates after config changes", async () => {
    const fetchFn = vi.fn(async (input: string | URL | Request) =>
      String(input).includes("/api/capabilities")
        ? Response.json({ protocolVersion: "protocol-3" })
        : Response.json({ sessions: [] })) as unknown as typeof fetch;
    const monitor = new FleetHealthMonitor(() => servers.slice(0, 1), 60_000, {
      fetchFn,
    });

    await Promise.all([monitor.list(), monitor.list()]);
    await monitor.list();
    expect(fetchFn).toHaveBeenCalledTimes(2);

    monitor.invalidate();
    await monitor.list();
    expect(fetchFn).toHaveBeenCalledTimes(4);
  });
});
