import { describe, expect, it, vi } from "vitest";
import type { TerminalSubscriptionPhase } from "../store/subscriptions";
import {
  TerminalSubscriptionController,
  type TerminalSubscriptionTransport,
} from "./terminalSubscription";

async function flush(): Promise<void> {
  for (let i = 0; i < 8; i += 1) await Promise.resolve();
}

describe("TerminalSubscriptionController severance", () => {
  it("re-subscribes after server-initiated surface severance without new phases", async () => {
    let calls = 0;
    const transport: TerminalSubscriptionTransport = {
      subscribe: vi.fn(async () => {
        calls += 1;
        return { status: "confirmed", reqId: `r${calls}` };
      }),
      unsubscribe: vi.fn(async () => {}),
    };
    const controller = new TerminalSubscriptionController(transport);

    controller.onOpen();
    controller.acquire("s1");
    controller.updateGeometry("s1", 120, 40);
    await flush();
    expect(controller.snapshot()).toMatchObject({ phase: "confirmed", sessionId: "s1" });

    controller.onSurfaceSevered("s1");
    await flush();

    expect(calls).toBe(2);
    expect(controller.snapshot()).toMatchObject({ phase: "confirmed", sessionId: "s1" });
    const phases: TerminalSubscriptionPhase[] = [
      "idle",
      "subscribing",
      "confirmed",
      "waiting",
      "blocked",
      "disconnected",
    ];
    expect(phases).toContain(controller.snapshot().phase);
  });
});
