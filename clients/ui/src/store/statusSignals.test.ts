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

  it("suppresses a replayed bell for the same session and epoch on reconnect", () => {
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

    // The replayed hello reaches this module through Connection, which
    // requalifies it, so both sides speak the same fleet key. This case used to
    // pass a bare id and rely on the module re-deriving an identity from three
    // possible sources; the suppression it pins is the same, but the identity is
    // now the row id and nothing else.
    const helloRequalified = {
      ...session("e1", 1, "idle", fleetId),
      serverId: "srv-a",
      upstreamSessionId: "s1",
      status: {
        lifecycle: "idle" as const,
        human_input: true,
        failure: "none" as const,
        signal: { kind: "bell" as const, epoch: "e1", sequence: 1 },
      },
    };
    expect(seedStatusSignals([helloRequalified])[0]?.status.signal).toBeUndefined();
    expect(consumeStatusSignals([membership])[0]?.status.signal).toBeUndefined();
  });

  it("keeps two servers' sessions apart when they share an upstream id", () => {
    // Two gateways can hand out the same session id. Before rows carried their
    // server, identity was re-derived from whichever of three fields looked
    // most qualified, so these two could collapse onto one consume entry and
    // one session's history would silence the other's.
    const row = (serverId: string, sequence: number) => ({
      ...session("e1", sequence, "idle", JSON.stringify([serverId, "s1"])),
      serverId,
      upstreamSessionId: "s1",
      status: {
        lifecycle: "idle" as const,
        human_input: true,
        failure: "none" as const,
        signal: { kind: "bell" as const, epoch: "e1", sequence },
      },
    });

    // srv-a advances to sequence 2.
    expect(consumeStatusSignals([row("srv-a", 2)])[0]?.status.signal).toBeDefined();
    // srv-b is a different session and has consumed nothing, so its first bell
    // is still observable. A shared entry would have swallowed it.
    expect(consumeStatusSignals([row("srv-b", 1)])[0]?.status.signal).toBeDefined();
    // srv-a's older sequence is a replay for srv-a alone.
    expect(consumeStatusSignals([row("srv-a", 1)])[0]?.status.signal).toBeUndefined();
  });
});
