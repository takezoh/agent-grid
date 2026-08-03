import { describe, expect, it, vi } from "vitest";
import {
  FleetSessionRuntime,
  HEALTH_CONNECTED_FRESHNESS_MS,
  HEALTH_PHASE_TIMEOUT_MS,
} from "../src/main/fleet-session-runtime.js";
import type { ServerConfig } from "../src/main/desktop-config.js";
import { decodeFleetHealthSnapshotV1 } from "../src/main/server-health.js";

const servers: ServerConfig[] = [
  { id: "a", display_name: "Same", enabled: true, url: "http://a", auth: { mode: "none" } },
  { id: "b", display_name: "Same", enabled: true, url: "http://b", auth: { mode: "none" } },
  {
    id: "disabled",
    display_name: "Disabled",
    enabled: false,
    url: "http://disabled",
    auth: { mode: "none" },
  },
];

describe("FleetSessionRuntime projection", () => {
  it("publishes one compound-keyed snapshot to every subscriber", () => {
    const runtime = new FleetSessionRuntime(servers, vi.fn() as typeof fetch);
    const first = vi.fn();
    const second = vi.fn();
    runtime.subscribe(first);
    runtime.subscribe(second);
    runtime.replace(servers[0]!, 1, {
      state: "connected",
      sessions: [{ id: "same" }],
    });
    runtime.replace(servers[1]!, 1, {
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
    runtime.replace(servers[0]!, 3, { state: "connected", sessions: [{ id: "new" }] });
    runtime.replace(servers[0]!, 3, {
      state: "unavailable",
      reason: "offline",
      sessions: [],
    });
    runtime.replace(servers[0]!, 2, { state: "connected", sessions: [{ id: "old" }] });

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
      runtime.replace(servers[1]!, 1, {
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

describe("FleetSessionRuntime health authority", () => {
  // rejects counterexample: independent FleetHealthMonitor probe counts and disabled workers
  it("projects one exclusive observation per configured server without disabled workers", () => {
    const runtime = new FleetSessionRuntime(servers, {
      fetchFn: vi.fn() as typeof fetch,
      hostInstanceId: "host-test",
    });
    const health = runtime.healthSnapshot();
    expect(health.servers).toHaveLength(3);
    expect(health.servers.map((server) => server.observation.kind)).toEqual([
      "checking",
      "checking",
      "disabled",
    ]);
    expect(runtime.enabledWorkerCount()).toBe(2);
    runtime.start();
    expect(runtime.startedWorkerCount()).toBe(2);
    runtime.stop();
    expect(runtime.liveHandleCounts()).toEqual({ timers: 0, fetches: 0, sockets: 0 });
  });

  it("marks exact protocol mismatch terminal and generation-fences late publications", async () => {
    const fetchFn = vi.fn(async () =>
      Response.json({ protocolVersion: "protocol-1" }),
    ) as unknown as typeof fetch;
    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      hostInstanceId: "host-mismatch",
      clock: {
        now: () => Date.now(),
        sleep: (ms) => new Promise((resolve) => setTimeout(resolve, Math.min(ms, 1))),
      },
    });
    const listener = vi.fn();
    runtime.subscribeHealth(listener);
    runtime.start();
    await vi.waitFor(() => {
      const latest = listener.mock.calls.at(-1)?.[0];
      expect(latest?.servers[0]?.observation.kind).toBe("incompatible_protocol");
    });
    expect(
      runtime.publishHealth("a", 0, {
        kind: "connected",
        observedAt: new Date().toISOString(),
      }),
    ).toBe(false);
    runtime.stop();
  });

  it("never places hostile protocolVersion material into health wire detail", async () => {
    const sentinel = "protocol-1; access_token=gho_must_not_leak Bearer secret";
    const fetchFn = vi.fn(async () =>
      Response.json({ protocolVersion: sentinel }),
    ) as unknown as typeof fetch;
    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      hostInstanceId: "host-hostile-protocol",
      clock: {
        now: () => Date.now(),
        sleep: (ms) => new Promise((resolve) => setTimeout(resolve, Math.min(ms, 1))),
      },
    });
    const listener = vi.fn();
    runtime.subscribeHealth(listener);
    runtime.start();
    await vi.waitFor(() => {
      const latest = listener.mock.calls.at(-1)?.[0];
      expect(latest?.servers[0]?.observation.kind).toBe("incompatible_protocol");
    });
    const encoded = JSON.stringify(listener.mock.calls.at(-1)?.[0]);
    expect(encoded).not.toContain("gho_");
    expect(encoded).not.toContain("Bearer secret");
    expect(encoded).not.toContain(sentinel);
    expect(encoded).toContain("server <invalid>");
    runtime.stop();
  });

  it("nests stale prior via production connected then failed refresh path", async () => {
    // rejects counterexample: manufacturing health_unavailable via publishHealth alone
    type Waiter = { at: number; resolve: () => void };
    let nowMs = 0;
    const waiters: Waiter[] = [];
    const clock = {
      now: () => nowMs,
      sleep: (ms: number) =>
        new Promise<void>((resolve) => {
          waiters.push({ at: nowMs + ms, resolve });
        }),
    };
    const advance = async (ms: number) => {
      nowMs += ms;
      const due = waiters.filter((waiter) => waiter.at <= nowMs).sort((a, b) => a.at - b.at);
      for (const waiter of due) {
        const index = waiters.indexOf(waiter);
        if (index >= 0) waiters.splice(index, 1);
        waiter.resolve();
      }
      await Promise.resolve();
      await Promise.resolve();
    };

    let messageHandler: ((event: Event) => void) | null = null;
    let closeHandler: (() => void) | null = null;
    let phase = 0;
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("/api/ws-ticket")) {
        return Response.json({ ticket: `t-${phase}` });
      }
      return new Response("missing", { status: 404 });
    }) as unknown as typeof fetch;

    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      clock,
      hostInstanceId: "host-stale-prior",
      random: () => 0,
      webSocketFactory: () => {
        phase += 1;
        const generation = phase;
        return {
          readyState: 1,
          close: vi.fn(() => {
            closeHandler?.();
          }),
          addEventListener: (type: string, handler: EventListener) => {
            if (type === "message") messageHandler = handler as (event: Event) => void;
            if (type === "close") closeHandler = handler as () => void;
            if (type === "error" && generation > 1) {
              // Second connection fails immediately after connect attempt.
              queueMicrotask(() => {
                (handler as () => void)();
              });
            }
          },
          removeEventListener: () => undefined,
        };
      },
    });
    const listener = vi.fn();
    runtime.subscribeHealth(listener);
    runtime.start();
    await vi.waitFor(() => expect(messageHandler).not.toBeNull());
    messageHandler?.(
      new MessageEvent("message", {
        data: JSON.stringify({ k: "h", sessions: [] }),
      }),
    );
    await vi.waitFor(() => {
      const latest = listener.mock.calls.at(-1)?.[0];
      expect(latest?.servers[0]?.observation.kind).toBe("connected");
    });
    // Force delivery/refresh failure by closing the live socket after Connected.
    closeHandler?.();
    await vi.waitFor(() => {
      const latest = listener.mock.calls.at(-1)?.[0];
      expect(latest?.servers[0]?.observation.kind).toBe("health_unavailable");
      expect(latest?.servers[0]?.observation).toMatchObject({
        kind: "health_unavailable",
        reason: "refresh_failed",
        prior: { kind: "connected" },
      });
    });
    runtime.stop();
    void advance;
  });

  it("drives capability, ticket, and hello three-second deadlines behaviorally", async () => {
    // rejects counterexample: historical pre-deadline "checking" alone green-lights
    // timeout tests; each phase must emit a post-deadline unavailable observation
    // and exercise phase-specific resource cancellation.
    type Waiter = { at: number; resolve: () => void };
    let nowMs = 1_000;
    const waiters: Waiter[] = [];
    const clock = {
      now: () => nowMs,
      sleep: (ms: number) =>
        new Promise<void>((resolve) => {
          waiters.push({ at: nowMs + ms, resolve });
        }),
    };
    const advance = async (ms: number) => {
      nowMs += ms;
      const due = waiters.filter((waiter) => waiter.at <= nowMs).sort((a, b) => a.at - b.at);
      for (const waiter of due) {
        const index = waiters.indexOf(waiter);
        if (index >= 0) waiters.splice(index, 1);
        waiter.resolve();
      }
      await Promise.resolve();
      await Promise.resolve();
    };
    const latestKind = (listener: ReturnType<typeof vi.fn>): string | undefined =>
      listener.mock.calls.at(-1)?.[0]?.servers?.[0]?.observation?.kind as string | undefined;
    const postDeadlineUnavailable = (
      listener: ReturnType<typeof vi.fn>,
      callsBeforeDeadline: number,
    ): boolean => {
      const after = listener.mock.calls.slice(callsBeforeDeadline);
      return after.some((call) => {
        const observation = call[0]?.servers?.[0]?.observation as
          | { kind?: string; reason?: string }
          | undefined;
        return observation?.kind === "unavailable";
      });
    };

    // Capability deadline — hang capabilities until abort/timeout.
    {
      let fetchAborted = false;
      const fetchFn = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
        return new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => {
            fetchAborted = true;
            reject(new Error("aborted"));
          });
        });
      }) as unknown as typeof fetch;
      const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
        fetchFn,
        clock,
        hostInstanceId: "host-cap-deadline",
        random: () => 0,
      });
      const listener = vi.fn();
      runtime.subscribeHealth(listener);
      runtime.start();
      await vi.waitFor(() => expect(fetchFn).toHaveBeenCalled());
      const callsBefore = listener.mock.calls.length;
      // Pre-deadline historical checking alone must not satisfy the assertion.
      expect(latestKind(listener)).toBe("checking");
      await advance(HEALTH_PHASE_TIMEOUT_MS);
      await vi.waitFor(() => {
        expect(fetchAborted).toBe(true);
        expect(postDeadlineUnavailable(listener, callsBefore)).toBe(true);
      });
      expect(latestKind(listener)).toBe("unavailable");
      runtime.stop();
    }

    // Ticket deadline (capabilities succeed, ticket hangs) — abort ticket fetch.
    {
      nowMs = 10_000;
      waiters.length = 0;
      let ticketAborted = false;
      let ticketFetchSeen = false;
      const fetchFn = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes("/api/capabilities")) {
          return Promise.resolve(Response.json({ protocolVersion: "protocol-3" }));
        }
        ticketFetchSeen = true;
        return new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => {
            ticketAborted = true;
            reject(new Error("aborted"));
          });
        });
      }) as unknown as typeof fetch;
      const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
        fetchFn,
        clock,
        hostInstanceId: "host-ticket-deadline",
        random: () => 0,
      });
      const listener = vi.fn();
      runtime.subscribeHealth(listener);
      runtime.start();
      await vi.waitFor(() => expect(ticketFetchSeen).toBe(true));
      const callsBefore = listener.mock.calls.length;
      expect(latestKind(listener)).toBe("checking");
      await advance(HEALTH_PHASE_TIMEOUT_MS);
      await vi.waitFor(() => {
        // Phase-specific cancellation: ticket request abort/timeout resource.
        expect(ticketAborted).toBe(true);
        expect(postDeadlineUnavailable(listener, callsBefore)).toBe(true);
      });
      expect(latestKind(listener)).toBe("unavailable");
      runtime.stop();
    }

    // Hello deadline (capabilities+ticket ok, socket silent) — close WebSocket.
    {
      nowMs = 20_000;
      waiters.length = 0;
      const socketClose = vi.fn();
      const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes("/api/capabilities")) {
          return Response.json({ protocolVersion: "protocol-3" });
        }
        if (url.includes("/api/ws-ticket")) {
          return Response.json({ ticket: "t-hello" });
        }
        return new Response("missing", { status: 404 });
      }) as unknown as typeof fetch;
      const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
        fetchFn,
        clock,
        hostInstanceId: "host-hello-deadline",
        random: () => 0,
        webSocketFactory: () => ({
          readyState: 1,
          close: socketClose,
          addEventListener: () => undefined,
          removeEventListener: () => undefined,
        }),
      });
      const listener = vi.fn();
      runtime.subscribeHealth(listener);
      runtime.start();
      await vi.waitFor(() => expect(fetchFn.mock.calls.length).toBeGreaterThanOrEqual(2));
      await vi.waitFor(() => expect(runtime.liveHandleCounts().sockets).toBeGreaterThan(0));
      const callsBefore = listener.mock.calls.length;
      expect(latestKind(listener)).toBe("checking");
      await advance(HEALTH_PHASE_TIMEOUT_MS);
      await vi.waitFor(() => {
        // Phase-specific cancellation: hello timeout closes the WebSocket.
        expect(socketClose).toHaveBeenCalled();
        expect(postDeadlineUnavailable(listener, callsBefore)).toBe(true);
      });
      expect(latestKind(listener)).toBe("unavailable");
      runtime.stop();
    }
  });

  it("aborts pending capability fetch on three-second timeout and retires live handles", async () => {
    // rejects counterexample: timeout rejects without aborting the underlying fetch
    type Waiter = { at: number; resolve: () => void };
    let nowMs = 1_000;
    const waiters: Waiter[] = [];
    const clock = {
      now: () => nowMs,
      sleep: (ms: number) =>
        new Promise<void>((resolve) => {
          waiters.push({ at: nowMs + ms, resolve });
        }),
    };
    const advance = async (ms: number) => {
      nowMs += ms;
      const due = waiters.filter((waiter) => waiter.at <= nowMs);
      for (const waiter of due) {
        const index = waiters.indexOf(waiter);
        if (index >= 0) waiters.splice(index, 1);
        waiter.resolve();
      }
      await Promise.resolve();
      await Promise.resolve();
    };

    let fetchSignal: AbortSignal | null = null;
    let resolveFetch: ((value: Response) => void) | null = null;
    let fetchAborted = false;
    const fetchFn = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      fetchSignal = init?.signal ?? null;
      fetchSignal?.addEventListener("abort", () => {
        fetchAborted = true;
      });
      return new Promise<Response>((resolve) => {
        resolveFetch = resolve;
      });
    }) as unknown as typeof fetch;

    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      clock,
      hostInstanceId: "host-timeout",
      random: () => 0,
    });
    const listener = vi.fn();
    runtime.subscribeHealth(listener);
    runtime.start();

    await vi.waitFor(() => expect(fetchFn).toHaveBeenCalled());
    expect(runtime.liveHandleCounts().fetches).toBeGreaterThan(0);

    await advance(HEALTH_PHASE_TIMEOUT_MS);
    await vi.waitFor(() => expect(fetchAborted).toBe(true));

    // Late publication from the timed-out generation must not land after stop.
    resolveFetch?.(Response.json({ protocolVersion: "protocol-3" }));
    await Promise.resolve();
    runtime.stop();
    await vi.waitFor(() => {
      expect(runtime.liveHandleCounts()).toEqual({ timers: 0, fetches: 0, sockets: 0 });
    });
    const kinds = listener.mock.calls.map(
      (call) => call[0]?.servers?.[0]?.observation?.kind as string | undefined,
    );
    expect(kinds.some((kind) => kind === "unavailable" || kind === "checking")).toBe(true);
  });

  it("closes connected freshness after five seconds via injected clock", async () => {
    type Waiter = { at: number; resolve: () => void };
    let nowMs = 0;
    const waiters: Waiter[] = [];
    const clock = {
      now: () => nowMs,
      sleep: (ms: number) =>
        new Promise<void>((resolve) => {
          waiters.push({ at: nowMs + ms, resolve });
        }),
    };
    const advance = async (ms: number) => {
      nowMs += ms;
      const due = waiters.filter((waiter) => waiter.at <= nowMs).sort((a, b) => a.at - b.at);
      for (const waiter of due) {
        const index = waiters.indexOf(waiter);
        if (index >= 0) waiters.splice(index, 1);
        waiter.resolve();
      }
      await Promise.resolve();
      await Promise.resolve();
    };

    let socketClose = vi.fn();
    let messageHandler: ((event: Event) => void) | null = null;
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("/api/ws-ticket")) {
        return Response.json({ ticket: "t-1" });
      }
      return new Response("missing", { status: 404 });
    }) as unknown as typeof fetch;

    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      clock,
      hostInstanceId: "host-freshness",
      random: () => 0,
      webSocketFactory: () => {
        const socket = {
          readyState: 1,
          close: socketClose,
          addEventListener: (type: string, handler: EventListener) => {
            if (type === "message") messageHandler = handler as (event: Event) => void;
            if (type === "close") {
              // no-op until close() is called
            }
          },
          removeEventListener: () => undefined,
        };
        socketClose = vi.fn(() => {
          // emulate close event absence; generation ends via close call observation
        });
        socket.close = socketClose;
        return socket;
      },
    });
    const listener = vi.fn();
    runtime.subscribeHealth(listener);
    runtime.start();
    await vi.waitFor(() => expect(messageHandler).not.toBeNull());
    messageHandler?.(
      new MessageEvent("message", {
        data: JSON.stringify({ k: "h", sessions: [] }),
      }),
    );
    await vi.waitFor(() => {
      const latest = listener.mock.calls.at(-1)?.[0];
      expect(latest?.servers[0]?.observation.kind).toBe("connected");
    });
    await advance(HEALTH_CONNECTED_FRESHNESS_MS);
    await vi.waitFor(() => expect(socketClose).toHaveBeenCalled());
    runtime.stop();
    await vi.waitFor(() => {
      expect(runtime.liveHandleCounts()).toEqual({ timers: 0, fetches: 0, sockets: 0 });
    });
  });

  it("renews connected freshness from p heartbeats across two quiet windows", async () => {
    type Waiter = { at: number; resolve: () => void };
    let nowMs = 0;
    const waiters: Waiter[] = [];
    const clock = {
      now: () => nowMs,
      sleep: (ms: number) =>
        new Promise<void>((resolve) => {
          waiters.push({ at: nowMs + ms, resolve });
        }),
    };
    const advance = async (ms: number) => {
      nowMs += ms;
      const due = waiters.filter((waiter) => waiter.at <= nowMs).sort((a, b) => a.at - b.at);
      for (const waiter of due) {
        const index = waiters.indexOf(waiter);
        if (index >= 0) waiters.splice(index, 1);
        waiter.resolve();
      }
      await Promise.resolve();
      await Promise.resolve();
    };

    let messageHandler: ((event: Event) => void) | null = null;
    let closeHandler: (() => void) | null = null;
    const socketClose = vi.fn(() => closeHandler?.());
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("/api/ws-ticket")) {
        return Response.json({ ticket: "t-heartbeat" });
      }
      return new Response("missing", { status: 404 });
    }) as unknown as typeof fetch;
    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      clock,
      hostInstanceId: "host-heartbeat",
      random: () => 0,
      webSocketFactory: () => ({
        readyState: 1,
        close: socketClose,
        addEventListener: (type: string, handler: EventListener) => {
          if (type === "message") messageHandler = handler as (event: Event) => void;
          if (type === "close") closeHandler = handler as () => void;
        },
        removeEventListener: () => undefined,
      }),
    });
    const listener = vi.fn();
    runtime.subscribeHealth(listener);
    runtime.start();
    await vi.waitFor(() => expect(messageHandler).not.toBeNull());
    messageHandler?.(
      new MessageEvent("message", { data: JSON.stringify({ k: "h", sessions: [] }) }),
    );
    await vi.waitFor(() => {
      expect(listener.mock.calls.at(-1)?.[0]?.servers[0]?.observation.kind).toBe("connected");
    });
    // Let the 25ms hello-phase poll observe sawHello before jumping the fake
    // clock across the 3s phase deadline. Production time advances normally;
    // a single coarse fake-clock jump would otherwise fire both waiters first.
    await advance(25);

    const step = HEALTH_CONNECTED_FRESHNESS_MS - 1_000;
    for (let index = 0; index < 3; index += 1) {
      await advance(step);
      messageHandler?.(new MessageEvent("message", { data: JSON.stringify({ k: "p" }) }));
      await Promise.resolve();
    }
    expect(3 * step).toBeGreaterThanOrEqual(2 * HEALTH_CONNECTED_FRESHNESS_MS);
    expect(socketClose).not.toHaveBeenCalled();
    expect(listener.mock.calls.at(-1)?.[0]?.servers[0]?.observation.kind).toBe("connected");
    runtime.stop();
  });

  it("stop aborts an established pending fetch without relying on counter reset", async () => {
    type Waiter = { at: number; resolve: () => void };
    let nowMs = 0;
    const waiters: Waiter[] = [];
    const clock = {
      now: () => nowMs,
      sleep: (ms: number) =>
        new Promise<void>((resolve) => {
          waiters.push({ at: nowMs + ms, resolve });
        }),
    };

    let fetchSignal: AbortSignal | null = null;
    const fetchFn = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      fetchSignal = init?.signal ?? null;
      return new Promise<Response>(() => {
        // pending until aborted
      });
    }) as unknown as typeof fetch;

    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      clock,
      hostInstanceId: "host-stop",
      random: () => 0,
    });
    runtime.start();
    await vi.waitFor(() => expect(fetchFn).toHaveBeenCalled());
    expect(runtime.liveHandleCounts().fetches).toBeGreaterThan(0);
    expect(fetchSignal?.aborted).toBe(false);
    runtime.stop();
    expect(fetchSignal?.aborted).toBe(true);
    await vi.waitFor(() => {
      expect(runtime.liveHandleCounts()).toEqual({ timers: 0, fetches: 0, sockets: 0 });
    });
    void waiters;
  });

  it("health snapshot decodes as credential-free V1 wire", () => {
    const runtime = new FleetSessionRuntime(
      [
        {
          id: "secret-server",
          display_name: "Secret",
          enabled: true,
          url: "https://user:pass@example.test/path?token=abc",
          auth: { mode: "bearer", token: "super-secret-token" },
        },
      ],
      { hostInstanceId: "host-redact" },
    );
    runtime.publishHealth("secret-server", 1, {
      kind: "unavailable",
      observedAt: "2026-08-03T00:00:00.000Z",
      reason: "connection_failed",
    });
    const encoded = JSON.stringify(runtime.healthSnapshot());
    expect(() => decodeFleetHealthSnapshotV1(JSON.parse(encoded))).not.toThrow();
    expect(encoded).not.toContain("super-secret-token");
    expect(encoded).not.toContain("user:pass");
    expect(encoded).not.toContain("token=abc");
  });
});

