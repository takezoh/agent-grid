import { describe, expect, it } from "vitest";
import { parseServerFrame, serializeClientFrame } from "./codec";
import { fixtures } from "./fixtures";

describe("parseServerFrame", () => {
  it.each(Object.entries(fixtures))("parses %s round-trip", (_name, raw) => {
    const parsed = parseServerFrame(raw);
    expect(parsed).not.toBeNull();
    // serialize back: output frame is array, others are objects
    const re = Array.isArray(parsed) ? JSON.stringify(parsed) : JSON.stringify(parsed);
    const reparsed = parseServerFrame(re);
    expect(reparsed).toEqual(parsed);
  });

  it("returns null for invalid JSON", () => {
    expect(parseServerFrame("not json")).toBeNull();
  });

  it("returns null for unknown discriminator", () => {
    expect(parseServerFrame('{"k":"xyz"}')).toBeNull();
  });

  it("returns null for missing discriminator", () => {
    expect(parseServerFrame("{}")).toBeNull();
  });

  it("returns null for malformed output array", () => {
    // [timeSec, "o", data] order — too short
    expect(parseServerFrame('[1.0,"o"]')).toBeNull();
    // timeSec must be number
    expect(parseServerFrame('["not-number","o","data"]')).toBeNull();
    // old wrong order ["o", number, string] must also return null
    expect(parseServerFrame('["o",1.0,"data"]')).toBeNull();
  });

  it("parses view-update with full View payload", () => {
    const frame = parseServerFrame(fixtures.viewUpdate);
    expect(frame).not.toBeNull();
    if (!frame || Array.isArray(frame) || frame.k !== "v") throw new Error("expected v frame");
    const sess = frame.sessions[0];
    expect(sess).toBeDefined();
    expect(sess?.view.card.title).toBe("T2");
    expect(sess?.view.status).toBe("idle");
  });

  it("returns null when sessions[].view is missing", () => {
    expect(parseServerFrame('{"k":"v","sessions":[{"id":"s1"}]}')).toBeNull();
  });

  it("returns null when sessions[].view.card is missing", () => {
    expect(
      parseServerFrame('{"k":"v","sessions":[{"id":"s1","view":{"status":"idle"}}]}'),
    ).toBeNull();
  });
});

describe("serializeClientFrame", () => {
  it("serializes input frame", () => {
    expect(serializeClientFrame({ k: "i", d: "abc" })).toBe('{"k":"i","d":"abc"}');
  });
  it("serializes resize frame", () => {
    expect(serializeClientFrame({ k: "r", cols: 80, rows: 24 })).toBe(
      '{"k":"r","cols":80,"rows":24}',
    );
  });
  it("serializes subscribe frame", () => {
    expect(serializeClientFrame({ k: "s", reqId: "r1", sessionId: "s1" })).toBe(
      '{"k":"s","reqId":"r1","sessionId":"s1"}',
    );
  });
});
