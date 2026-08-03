import { describe, expect, it } from "vitest";
import {
  classifyHealthFailure,
  connectionFailureReason,
  decodeFleetHealthSnapshotV1,
  healthReasonLabel,
  legacyFleetHealthFromSnapshot,
} from "../src/main/server-health.js";

describe("health classifier", () => {
  // rejects counterexample: raw fetch internals or credential-bearing strings as reasons
  it("classifies transport failures into a closed HealthReason enum", () => {
    expect(classifyHealthFailure(new TypeError("fetch failed"))).toEqual({
      reason: "connection_failed",
    });
    expect(classifyHealthFailure(new Error("The operation was aborted due to timeout"))).toEqual({
      reason: "connection_timeout",
    });
    expect(classifyHealthFailure(new Error("HTTP 503"))).toEqual({ reason: "http_error" });
    expect(classifyHealthFailure(new Error("websocket failed"))).toEqual({
      reason: "websocket_failed",
    });
    expect(connectionFailureReason(new TypeError("fetch failed"))).toBe(
      "Could not connect to server",
    );
    expect(healthReasonLabel("protocol_mismatch", "incompatible protocol: client protocol-3, server old")).toBe(
      "incompatible protocol: client protocol-3, server old",
    );
  });

  it("decodes V1 snapshots and legacy projection remains credential-free", () => {
    const snapshot = decodeFleetHealthSnapshotV1({
      schemaVersion: 1,
      hostInstanceId: "host-1",
      revision: 2,
      servers: [
        {
          serverId: "healthy",
          serverLabel: "Healthy",
          enabled: true,
          observation: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
        },
        {
          serverId: "down",
          serverLabel: "Down",
          enabled: true,
          observation: {
            kind: "unavailable",
            observedAt: "2026-08-03T00:00:00.000Z",
            reason: "http_error",
          },
        },
        {
          serverId: "disabled",
          serverLabel: "Disabled",
          enabled: false,
          observation: { kind: "disabled" },
        },
      ],
    });
    const legacy = legacyFleetHealthFromSnapshot(snapshot);
    expect(legacy.servers).toEqual([
      { serverId: "healthy", displayName: "Healthy", state: "connected" },
      {
        serverId: "down",
        displayName: "Down",
        state: "unavailable",
        reason: "Server HTTP error",
      },
      { serverId: "disabled", displayName: "Disabled", state: "disabled" },
    ]);
    expect(JSON.stringify(legacy)).not.toMatch(/Bearer |token=|ticket=/i);
  });

  it("rejects exclusive observation and forbidden prior shapes (UI decoder parity)", () => {
    // rejects counterexample: silently accepting checking/connected forbidden fields
    const base = {
      schemaVersion: 1 as const,
      hostInstanceId: "host-1",
      revision: 1,
      servers: [
        {
          serverId: "a",
          serverLabel: "A",
          enabled: true,
          observation: {} as Record<string, unknown>,
        },
      ],
    };
    const reject = (observation: Record<string, unknown>) => {
      expect(() =>
        decodeFleetHealthSnapshotV1({
          ...base,
          servers: [{ ...base.servers[0]!, observation }],
        }),
      ).toThrow();
    };
    reject({
      kind: "checking",
      observedAt: "2026-08-03T00:00:00.000Z",
      reason: "connection_failed",
    });
    reject({
      kind: "checking",
      observedAt: "2026-08-03T00:00:00.000Z",
      prior: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
    });
    reject({
      kind: "connected",
      observedAt: "2026-08-03T00:00:00.000Z",
      detail: "nope",
    });
    reject({
      kind: "unavailable",
      observedAt: "2026-08-03T00:00:00.000Z",
      reason: "http_error",
      prior: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
    });
    reject({
      kind: "health_unavailable",
      observedAt: "2026-08-03T00:00:00.000Z",
      prior: { kind: "checking", observedAt: "2026-08-03T00:00:00.000Z" },
    });
    reject({
      kind: "health_unavailable",
      observedAt: "2026-08-03T00:00:00.000Z",
      prior: { kind: "connected" },
    });
  });
});
