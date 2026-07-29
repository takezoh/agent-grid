import { beforeEach, describe, expect, it } from "vitest";
import { sessionKey } from "../fleet/session-protocol";
import type { SessionInfo } from "../wire/server";
import {
  consumeStatusSignals,
  resetConsumedStatusSignals,
  seedStatusSignals,
} from "./statusSignals";

function session(
  epoch: string,
  sequence: number,
  lifecycle: "active" | "idle" = "idle",
  id = "s1",
): SessionInfo {
  return {
    id,
    project: "p",
    command: "codex",
    created_at: "2026-07-26T00:00:00Z",
    status: {
      lifecycle,
      human_input: false,
      failure: "none",
      signal: { kind: "bell", epoch, sequence },
    },
    view: { card: {} },
  };
}

describe("consumeStatusSignals", () => {
  beforeEach(resetConsumedStatusSignals);

  it("keeps a sequence visible during one connection and suppresses reconnect replay", () => {
    expect(consumeStatusSignals([session("e1", 1)])[0]?.status.signal).toBeDefined();
    expect(consumeStatusSignals([session("e1", 1)])[0]?.status.signal).toBeDefined();
    expect(seedStatusSignals([session("e1", 1)])[0]?.status.signal).toBeUndefined();
    expect(consumeStatusSignals([session("e1", 1)])[0]?.status.signal).toBeUndefined();
  });

  it("accepts a newer sequence and a new server epoch", () => {
    consumeStatusSignals([session("e1", 1)]);
    expect(consumeStatusSignals([session("e1", 2)])[0]?.status.signal?.sequence).toBe(2);
    expect(consumeStatusSignals([session("e2", 1)])[0]?.status.signal?.epoch).toBe("e2");
  });

  it("defers a turn-complete bell until the following idle frame", () => {
    expect(consumeStatusSignals([session("e1", 1, "active")])[0]?.status.signal).toBeUndefined();
    expect(consumeStatusSignals([session("e1", 1, "idle")])[0]?.status.signal).toBeDefined();
    expect(consumeStatusSignals([session("e1", 1, "idle")])[0]?.status.signal).toBeDefined();
  });

  it("matches bare wire ids with fleet-qualified membership keys on reconnect", () => {
    const fleetId = sessionKey({ serverId: "srv-a", sessionId: "s1" });
    const membership = {
      ...session("e1", 1, "idle", fleetId),
      upstreamSessionId: "s1",
      serverId: "srv-a",
      status: {
        lifecycle: "idle" as const,
        human_input: true,
        failure: "none" as const,
        signal: { kind: "bell" as const, epoch: "e1", sequence: 1 },
      },
    };
    expect(consumeStatusSignals([membership])[0]?.status.signal).toBeDefined();

    // Hello payload still uses the bare upstream id (+ serverId tag).
    const helloBare = {
      ...session("e1", 1, "idle", "s1"),
      serverId: "srv-a",
      status: {
        lifecycle: "idle" as const,
        human_input: true,
        failure: "none" as const,
        signal: { kind: "bell" as const, epoch: "e1", sequence: 1 },
      },
    };
    expect(seedStatusSignals([helloBare])[0]?.status.signal).toBeUndefined();
    expect(consumeStatusSignals([membership])[0]?.status.signal).toBeUndefined();
  });
});
