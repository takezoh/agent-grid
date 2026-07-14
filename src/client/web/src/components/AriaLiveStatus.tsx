// AriaLiveStatus — the terminal's single visually-hidden aria-live='polite' slot
// (ADR 0073, role-separated from the palette's slot per ADR 0057).
//
// ADR 0057 mandates exactly one aria-live inside the command palette; this is a
// *different* slot with a *different* role: the palette announces open/close,
// the terminal announces mode changes (e.g. 'Returned to view mode'). The two
// never emit simultaneously in the current architecture, so role separation is
// preserved without a shared channel.
//
// The text comes from useAnnouncer (debounced upstream). `key={seq}` remounts the
// inner node on every accepted emit so an identical re-announce still reaches the
// screen reader. The element is visually hidden via terminal-mode.css.

import type { JSX } from "react";
import "../css/terminal-mode.css";
import { useAnnouncer } from "../hooks/useAnnouncer";

export function AriaLiveStatus(): JSX.Element {
  const { text, seq } = useAnnouncer();

  return (
    <div
      className="terminal-aria-live"
      aria-live="polite"
      aria-atomic="true"
      data-testid="terminal-aria-live"
    >
      {text === "" ? null : <span key={seq}>{text}</span>}
    </div>
  );
}
