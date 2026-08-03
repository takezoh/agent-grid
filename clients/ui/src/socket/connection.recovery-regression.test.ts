import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { Connection } from "./connection";

class RecoveryWS {
  static instances: RecoveryWS[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  readyState = 0;
  closed = false;
  url: string;

  constructor(url = "ws://gateway/ws") {
    this.url = url;
    RecoveryWS.instances.push(this);
  }

  send(): void {}

  close(): void {
    if (this.closed) return;
    this.closed = true;
    this.readyState = 3;
    this.onclose?.();
  }

  open(): void {
    this.readyState = 1;
    this.onopen?.();
  }

  receive(raw: string): void {
    this.onmessage?.({ data: raw });
  }
}

const helloPayload = JSON.stringify({
  k: "h",
  sessions: [],
  features: [],
  serverTime: 1,
});

function makeConn(opts?: {
  fetchFn?: typeof fetch;
  sleep?: (ms: number) => Promise<void>;
}): Connection {
  return new Connection({
    ticketEndpoint: "/api/ws-ticket",
    wsUrl: (t) => `ws://gateway/ws?ticket=${t}`,
    bearerToken: "token",
    fetchFn:
      opts?.fetchFn ??
      (vi.fn(async () => ({
        ok: true,
        status: 200,
        json: async () => ({ ticket: "ticket" }),
      })) as unknown as typeof fetch),
    wsFactory: (u) => new RecoveryWS(u) as unknown as WebSocket,
    sleep: opts?.sleep ?? (async () => {}),
  });
}

async function flush(): Promise<void> {
  for (let i = 0; i < 12; i += 1) await Promise.resolve();
}

/** Drive exactly `count` failed attempts (open → close, no hello). */
async function failAttempts(count: number): Promise<void> {
  for (let i = 0; i < count; i += 1) {
    const n = RecoveryWS.instances.length;
    await vi.waitFor(() =>
      expect(RecoveryWS.instances.length).toBeGreaterThan(i === 0 ? 0 : n - 1),
    );
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBeGreaterThan(i));
    const socket = RecoveryWS.instances[i];
    socket?.open();
    socket?.close();
    await flush();
  }
}

