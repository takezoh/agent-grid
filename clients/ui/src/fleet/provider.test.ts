import { afterEach, describe, expect, it, vi } from "vitest";
// vi.waitFor is available on the vitest namespace
import { fleetIsDegraded, reduceStatusAxes } from "./presentation";
import { makeFleetProvider } from "./provider";
import type { FleetHealthSnapshotV1, FleetHealthView } from "./session-protocol";

afterEach(() => {
  window.agentGridWorkspace = undefined;
});

const sampleHealth: FleetHealthSnapshotV1 = {
  schemaVersion: 1,
  hostInstanceId: "host-1",
  revision: 4,
  servers: [
    {
      serverId: "srv-a",
      serverLabel: "A",
      enabled: true,
      observation: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
    },
  ],
};

describe("FleetProvider host parity", () => {
  it("Web transport preserves all configured servers and qualifies every request", async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/host/config") {
        return Response.json({
          schema_version: 2,
          servers: [
            { serverId: "srv-a", displayName: "A", enabled: true, defaultExecution: false },
            { serverId: "srv-b", displayName: "B", enabled: true, defaultExecution: true },
          ],
        });
      }
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("srv-a")) return Response.json({ projects: ["/a"] });
      return new Response("not ready", { status: 503, headers: { "Retry-After": "1" } });
    }) as typeof fetch;

    const outcomes = await makeFleetProvider(fetchImpl).listConfigs();

    expect(outcomes).toEqual([
      {
        serverId: "srv-a",
        serverLabel: "A",
        isDefaultExecution: false,
        state: "connected",
        config: { projects: ["/a"] },
      },
      {
        serverId: "srv-b",
        serverLabel: "B",
        isDefaultExecution: true,
        state: "unavailable",
        reason: "HTTP 503",
      },
    ]);
    expect(fetchImpl).toHaveBeenCalledWith("/host/servers/srv-a/api/session-config");
    expect(fetchImpl).toHaveBeenCalledWith("/host/servers/srv-b/api/session-config");
  });

  it("preserves an exact protocol mismatch as the server-list cause", async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/config") {
        return Response.json({
          servers: [{ serverId: "srv-a", displayName: "A", enabled: true, defaultExecution: true }],
        });
      }
      return Response.json({ protocolVersion: "protocol-1" });
    }) as typeof fetch;

    await expect(makeFleetProvider(fetchImpl).listConfigs()).resolves.toEqual([
      {
        serverId: "srv-a",
        serverLabel: "A",
        isDefaultExecution: true,
        state: "incompatible_protocol",
        reason: "incompatible protocol: client protocol-3, server protocol-1",
      },
    ]);
    expect(fetchImpl).not.toHaveBeenCalledWith("/host/servers/srv-a/api/session-config");
  });

  it("sanitizes hostile protocolVersion so credential sentinels never reach reason text", async () => {
    const sentinel = "old; access_token=gho_renderer_leak Bearer secret";
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/config") {
        return Response.json({
          servers: [{ serverId: "srv-a", displayName: "A", enabled: true, defaultExecution: true }],
        });
      }
      return Response.json({ protocolVersion: sentinel });
    }) as typeof fetch;

    const outcomes = await makeFleetProvider(fetchImpl).listConfigs();
    expect(outcomes[0]?.state).toBe("incompatible_protocol");
    expect(outcomes[0]?.reason).toBe("incompatible protocol: client protocol-3, server <invalid>");
    expect(JSON.stringify(outcomes)).not.toContain("gho_");
    expect(JSON.stringify(outcomes)).not.toContain(sentinel);
  });

  it("delivery loss reconnects on a replacement socket and ignores old-socket late callbacks", async () => {
    // rejects counterexample: recover from a message on the already-closed socket
    const sockets: Array<{
      onmessage: ((event: MessageEvent) => void) | null;
      onclose: (() => void) | null;
      close: ReturnType<typeof vi.fn>;
      closed: boolean;
    }> = [];
    const pendingSleeps: Array<() => void> = [];
    class FakeSocket {
      onmessage: ((event: MessageEvent) => void) | null = null;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      closed = false;
      close = vi.fn(() => {
        if (this.closed) return;
        this.closed = true;
        this.onclose?.();
      });
      constructor() {
        sockets.push(this);
      }
    }
    vi.stubGlobal("WebSocket", FakeSocket as unknown as typeof WebSocket);
    // Patch sleep via Web path by stubbing setTimeout used by default sleep.
    const realSetTimeout = globalThis.setTimeout;
    vi.stubGlobal("setTimeout", ((handler: TimerHandler, _ms?: number, ...args: unknown[]) => {
      if (typeof handler === "function") {
        pendingSleeps.push(() => handler(...args));
        return 0 as unknown as ReturnType<typeof setTimeout>;
      }
      return realSetTimeout(handler, 0, ...args);
    }) as typeof setTimeout);

    const views: unknown[] = [];
    const provider = makeFleetProvider(vi.fn() as typeof fetch);
    const unsubscribe = provider.subscribeHealth((view) => views.push(view));
    expect(sockets).toHaveLength(1);
    const first = sockets[0];
    expect(first).toBeDefined();
    if (!first) throw new Error("expected first health socket");
    first.onmessage?.(new MessageEvent("message", { data: JSON.stringify(sampleHealth) }));
    first.onclose?.();
    expect(views.at(-1)).toMatchObject({
      hostInstanceId: "host-1",
      revision: 4,
      delivery: "lost",
    });

    // Flush reconnect delay → replacement socket.
    pendingSleeps.shift()?.();
    await Promise.resolve();
    expect(sockets.length).toBeGreaterThanOrEqual(2);
    const second = sockets.at(-1);
    expect(second).toBeDefined();
    if (!second) throw new Error("expected replacement health socket");
    expect(second).not.toBe(first);

    // Late message on the retired first socket must not recover delivery.
    first.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify({ ...sampleHealth, revision: 5 }),
      }),
    );
    expect(views.at(-1)).toMatchObject({ delivery: "lost", revision: 4 });

    // Only the replacement socket may restore live delivery.
    second.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify({ ...sampleHealth, revision: 5 }),
      }),
    );
    expect(views.at(-1)).toMatchObject({
      hostInstanceId: "host-1",
      revision: 5,
      delivery: "live",
    });

    const socketsBeforeUnsub = sockets.length;
    const pendingBeforeUnsub = pendingSleeps.length;
    unsubscribe();
    // Unsubscribe cancels scheduled reconnect and leaves no further socket work.
    first.onclose?.();
    second.onclose?.();
    while (pendingSleeps.length > 0) pendingSleeps.shift()?.();
    await Promise.resolve();
    expect(sockets.length).toBe(socketsBeforeUnsub);
    expect(pendingSleeps.length).toBeLessThanOrEqual(pendingBeforeUnsub);
    vi.unstubAllGlobals();
  });

  it("recovers after more than 16 failed replacement sockets via cooldown cycles", async () => {
    // rejects counterexample: stop forever after 16 failures with no recoverability
    const sockets: Array<{
      onclose: (() => void) | null;
      onmessage: ((event: MessageEvent) => void) | null;
      closed: boolean;
      close: ReturnType<typeof vi.fn>;
    }> = [];
    const pendingSleeps: Array<{ ms: number; resolve: () => void }> = [];
    let concurrentSleeps = 0;
    let maxConcurrentSleeps = 0;

    class FakeSocket {
      onmessage: ((event: MessageEvent) => void) | null = null;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      closed = false;
      close = vi.fn(() => {
        if (this.closed) return;
        this.closed = true;
        this.onclose?.();
      });
      constructor() {
        sockets.push(this);
      }
    }
    vi.stubGlobal("WebSocket", FakeSocket as unknown as typeof WebSocket);
    const realSetTimeout = globalThis.setTimeout;
    vi.stubGlobal("setTimeout", ((handler: TimerHandler, ms?: number, ...args: unknown[]) => {
      if (typeof handler === "function") {
        concurrentSleeps += 1;
        maxConcurrentSleeps = Math.max(maxConcurrentSleeps, concurrentSleeps);
        pendingSleeps.push({
          ms: ms ?? 0,
          resolve: () => {
            concurrentSleeps = Math.max(0, concurrentSleeps - 1);
            handler(...args);
          },
        });
        return 0 as unknown as ReturnType<typeof setTimeout>;
      }
      return realSetTimeout(handler, 0, ...args);
    }) as typeof setTimeout);

    const views: Array<{ delivery?: string; revision?: number | null }> = [];
    const provider = makeFleetProvider(vi.fn() as typeof fetch);
    const unsubscribe = provider.subscribeHealth((view) => views.push(view));
    expect(sockets).toHaveLength(1);

    // Fail the initial socket + 16 replacements (17 total closes) without recovery.
    for (let i = 0; i < 17; i += 1) {
      const current = sockets.at(-1);
      expect(current).toBeDefined();
      current?.onclose?.();
      expect(views.at(-1)?.delivery).toBe("lost");
      // Single-flight: at most one scheduled reconnect work item.
      expect(pendingSleeps.length).toBeLessThanOrEqual(1);
      const sleep = pendingSleeps.shift();
      expect(sleep).toBeDefined();
      sleep?.resolve();
      await Promise.resolve();
      await Promise.resolve();
    }
    // After the first retry window, a later replacement must still open.
    expect(sockets.length).toBeGreaterThan(17);
    expect(maxConcurrentSleeps).toBe(1);

    const recovery = sockets.at(-1);
    expect(recovery).toBeDefined();
    recovery?.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify({ ...sampleHealth, revision: 9 }),
      }),
    );
    expect(views.at(-1)).toMatchObject({
      hostInstanceId: "host-1",
      revision: 9,
      delivery: "live",
    });

    unsubscribe();
    while (pendingSleeps.length > 0) pendingSleeps.shift()?.resolve();
    vi.unstubAllGlobals();
  });

  it("seeds configured servers when health WS fails before any snapshot", async () => {
    // rejects counterexample: delivery-lost with zero servers hides Connections
    const sockets: Array<{ onclose: (() => void) | null }> = [];
    class FakeSocket {
      onmessage: ((event: MessageEvent) => void) | null = null;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      close = vi.fn(() => this.onclose?.());
      constructor() {
        sockets.push(this);
      }
    }
    vi.stubGlobal("WebSocket", FakeSocket as unknown as typeof WebSocket);

    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/config") {
        return Response.json({
          servers: [
            { serverId: "srv-a", displayName: "Alpha", enabled: true },
            { serverId: "srv-b", displayName: "Beta", enabled: false },
            { serverId: "srv-c", displayName: "Gamma", enabled: true },
          ],
        });
      }
      return new Response("unused", { status: 404 });
    }) as typeof fetch;

    const views: unknown[] = [];
    const provider = makeFleetProvider(fetchImpl);
    const unsubscribe = provider.subscribeHealth((view) => views.push(view));
    expect(sockets).toHaveLength(1);

    // Close before any accepted snapshot.
    sockets[0]?.onclose?.();

    await vi.waitFor(() => {
      const lost = [...views]
        .reverse()
        .find((view) => (view as { delivery?: string }).delivery === "lost") as {
        delivery: string;
        servers: Array<{ serverId: string; observation: { kind: string } }>;
      };
      expect(lost?.servers?.map((s) => s.serverId).sort()).toEqual(["srv-a", "srv-b", "srv-c"]);
      expect(lost.servers.find((s) => s.serverId === "srv-a")?.observation.kind).toBe(
        "health_unavailable",
      );
      expect(lost.servers.find((s) => s.serverId === "srv-b")?.observation.kind).toBe("disabled");
      expect(lost.servers.find((s) => s.serverId === "srv-c")?.observation.kind).toBe(
        "health_unavailable",
      );
    });

    // No accepted host identity/revision and no secrets in the seed.
    const lost = views.at(-1) as {
      hostInstanceId: string | null;
      revision: number | null;
      servers: unknown[];
    };
    expect(lost.hostInstanceId).toBeNull();
    expect(lost.revision).toBeNull();
    expect(JSON.stringify(lost)).not.toMatch(/Bearer |token=|gho_/i);

    unsubscribe();
    vi.unstubAllGlobals();
  });

  it("config failure plus pre-first-snapshot health loss is degraded, not nominal", async () => {
    // Production path: /host/config non-2xx while /host/fleet/health closes before
    // any snapshot. Empty servers must still surface delivery-lost diagnosis even
    // when an active qualified session remains usable (connection open).
    const sockets: Array<{ onclose: (() => void) | null }> = [];
    class FakeSocket {
      onmessage: ((event: MessageEvent) => void) | null = null;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      close = vi.fn(() => this.onclose?.());
      constructor() {
        sockets.push(this);
      }
    }
    vi.stubGlobal("WebSocket", FakeSocket as unknown as typeof WebSocket);

    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/config") {
        return new Response("config unavailable", { status: 503 });
      }
      return new Response("unused", { status: 404 });
    }) as typeof fetch;

    const views: FleetHealthView[] = [];
    const provider = makeFleetProvider(fetchImpl);
    const unsubscribe = provider.subscribeHealth((view) => views.push(view));
    expect(sockets).toHaveLength(1);

    sockets[0]?.onclose?.();

    await vi.waitFor(() => {
      expect(views.some((view) => view.delivery === "lost")).toBe(true);
    });

    const lost = [...views].reverse().find((view) => view.delivery === "lost");
    expect(lost).toMatchObject({
      delivery: "lost",
      hostInstanceId: null,
      revision: null,
      servers: [],
    });
    // Config failure leaves no seed; delivery loss alone must degrade diagnosis.
    expect(fleetIsDegraded(lost ?? null)).toBe(true);
    const axes = reduceStatusAxes({
      connectionStatus: "open",
      daemonDisconnected: false,
      health: lost ?? null,
    });
    expect(axes.nominal).toBe(false);
    expect(axes.showConnections).toBe(true);
    expect(axes.fleetSummary).toMatch(/delivery lost|unavailable/i);

    unsubscribe();
    vi.unstubAllGlobals();
  });

  it("Workspace transport exposes the same contract without bearer material", async () => {
    const projected = [
      {
        serverId: "srv-a",
        serverLabel: "A",
        state: "connected" as const,
        config: { projects: [{ path: "/a" }] },
      },
    ];
    const openSettings = vi.fn().mockResolvedValue(undefined);
    const createRepositorySession = vi.fn().mockResolvedValue({
      preparation_id: "prep-1",
      intent_id: "intent-1",
      stage: "confirmed",
      revision: 1,
    });
    const request = vi.fn().mockResolvedValue({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        preparation_id: "prep-1",
        intent_id: "intent-1",
        server_id: "srv-a",
        repository: { repository_id: "1", owner: "alice", name: "demo" },
        stage: "failed",
        failure_code: "clone_failed",
        retry_origin: "clone",
        revision: 3,
      }),
    });
    window.agentGridWorkspace = {
      hostedModeInfo: {
        hosted: true,
        serverId: "srv-a",
        baseUrl: "https://gateway.invalid",
        token: "renderer-token",
      },
      openSettings,
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue(projected),
        create: vi.fn(),
        createRepositorySession,
        request,
        subscribeHealth: (listener) => {
          listener(sampleHealth);
          return () => undefined;
        },
      },
      github: {
        action: vi.fn().mockResolvedValue({
          outcome: "success",
          account: { state: "authenticated", login: "alice" },
          catalog: {
            outcome: "complete-current",
            generation: 1,
            repositories: [{ repository_id: "1", owner: "alice", name: "demo" }],
          },
        }),
        openAuthorization: vi.fn(),
      },
    };

    const provider = makeFleetProvider();
    const outcomes = await provider.listConfigs();
    const healthViews: unknown[] = [];
    provider.subscribeHealth((view) => healthViews.push(view));

    expect(outcomes).toEqual(projected);
    expect(healthViews[0]).toMatchObject({ hostInstanceId: "host-1", delivery: "live" });
    expect(JSON.stringify(outcomes)).not.toContain("renderer-token");

    const repo = await provider.repositoryProjection();
    expect(repo.targets.some((t) => t.source === "project")).toBe(true);
    expect(repo.targets.some((t) => t.source === "repository")).toBe(true);
    expect(JSON.stringify(repo)).not.toContain("renderer-token");
    expect(JSON.stringify(repo)).not.toMatch(/gho_|ghu_|github_pat/i);
    await provider.openGitHubSettings?.();
    expect(openSettings).toHaveBeenCalledWith("github");
    await provider.createRepositorySession("srv-a", {
      repository_id: "1",
      owner: "alice",
      name: "demo",
      version: 2,
      profile_id: "shell",
      profile_revision: "revision-shell",
    });
    expect(createRepositorySession).toHaveBeenCalledWith("srv-a", {
      repository_id: "1",
      owner: "alice",
      name: "demo",
      version: 2,
      profile_id: "shell",
      profile_revision: "revision-shell",
    });
    const updated = await provider.repositoryProjection();
    expect(request).toHaveBeenCalledWith("srv-a", "/api/repository-preparations/prep-1", {
      method: "GET",
    });
    expect(updated.preparations).toMatchObject([
      { preparation_id: "prep-1", stage: "failed", failure_code: "clone_failed", revision: 3 },
    ]);
  });

  it("Web repositoryProjection treats 404 as transient failure (not empty wipe)", async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/repository") {
        return new Response("missing", { status: 404 });
      }
      return new Response("nope", { status: 500 });
    }) as typeof fetch;
    await expect(makeFleetProvider(fetchImpl).repositoryProjection()).rejects.toThrow(/HTTP 404/);
  });

  it("does not synthesize a server from the browser origin", () => {
    window.history.replaceState({}, "", "/");
    expect(makeFleetProvider(vi.fn() as typeof fetch).initialServerId()).toBeNull();
  });
});
