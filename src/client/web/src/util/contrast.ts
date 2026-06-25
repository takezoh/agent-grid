/**
 * WCAG 2.1 contrast utilities — dependency-free stdlib JS only.
 * Used by TagPill (m5) to decide fg black/white for FR-TAGPILL-001.
 */

/** Convert a single sRGB channel (0–255 integer or 0–1 float) to linear light. */
export function srgbToLinear(channel: number): number {
  const c = channel > 1 ? channel / 255 : channel;
  return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

/** WCAG 2.1 relative luminance from an sRGB triplet (each channel 0–255 or 0–1). */
export function relativeLuminance(rgb: { r: number; g: number; b: number }): number {
  return 0.2126 * srgbToLinear(rgb.r) + 0.7152 * srgbToLinear(rgb.g) + 0.0722 * srgbToLinear(rgb.b);
}

/**
 * WCAG 2.1 contrast ratio between two sRGB colours.
 * Returns a value in [1, 21]. 4.5 is the AA threshold for normal text.
 */
export function contrastRatio(
  fg: { r: number; g: number; b: number },
  bg: { r: number; g: number; b: number },
): number {
  const l1 = relativeLuminance(fg);
  const l2 = relativeLuminance(bg);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

/**
 * Parse a CSS colour string into an { r, g, b } triplet (0–255 integers).
 * Accepts #RGB, #RRGGBB, and rgb(r, g, b). Returns null for unrecognised input.
 */
export function parseColor(input: string): { r: number; g: number; b: number } | null {
  const s = input.trim();

  const hex6 = /^#([0-9a-f]{6})$/i.exec(s);
  if (hex6?.[1]) {
    const n = Number.parseInt(hex6[1], 16);
    return { r: (n >> 16) & 0xff, g: (n >> 8) & 0xff, b: n & 0xff };
  }

  const hex3 = /^#([0-9a-f])([0-9a-f])([0-9a-f])$/i.exec(s);
  if (hex3?.[1] && hex3[2] && hex3[3]) {
    return {
      r: Number.parseInt(hex3[1] + hex3[1], 16),
      g: Number.parseInt(hex3[2] + hex3[2], 16),
      b: Number.parseInt(hex3[3] + hex3[3], 16),
    };
  }

  const rgb = /^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$/i.exec(s);
  if (rgb?.[1] && rgb[2] && rgb[3]) {
    return { r: Number(rgb[1]), g: Number(rgb[2]), b: Number(rgb[3]) };
  }

  return null;
}
