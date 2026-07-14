// favicon — derive a static SVG favicon from the aggregate session status.
//
// The Web UI shows many sessions at once. The favicon picks the highest
// priority status across them so the browser tab reflects the loudest signal.
//
// Priority (highest first): running > pending > waiting > idle > stopped.
// `unknown` falls below stopped — it's the no-data sentinel, not a state.
//
// Shapes mirror StatusIcon's geometry so users learn one symbol set. Motion
// is INTENTIONALLY dropped (unlike StatusIcon): a favicon is rendered as a
// static raster by browsers, so the CSS animations would never play anyway.
// Status is therefore encoded in shape + hue alone — opacities also mirror
// StatusIcon's CSS so the two glyphs read identically side-by-side.

import { type StatusKind, normalizeStatus } from "../components/StatusIcon";
import type { SessionInfo } from "../wire/server";

export const PRIORITY: readonly StatusKind[] = [
  "running",
  "pending",
  "waiting",
  "idle",
  "stopped",
  "unknown",
];

/** Pick the highest-priority status across all sessions.
 *  Returns "unknown" for an empty list (matches the pre-JS HTML default). */
export function selectTopStatus(sessions: readonly SessionInfo[]): StatusKind {
  const present = new Set(sessions.map((s) => normalizeStatus(s.view.status)));
  return PRIORITY.find((k) => present.has(k)) ?? "unknown";
}

/** Per-status colors — runtime values read from CSS variables by useFavicon().
 *  Hardcoded fallbacks match the dark-theme tokens.css defaults so the favicon
 *  is sensible even if getComputedStyle returns an empty string (jsdom, very
 *  early in boot, headless render). */
export type FaviconColors = Record<StatusKind, string>;

export const FALLBACK_COLORS: FaviconColors = {
  running: "#2c7a4d",
  pending: "#2c4a8a",
  waiting: "#b59300",
  idle: "#555555",
  stopped: "#6e2222",
  unknown: "#555555",
};

/** Build a 24×24 SVG document (static — no <animate> or CSS animations).
 *  Returned as a string ready for data: URI encoding. */
export function buildFaviconSvg(kind: StatusKind, colors: FaviconColors): string {
  const c = colors[kind];
  const inner = GEOMETRY[kind](c);
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">${inner}</svg>`;
}

/** Encode an SVG string into a data: URI suitable for a <link rel="icon"> href.
 *  Uses encodeURIComponent (not base64) so the URI stays human-inspectable in
 *  devtools and tests can do string contains checks. */
export function svgToDataUri(svg: string): string {
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

// `satisfies` enforces exhaustive coverage at compile time: adding a new
// StatusKind without a GEOMETRY entry is a type error. Shapes are the static
// snapshot of StatusIcon's GEOMETRY — running's arc is drawn at the canonical
// start position (the animation that rotates it is dropped). Opacities mirror
// status-icon.css base values so the favicon reads at the same weight as the
// in-app glyph (running ring 0.28, pending 0.75, stopped 0.85, unknown 0.5).
const GEOMETRY = {
  running: (c: string) =>
    `<circle cx="12" cy="12" r="9" fill="none" stroke="${c}" stroke-width="3" opacity="0.28"/>` +
    `<path d="M21 12 A 9 9 0 1 1 12 3" fill="none" stroke="${c}" stroke-width="3" stroke-linecap="round"/>`,
  pending: (c: string) =>
    `<circle cx="12" cy="12" r="8" fill="none" stroke="${c}" stroke-width="2" stroke-dasharray="3 3" opacity="0.75"/>`,
  waiting: (c: string) =>
    `<circle cx="5" cy="12" r="2.4" fill="${c}"/>` +
    `<circle cx="12" cy="12" r="2.4" fill="${c}"/>` +
    `<circle cx="19" cy="12" r="2.4" fill="${c}"/>`,
  idle: (c: string) => `<circle cx="12" cy="12" r="5" fill="${c}" opacity="0.75"/>`,
  stopped: (c: string) =>
    `<rect x="6" y="6" width="12" height="12" rx="2" fill="${c}" opacity="0.85"/>`,
  unknown: (c: string) =>
    `<line x1="6" y1="12" x2="18" y2="12" stroke="${c}" stroke-width="2" stroke-linecap="round" opacity="0.5"/>`,
} satisfies Record<StatusKind, (color: string) => string>;
