import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import type { SessionInfo } from "../wire/server";
import {
  type FleetMember,
  applyHealthDeliveryLoss,
  decodeFleetHealthSnapshotV1,
  encodeFleetHealthSnapshotV1,
  healthViewFromSnapshot,
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

const fixturesDir = join(dirname(fileURLToPath(import.meta.url)), "fixtures");

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

describe("FleetHealthSnapshotV1 codec", () => {
  // rejects counterexample: free-form stale-as-current dual fields and unknown reasons
  it("round-trips the canonical credential-free fixture", () => {
    const raw = JSON.parse(readFileSync(join(fixturesDir, "fleet-health-v1.json"), "utf8"));
    const decoded = decodeFleetHealthSnapshotV1(raw);
    expect(encodeFleetHealthSnapshotV1(decoded)).toEqual(decoded);
    expect(JSON.stringify(decoded)).not.toMatch(/Bearer |token=|ticket=|gho_/i);
    expect(decoded.servers.map((server) => server.serverId)).toEqual([
      "alpha",
      "beta",
      "gamma",
      "delta",
      "epsilon",
      "zeta",
    ]);
    expect(
      decoded.servers.find((server) => server.serverId === "delta")?.observation,
    ).toMatchObject({
      kind: "health_unavailable",
      prior: { kind: "connected" },
    });
  });

  it("rejects unknown, invalid, and stale-as-current payloads", () => {
    const catalog = JSON.parse(
      readFileSync(join(fixturesDir, "fleet-health-v1-invalid.json"), "utf8"),
    ) as { cases: Array<{ name: string; payload: unknown }> };
    for (const testCase of catalog.cases) {
      expect(() => decodeFleetHealthSnapshotV1(testCase.payload), testCase.name).toThrow();
    }
  });

  it("delivery loss preserves host identity/revision and nests prior as stale", () => {
    const snapshot = decodeFleetHealthSnapshotV1(
      JSON.parse(readFileSync(join(fixturesDir, "fleet-health-v1.json"), "utf8")),
    );
    const lost = applyHealthDeliveryLoss(snapshot, "2026-08-03T00:01:00.000Z");
    expect(lost.hostInstanceId).toBe(snapshot.hostInstanceId);
    expect(lost.revision).toBe(snapshot.revision);
    expect(lost.delivery).toBe("lost");
    const alpha = lost.servers.find((server) => server.serverId === "alpha");
    expect(alpha?.observation).toEqual({
      kind: "health_unavailable",
      observedAt: "2026-08-03T00:01:00.000Z",
      reason: "delivery_lost",
      prior: { kind: "connected", observedAt: "2026-08-03T00:00:05.000Z" },
    });
    const gamma = lost.servers.find((server) => server.serverId === "gamma");
    expect(gamma?.observation).toEqual({ kind: "disabled" });
    expect(healthViewFromSnapshot(snapshot).delivery).toBe("live");
  });
});
