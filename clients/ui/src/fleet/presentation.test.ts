import { describe, expect, it } from "vitest";
import { fleetOutcomeLabel, summarizeFleet } from "./presentation";

describe("fleet presentation", () => {
  it("keeps incompatible protocol distinct from transport unavailability", () => {
    expect(
      fleetOutcomeLabel({
        state: "incompatible_protocol",
        reason: "incompatible protocol: client protocol-3, server protocol-1",
      }),
    ).toBe("Incompatible protocol — incompatible protocol: client protocol-3, server protocol-1");
  });

  it("summarizes mixed non-operable outcomes without hiding healthy peers", () => {
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
});
