import { describe, expect, it } from "vitest";
import { parseServerFrame, serializeClientFrame } from "./codec";

describe("lifecycle v2 wire", () => {
  it("round-trips a complete desired publication", () => {
    const raw = serializeClientFrame({
      k: "ld",
      reqId: "r1",
      sessionId: "s1",
      cols: 120,
      rows: 40,
      desired: true,
      correlation: { clientInstanceID: "c1", connectionGeneration: 2, clientRevision: 7 },
    });
    expect(JSON.parse(raw)).toMatchObject({ k: "ld", desired: true });
  });

  it("rejects lifecycle frames without public correlation", () => {
    expect(parseServerFrame('{"k":"lo","status":"applied"}')).toBeNull();
  });

  it("accepts authoritative status and diagnostic evidence", () => {
    const correlation = { clientInstanceID: "c1", connectionGeneration: 2, clientRevision: 7 };
    expect(
      parseServerFrame(JSON.stringify({ k: "lo", correlation, status: "waiting" })),
    ).toMatchObject({ k: "lo", status: "waiting" });
    expect(
      parseServerFrame(JSON.stringify({ k: "lg", correlation, watermark: 4, unknown: true })),
    ).toMatchObject({ k: "lg", watermark: 4, unknown: true });
  });
});
