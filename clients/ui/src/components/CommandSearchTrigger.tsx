/**
 * CommandSearchTrigger — FR-PALETTE-TRIGGER-001 / ADR-0062 / FR-014
 *
 * Opens the command palette on click/tap via usePaletteStore.openPalette().
 * Sidebar variant (FR-014): compact Cmd/Ctrl+K hint in the brand row.
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
import { Icon } from "./icons/Icon";

export type CommandSearchTriggerProps = {
  /** "sidebar" = brand-row compact hint; "header" = legacy full search bar (removed in m2). */
  variant?: "sidebar" | "header";
};

export function CommandSearchTrigger({
  variant = "sidebar",
}: CommandSearchTriggerProps = {}): JSX.Element {
  const daemonSnapshot = useDaemonSnapshot();

  const handleClick = (e: MouseEvent<HTMLButtonElement>) => {
    usePaletteStore.getState().openPalette({
      opener: e.currentTarget,
      daemonSnapshot,
    });
  };

  const hint = isMacPlatform() ? "⌘K" : "Ctrl+K";

  return (
    <button
      type="button"
      className={[
        "command-search-trigger",
        variant === "sidebar" ? "command-search-trigger--sidebar" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      data-role="command-search-trigger"
      aria-label="Open command menu"
      onClick={handleClick}
    >
      <Icon name="search" size={14} className="command-search-trigger__icon" />
      {variant === "header" && (
        <span className="command-search-trigger__placeholder">Search commands…</span>
      )}
      <span className="command-search-trigger__hint">{hint}</span>
    </button>
  );
}
