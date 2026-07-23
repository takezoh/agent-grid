import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { contrastRatio, parseColor } from "../../util/contrast";
import { TagPill, resolveTagPillStyle } from "./TagPill";

describe("resolveTagPillStyle — unit (FR-TAGPILL-001)", () => {
  it("low-contrast fg=#777 bg=#fff: flips fg to #000 and adds border", () => {
    const style = resolveTagPillStyle("#777777", "#ffffff");
    expect(style.color).toBe("#000000");
    expect(style.backgroundColor).toBe("#ffffff");
    expect(style.border).toBe("1px solid currentColor");

    const actualFg = parseColor(style.color);
    const actualBg = parseColor(style.backgroundColor);
    if (!actualFg || !actualBg) throw new Error("parseColor returned null for resolved style");
    const ratio = contrastRatio(actualFg, actualBg);
    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });

  it("high-contrast fg=#000 bg=#fff: keeps original colors, no border", () => {
    const style = resolveTagPillStyle("#000000", "#ffffff");
    expect(style.color).toBe("#000000");
    expect(style.backgroundColor).toBe("#ffffff");
    expect(style.border).toBeUndefined();
  });

  it("high-contrast fg=#fff bg=#000: keeps original colors, no border", () => {
    const style = resolveTagPillStyle("#ffffff", "#000000");
    expect(style.color).toBe("#ffffff");
    expect(style.backgroundColor).toBe("#000000");
    expect(style.border).toBeUndefined();
  });

  it("low-contrast dark bg: flips to white fg", () => {
    const style = resolveTagPillStyle("#555555", "#111111");
    expect(style.color).toBe("#ffffff");
    expect(style.border).toBe("1px solid currentColor");

    const actualFg = parseColor(style.color);
    const actualBg = parseColor(style.backgroundColor);
    if (!actualFg || !actualBg) throw new Error("parseColor returned null for resolved style");
    const ratio = contrastRatio(actualFg, actualBg);
    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });

  it("invalid fg falls back to token default, ratio still computed", () => {
    const style = resolveTagPillStyle("not-a-color", "#ffffff");
    expect(style.color).toBe("#000000");
    expect(style.backgroundColor).toBe("#ffffff");
    expect(style.border).toBe("1px solid currentColor");
  });

  it("invalid bg falls back to token default", () => {
    const style = resolveTagPillStyle("#000000", "bad-color");
    expect(style.color).toBe("#ffffff");
    expect(style.backgroundColor).toBe("rgb(51,51,51)");
    expect(style.border).toBe("1px solid currentColor");
  });
});

describe("TagPill DOM — FR-TAGPILL-001 computed style", () => {
  it("low-contrast pill has border and high-contrast fg in computed style", () => {
    const { container } = render(
      <TagPill tag={{ text: "LowContrast", fg: "#777777", bg: "#ffffff" }} />,
    );
    const pill = container.querySelector(".driver-tag-pill");
    expect(pill).not.toBeNull();

    const el = pill as HTMLElement;
    expect(el.style.color).toBeTruthy();
    expect(el.style.border.toLowerCase()).toBe("1px solid currentcolor");

    const fgStr = el.style.color;
    const bgStr = el.style.backgroundColor;
    const fg = parseColor(fgStr) ?? { r: 0, g: 0, b: 0 };
    const bg = parseColor(bgStr) ?? { r: 255, g: 255, b: 255 };
    const ratio = contrastRatio(fg, bg);
    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });

  it("high-contrast pill has no border in DOM", () => {
    const { container } = render(
      <TagPill tag={{ text: "HighContrast", fg: "#000000", bg: "#ffffff" }} />,
    );
    const pill = container.querySelector(".driver-tag-pill") as HTMLElement;
    expect(pill).not.toBeNull();
    expect(pill.style.border).toBeFalsy();
  });
});
