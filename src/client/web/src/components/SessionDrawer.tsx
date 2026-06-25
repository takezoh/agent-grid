/**
 * SessionDrawer — off-canvas drawer for mobile (<768px) session list.
 *
 * ADR-0060: Three-layer guard (AND required, OR forbidden):
 *   1. inert attribute   — blocks AT/keyboard navigation into main
 *   2. aria-hidden='true' — hides from VoiceOver rotor
 *   3. CSS class .main-content--inert { pointer-events: none }
 *      — blocks scrim-pass-through taps
 *
 * Close paths:
 *   - Selection close: row click → onSelectionClose(selectedId)
 *   - Cancel close:   scrim click / Esc keydown / left→right swipe
 *                     → onCancelClose()
 *
 * FR-DRAWER-007: if open and window.innerWidth >= 1024, calls onCancelClose
 * (defensive, idempotent; AppShell also closes via ResizeObserver with 50ms
 * debounce — both can fire without harm).
 */

import {
  type KeyboardEvent,
  type ReactNode,
  type TouchEvent,
  useCallback,
  useEffect,
  useRef,
} from "react";
import { useDaemonStore } from "../store/daemon";
import { type SwipePoint, isLeftToRightSwipe, pointFromTouch } from "../util/swipe";
import { displayLabel } from "./SessionList";

// ─── constants ────────────────────────────────────────────────────────────────

/** ID of the main content region that must be guarded while open. */
const MAIN_CONTENT_ID = "main-content";

/** CSS class applied to main content while the drawer is open. */
const INERT_CLASS = "main-content--inert";

/** Breakpoint at which the drawer must auto-close (FR-DRAWER-007). */
const DESKTOP_BREAKPOINT_PX = 1024;

/** Focusable elements selector used by the focus trap (ADR-0060 corollary). */
const FOCUSABLE_SELECTOR =
  "a[href],button:not([disabled]),input:not([disabled]),select:not([disabled])," +
  'textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

// ─── types ────────────────────────────────────────────────────────────────────

export interface SessionDrawerProps {
  open: boolean;
  /**
   * Called when a session row is selected inside the drawer (i.e. when
   * activeSessionID changes while the drawer is open).
   *
   * Arguments:
   *  - previousSessionId: the activeSessionID held immediately before the
   *    selection took effect (null when no session was previously active).
   *  - newSessionId: the activeSessionID after selection (non-null).
   *  - newSessionLabel: a human-readable label for the new session — used by
   *    UndoSnackbar's "Switched to <label>" announce (FR-TOAST-001 / FR-DRAWER-004).
   *
   * The drawer observes the daemon store's activeSessionID directly (no
   * coupling to SessionList internals) so any close path that toggles the
   * active session — including drag-to-fix-active flows — fires this hook.
   */
  onSelectionClose: (
    previousSessionId: string | null,
    newSessionId: string,
    newSessionLabel: string,
  ) => void;
  /** Called when the user cancels (scrim click / Esc / left→right swipe). */
  onCancelClose: () => void;
  children: ReactNode;
}

// ─── helpers ──────────────────────────────────────────────────────────────────

/** Return the first focusable element within a container, or null. */
function firstFocusable(container: Element): HTMLElement | null {
  return container.querySelector<HTMLElement>(FOCUSABLE_SELECTOR);
}

/** Return the hamburger toggle button if it is visible in the DOM. */
function hamburgerButton(): HTMLElement | null {
  const btn = document.querySelector<HTMLElement>("[data-role='hamburger']");
  if (!btn) return null;
  // offsetParent is null for display:none elements.
  if (btn.offsetParent === null) return null;
  return btn;
}

// ─── SessionDrawer ────────────────────────────────────────────────────────────

