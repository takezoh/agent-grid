import { describe, expect, it } from "vitest";
import {
  HEALTH_DELIVERY_LOST_SUMMARY,
  fleetIsDegraded,
  fleetOutcomeLabel,
  reduceStatusAxes,
  summarizeFleet,
} from "./presentation";
import type { FleetHealthView } from "./session-protocol";

describe("fleet presentation", () => {
  it("keeps incompatible protocol distinct from transport unavailability", () => {
    expect(
      fleetOutcomeLabel({
        state: "incompatible_protocol",
        reason: "incompatible protocol: client protocol-3, server protocol-1",
      }),
    ).toBe("Incompatible protocol — incompatible protocol: client protocol-3, server protocol-1");
  });

  it("summarizes mixed non-operable outcomes without counting disabled servers", () => {
    // rejects counterexample: disabled entries inflate denominator
    expect(
      summarizeFleet([
        { state: "connected" },
        { state: "incompatible_protocol" },
        { state: "unavailable" },
        { state: "disabled" },
      ]),
    ).toBe("1/3 servers incompatible; 1/3 servers unavailable");
  });

  it("preserves the existing unavailable-only summary", () => {
    expect(summarizeFleet([{ state: "connected" }, { state: "unavailable" }])).toBe(
      "1/2 servers unavailable",
    );
  });

  it("uses explicit total-outage wording for all enabled unavailable", () => {
    expect(
      summarizeFleet([
        { state: "unavailable" },
        { state: "health_unavailable" },
        { state: "disabled" },
      ]),
    ).toBe("all enabled servers unavailable");
  });
});

describe("independent status axes", () => {
  const degraded: FleetHealthView = {
    hostInstanceId: "host-1",
    revision: 1,
    delivery: "live",
    servers: [
      {
        serverId: "a",
        serverLabel: "A",
        enabled: true,
        observation: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
      },
      {
        serverId: "b",
        serverLabel: "B",
        enabled: true,
        observation: {
          kind: "unavailable",
          observedAt: "2026-08-03T00:00:00.000Z",
          reason: "http_error",
        },
      },
    ],
  };

  it("keeps fleet degradation after active hello", () => {
    const axes = reduceStatusAxes({
      connectionStatus: "open",
      daemonDisconnected: false,
      health: degraded,
    });
    expect(axes.showConnections).toBe(true);
    expect(axes.fleetSummary).toContain("unavailable");
    expect(axes.showRetry).toBe(false);
    expect(axes.nominal).toBe(false);
  });

  it("keeps closed Retry independent of fleet recovery", () => {
    const healthy: FleetHealthView = {
      ...degraded,
      servers: degraded.servers.map((server) => ({
        ...server,
        observation: { kind: "connected" as const, observedAt: "2026-08-03T00:00:00.000Z" },
      })),
    };
    const axes = reduceStatusAxes({
      connectionStatus: "closed",
      daemonDisconnected: true,
      health: healthy,
    });
    expect(axes.showRetry).toBe(true);
    expect(axes.showConnections).toBe(false);
    expect(axes.activeText).toBe("connection closed");
  });

  it("treats delivery-lost with empty servers as degraded, not nominal connected", () => {
    // rejects counterexample: active qualified session usable + empty lost view
    // looks healthy and hides Connections.
    const lostEmpty: FleetHealthView = {
      hostInstanceId: null,
      revision: null,
      delivery: "lost",
      servers: [],
    };
    expect(fleetIsDegraded(lostEmpty)).toBe(true);
    const axes = reduceStatusAxes({
      connectionStatus: "open",
      daemonDisconnected: false,
      health: lostEmpty,
    });
    expect(axes.nominal).toBe(false);
    expect(axes.showConnections).toBe(true);
    expect(axes.fleetSummary).toBe(HEALTH_DELIVERY_LOST_SUMMARY);
    expect(axes.activeText).toBe(HEALTH_DELIVERY_LOST_SUMMARY);
  });
});
