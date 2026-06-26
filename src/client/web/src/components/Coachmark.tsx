// Coachmark — the first-run dismissible hint shown next to the KeyboardFAB
// (ADR 0072, FR-MOB-COACH-001/002).
//
// Open Question 3 grep (Tooltip / Popover / Hint / Snackbar primitives under
// src/client/web/src/components): the only dismissible surfaces are
// NotificationToast (store-driven passive toasts) and UndoSnackbar (action-undo),
// neither of which is a static one-line hint anchored to a control. Per ADR 0072
// (4) "if none exists, a minimal <div role='status'>" we implement the minimal surface
// rather than bending a toast/snackbar into a coachmark.
//
// ADR 0072 (3): the element is `<div role='status'>` with NO explicit `aria-live`
// — the terminal's single live region is AriaLiveStatus (ADR 0073); a coachmark
// `aria-live` would double-announce. `data-overlay` opts the tap out of the host
// pointer interceptor's outside-tap detection (it is an overlay, not the grid).
// The once-gate, the idempotent `hintSeen` write, and the tap-or-5s dismiss timer
// all live in useCoachmarkOnce; this component is purely the rendered surface and
// forwards a tap to `onDismiss`.

import type { JSX } from "react";

/**
 * Spec hint copy ("tap to type / two fingers to resize"), \u-escaped to satisfy
 * ADR-0049 (english-only source) while keeping the rendered string faithful.
 */
const COACHMARK_TEXT =
  "\u30BF\u30C3\u30D7\u3067\u5165\u529B / 2 \u672C\u6307\u3067\u6587\u5B57\u30B5\u30A4\u30BA";

export interface CoachmarkProps {
  /** Tap handler (useCoachmarkOnce.dismiss); ends the coachmark immediately. */
  onDismiss: () => void;
}

export function Coachmark({ onDismiss }: CoachmarkProps): JSX.Element {
  // Enter / Space also dismiss so the click handler has a keyboard counterpart;
  // the primary dismiss paths remain tap and the 5s auto timer (the KeyboardFAB
  // is the real control — this is a passive hint, never focused on its own).
  const onKeyDown = (event: { key: string }): void => {
    if (event.key === "Enter" || event.key === " ") onDismiss();
  };
  return (
    <div
      className="terminal-coachmark"
      // biome-ignore lint/a11y/useSemanticElements: ADR 0072 mandates <div role='status'> (no aria-live); an <output>/<button> would change the SR semantics from a passive hint to a control / live region.
      role="status"
      data-coachmark=""
      data-overlay=""
      onClick={onDismiss}
      onKeyDown={onKeyDown}
    >
      {COACHMARK_TEXT}
    </div>
  );
}
