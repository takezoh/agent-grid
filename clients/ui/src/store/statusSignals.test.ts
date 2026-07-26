import { beforeEach, describe, expect, it } from "vitest";
import type { SessionInfo } from "../wire/server";
import { consumeStatusSignals, resetConsumedStatusSignals } from "./statusSignals";

function session(epoch: string, sequence: number): SessionInfo {
  return {
    id: "s1",
    project: "p",
    command: "codex",
    created_at: "2026-07-26T00:00:00Z",
    status: {
      lifecycle: "idle",
      human_input: false,
      failure: "none",
      signal: { kind: "bell", epoch, sequence },
    },
    view: { card: {} },
  };
}

describe("consumeStatusSignals", () => {
  beforeEach(resetConsumedStatusSignals);

  it("shows a sequence once and suppresses reconnect replay", () => {
    expect(consumeStatusSignals([session("e1", 1)])[0]?.status.signal).toBeDefined();
    expect(consumeStatusSignals([session("e1", 1)])[0]?.status.signal).toBeUndefined();
  });

  it("accepts a newer sequence and a new server epoch", () => {
    consumeStatusSignals([session("e1", 1)]);
    expect(consumeStatusSignals([session("e1", 2)])[0]?.status.signal?.sequence).toBe(2);
    expect(consumeStatusSignals([session("e2", 1)])[0]?.status.signal?.epoch).toBe("e2");
  });
});