describe("FleetSessionRuntime manual reconnect", () => {
  it("rejects unknown and disabled server IDs with credential-free errors", async () => {
    const runtime = new FleetSessionRuntime(servers, {
      fetchFn: vi.fn() as typeof fetch,
      hostInstanceId: "host-reconnect-reject",
    });
    await expect(runtime.reconnect("missing")).rejects.toThrow(
      /unknown or disabled server 'missing'/,
    );
    await expect(runtime.reconnect("disabled")).rejects.toThrow(
      /unknown or disabled server 'disabled'/,
    );
    const encoded = JSON.stringify({
      missing: "missing",
      disabled: "disabled",
    });
    expect(encoded).not.toMatch(/Bearer |token=|gho_/i);
    runtime.stop();
  });

  it("makes start idempotent and keeps reconnect-before-start to one worker per server", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("/api/ws-ticket")) {
        return Response.json({ ticket: "ticket" });
      }
      return new Response("missing", { status: 404 });
    }) as unknown as typeof fetch;
    const socketsOpened: string[] = [];
    const runtime = new FleetSessionRuntime(servers.slice(0, 2), {
      fetchFn,
      hostInstanceId: "host-lifecycle",
      clock: {
        now: () => Date.now(),
        sleep: (ms) => new Promise((resolve) => setTimeout(resolve, Math.min(ms, 1))),
      },
      webSocketFactory: (url) => {
        socketsOpened.push(url);
        return {
          readyState: 1,
          close: vi.fn(),
          addEventListener: () => undefined,
          removeEventListener: () => undefined,
        };
      },
    });

    // reconnect before start: baseline once, then fence/replace selected only.
    await runtime.reconnect("a");
    expect(runtime.startedWorkerCount()).toBe(3); // 2 baseline + 1 reconnect for a
    // Later start must not duplicate workers.
    runtime.start();
    runtime.start();
    expect(runtime.startedWorkerCount()).toBe(3);
    expect(runtime.enabledWorkerCount()).toBe(2);

    // After stop, start may re-arm baseline once (preserve stop semantics).
    runtime.stop();
    expect(runtime.liveHandleCounts()).toEqual({ timers: 0, fetches: 0, sockets: 0 });
    runtime.start();
    expect(runtime.startedWorkerCount()).toBe(5); // prior 3 + 2 re-armed baseline
    runtime.start();
    expect(runtime.startedWorkerCount()).toBe(5);
    runtime.stop();
  });

  it("reconnects only the selected server, fences stale callbacks, and preserves peers", async () => {
    type Waiter = { at: number; resolve: () => void };
    let nowMs = 0;
    const waiters: Waiter[] = [];
    const clock = {
      now: () => nowMs,
      sleep: (ms: number) =>
        new Promise<void>((resolve) => {
          waiters.push({ at: nowMs + ms, resolve });
        }),
    };
    const advance = async (ms: number) => {
      nowMs += ms;
      const due = waiters.filter((waiter) => waiter.at <= nowMs).sort((a, b) => a.at - b.at);
      for (const waiter of due) {
        const index = waiters.indexOf(waiter);
        if (index >= 0) waiters.splice(index, 1);
        waiter.resolve();
      }
      await Promise.resolve();
      await Promise.resolve();
    };

    const messageHandlers = new Map<string, (event: Event) => void>();
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("/api/ws-ticket")) {
        return Response.json({ ticket: "ticket" });
      }
      return new Response("missing", { status: 404 });
    }) as unknown as typeof fetch;

    const runtime = new FleetSessionRuntime(servers.slice(0, 2), {
      fetchFn,
      clock,
      hostInstanceId: "host-reconnect-select",
      random: () => 0,
      webSocketFactory: (url) => {
        const serverId = url.includes("://a") ? "a" : "b";
        return {
          readyState: 1,
          close: vi.fn(),
          addEventListener: (type: string, handler: EventListener) => {
            if (type === "message") {
              messageHandlers.set(serverId, handler as (event: Event) => void);
            }
          },
          removeEventListener: () => undefined,
        };
      },
    });
    const healthListener = vi.fn();
    const sessionListener = vi.fn();
    runtime.subscribeHealth(healthListener);
    runtime.subscribe(sessionListener);
    runtime.start();
    const workersAtStart = runtime.startedWorkerCount();
    expect(workersAtStart).toBe(2);

    await vi.waitFor(() => expect(messageHandlers.size).toBe(2));
    messageHandlers.get("a")?.(
      new MessageEvent("message", {
        data: JSON.stringify({ k: "h", sessions: [{ id: "sa" }] }),
      }),
    );
    messageHandlers.get("b")?.(
      new MessageEvent("message", {
        data: JSON.stringify({ k: "h", sessions: [{ id: "sb" }] }),
      }),
    );
    await vi.waitFor(() => {
      const latest = healthListener.mock.calls.at(-1)?.[0];
      expect(latest?.servers.find((s: { serverId: string }) => s.serverId === "a")?.observation
        .kind).toBe("connected");
      expect(latest?.servers.find((s: { serverId: string }) => s.serverId === "b")?.observation
        .kind).toBe("connected");
    });

    const genBeforeReconnect = healthListener.mock.calls.at(-1)?.[0]?.revision as number;
    messageHandlers.clear();
    await runtime.reconnect("a");
    expect(runtime.startedWorkerCount()).toBe(workersAtStart + 1);

    await vi.waitFor(() => {
      const latest = healthListener.mock.calls.at(-1)?.[0];
      expect(latest?.revision).toBeGreaterThan(genBeforeReconnect);
      expect(
        latest?.servers.find((s: { serverId: string }) => s.serverId === "a")?.observation.kind,
      ).toBe("checking");
    });

    // Stale generation must not overwrite the reconnected server or peer state.
    expect(
      runtime.publishHealth("a", 0, {
        kind: "unavailable",
        observedAt: new Date(nowMs).toISOString(),
        reason: "connection_failed",
      }),
    ).toBe(false);

    await vi.waitFor(() => expect(messageHandlers.has("a")).toBe(true));
    messageHandlers.get("a")?.(
      new MessageEvent("message", {
        data: JSON.stringify({ k: "h", sessions: [{ id: "sa-new" }] }),
      }),
    );
    await vi.waitFor(() => {
      const latest = healthListener.mock.calls.at(-1)?.[0];
      expect(
        latest?.servers.find((s: { serverId: string }) => s.serverId === "a")?.observation.kind,
      ).toBe("connected");
      // Peer B must remain connected — reconnect is not fleet-wide.
      expect(
        latest?.servers.find((s: { serverId: string }) => s.serverId === "b")?.observation.kind,
      ).toBe("connected");
    });
    const sessions = sessionListener.mock.calls.at(-1)?.[0]?.sessions ?? [];
    expect(sessions.some((s: { ref: { sessionId: string } }) => s.ref.sessionId === "sb")).toBe(
      true,
    );
    runtime.stop();
    void advance;
  });

  it("resets terminal protocol mismatch so a fresh worker can retry", async () => {
    let protocol = "protocol-1";
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: protocol });
      }
      if (url.includes("/api/ws-ticket")) {
        return Response.json({ ticket: "t" });
      }
      return new Response("missing", { status: 404 });
    }) as unknown as typeof fetch;
    let messageHandler: ((event: Event) => void) | null = null;
    const runtime = new FleetSessionRuntime(servers.slice(0, 1), {
      fetchFn,
      hostInstanceId: "host-reconnect-terminal",
      clock: {
        now: () => Date.now(),
        sleep: (ms) => new Promise((resolve) => setTimeout(resolve, Math.min(ms, 1))),
      },
      webSocketFactory: () => ({
        readyState: 1,
        close: vi.fn(),
        addEventListener: (type: string, handler: EventListener) => {
          if (type === "message") messageHandler = handler as (event: Event) => void;
        },
        removeEventListener: () => undefined,
      }),
    });
    const listener = vi.fn();
    runtime.subscribeHealth(listener);
    runtime.start();
    await vi.waitFor(() => {
      expect(listener.mock.calls.at(-1)?.[0]?.servers[0]?.observation.kind).toBe(
        "incompatible_protocol",
      );
    });
    const workersBefore = runtime.startedWorkerCount();
    protocol = "protocol-3";
    await runtime.reconnect("a");
    expect(runtime.startedWorkerCount()).toBe(workersBefore + 1);
    await vi.waitFor(() => expect(messageHandler).not.toBeNull());
    messageHandler?.(
      new MessageEvent("message", {
        data: JSON.stringify({ k: "h", sessions: [] }),
      }),
    );
    await vi.waitFor(() => {
      expect(listener.mock.calls.at(-1)?.[0]?.servers[0]?.observation.kind).toBe("connected");
    });
    runtime.stop();
  });
});
