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
      subscribe: vi.fn(async (sessionId: string, cols: number, rows: number) => {
        log.push(`subscribe:${sessionId}`);
        return subscribe(sessionId, cols, rows);
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
  it("does not subscribe until fresh fitted geometry is available", async () => {
    const { transport, log } = fakeTransport();
    const controller = new TerminalSubscriptionController(transport);

    controller.onOpen();
    controller.acquire("s1");
    await flush();
    expect(log).toEqual([]);

    controller.updateGeometry("s1", 132, 47);
    await flush();
    expect(transport.subscribe).toHaveBeenCalledWith("s1", 132, 47);
    expect(controller.snapshot().phase).toBe("confirmed");
  });

  it("re-attaches when fitted geometry changes during subscribe", async () => {
    const first = deferred<{ status: "confirmed"; reqId: string }>();
    let calls = 0;
    const { transport } = fakeTransport(async () => {
      calls += 1;
      if (calls === 1) return first.promise;
      return { status: "confirmed", reqId: "r2" };
    });
    const controller = new TerminalSubscriptionController(transport);

    controller.onOpen();
    controller.acquire("s1");
    controller.updateGeometry("s1", 80, 24);
    await flush();
    controller.updateGeometry("s1", 132, 47);
    first.resolve({ status: "confirmed", reqId: "r1" });
    await flush();

    expect(transport.unsubscribe).toHaveBeenCalledWith("s1");
    expect(transport.subscribe).toHaveBeenNthCalledWith(2, "s1", 132, 47);
    expect(controller.snapshot().phase).toBe("confirmed");
  });

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
    controller.updateGeometry("s1", 120, 40);
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
    controller.updateGeometry("s1", 120, 40);
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
    controller.updateGeometry("s1", 120, 40);
    await flush();

    oldLease.release();
    controller.acquire("s1");
    controller.updateGeometry("s1", 120, 40);
    await flush();

    expect(log).toEqual(["subscribe:s1"]);
    expect(controller.snapshot().phase).toBe("confirmed");
  });

  it("serializes a session switch as unsubscribe then subscribe", async () => {
    const { transport, log } = fakeTransport();
    const controller = new TerminalSubscriptionController(transport);
    controller.onOpen();
    const lease = controller.acquire("s1");
    controller.updateGeometry("s1", 120, 40);
    await flush();

    lease.release();
    controller.acquire("s2");
    controller.updateGeometry("s2", 120, 40);
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
    controller.updateGeometry("s1", 120, 40);
    await flush();
    expect(controller.snapshot().phase).toBe("waiting");

    lease.release();
    controller.acquire("s2");
    controller.updateGeometry("s2", 120, 40);
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
    controller.updateGeometry("s1", 120, 40);
    await flush();
    expect(controller.snapshot()).toMatchObject({ phase: "blocked", lastError: "unauthorized" });

    controller.acquire("s1");
    controller.updateGeometry("s1", 120, 40);
    await flush();
    expect(calls).toBe(2);
    expect(controller.snapshot().phase).toBe("confirmed");
  });

  it("keeps the watchdog through an unresponsive publication and requests connection recovery", async () => {
    vi.useFakeTimers();
    try {
      const never = new Promise<{ status: "confirmed"; reqId: string }>(() => {});
      const onDeliveryTimeout = vi.fn();
      const { transport } = fakeTransport(async () => never);
      const controller = new TerminalSubscriptionController(transport, { onDeliveryTimeout });

      controller.onOpen();
      controller.acquire("s1");
      controller.updateGeometry("s1", 120, 40);
      await flush();

      vi.advanceTimersByTime(4000);
      expect(onDeliveryTimeout).toHaveBeenCalledOnce();
      expect(controller.snapshot()).toMatchObject({
        phase: "disconnected",
        lastError: "delivery-timeout",
      });
    } finally {
      vi.useRealTimers();
    }
  });

  // Lease renewal — ADR terminal-lifecycle-bounds mandates a 4s renewal cadence
  // so daemon-side 12s expiry never fires under steady-state operation. Without
  // it, TerminalRelay silently unsubscribes and output stops flowing.
  it("renews the confirmed subscription every 4s to keep the daemon lease alive", async () => {
    vi.useFakeTimers();
    try {
      const { transport } = fakeTransport();
      const controller = new TerminalSubscriptionController(transport);

      controller.onOpen();
      controller.acquire("s1");
      controller.updateGeometry("s1", 120, 40);
      await flush();
      expect(controller.snapshot().phase).toBe("confirmed");
      expect(transport.subscribe).toHaveBeenCalledTimes(1);

      // Renewal must fire every 4s while phase="confirmed" — same session, same
      // geometry, but the daemon needs a fresh correlation to reset the 12s TTL.
      vi.advanceTimersByTime(4000);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(2);
      expect(transport.subscribe).toHaveBeenLastCalledWith("s1", 120, 40);
      expect(controller.snapshot().phase).toBe("confirmed");

      vi.advanceTimersByTime(4000);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(3);

      // Third renewal to prove the loop is unbounded, not one-shot.
      vi.advanceTimersByTime(4000);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(4);
    } finally {
      vi.useRealTimers();
    }
  });

  it("stops the renewal timer when the last lease is released", async () => {
    vi.useFakeTimers();
    try {
      const { transport } = fakeTransport();
      const controller = new TerminalSubscriptionController(transport);
      controller.onOpen();
      const lease = controller.acquire("s1");
      controller.updateGeometry("s1", 120, 40);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(1);

      lease.release();
      await flush();

      vi.advanceTimersByTime(20000);
      await flush();
      // No further subscribe calls after the release — renewal must not resurrect
      // a released subscription.
      expect(transport.subscribe).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("stops the renewal timer on socket close and resumes after re-open", async () => {
    vi.useFakeTimers();
    try {
      const { transport } = fakeTransport();
      const controller = new TerminalSubscriptionController(transport);
      controller.onOpen();
      controller.acquire("s1");
      controller.updateGeometry("s1", 120, 40);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(1);

      controller.onClose();
      await flush();
      vi.advanceTimersByTime(20000);
      await flush();
      // While disconnected, renewal must not fire — it would leak wire frames
      // into a torn-down transport.
      expect(transport.subscribe).toHaveBeenCalledTimes(1);

      controller.onOpen();
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(2);

      vi.advanceTimersByTime(4000);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(3);
    } finally {
      vi.useRealTimers();
    }
  });

  it("forceRenewal triggers an immediate re-subscribe while confirmed", async () => {
    vi.useFakeTimers();
    try {
      const { transport } = fakeTransport();
      const controller = new TerminalSubscriptionController(transport);
      controller.onOpen();
      controller.acquire("s1");
      controller.updateGeometry("s1", 120, 40);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(1);

      // visibilitychange → visible calls forceRenewal to recover from any
      // throttling-induced silent expiry.
      controller.forceRenewal();
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(2);
      expect(controller.snapshot().phase).toBe("confirmed");

      // Renewal cadence resumes from the forced renewal — next automatic renewal
      // is 4s later, not layered on top of the previous timer.
      vi.advanceTimersByTime(4000);
      await flush();
      expect(transport.subscribe).toHaveBeenCalledTimes(3);
    } finally {
      vi.useRealTimers();
    }
  });

  it("forceRenewal is a no-op while not confirmed", async () => {
    const { transport } = fakeTransport();
    const controller = new TerminalSubscriptionController(transport);
    controller.onOpen();
    // Never acquire: forceRenewal must not fabricate a subscription.
    controller.forceRenewal();
    await flush();
    expect(transport.subscribe).not.toHaveBeenCalled();
  });
});
