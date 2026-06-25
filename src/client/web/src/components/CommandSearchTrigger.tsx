/**
 * CommandSearchTrigger — FR-PALETTE-TRIGGER-001 / ADR-0062
 *
 * A search-bar-style button in the AppShell header that:
 *  - Shows a magnifying glass icon + "Search commands…" placeholder text
 *  - Shows a keyboard hint badge (⌘K on Mac, Ctrl+K on other platforms)
 *  - Opens the command palette on click/tap via usePaletteStore.openPalette()
 *
 * ADR-0062: "New Session" is NOT a separate button here; it is a suggested
 * action surfaced inside the palette itself (palette opens at toolSelect
 * phase; the new-session tool surfaces at the top of the tool list).
 *
 * Accessibility (FR-A11Y-001):
 *  - min-width: 44px / min-height: 44px (WCAG 2.5.5 touch target, via CSS)
 *  - aria-label="Open command menu" (label separate from visible text)
 *  - data-role='command-search-trigger' for integration tests.
 */

import type { JSX, MouseEvent } from "react";
import { isMacPlatform } from "../lib/platform";
import { usePaletteStore } from "../store/palette";
import { useDaemonSnapshot } from "../store/useDaemonSnapshot";

export function CommandSearchTrigger(): JSX.Element {
  const daemonSnapshot = useDaemonSnapshot();

  const handleClick = (e: MouseEvent<HTMLButtonElement>) => {
    usePaletteStore.getState().openPalette({
      opener: e.currentTarget,
      daemonSnapshot,
    });
  };

  return (
    <button
      type="button"
      className="command-search-trigger"
      data-role="command-search-trigger"
      aria-label="Open command menu"
      onClick={handleClick}
    >
      <span className="command-search-trigger__icon" aria-hidden="true">
        🔍
      </span>
      <span className="command-search-trigger__placeholder">Search commands…</span>
      <span className="command-search-trigger__hint">{isMacPlatform() ? "⌘K" : "Ctrl+K"}</span>
    </button>
  );
}
