/**
 * AppShell — FR-LAYOUT-001/003/004 / ADR-0059 / ADR-0062
 *
 * Responsibilities:
 *  - Wraps children with ThemeProvider (data-theme propagation).
 *  - Provides the named-grid-area skeleton with breakpoint switching:
 *      <768px:   banner / header / main  (sidebar off-canvas, hamburger visible)
 *      768-1023: banner banner / header header / sidebar main
 *      >=1024:   banner banner / header header / sidebar main
 *  - Maintains drawerOpen / previousActiveSessionId as UI-local useState
 *    (FR-STORE-001: NOT in Zustand store).
 *  - Monitors window width via ResizeObserver and closes the drawer with a
 *    50ms debounce when crossing the 768px breakpoint (FR-DRAWER-007 prep).
 */

import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useDaemonStore } from "../store/daemon";
import { SessionDrawer } from "./SessionDrawer";
import { ThemeProvider } from "./ThemeProvider";
import { UndoSnackbar } from "./UndoSnackbar";

// ─── constants ────────────────────────────────────────────────────────────────

const MOBILE_BREAKPOINT_PX = 768;
const DRAWER_DEBOUNCE_MS = 50;

// ─── types ────────────────────────────────────────────────────────────────────

export interface AppShellProps {
  /** Session list panel (sidebar / off-canvas drawer on mobile). */
  sidebar: ReactNode;
  /** Top banner for status messages (spans full width). */
  banner: ReactNode;
  /** Header bar (command trigger, theme control, hamburger on mobile). */
  header: ReactNode;
  /** Main content area (terminal / tabs). */
  main: ReactNode;
  /** Portal-mounted overlays (command palette, toasts). */
  overlays?: ReactNode;
  /**
   * Electron Workspace hosted mode (contract-hosted-mode-existing-spa-compat).
   * Suppresses session-list chrome; browser path must remain unchanged when false.
   */
  hosted?: boolean;
}

// ─── AppShell ─────────────────────────────────────────────────────────────────

/**
 * AppShell lays out the four named grid areas (banner / header / sidebar /
 * main) and owns the mobile-drawer open/close state.
 */
