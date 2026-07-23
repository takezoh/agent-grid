/**
 * token-contrast.test.ts — NFR-001 / m1: WCAG AA contrast for semantic tokens
 * in both dark (:root) and light ([data-theme="light"]) themes.
 *
 * Uses util/contrast + cssTokenResolver against tokens.css literal values.
 */

import * as path from "node:path";
import { describe, expect, it } from "vitest";
import { loadThemeTokenMapsFromFile, resolveRaw } from "../lib/cssTokenResolver";
import { type Rgb, blendOver, contrastRatio, isRgba, parseColor } from "../util/contrast";

const tokensPath = path.resolve(__dirname, "../css/tokens.css");
const { dark, light } = loadThemeTokenMapsFromFile(tokensPath);

const WCAG_AA_NORMAL = 4.5;
const WCAG_AA_UI = 3.0;

type ColorPair = { fg: string; bg: string; label: string };

/** Normal body text pairs — 4.5:1 */
const NORMAL_PAIRS: ColorPair[] = [
  { fg: "--fg", bg: "--bg", label: "body text" },
  { fg: "--fg-muted", bg: "--bg", label: "muted text" },
  { fg: "--editor-fg", bg: "--editor-bg", label: "editor text" },
  { fg: "--on-accent", bg: "--accent", label: "accent button label" },
];

/**
 * Large text / UI components — 3:1 (NFR-001).
 * Status dots are shape+color coded (UAC-002) and exempt from canvas contrast here.
 */
const UI_PAIRS: ColorPair[] = [
  { fg: "--text-faint", bg: "--bg", label: "faint metadata" },
  { fg: "--accent", bg: "--bg", label: "accent on canvas" },
  { fg: "--focus-ring", bg: "--bg", label: "focus ring" },
  /* Soft pills: vivid status text over translucent status fill (blended over --bg). */
  { fg: "--status-running", bg: "--status-running-soft", label: "running pill" },
  { fg: "--status-stopped", bg: "--status-stopped-soft", label: "stopped pill" },
  { fg: "--status-waiting", bg: "--status-waiting-soft", label: "waiting pill" },
  { fg: "--status-pending", bg: "--status-pending-soft", label: "pending pill" },
];

function toOpaqueRgb(tokens: typeof dark, token: string): Rgb | null {
  const raw = resolveRaw(tokens, token);
  if (raw === null) return null;
  const parsed = parseColor(raw);
  if (parsed === null) return null;
  if (isRgba(parsed)) {
    const baseRaw = resolveRaw(tokens, "--bg");
    const base = baseRaw ? parseColor(baseRaw) : null;
    if (base === null || isRgba(base)) return null;
    return blendOver(parsed, base);
  }
  return parsed;
}

function contrastForPair(tokens: typeof dark, pair: ColorPair): number | null {
  const fg = toOpaqueRgb(tokens, pair.fg);
  const bg = toOpaqueRgb(tokens, pair.bg);
  if (fg === null || bg === null) return null;
  return contrastRatio(fg, bg);
}

/** Selected session row: --fg on --accent-soft composited over --bg */
function selectedRowContrast(tokens: typeof dark): number | null {
  const fg = toOpaqueRgb(tokens, "--fg");
  const softRaw = resolveRaw(tokens, "--accent-soft");
  const bg = toOpaqueRgb(tokens, "--bg");
  if (fg === null || softRaw === null || bg === null) return null;
  const soft = parseColor(softRaw);
  if (soft === null || !isRgba(soft)) return null;
  const effectiveBg = blendOver(soft, bg);
  return contrastRatio(fg, effectiveBg);
}

for (const [themeName, tokenMap] of [
  ["dark", dark],
  ["light", light],
] as const) {
  describe(`NFR-001 token contrast (${themeName} theme)`, () => {
    for (const pair of NORMAL_PAIRS) {
      it(`${pair.label}: ${pair.fg} on ${pair.bg} >= ${WCAG_AA_NORMAL}:1`, () => {
        const ratio = contrastForPair(tokenMap, pair);
        expect(ratio, `could not resolve ${pair.fg}/${pair.bg}`).not.toBeNull();
        expect(
          ratio as number,
          `${themeName} ${pair.label} contrast ${ratio?.toFixed(2)}`,
        ).toBeGreaterThanOrEqual(WCAG_AA_NORMAL);
      });
    }

    for (const pair of UI_PAIRS) {
      it(`${pair.label}: ${pair.fg} on ${pair.bg} >= ${WCAG_AA_UI}:1`, () => {
        const ratio = contrastForPair(tokenMap, pair);
        expect(ratio, `could not resolve ${pair.fg}/${pair.bg}`).not.toBeNull();
        expect(
          ratio as number,
          `${themeName} ${pair.label} contrast ${ratio?.toFixed(2)}`,
        ).toBeGreaterThanOrEqual(WCAG_AA_UI);
      });
    }

    it("selected row: --fg on --accent-soft over --bg >= 4.5:1", () => {
      const ratio = selectedRowContrast(tokenMap);
      expect(ratio).not.toBeNull();
      expect(ratio as number).toBeGreaterThanOrEqual(WCAG_AA_NORMAL);
    });
  });
}
