import { describe, expect, it } from "vitest";
import {
  type PublicCorrelation,
  observeTransport,
  reduceTransportObservation,
} from "./transportObservation";

const correlation: PublicCorrelation = {
  clientInstanceID: "client-1",
  connectionGeneration: 3,
  clientRevision: 8,
};

describe("TransportObservation", () => {
  it("keeps the watchdog active for nonterminal statuses", () => {
    const observing = observeTransport(correlation, 4000);
    expect(
      reduceTransportObservation(observing, {
        type: "authoritative_terminal",
        correlation: { ...correlation, clientRevision: 9 },
      }),
    ).toEqual(observing);
  });

  it("accepts only a matching authoritative terminal outcome", () => {
    const observing = observeTransport(correlation, 4000);
    expect(
      reduceTransportObservation(observing, {
        type: "authoritative_terminal",
        correlation,
      }).kind,
    ).toBe("observed_remote");
  });

  it("records publication replacement without closing the socket", () => {
    const observing = observeTransport(correlation, 4000);
    const replaced = reduceTransportObservation(observing, {
      type: "publication_replace",
      nextRevision: 9,
    });
    expect(replaced).toEqual({
      kind: "publication_replaced",
      correlation,
      nextRevision: 9,
    });
  });

  it("makes deadline and socket close mutually exclusive terminal observations", () => {
    const observing = observeTransport(correlation, 4000);
    expect(reduceTransportObservation(observing, { type: "deadline", nowMs: 4000 }).kind).toBe(
      "delivery_timeout",
    );
    expect(reduceTransportObservation(observing, { type: "socket_close" }).kind).toBe(
      "socket_closed",
    );
  });
});