describe("Connection daemon recovery regressions", () => {
  beforeEach(() => {
    RecoveryWS.instances = [];
    useDaemonStore.getState().reset();
  });

  // rejects counterexample: open-on-socket Connected interval (UAC-002) —
  // status must stay non-nominal until current-attempt daemon hello.
  it("bounds automatic retries at initiating+16, supports reusable manual retry, and opens only on hello", async () => {
    const conn = makeConn();

    await conn.start();
    // initiating + 16 automatic = 17 sockets; no 18th automatic
    await failAttempts(17);

    await vi.waitFor(() => expect(useDaemonStore.getState().status).toBe("closed"));
    expect(RecoveryWS.instances).toHaveLength(17);

    // No 17th automatic retry is scheduled after exhaustion.
    await flush();
    expect(RecoveryWS.instances).toHaveLength(17);

    conn.retry();
    await vi.waitFor(() => expect(RecoveryWS.instances).toHaveLength(18));
    const recovered = RecoveryWS.instances[17];
    recovered?.open();
    expect(useDaemonStore.getState().status).not.toBe("open");
    recovered?.receive(JSON.stringify({ k: "c", data: "daemon-disconnected" }));
    expect(useDaemonStore.getState().daemonDisconnected).toBe(true);
    recovered?.receive(helloPayload);

    expect(useDaemonStore.getState()).toMatchObject({
      status: "open",
      daemonDisconnected: false,
    });

    // Manual lifecycle can exhaust and Retry remains reusable (UAC-011).
    recovered?.close();
    await flush();
    await failAttemptsFrom(18, 16);
    await vi.waitFor(() => expect(useDaemonStore.getState().status).toBe("closed"));
    // 18 (retry initiate) + 16 automatic = 34 total from start of second lifecycle's
    // remaining fails after the recovered socket already closed above.
    // recovered was index 17; its close starts auto retries → 16 more sockets.
    expect(RecoveryWS.instances.length).toBe(17 + 1 + 16);

    conn.retry();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(17 + 1 + 16 + 1));
    expect(useDaemonStore.getState().status).toBe("connecting");
  });

  it("does not publish open on socket open or ticket success without hello", async () => {
    const conn = makeConn();
    await conn.start();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(1));
    expect(useDaemonStore.getState().status).toBe("connecting");

    RecoveryWS.instances[0]?.open();
    expect(useDaemonStore.getState().status).toBe("connecting");

    RecoveryWS.instances[0]?.receive(helloPayload);
    expect(useDaemonStore.getState().status).toBe("open");
  });

  // rejects counterexample: late hello from attempt n after n+1 started mutates store
  it("fences late close/hello from retired attempts and generations", async () => {
    const sleepResolvers: Array<() => void> = [];
    const conn = makeConn({
      sleep: () =>
        new Promise<void>((resolve) => {
          sleepResolvers.push(resolve);
        }),
    });

    await conn.start();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(1));
    const attempt1 = RecoveryWS.instances[0];
    if (!attempt1) throw new Error("expected attempt1");
    attempt1.open();
    attempt1.receive(helloPayload);
    expect(useDaemonStore.getState().status).toBe("open");

    // Close established connection → schedules automatic retry (sleep pending).
    attempt1.close();
    expect(useDaemonStore.getState().status).toBe("reconnecting");
    expect(sleepResolvers.length).toBe(1);

    // Fire scheduled retry → attempt 2 starts.
    sleepResolvers[0]?.();
    await flush();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(2));
    const attempt2 = RecoveryWS.instances[1];
    if (!attempt2) throw new Error("expected attempt2");

    // Stale attempt1 hello must not open / reset accounting.
    useDaemonStore.getState().setStatus("reconnecting");
    attempt1.receive(helloPayload);
    expect(useDaemonStore.getState().status).toBe("reconnecting");

    // Stale attempt1 close must not schedule a competing retry.
    const socketsBefore = RecoveryWS.instances.length;
    attempt1.onclose?.();
    await flush();
    sleepResolvers[1]?.();
    await flush();
    expect(RecoveryWS.instances.length).toBe(socketsBefore);

    // Current attempt2 hello recovers.
    attempt2.open();
    attempt2.receive(helloPayload);
    expect(useDaemonStore.getState().status).toBe("open");
  });

  it("Retry is single-flight: repeated activation while active is a no-op", async () => {
    const conn = makeConn();
    await conn.start();
    await failAttempts(17);
    await vi.waitFor(() => expect(useDaemonStore.getState().status).toBe("closed"));
    expect(RecoveryWS.instances).toHaveLength(17);

    conn.retry();
    conn.retry();
    conn.retry();
    await flush();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(18));
    // Only one new initiating attempt from the triple activation.
    expect(RecoveryWS.instances).toHaveLength(18);
    expect(useDaemonStore.getState().status).toBe("connecting");
  });

  it("close/shutdown fences in-flight ticket and cancels scheduled retry", async () => {
    let releaseTicket: (() => void) | null = null;
    const ticketGate = new Promise<void>((resolve) => {
      releaseTicket = resolve;
    });
    const sleepResolvers: Array<() => void> = [];
    const fetchFn = vi.fn(async () => {
      await ticketGate;
      return {
        ok: true,
        status: 200,
        json: async () => ({ ticket: "ticket" }),
      };
    }) as unknown as typeof fetch;

    const conn = makeConn({
      fetchFn,
      sleep: () =>
        new Promise<void>((resolve) => {
          sleepResolvers.push(resolve);
        }),
    });

    void conn.start();
    await vi.waitFor(() => expect(fetchFn).toHaveBeenCalledTimes(1));
    conn.close();
    expect(useDaemonStore.getState().status).toBe("closed");

    // Late ticket completion must not create a socket or reopen.
    releaseTicket?.();
    await flush();
    expect(RecoveryWS.instances).toHaveLength(0);
    expect(useDaemonStore.getState().status).toBe("closed");

    // Fresh start, open, close → scheduled retry, then shutdown before fire.
    const conn2 = makeConn({
      sleep: () =>
        new Promise<void>((resolve) => {
          sleepResolvers.push(resolve);
        }),
    });
    await conn2.start();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(1));
    RecoveryWS.instances[0]?.open();
    RecoveryWS.instances[0]?.close();
    expect(useDaemonStore.getState().status).toBe("reconnecting");
    expect(sleepResolvers.length).toBeGreaterThan(0);
    const socketsAtSchedule = RecoveryWS.instances.length;
    conn2.close();
    // Fire all pending sleeps — retired work must not create another socket.
    for (const r of sleepResolvers) r();
    await flush();
    expect(RecoveryWS.instances.length).toBe(socketsAtSchedule);
    expect(useDaemonStore.getState().status).toBe("closed");
  });

  it("server-switch style replace: new lifecycle ignores prior socket events", async () => {
    const conn = makeConn();
    await conn.start();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(1));
    const oldSocket = RecoveryWS.instances[0];
    oldSocket?.open();
    oldSocket?.receive(helloPayload);
    expect(useDaemonStore.getState().status).toBe("open");

    // Simulate App effect teardown + remount on serverId change.
    conn.close();
    expect(useDaemonStore.getState().status).toBe("closed");
    await conn.start();
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBe(2));

    // Late events from the retired socket must not affect the new lifecycle.
    oldSocket?.receive(helloPayload);
    oldSocket?.close();
    await flush();
    expect(useDaemonStore.getState().status).toBe("connecting");

    RecoveryWS.instances[1]?.open();
    RecoveryWS.instances[1]?.receive(helloPayload);
    expect(useDaemonStore.getState().status).toBe("open");
  });
});

/** Fail `count` automatic attempts starting from current instances length. */
async function failAttemptsFrom(startIndex: number, count: number): Promise<void> {
  for (let i = 0; i < count; i += 1) {
    const idx = startIndex + i;
    await vi.waitFor(() => expect(RecoveryWS.instances.length).toBeGreaterThan(idx));
    const socket = RecoveryWS.instances[idx];
    socket?.open();
    socket?.close();
    await flush();
  }
}