export function SessionDrawer({
  open,
  onSelectionClose,
  onCancelClose,
  children,
}: SessionDrawerProps): ReactNode {
  const drawerRef = useRef<HTMLDialogElement>(null);
  // m3-minor: share SwipePoint with util/swipe via type import (avoid inline duplicate).
  const touchStartRef = useRef<SwipePoint | null>(null);

  // B1: detect activeSessionID changes while the drawer is open and notify
  // the parent (AppShell) via onSelectionClose so it can:
  //   - close the drawer
  //   - capture previousActiveSessionId for UndoSnackbar (FR-TOAST-003)
  //   - announce 'Switched to <label>' on the snackbar's aria-live slot
  // We observe the daemon store directly so the SessionList selection
  // (selectSession) is the single trigger — no coupling to SessionList internals.
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const sessions = useDaemonStore((s) => s.sessions);
  const previousActiveIdRef = useRef<string | null>(activeSessionID);
  useEffect(() => {
    // Track latest seen activeSessionID; do not emit while drawer is closed.
    const prev = previousActiveIdRef.current;
    previousActiveIdRef.current = activeSessionID;
    if (!open) return;
    if (activeSessionID === null) return;
    if (prev === activeSessionID) return;
    // displayLabel chain (ADR-0033): title → subtitle → id (shared with SessionList).
    const sess = sessions.find((s) => s.id === activeSessionID);
    const label = displayLabel(sess?.view?.card ?? {}, activeSessionID);
    onSelectionClose(prev, activeSessionID, label);
  }, [open, activeSessionID, sessions, onSelectionClose]);

  // --- Guard: apply / remove three-layer guard on main content ----------------
  useEffect(() => {
    const mainEl = document.getElementById(MAIN_CONTENT_ID);
    if (!mainEl) {
      // M7: surface that ADR-0060's three-layer guard is INACTIVE when the
      // host page lacks the expected anchor. Silent skip would let AT users
      // navigate into the dimmed main content via VoiceOver rotor / keyboard
      // while believing the drawer-modal contract is intact.
      console.warn(
        '[SessionDrawer] #main-content not found — three-layer guard is INACTIVE. AppShell must render an element with id="main-content".',
      );
      return;
    }

    if (open) {
      mainEl.setAttribute("inert", "");
      mainEl.setAttribute("aria-hidden", "true");
      mainEl.classList.add(INERT_CLASS);
    } else {
      mainEl.removeAttribute("inert");
      mainEl.removeAttribute("aria-hidden");
      mainEl.classList.remove(INERT_CLASS);
    }

    return () => {
      // Always clean up on unmount regardless of current open state.
      mainEl.removeAttribute("inert");
      mainEl.removeAttribute("aria-hidden");
      mainEl.classList.remove(INERT_CLASS);
    };
  }, [open]);

  // --- Focus: move focus into drawer on open; restore on close ----------------
  useEffect(() => {
    if (open) {
      const drawer = drawerRef.current;
      if (!drawer) return;
      const first = firstFocusable(drawer);
      if (first) {
        first.focus();
      } else {
        drawer.focus();
      }
    } else {
      // Restore focus to hamburger (if visible) or first focusable in header.
      const hamburger = hamburgerButton();
      if (hamburger) {
        hamburger.focus();
        return;
      }
      const headerArea = document.querySelector<HTMLElement>(".app-header-inner");
      if (headerArea) {
        const first = firstFocusable(headerArea);
        if (first) first.focus();
      }
    }
  }, [open]);

  // --- FR-DRAWER-007: auto-close on viewport >= DESKTOP_BREAKPOINT_PX --------
  useEffect(() => {
    if (!open) return;

    const handleResize = () => {
      if (window.innerWidth >= DESKTOP_BREAKPOINT_PX) {
        onCancelClose();
      }
    };

    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
    };
  }, [open, onCancelClose]);

  // --- Keyboard handler (Esc → cancel close; Tab/Shift+Tab → focus trap) ------
  // M1 (focus trap, FR-DRAWER-001 corollary): keep focus inside the drawer
  // subtree while open so AT and keyboard users cannot Tab into the inert
  // main content (three-layer guard already blocks pointer / aria, but Tab
  // order in some browsers still reaches inert subtrees on first press).
  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLDialogElement>) => {
      if (e.key === "Escape") {
        e.preventDefault(); // prevent native <dialog> close (we handle it)
        onCancelClose();
        return;
      }
      if (e.key !== "Tab") return;
      const drawer = drawerRef.current;
      if (!drawer) return;
      const focusables = Array.from(
        drawer.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
      ).filter((el) => el.offsetParent !== null || el === drawer);
      if (focusables.length === 0) {
        // Keep focus on the dialog container itself.
        e.preventDefault();
        drawer.focus();
        return;
      }
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (first === undefined || last === undefined) return;
      const active = document.activeElement as HTMLElement | null;
      // Determine wrap direction. We always preventDefault when wrapping,
      // and additionally when focus has somehow leaked outside the drawer
      // (active not in subtree) — pull it back to the boundary.
      const inSubtree = active !== null && drawer.contains(active);
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
    [onCancelClose],
  );

  // --- Touch handlers (left→right swipe → cancel close) -----------------------
  const handleTouchStart = useCallback((e: TouchEvent<HTMLDialogElement>) => {
    const t = e.touches[0];
    if (t) {
      touchStartRef.current = pointFromTouch(t);
    }
  }, []);

  const handleTouchEnd = useCallback(
    (e: TouchEvent<HTMLDialogElement>) => {
      const start = touchStartRef.current;
      touchStartRef.current = null;
      if (!start) return;
      const t = e.changedTouches[0];
      if (!t) return;
      const end = pointFromTouch(t);
      if (isLeftToRightSwipe(start, end)) {
        onCancelClose();
      }
    },
    [onCancelClose],
  );

  // --- Scrim click / keyboard handler -----------------------------------------
  const handleScrimClick = useCallback(() => {
    onCancelClose();
  }, [onCancelClose]);

  const handleScrimKeyDown = useCallback(
    (e: KeyboardEvent<HTMLButtonElement>) => {
      if (e.key === "Enter" || e.key === " ") {
        onCancelClose();
      }
    },
    [onCancelClose],
  );

  // Do not render anything when closed (no DOM footprint).
  if (!open) {
    return null;
  }

  return (
    <>
      {/*
       * Scrim — covers content behind the drawer.
       * Implemented as a visually hidden button so both mouse and keyboard
       * users can dismiss (satisfies useKeyWithClickEvents lint rule).
       * aria-label is "Close sessions drawer" to provide accessible name
       * while keeping it outside the dialog for structural clarity.
       */}
      <button
        type="button"
        className="session-drawer__scrim"
        aria-label="Close sessions drawer"
        onClick={handleScrimClick}
        onKeyDown={handleScrimKeyDown}
      />

      {/* Drawer — native <dialog> for semantic role=dialog (useSemanticElements) */}
      <dialog
        ref={drawerRef}
        aria-modal="true"
        aria-label="Sessions"
        className="session-drawer session-drawer--open"
        // tabIndex allows the drawer container itself to receive focus as fallback.
        tabIndex={-1}
        onKeyDown={handleKeyDown}
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
        // open prop keeps the <dialog> in the non-modal state (we manage focus/guard manually)
        open
      >
        <div className="session-drawer__slide">{children}</div>
      </dialog>
    </>
  );
}
