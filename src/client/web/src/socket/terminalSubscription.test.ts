import { describe, expect, it, vi } from "vitest";
import {
  TerminalSubscriptionController,
  type TerminalSubscriptionTransport,
} from "./terminalSubscription";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

function fakeTransport(
  subscribe: TerminalSubscriptionTransport["subscribe"] = async () => ({
    status: "confirmed",
    reqId: "r1",
  }),
) {
  const log: string[] = [];
  return {
    log,
    transport: {
      subscribe: vi.fn(async (sessionId: string) => {
        log.push(`subscribe:${sessionId}`);
        return subscribe(sessionId);
      }),
      unsubscribe: vi.fn(async (sessionId: string) => {
        log.push(`unsubscribe:${sessionId}`);
      }),
    } satisfies TerminalSubscriptionTransport,
  };
}

async function flush(): Promise<void> {
  for (let i = 0; i < 8; i += 1) await Promise.resolve();
}

describe("TerminalSubscriptionController", () => {
  it("retries a desired session after a frame-not-ready burst is exhausted", async () => {
    const cooldown = deferred<void>();
    let calls = 0;
    const { transport } = fakeTransport(async () => {
      calls += 1;
      return calls === 1
        ? { status: "exhausted", lastError: "frame-not-ready" }
        : { status: "confirmed", reqId: "r2" };
    });
    const controller = new TerminalSubscriptionController(transport, {
      cooldown: () => cooldown.promise,
    });

    controller.onOpen();
    controller.acquire("s1");
    await flush();
    expect(controller.snapshot()).toMatchObject({ sessionId: "s1", phase: "waiting" });

    cooldown.resolve();
    await flush();
    expect(calls).toBe(2);
    expect(controller.snapshot()).toMatchObject({ sessionId: "s1", phase: "confirmed" });
  });

  it("re-subscribes the desired session after a disconnect during subscribe", async () => {
    const first = deferred<{ status: "exhausted"; lastError: string }>();
    let calls = 0;
    const { transport } = fakeTransport(async () => {
      calls += 1;
      if (calls === 1) return first.promise;
      return { status: "confirmed", reqId: "r2" };
    });
    const controller = new TerminalSubscriptionController(transport);

    controller.onOpen();
    controller.acquire("s1");
    await flush();
    controller.onClose();
    first.resolve({ status: "exhausted", lastError: "connection-closed" });
    await flush();
    expect(controller.snapshot().phase).toBe("disconnected");

    controller.onOpen();
    await flush();
    expect(calls).toBe(2);
    expect(controller.snapshot().phase).toBe("confirmed");
  });

  it("hands the same session to a new owner without unsubscribe", async () => {
    const { transport, log } = fakeTransport();
    const controller = new TerminalSubscriptionController(transport);
    controller.onOpen();
    const oldLease = controller.acquire("s1");
    await flush();

    oldLease.release();
    controller.acquire("s1");
    await flush();

    expect(log).toEqual(["subscribe:s1"]);
    expect(controller.snapshot().phase).toBe("confirmed");
  });

  it("serializes a session switch as unsubscribe then subscribe", async () => {
    const { transport, log } = fakeTransport();
    const controller = new TerminalSubscriptionController(transport);
    controller.onOpen();
    const lease = controller.acquire("s1");
    await flush();

    lease.release();
    controller.acquire("s2");
    await flush();

    expect(log).toEqual(["subscribe:s1", "unsubscribe:s1", "subscribe:s2"]);
  });

  it("interrupts frame-not-ready cooldown when the desired session changes", async () => {
    const never = new Promise<void>(() => {});
    const { transport, log } = fakeTransport(async (sessionId) =>
      sessionId === "s1"
        ? { status: "exhausted", lastError: "frame-not-ready" }
        : { status: "confirmed", reqId: "r2" },
    );
    const controller = new TerminalSubscriptionController(transport, { cooldown: () => never });
    controller.onOpen();
    const lease = controller.acquire("s1");
    await flush();
    expect(controller.snapshot().phase).toBe("waiting");

    lease.release();
    controller.acquire("s2");
    await flush();

    expect(log).toEqual(["subscribe:s1", "subscribe:s2"]);
    expect(controller.snapshot()).toMatchObject({ sessionId: "s2", phase: "confirmed" });
  });

  it("blocks permanent errors until the desired session is explicitly reacquired", async () => {
    let calls = 0;
    const { transport } = fakeTransport(async () => {
      calls += 1;
      return calls === 1
        ? { status: "exhausted", lastError: "unauthorized" }
        : { status: "confirmed", reqId: "r2" };
    });
    const controller = new TerminalSubscriptionController(transport);
    controller.onOpen();
    controller.acquire("s1");
    await flush();
    expect(controller.snapshot()).toMatchObject({ phase: "blocked", lastError: "unauthorized" });

    controller.acquire("s1");
    await flush();
    expect(calls).toBe(2);
    expect(controller.snapshot().phase).toBe("confirmed");
  });
});
