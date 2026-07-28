import { describe, expect, it, vi } from "vitest";
import { FleetSessionRuntime } from "../src/main/fleet-session-runtime.js";
import type { ServerConfig } from "../src/main/desktop-config.js";

const servers: ServerConfig[] = [
  { id: "a", display_name: "Same", enabled: true, url: "http://a", auth: { mode: "none" } },
  { id: "b", display_name: "Same", enabled: true, url: "http://b", auth: { mode: "none" } },
];

describe("FleetSessionRuntime projection", () => {
  it("publishes one compound-keyed snapshot to every subscriber", () => {
    const runtime = new FleetSessionRuntime(servers, vi.fn() as typeof fetch);
    const first = vi.fn();
    const second = vi.fn();
    runtime.subscribe(first);
    runtime.subscribe(second);
    runtime.replace(servers[0], 1, {
      state: "connected",
      sessions: [{ id: "same" }],
    });
    runtime.replace(servers[1], 1, {
      state: "connected",
      sessions: [{ id: "same" }],
    });

    expect(first.mock.lastCall?.[0]).toEqual(second.mock.lastCall?.[0]);
    expect(first.mock.lastCall?.[0].sessions.map((entry: { ref: unknown }) => entry.ref)).toEqual([
      { serverId: "a", sessionId: "same" },
      { serverId: "b", sessionId: "same" },
    ]);
  });

  it("retains last-known sessions on failure and rejects an old generation", () => {
    const runtime = new FleetSessionRuntime(servers, vi.fn() as typeof fetch);
    const listener = vi.fn();
    runtime.subscribe(listener);
    runtime.replace(servers[0], 3, { state: "connected", sessions: [{ id: "new" }] });
    runtime.replace(servers[0], 3, {
      state: "unavailable",
      reason: "offline",
      sessions: [],
    });
    runtime.replace(servers[0], 2, { state: "connected", sessions: [{ id: "old" }] });

    expect(listener.mock.lastCall?.[0].members[0]).toMatchObject({
      generation: 3,
      state: "unavailable",
      stale: true,
      sessions: [{ id: "new" }],
    });
  });

  it("confirms create only after the compound identity is observed", async () => {
    vi.useFakeTimers();
    try {
      const runtime = new FleetSessionRuntime(servers, vi.fn() as typeof fetch);
      const observed = runtime.waitFor({ serverId: "b", sessionId: "created" }, 10_000);
      runtime.replace(servers[1], 1, {
        state: "connected",
        sessions: [{ id: "created" }],
      });
      await expect(observed).resolves.toBe(true);

      const unconfirmed = runtime.waitFor({ serverId: "a", sessionId: "missing" }, 10_000);
      await vi.advanceTimersByTimeAsync(10_000);
      await expect(unconfirmed).resolves.toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });
});
