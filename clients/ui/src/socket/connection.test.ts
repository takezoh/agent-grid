import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { useFrameMessagingStore } from "../store/frameMessaging";
import { useNotificationsStore } from "../store/notifications";
import { useSubscriptionStore } from "../store/subscriptions";
import { useTranscriptStore } from "../store/transcripts";
import { Connection } from "./connection";

class FakeWS {
  static instances: FakeWS[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  sent: string[] = [];
  url: string;
  constructor(url: string) {
    this.url = url;
    FakeWS.instances.push(this);
  }
  send(d: string) {
    this.sent.push(d);
  }
  close() {
    this.onclose?.();
  }
  open() {
    this.onopen?.();
  }
  receive(raw: string) {
    this.onmessage?.({ data: raw });
  }
}

describe("Connection", () => {
  beforeEach(() => {
    FakeWS.instances = [];
    useDaemonStore.getState().reset();
    useFrameMessagingStore.getState().reset();
    useTranscriptStore.getState().reset();
    useNotificationsStore.getState().clear();
    useSubscriptionStore.getState().reset();
  });

  it("starts → fetches ticket → opens ws → sets status open", async () => {
    const fetchFn = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ ticket: "tkt" }),
    })) as unknown as typeof fetch;
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: (t) => `ws://h/ws?ticket=${t}`,
      bearerToken: "tok",
      wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
      sleep: async () => {},
      fetchFn,
    });
    await conn.start();
    FakeWS.instances[0]?.open();
    expect(useDaemonStore.getState().status).toBe("open");
  });

  it("reconnects on close and re-sends active subscriptions", async () => {
    const fetchFn = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ ticket: "tkt" }),
    })) as unknown as typeof fetch;
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: (t) => `ws://h/ws?ticket=${t}`,
      bearerToken: "tok",
      wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
      sleep: async () => {},
      fetchFn,
    });
    await conn.start();
    const ws1 = FakeWS.instances[0];
    if (!ws1) throw new Error("expected ws1");
    ws1.open();

    // Acquire the desired terminal and confirm its first subscribe.
    conn.acquireTerminal("s1");
    conn.updateTerminalGeometry("s1", 120, 40);
    expect(useSubscriptionStore.getState()).toMatchObject({ sessionId: "s1" });
    await Promise.resolve();
    // first ws receives the subscribe frame
    expect(ws1.sent.some((s) => s.includes('"k":"ld"'))).toBe(true);
    // server responds OK
    const sentSubFrame = JSON.parse(ws1.sent.find((s) => s.includes('"k":"ld"')) ?? "{}") as {
      reqId: string;
    };
    ws1.receive(JSON.stringify({ k: "r", reqId: sentSubFrame.reqId }));
    await Promise.resolve();

    // close → reconnect path
    ws1.close();
    // allow microtasks and the reconnect sleep (sleep: async () => {}) to resolve
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    const ws2 = FakeWS.instances[1];
    if (!ws2) throw new Error("expected ws2 after reconnect");
    ws2.open();
    // active subscription was re-sent on new socket
    expect(ws2.sent.some((s) => s.includes('"k":"ld"'))).toBe(true);
  });

  it("control frame daemon-disconnected sets store flag", async () => {
    const fetchFn = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ ticket: "tkt" }),
    })) as unknown as typeof fetch;
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: (t) => `ws://h/ws?ticket=${t}`,
      bearerToken: "tok",
      wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
      sleep: async () => {},
      fetchFn,
    });
    await conn.start();
    const ws = FakeWS.instances[0];
    ws?.open();
    // ControlFrame: k="c", data carries the event payload string
    ws?.receive(JSON.stringify({ k: "c", data: "daemon-disconnected" }));
    expect(useDaemonStore.getState().daemonDisconnected).toBe(true);
  });

  it("close() stops reconnect and clears registry", async () => {
    const fetchFn = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ ticket: "tkt" }),
    })) as unknown as typeof fetch;
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: (t) => `ws://h/ws?ticket=${t}`,
      bearerToken: "tok",
      wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
      sleep: async () => {},
      fetchFn,
    });
    await conn.start();
    FakeWS.instances[0]?.open();
    conn.close();
    expect(useDaemonStore.getState().status).toBe("closed");
    // no new ws should be created after user-initiated close
    const countBefore = FakeWS.instances.length;
    await Promise.resolve();
    await Promise.resolve();
    expect(FakeWS.instances.length).toBe(countBefore);
  });

  it("onerror followed by onclose fires reconnect only once (not twice)", async () => {
    // Real browsers fire onerror → onclose sequentially. Reconnect must not double-trigger.
    let fetchCallCount = 0;
    const fetchFn = vi.fn(async () => {
      fetchCallCount += 1;
      return {
        ok: true,
        status: 200,
        json: async () => ({ ticket: "tkt" }),
      };
    }) as unknown as typeof fetch;
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: (t) => `ws://h/ws?ticket=${t}`,
      bearerToken: "tok",
      wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
      sleep: async () => {},
      fetchFn,
    });
    await conn.start();
    const ws1 = FakeWS.instances[0];
    if (!ws1) throw new Error("expected ws1");
    ws1.open();
    // reset count after initial connect
    fetchCallCount = 0;

    // Simulate browser firing onerror then onclose in sequence
    ws1.onerror?.();
    ws1.onclose?.();

    // Allow microtasks (sleep is noop, so reconnect happens in next microtask tick)
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();

    // fetchFn should have been called exactly once for the reconnect attempt
    expect(fetchCallCount).toBe(1);
    // Only one new WebSocket instance created
    expect(FakeWS.instances.length).toBe(2);
  });

  it("re-subscribes desired terminal when WS closes during the initial subscribe", async () => {
    const fetchFn = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ ticket: "tkt" }),
    })) as unknown as typeof fetch;
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: (t) => `ws://h/ws?ticket=${t}`,
      bearerToken: "tok",
      wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
      sleep: async () => {},
      fetchFn,
    });
    await conn.start();
    const ws1 = FakeWS.instances[0];
    if (!ws1) throw new Error("expected ws1");
    ws1.open();

    // Start subscribe — server never responds, so awaitResponse hangs unless drained
    conn.acquireTerminal("s1");
    conn.updateTerminalGeometry("s1", 120, 40);

    // Close the WS before server responds — pending promise must resolve, not hang
    ws1.onclose?.();
    expect(useSubscriptionStore.getState()).toMatchObject({
      sessionId: "s1",
      phase: "disconnected",
    });

    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    const ws2 = FakeWS.instances[1];
    if (!ws2) throw new Error("expected reconnect socket");
    ws2.open();
    await Promise.resolve();
    expect(ws2.sent.some((s) => s.includes('"k":"ld"') && s.includes('"sessionId":"s1"'))).toBe(
      true,
    );
  });

  // Helper to create a connected FakeWS via Connection
  async function makeConnectedWS() {
    const fetchFn = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ ticket: "tkt" }),
    })) as unknown as typeof fetch;
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: (t) => `ws://h/ws?ticket=${t}`,
      bearerToken: "tok",
      wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
      sleep: async () => {},
      fetchFn,
    });
    await conn.start();
    const ws = FakeWS.instances[FakeWS.instances.length - 1];
    if (!ws) throw new Error("expected FakeWS");
    ws.open();
    return { conn, ws };
  }

  it("TestDispatchesTranscriptTail: tt frame appends to transcript buffer", async () => {
    const { ws } = await makeConnectedWS();
    ws.receive(JSON.stringify({ k: "tt", sessionId: "s1", line: "a" }));
    const buf = useTranscriptStore.getState().buffers["s1:transcript"];
    expect(buf?.lines).toEqual(["a"]);
  });

  it("TestDispatchesEventLogTail: et frame appends to event-log buffer", async () => {
    const { ws } = await makeConnectedWS();
    ws.receive(JSON.stringify({ k: "et", sessionId: "s1", line: "b" }));
    const buf = useTranscriptStore.getState().buffers["s1:event-log"];
    expect(buf?.lines).toEqual(["b"]);
  });

  it("TestDispatchesNotification: n frame adds notification to store", async () => {
    const { ws } = await makeConnectedWS();
    ws.receive(JSON.stringify({ k: "n", sessionId: "s1", cmd: 9, title: "t", nowMs: 1 }));
    expect(useNotificationsStore.getState().items.length).toBe(1);
    expect(useNotificationsStore.getState().items[0]?.title).toBe("t");
  });

  it("TestUnsupportedKindIgnored: unknown frame kind does not throw", async () => {
    const { ws } = await makeConnectedWS();
    expect(() => ws.receive(JSON.stringify({ k: "zz", data: "unknown" }))).not.toThrow();
  });

  it("brokers frame messaging summaries from hello/view-update into the dedicated store", async () => {
    const { ws } = await makeConnectedWS();
    ws.receive(
      JSON.stringify({
        k: "h",
        sessions: [
          {
            id: "s1",
            project: "/repo/app",
            command: "codex",
            created_at: "2026-07-06T00:00:00Z",
            view: {
              card: { title: "Agent" },
              frame_messaging_summary: {
                unread_count: 2,
                latest_message_preview: "Review this",
                pending_delivery_count: 1,
                last_delivery_status: "pending",
              },
            },
          },
        ],
        features: ["surface"],
        serverTime: 1,
      }),
    );
    expect(useFrameMessagingStore.getState().summaries).toEqual({
      s1: {
        unreadCount: 2,
        latestMessagePreview: "Review this",
        pendingDeliveryCount: 1,
        lastDeliveryStatus: "pending",
      },
    });

    ws.receive(
      JSON.stringify({
        k: "v",
        sessions: [
          {
            id: "s1",
            project: "/repo/app",
            command: "codex",
            created_at: "2026-07-06T00:00:00Z",
            view: {
              card: { title: "Agent" },
            },
          },
        ],
      }),
    );
    expect(useFrameMessagingStore.getState().summaries).toEqual({});
  });
});
