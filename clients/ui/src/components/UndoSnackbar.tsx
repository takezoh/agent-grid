/**
 * UndoSnackbar — FR-TOAST-001 / FR-TOAST-003 / FR-DRAWER-004 / ADR-0063
 *
 * Responsibilities:
 *  - Visible when previousActiveSessionId is non-null.
 *  - Announces 'Switched to <label>' in an independent aria-live='polite'
 *    role='status' slot (live region) — FR-TOAST-001.
 *  - Undo button is a sibling element outside the live region — FR-TOAST-001.
 *  - Auto-dismisses after 5 seconds via onDismiss callback.
 *  - Undo button click invokes onUndo (not onDismiss) — FR-DRAWER-004.
 *  - Min touch target: 44x44px on the Undo button — FR-A11Y-001.
 */

import { useEffect } from "react";

// ─── constants ────────────────────────────────────────────────────────────────

const AUTO_DISMISS_MS = 5000;

// ─── types ────────────────────────────────────────────────────────────────────

export interface UndoSnackbarProps {
  /** When non-null, the snackbar is visible. The session ID being undone to. */
  previousActiveSessionId: string | null;
  /** Human-readable label of the previous session. */
  previousLabel: string | null;
  /** Called when the user clicks Undo. Does NOT dismiss the snackbar. */
  onUndo: () => void;
  /** Called when the snackbar auto-dismisses (5s timeout). */
  onDismiss: () => void;
}

// ─── UndoSnackbar ─────────────────────────────────────────────────────────────

/**
 * UndoSnackbar renders only when previousActiveSessionId is non-null.
 * It splits into two sibling elements:
 *  1. Live region (role=status / aria-live=polite) for passive announcement.
 *  2. Actions wrapper (outside live region) for interactive Undo button.
 */
export function UndoSnackbar({
  previousActiveSessionId,
  previousLabel,
  onUndo,
  onDismiss,
}: UndoSnackbarProps): JSX.Element | null {
  // Auto-dismiss after 5 seconds — cleared on unmount or when deps change.
  useEffect(() => {
    if (previousActiveSessionId === null) return;

    const t = setTimeout(() => {
      onDismiss();
    }, AUTO_DISMISS_MS);

    return () => {
      clearTimeout(t);
    };
  }, [previousActiveSessionId, onDismiss]);

  if (previousActiveSessionId === null) {
    return null;
  }

  const displayLabel = previousLabel ?? previousActiveSessionId;

  return (
    <div className="undo-snackbar">
      {/* Live region: passive announcement only — FR-TOAST-001 / ADR-0063 */}
      {/* biome-ignore lint/a11y/useSemanticElements: spec requires explicit role='status' aria-live='polite' (FR-TOAST-001); <output> has different semantics */}
      <div role="status" aria-live="polite" className="undo-snackbar__status">
        Switched to {displayLabel}
      </div>

      {/* Actions: interactive area outside live region — FR-TOAST-001 */}
      <div className="undo-snackbar__actions">
        <button type="button" className="undo-snackbar__undo-btn" onClick={onUndo}>
          Undo
        </button>
      </div>
    </div>
  );
}
