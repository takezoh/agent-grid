import { describe, expect, it } from "vitest";
import {
  parseControlLine,
  replyError,
  replyOk,
} from "../src/shared/control-envelope.js";

describe("control-envelope closed schema", () => {
  it.each([
    ['{"op":"openSession","server_id":"local","session_id":"sess-1"}', "openSession"],
    ['{"op":"activate"}', "activate"],
    ['{"op":"quit"}', "quit"],
    ['{"op":"openSession","server_id":"local","session_id":"s","schema_version":2}', "openSession"],
  ])("accepts %s", (line, op) => {
    const r = parseControlLine(line);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.envelope.op).toBe(op);
  });

  it.each([
    ['{"op":"openSession","server_id":"local","session_id":"s","extra":1}', "unknown field"],
    ['{"op":"openSession","server_id":"local","session_id":"s","health":"ok"}', "unknown field"],
    ['{"op":"nope"}', "unknown op"],
    ['{"op":"openSession"}', "requires server_id"],
    ["not-json", "malformed"],
    ["", "empty"],
  ])("rejects %s", (line, fragment) => {
    const r = parseControlLine(line);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error.toLowerCase()).toContain(fragment);
  });

  it("reply shapes", () => {
    expect(replyOk()).toContain('"ok":true');
    expect(replyError("boom")).toContain("boom");
  });
});
