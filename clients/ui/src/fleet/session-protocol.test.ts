import { describe, expect, it } from "vitest";
import type { SessionInfo } from "../wire/server";
import {
  type FleetMember,
  initialFleetSnapshot,
  reduceFleetMember,
  sessionKey,
} from "./session-protocol";

function session(id: string): SessionInfo {
  return {
    id,
    project: "/work",
    command: "codex",
    created_at: "2026-07-27T00:00:00Z",
    status: { lifecycle: "idle", human_input: false, failure: "none" },
    view: { card: { title: id } },
  };
}

function member(serverId: string, generation: number, sessions: SessionInfo[]): FleetMember {
  return {
    serverId,
    serverLabel: `Server ${serverId}`,
    generation,
    state: "connected",
    sessions,
  };
}

describe("fleet session protocol", () => {
  it("keeps colliding local IDs as distinct compound identities", () => {
    let snapshot = initialFleetSnapshot([
      { serverId: "a", serverLabel: "Alpha" },
      { serverId: "b", serverLabel: "Beta" },
    ]);
    snapshot = reduceFleetMember(snapshot, member("a", 1, [session("same")]));
    snapshot = reduceFleetMember(snapshot, member("b", 1, [session("same")]));

    expect(snapshot.sessions.map((entry) => sessionKey(entry.ref))).toEqual([
      '["a","same"]',
      '["b","same"]',
    ]);
    expect(snapshot.sessions.map((entry) => entry.serverLabel)).toEqual(["Server a", "Server b"]);
    expect(snapshot.revision).toBe(2);
  });

  it("retains an unavailable member's last-known sessions as explicitly stale", () => {
    let snapshot = reduceFleetMember(
      initialFleetSnapshot([{ serverId: "a", serverLabel: "Alpha" }]),
      member("a", 4, [session("s1")]),
    );
    snapshot = reduceFleetMember(snapshot, {
      ...member("a", 4, []),
      state: "unavailable",
      reason: "connection refused",
    });

    expect(snapshot.members[0]).toMatchObject({
      serverId: "a",
      state: "unavailable",
      stale: true,
      reason: "connection refused",
    });
    expect(snapshot.sessions.map((entry) => entry.ref)).toEqual([
      { serverId: "a", sessionId: "s1" },
    ]);
  });

  it("ignores a superseded member generation", () => {
    const snapshot = reduceFleetMember(
      initialFleetSnapshot([{ serverId: "a", serverLabel: "Alpha" }]),
      member("a", 8, [session("new")]),
    );
    const next = reduceFleetMember(snapshot, member("a", 7, [session("old")]));

    expect(next).toBe(snapshot);
    expect(next.sessions[0]?.ref.sessionId).toBe("new");
  });
});
