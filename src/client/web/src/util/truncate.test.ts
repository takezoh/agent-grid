import { describe, expect, it } from "vitest";
import { SESSION_LABEL_MAX_LENGTH, truncateLabel } from "./truncate";

describe("truncateLabel", () => {
  it("returns short text unchanged", () => {
    expect(truncateLabel("alpha")).toBe("alpha");
  });

  it("returns text exactly at maxLength unchanged", () => {
    const text = "a".repeat(SESSION_LABEL_MAX_LENGTH);
    expect(truncateLabel(text)).toBe(text);
  });

  it("truncates text past maxLength and appends an ellipsis", () => {
    const text = "a".repeat(SESSION_LABEL_MAX_LENGTH + 20);
    const result = truncateLabel(text);
    expect(result.length).toBe(SESSION_LABEL_MAX_LENGTH);
    expect(result.endsWith("…")).toBe(true);
    expect(result.slice(0, -1)).toBe("a".repeat(SESSION_LABEL_MAX_LENGTH - 1));
  });

  it("trims trailing whitespace left dangling by the cut before the ellipsis", () => {
    const text = `${"a".repeat(SESSION_LABEL_MAX_LENGTH - 1)} bbbb`;
    const result = truncateLabel(text);
    expect(result).toBe(`${"a".repeat(SESSION_LABEL_MAX_LENGTH - 1)}…`);
  });

  it("honors a custom maxLength", () => {
    expect(truncateLabel("abcdefgh", 4)).toBe("abc…");
    expect(truncateLabel("abcd", 4)).toBe("abcd");
  });
});
