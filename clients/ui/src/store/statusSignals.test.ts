import { beforeEach, describe, expect, it } from "vitest";
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
): SessionInfo {
  return {
    id: "s1",
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
});
