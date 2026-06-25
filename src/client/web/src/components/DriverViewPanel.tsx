import { formatElapsed, useNow1Hz } from "../hooks/useNow1Hz";
import { contrastRatio, parseColor } from "../util/contrast";
import type { Tag as TagType, View } from "../wire/server";
import { RunStateBadge } from "./RunStateBadge";
import "../css/view.css";

export type DriverViewPanelProps = {
  view: View;
};

/** Default token fg/bg used when driver provides an invalid color string. */
const TOKEN_DEFAULT_FG = { r: 230, g: 230, b: 230 }; // #e6e6e6
const TOKEN_DEFAULT_FG_STR = `rgb(${TOKEN_DEFAULT_FG.r},${TOKEN_DEFAULT_FG.g},${TOKEN_DEFAULT_FG.b})`;
const TOKEN_DEFAULT_BG = { r: 51, g: 51, b: 51 }; // #333 (--status-unknown)
const TOKEN_DEFAULT_BG_STR = `rgb(${TOKEN_DEFAULT_BG.r},${TOKEN_DEFAULT_BG.g},${TOKEN_DEFAULT_BG.b})`;

const BLACK = { r: 0, g: 0, b: 0 };
const WHITE = { r: 255, g: 255, b: 255 };

const WCAG_AA = 4.5;

/**
 * FR-TAGPILL-001: Compute accessible fg/bg for a driver-supplied tag.
 * When contrast ratio < 4.5, replace fg with whichever of black/white
 * gives higher contrast against bg, and add a border indicator.
 *
 * Invalid color inputs are replaced by token defaults for BOTH the ratio
 * calculation AND the emitted CSS value so they cannot diverge (FR-WIRE-001).
 */
export function resolveTagPillStyle(
  fgInput: string | undefined,
  bgInput: string | undefined,
): { color: string; backgroundColor: string; border?: string } {
  const fgParsed = fgInput ? parseColor(fgInput) : null;
  const bgParsed = bgInput ? parseColor(bgInput) : null;

  if (fgParsed === null && fgInput) {
    console.warn("[TagPill] Invalid driver fg color (FR-WIRE-001):", fgInput);
  }
  if (bgParsed === null && bgInput) {
    console.warn("[TagPill] Invalid driver bg color (FR-WIRE-001):", bgInput);
  }

  // Use resolved (parsed) values for both ratio computation AND CSS emission.
  // When a driver color is invalid, fall back to the token default string so
  // the rendered output matches what the ratio was computed against.
  const fg = fgParsed ?? TOKEN_DEFAULT_FG;
  const bg = bgParsed ?? TOKEN_DEFAULT_BG;
  // fgInput/bgInput are only used when parseColor succeeded (non-null),
  // so they are guaranteed to be defined at that point.
  const fgStr = fgParsed !== null ? (fgInput ?? TOKEN_DEFAULT_FG_STR) : TOKEN_DEFAULT_FG_STR;
  const bgStr = bgParsed !== null ? (bgInput ?? TOKEN_DEFAULT_BG_STR) : TOKEN_DEFAULT_BG_STR;

  const ratio = contrastRatio(fg, bg);

  if (ratio < WCAG_AA) {
    const blackRatio = contrastRatio(BLACK, bg);
    const whiteRatio = contrastRatio(WHITE, bg);
    const newFg = blackRatio >= whiteRatio ? "#000000" : "#ffffff";
    return {
      color: newFg,
      backgroundColor: bgStr,
      border: "1px solid currentColor",
    };
  }

  return {
    color: fgStr,
    backgroundColor: bgStr,
  };
}

function TagPill({ tag }: { tag: TagType }) {
  const style = resolveTagPillStyle(tag.fg, tag.bg);
  return (
    <span className="driver-tag driver-tag-pill" style={style}>
      {tag.text}
    </span>
  );
}

export function DriverViewPanel({ view }: DriverViewPanelProps) {
  const now = useNow1Hz();
  const card = view.card;
  const elapsed = view.status_changed_at
    ? formatElapsed(now - new Date(view.status_changed_at).getTime())
    : "";
  return (
    <section className="driver-view-panel" aria-label="driver view">
      <header className="driver-view-header">
        <div className="driver-view-titles">
          {card.title && <h2 className="driver-view-title">{card.title}</h2>}
          {card.subtitle && <p className="driver-view-subtitle">{card.subtitle}</p>}
        </div>
        <RunStateBadge status={view.status} />
      </header>
      {card.tags && card.tags.length > 0 && (
        <div className="driver-view-tags">
          {card.tags.map((t, i) => (
            <TagPill key={`${i}-${t.text}`} tag={t} />
          ))}
        </div>
      )}
      {(card.border_title?.text || card.border_title_secondary?.text || card.border_badge) && (
        <div className="driver-view-border">
          {card.border_title?.text && (
            <span className="border-title">{card.border_title.text}</span>
          )}
          {card.border_title_secondary?.text && (
            <span className="border-title-secondary">{card.border_title_secondary.text}</span>
          )}
          {card.border_badge && <span className="border-badge">{card.border_badge}</span>}
        </div>
      )}
      {view.status_line && (
        <footer className="driver-view-footer">
          <span className="status-line">{view.status_line}</span>
          {elapsed && (
            <span className="status-elapsed" aria-label="elapsed">
              {elapsed}
            </span>
          )}
        </footer>
      )}
    </section>
  );
}
