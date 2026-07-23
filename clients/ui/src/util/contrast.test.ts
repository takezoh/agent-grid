import { describe, expect, it } from "vitest";
import { blendOver, contrastRatio, parseColor, relativeLuminance, srgbToLinear } from "./contrast";

describe("srgbToLinear", () => {
  it("returns 0 for black channel (0 integer)", () => {
    expect(srgbToLinear(0)).toBeCloseTo(0, 10);
  });

  it("returns 1 for white channel (255 integer)", () => {
    expect(srgbToLinear(255)).toBeCloseTo(1, 10);
  });

  it("handles float input 0..1 branch (c <= 0.03928)", () => {
    expect(srgbToLinear(0.0)).toBeCloseTo(0, 10);
    expect(srgbToLinear(0.03928)).toBeCloseTo(0.03928 / 12.92, 10);
  });

  it("handles float input > 0.03928", () => {
    const c = 0.5;
    const expected = ((c + 0.055) / 1.055) ** 2.4;
    expect(srgbToLinear(c)).toBeCloseTo(expected, 10);
  });
});

describe("relativeLuminance", () => {
  it("black (#000) has luminance 0", () => {
    expect(relativeLuminance({ r: 0, g: 0, b: 0 })).toBeCloseTo(0, 10);
  });

  it("white (#fff) has luminance 1", () => {
    expect(relativeLuminance({ r: 255, g: 255, b: 255 })).toBeCloseTo(1, 10);
  });
});

describe("contrastRatio", () => {
  const black = { r: 0, g: 0, b: 0 };
  const white = { r: 255, g: 255, b: 255 };
  const gray777 = { r: 0x77, g: 0x77, b: 0x77 };
  const blue = { r: 0, g: 0, b: 255 };

  it("#000 vs #fff = 21.0", () => {
    expect(contrastRatio(black, white)).toBeCloseTo(21.0, 1);
  });

  it("#fff vs #000 = 21.0 (order-independent)", () => {
    expect(contrastRatio(white, black)).toBeCloseTo(21.0, 1);
  });

  it("#777 vs #000 ≈ 4.69", () => {
    expect(contrastRatio(gray777, black)).toBeCloseTo(4.69, 1);
  });

  it("#0000ff vs #ffffff ≈ 8.59", () => {
    expect(contrastRatio(blue, white)).toBeCloseTo(8.59, 1);
  });

  it("same colour has ratio 1.0", () => {
    expect(contrastRatio(white, white)).toBeCloseTo(1.0, 10);
  });
});

describe("parseColor", () => {
  it("parses #RRGGBB", () => {
    expect(parseColor("#0000ff")).toEqual({ r: 0, g: 0, b: 255 });
    expect(parseColor("#ffffff")).toEqual({ r: 255, g: 255, b: 255 });
    expect(parseColor("#000000")).toEqual({ r: 0, g: 0, b: 0 });
  });

  it("parses #RGB (shorthand)", () => {
    expect(parseColor("#fff")).toEqual({ r: 255, g: 255, b: 255 });
    expect(parseColor("#000")).toEqual({ r: 0, g: 0, b: 0 });
    expect(parseColor("#f0f")).toEqual({ r: 255, g: 0, b: 255 });
  });

  it("parses rgb(r, g, b)", () => {
    expect(parseColor("rgb(0, 0, 255)")).toEqual({ r: 0, g: 0, b: 255 });
    expect(parseColor("rgb(255, 255, 255)")).toEqual({ r: 255, g: 255, b: 255 });
    expect(parseColor("rgb( 10 , 20 , 30 )")).toEqual({ r: 10, g: 20, b: 30 });
  });

  it("returns null for unrecognised input", () => {
    expect(parseColor("red")).toBeNull();
    expect(parseColor("#gggggg")).toBeNull();
    expect(parseColor("")).toBeNull();
    expect(parseColor("hsl(0, 100%, 50%)")).toBeNull();
  });

  it("is case-insensitive for hex", () => {
    expect(parseColor("#FFFFFF")).toEqual({ r: 255, g: 255, b: 255 });
    expect(parseColor("#FFF")).toEqual({ r: 255, g: 255, b: 255 });
  });

  it("parses rgba(r, g, b, a)", () => {
    expect(parseColor("rgba(74, 158, 255, 0.3)")).toEqual({
      r: 74,
      g: 158,
      b: 255,
      a: 0.3,
    });
  });
});

describe("blendOver", () => {
  it("composites semi-transparent overlay onto opaque base", () => {
    const base = { r: 30, g: 30, b: 30 };
    const overlay = { r: 74, g: 158, b: 255, a: 0.12 };
    const blended = blendOver(overlay, base);
    expect(blended.r).toBe(Math.round(74 * 0.12 + 30 * 0.88));
    expect(blended.g).toBe(Math.round(158 * 0.12 + 30 * 0.88));
    expect(blended.b).toBe(Math.round(255 * 0.12 + 30 * 0.88));
  });
});
