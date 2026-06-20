import { beforeEach, describe, expect, it, vi } from "vitest";
import { Connection } from "./connection";
import { useDaemonStore } from "../store/daemon";

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

    // simulate confirmed subscribe by calling subscribe and responding OK
    const subPromise = conn.subscribe("s1");
    // first ws receives the subscribe frame
    expect(ws1.sent.some((s) => s.includes('"k":"s"'))).toBe(true);
    // server responds OK
    const sentSubFrame = JSON.parse(ws1.sent.find((s) => s.includes('"k":"s"')) ?? "{}") as {
      reqId: string;
    };
    ws1.receive(JSON.stringify({ k: "r", reqId: sentSubFrame.reqId }));
    await subPromise;

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
    expect(ws2.sent.some((s) => s.includes('"k":"s"'))).toBe(true);
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
});