export function AppShell({
  sidebar,
  banner,
  header,
  main,
  overlays,
  hosted = false,
}: AppShellProps): ReactNode {
  // UI-local state — FR-STORE-001: do NOT lift to Zustand.
  const [drawerOpen, setDrawerOpen] = useState(false);
  // previousActiveSessionId tracks the active session immediately BEFORE the
  // drawer-selection event that closed the drawer (FR-DRAWER-004 / FR-TOAST-003).
  // When non-null, UndoSnackbar renders with a 5s auto-dismiss; clicking Undo
  // calls daemonStore.selectSession(previousActiveSessionId) and clears the slot.
  // Maintained here per FR-STORE-001; populated by the drawer's onSelectionClose hook.
  const [previousActiveSessionId, setPreviousActiveSessionId] = useState<string | null>(null);
  const [previousLabel, setPreviousLabel] = useState<string | null>(null);
  // Slot element mounted by NotificationToast — we read it via querySelector
  // after each render so we can portal the snackbar into the right slot.
  const [snackbarSlot, setSnackbarSlot] = useState<HTMLElement | null>(null);

  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const shellRef = useRef<HTMLDivElement>(null);

  // Close drawer with debounce when window width crosses >= MOBILE_BREAKPOINT_PX.
  // This prepares the coordination contract with m3-session-drawer (FR-DRAWER-007).
  const handleWidthChange = useCallback(
    (width: number) => {
      if (width >= MOBILE_BREAKPOINT_PX && drawerOpen) {
        if (debounceTimerRef.current !== null) {
          clearTimeout(debounceTimerRef.current);
        }
        debounceTimerRef.current = setTimeout(() => {
          setDrawerOpen(false);
          debounceTimerRef.current = null;
        }, DRAWER_DEBOUNCE_MS);
      }
    },
    [drawerOpen],
  );

  // ResizeObserver on the shell element — window width monitoring.
  useEffect(() => {
    const el = shellRef.current;
    if (!el) return;

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) {
        handleWidthChange(entry.contentRect.width);
      }
    });

    observer.observe(el);

    return () => {
      observer.disconnect();
      if (debounceTimerRef.current !== null) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, [handleWidthChange]);

  const toggleDrawer = useCallback(() => {
    setDrawerOpen((prev) => !prev);
  }, []);

  // B1 / B2 wire-up: drawer reports (previousId, newId, label) when
  // activeSessionID changes during the open period. We capture previousId +
  // label and close the drawer in one tick so UndoSnackbar can render with
  // an accurate "Switched to <label>" announce (FR-TOAST-001).
  const handleSelectionClose = useCallback(
    (previousSessionId: string | null, _newSessionId: string, newLabel: string) => {
      setPreviousActiveSessionId(previousSessionId);
      setPreviousLabel(newLabel);
      setDrawerOpen(false);
    },
    [],
  );

  const handleCancelClose = useCallback(() => {
    setDrawerOpen(false);
  }, []);

  // B2: Undo restores the previous activeSessionID via the daemon store
  // (the only owner of activeSessionID — see web_active_session_ownership
  // memory). We then clear the snackbar slot so it auto-hides immediately.
  const handleUndo = useCallback(() => {
    const prev = previousActiveSessionId;
    setPreviousActiveSessionId(null);
    setPreviousLabel(null);
    if (prev !== null) {
      useDaemonStore.getState().selectSession(prev);
    }
  }, [previousActiveSessionId]);

  const handleSnackbarDismiss = useCallback(() => {
    setPreviousActiveSessionId(null);
    setPreviousLabel(null);
  }, []);

  // Locate the NotificationToast's reserved slot once it lands in the DOM.
  // We do this on every render via a callback ref-like effect; the slot is
  // mounted by NotificationToast (sibling under overlays), so it is available
  // after the first commit.
  useEffect(() => {
    const slot = document.querySelector<HTMLElement>(".notification-toast__undosnackbar-slot");
    if (slot !== snackbarSlot) setSnackbarSlot(slot);
  });

  // Hosted mode: mark document for CSS (titlebar drag, OS scrollbar, no session chrome).
  useEffect(() => {
    if (!hosted) return;
    document.documentElement.dataset.hosted = "1";
    document.body.dataset.hosted = "1";
    return () => {
      delete document.documentElement.dataset.hosted;
      delete document.body.dataset.hosted;
    };
  }, [hosted]);

  return (
    <ThemeProvider>
      <div
        ref={shellRef}
        className="app-shell"
        data-drawer-open={drawerOpen ? "true" : "false"}
        data-hosted={hosted ? "true" : "false"}
      >
        {/* banner: spans full width */}
        <div className="app-banner">{banner}</div>

        {/* header: HeaderBar + hamburger (left on mobile, FR-013) */}
        <div className="app-header-area app-drag-region">
          <div className="app-header-inner">
            {!hosted && (
              <button
                type="button"
                className="hamburger-toggle app-no-drag"
                data-role="hamburger"
                aria-label="Open sessions"
                aria-expanded={drawerOpen ? "true" : "false"}
                onClick={toggleDrawer}
              >
                ☰
              </button>
            )}
            <div className="app-no-drag app-header-content">{header}</div>
          </div>
        </div>

        {/* sidebar: hidden on <768px via CSS; shown as drawer via SessionDrawer */}
        {!hosted && (
          <div
            className="app-sidebar"
            data-drawer-open={drawerOpen ? "true" : "false"}
            aria-hidden={drawerOpen ? "false" : undefined}
          >
            {sidebar}
          </div>
        )}

        {/* main content — id used by SessionDrawer for three-layer guard */}
        <div id="main-content" className="app-main">
          {main}
        </div>
      </div>

      {/* SessionDrawer — off-canvas drawer on mobile (FR-DRAWER-001/002/ADR-0060) */}
      {!hosted && (
        <SessionDrawer
          open={drawerOpen}
          onSelectionClose={handleSelectionClose}
          onCancelClose={handleCancelClose}
        >
          {sidebar}
        </SessionDrawer>
      )}

      {/* Portal-mounted overlays (palette, toasts) */}
      {overlays}

      {/* UndoSnackbar — portaled into NotificationToast's reserved slot so
          the 3 aria-live streams stay isolated (FR-TOAST-003). The slot is
          rendered by NotificationToast which is itself inside `overlays`,
          so the slot is available after overlays mount on the same tick. */}
      {previousActiveSessionId !== null &&
        snackbarSlot !== null &&
        createPortal(
          <UndoSnackbar
            previousActiveSessionId={previousActiveSessionId}
            previousLabel={previousLabel}
            onUndo={handleUndo}
            onDismiss={handleSnackbarDismiss}
          />,
          snackbarSlot,
        )}
    </ThemeProvider>
  );
}
