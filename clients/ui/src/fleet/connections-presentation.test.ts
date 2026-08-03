import { describe, expect, it } from "vitest";
import {
  DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION,
  DELIVERY_LOST_BEFORE_CONFIG_RECOVERY,
  projectConnectionsDiagnosis,
} from "./connections-presentation";
import type { FleetHealthView } from "./session-protocol";

const health: FleetHealthView = {
  hostInstanceId: "host-1",
  revision: 3,
  delivery: "live",
  servers: [
    {
      serverId: "a",
      serverLabel: "Same",
      enabled: true,
      observation: {
        kind: "unavailable",
        observedAt: "2026-08-03T00:00:00.000Z",
        reason: "http_error",
      },
    },
    {
      serverId: "b",
      serverLabel: "Same",
      enabled: true,
      observation: {
        kind: "health_unavailable",
        observedAt: "2026-08-03T00:00:10.000Z",
        reason: "refresh_failed",
        prior: { kind: "connected", observedAt: "2026-08-03T00:00:05.000Z" },
      },
    },
    {
      serverId: "c",
      serverLabel: "Off",
      enabled: false,
      observation: { kind: "disabled" },
    },
  ],
};

describe("connections presentation", () => {
  // rejects counterexample: display-name collisions collapse rows or hide reasons
  it("projects server-qualified rows with closed reasons for both hosts", () => {
    const diagnosis = projectConnectionsDiagnosis(health, {
      configPath: "/etc/agent-grid/servers.json",
      hostKind: "browser",
    });
    expect(diagnosis.rows.map((row) => row.label)).toEqual(["Same · a", "Same · b", "Off"]);
    expect(diagnosis.rows[0]).toMatchObject({
      serverId: "a",
      state: "unavailable",
      stateLabel: "Unavailable",
    });
    expect(diagnosis.rows[1]).toMatchObject({
      state: "health_unavailable",
      stateLabel: "Health unavailable",
      stalePrior: true,
    });
    expect(diagnosis.rows[2]).toMatchObject({ state: "disabled", stateLabel: "Disabled" });
    expect(diagnosis.configPath).toBe("/etc/agent-grid/servers.json");
    expect(diagnosis.restartInstruction).toMatch(/restart/i);
    expect(diagnosis.deliveryLossExplanation).toBeUndefined();
    expect(JSON.stringify(diagnosis)).not.toMatch(/Bearer |ticket=|gho_/i);
  });

  it("never invents Retry semantics", () => {
    const diagnosis = projectConnectionsDiagnosis(health);
    expect(JSON.stringify(diagnosis).toLowerCase()).not.toContain("retry");
  });

  // rejects counterexample: zero-server delivery-lost projects blank diagnosis
  it("explains delivery lost before configuration when no server rows exist", () => {
    const lostEmpty: FleetHealthView = {
      hostInstanceId: null,
      revision: null,
      delivery: "lost",
      servers: [],
    };
    const diagnosis = projectConnectionsDiagnosis(lostEmpty, { hostKind: "browser" });
    expect(diagnosis.rows).toEqual([]);
    expect(diagnosis.deliveryLossExplanation).toBe(DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION);
    expect(diagnosis.recoveryGuidance).toBe(DELIVERY_LOST_BEFORE_CONFIG_RECOVERY);
    expect(diagnosis.restartInstruction).toMatch(/servers\.json|restart/i);
    expect(JSON.stringify(diagnosis).toLowerCase()).not.toContain("retry");
    expect(JSON.stringify(diagnosis)).not.toMatch(/Bearer |ticket=|gho_/i);
  });

  it("does not invent delivery-loss copy for a normal empty live fleet", () => {
    const emptyLive: FleetHealthView = {
      hostInstanceId: "host-1",
      revision: 1,
      delivery: "live",
      servers: [],
    };
    const diagnosis = projectConnectionsDiagnosis(emptyLive, {
      configPath: "/etc/agent-grid/servers.json",
      hostKind: "browser",
    });
    expect(diagnosis.rows).toEqual([]);
    expect(diagnosis.deliveryLossExplanation).toBeUndefined();
    expect(diagnosis.recoveryGuidance).toBeUndefined();
  });
});
