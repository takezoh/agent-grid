/**
 * ConfirmDialog — 汎用 confirm modal.
 *
 * SessionDrawer (`SessionDrawer.tsx`) と同じ三層 pattern を踏襲:
 *   - native `<dialog aria-modal="true">` (semantic dialog role)
 *   - scrim を視覚的にも button にして mouse + keyboard で dismiss
 *   - Esc → cancel
 *   - Focus trap: Tab / Shift+Tab で dialog 内を wrap
 *   - close 時に opener (props.openerRef) に focus を戻す
 *
 * variant:
 *   - "modal":  PC 向け中央寄せ modal (デフォルト)
 *   - "sheet":  mobile 向け bottom sheet (full-width, slide-up)
 *
 * Destructive action 用 (`destructive: true`) で confirm button に
 * destructive variant の class を付ける. open 時は cancel button に
 * 初期フォーカス (デフォルト破壊回避).
 */

import {
  type KeyboardEvent,
  type ReactNode,
  type RefObject,
  useCallback,
  useEffect,
  useRef,
} from "react";
import "../css/confirm-dialog.css";

const FOCUSABLE_SELECTOR =
  "a[href],button:not([disabled]),input:not([disabled]),select:not([disabled])," +
  'textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

export type ConfirmDialogVariant = "modal" | "sheet";

export interface ConfirmDialogProps {
  open: boolean;
  /** Dialog title (h2 で render される). */
  title: string;
  /** 本文. plain text or ReactNode. */
  body: ReactNode;
  confirmLabel: string;
  cancelLabel: string;
  /** Confirm button を destructive style にする. */
  destructive?: boolean;
  /** 処理中表示. pending=true の間は両 button disabled, confirm label 差替え. */
  pending?: boolean;
  /** Pending 時の confirm button text (例: "終了中…"). */
  pendingLabel?: string;
  /** Confirm 押下時. */
  onConfirm: () => void;
  /** Cancel / Esc / scrim. */
  onCancel: () => void;
  /** Close 時に focus を戻す要素. nullable. */
  openerRef?: RefObject<HTMLElement | null>;
  variant?: ConfirmDialogVariant;
}

function firstFocusable(container: Element): HTMLElement | null {
  return container.querySelector<HTMLElement>(FOCUSABLE_SELECTOR);
}

export function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel,
  cancelLabel,
  destructive = false,
  pending = false,
  pendingLabel,
  onConfirm,
  onCancel,
  openerRef,
  variant = "modal",
}: ConfirmDialogProps): ReactNode {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const cancelBtnRef = useRef<HTMLButtonElement>(null);

  // open 時: cancel button に focus.
  // close 時: openerRef があれば opener に focus を戻す.
  useEffect(() => {
    if (open) {
      const dialog = dialogRef.current;
      if (!dialog) return;
      const focusTarget = cancelBtnRef.current ?? firstFocusable(dialog) ?? dialog;
      focusTarget.focus();
    } else {
      const opener = openerRef?.current;
      if (opener && typeof opener.focus === "function") {
        opener.focus();
      }
    }
  }, [open, openerRef]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLDialogElement>) => {
      if (e.key === "Escape") {
        e.preventDefault();
        if (!pending) onCancel();
        return;
      }
      if (e.key !== "Tab") return;
      const dialog = dialogRef.current;
      if (!dialog) return;
      const focusables = Array.from(
        dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
      ).filter((el) => el.offsetParent !== null || el === dialog);
      if (focusables.length === 0) {
        e.preventDefault();
        dialog.focus();
        return;
      }
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (!first || !last) return;
      const active = document.activeElement as HTMLElement | null;
      const inSubtree = active !== null && dialog.contains(active);
      if (!inSubtree) {
        e.preventDefault();
        (e.shiftKey ? last : first).focus();
        return;
      }
      if (e.shiftKey && active === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    },
    [onCancel, pending],
  );

  const handleScrimClick = useCallback(() => {
    if (!pending) onCancel();
  }, [onCancel, pending]);

  const handleScrimKeyDown = useCallback(
    (e: KeyboardEvent<HTMLButtonElement>) => {
      if ((e.key === "Enter" || e.key === " ") && !pending) {
        onCancel();
      }
    },
    [onCancel, pending],
  );

  if (!open) return null;

  const confirmText = pending && pendingLabel ? pendingLabel : confirmLabel;

  return (
    <>
      <button
        type="button"
        className="confirm-dialog__scrim"
        aria-label="Close dialog"
        onClick={handleScrimClick}
        onKeyDown={handleScrimKeyDown}
        data-variant={variant}
      />
      <dialog
        ref={dialogRef}
        className={`confirm-dialog confirm-dialog--${variant}`}
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-body"
        tabIndex={-1}
        onKeyDown={handleKeyDown}
        open
      >
        <div className="confirm-dialog__panel">
          <h2 id="confirm-dialog-title" className="confirm-dialog__title">
            {title}
          </h2>
          <div id="confirm-dialog-body" className="confirm-dialog__body">
            {body}
          </div>
          <div className="confirm-dialog__actions">
            <button
              ref={cancelBtnRef}
              type="button"
              className="confirm-dialog__btn confirm-dialog__btn--cancel"
              onClick={onCancel}
              disabled={pending}
            >
              {cancelLabel}
            </button>
            <button
              type="button"
              className={`confirm-dialog__btn confirm-dialog__btn--confirm${destructive ? " confirm-dialog__btn--destructive" : ""}`}
              onClick={onConfirm}
              disabled={pending}
              data-pending={pending ? "true" : "false"}
            >
              {confirmText}
            </button>
          </div>
        </div>
      </dialog>
    </>
  );
}
