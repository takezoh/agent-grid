/**
 * AppShell — FR-LAYOUT-001/003/004 / ADR-0059 / ADR-0062
 *
 * Responsibilities:
 *  - Wraps children with ThemeProvider (data-theme propagation).
 *  - Provides the named-grid-area skeleton with breakpoint switching:
 *      <768px:   banner / header / main  (sidebar off-canvas, hamburger visible)
 *      768-1023: banner banner / header header / sidebar main
 *      >=1024:   banner banner / header header / sidebar main
 *  - Maintains drawerOpen as UI-local useState.
 *  - Monitors window width via ResizeObserver and closes the drawer with a
 *    50ms debounce when crossing the 768px breakpoint (FR-DRAWER-007 prep).
 */

import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { SessionDrawer } from "./SessionDrawer";
import { ThemeProvider } from "./ThemeProvider";

// ─── constants ────────────────────────────────────────────────────────────────

const MOBILE_BREAKPOINT_PX = 768;
const DRAWER_DEBOUNCE_MS = 50;

/**
 * Desktop sidebar collapse persistence (user decision 2026-07-25): the
 * sidebar defaults to visible in every mode (browser AND hosted), can be
 * toggled, and the choice survives restarts. localStorage is the cache in
 * both contexts (the Electron Workspace partition persists it too).
 */
const SIDEBAR_COLLAPSED_KEY = "agent-grid.sidebar-collapsed";

function readSidebarCollapsed(): boolean {
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "true";
  } catch {
    return false;
  }
}

function writeSidebarCollapsed(collapsed: boolean): void {
  try {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(collapsed));
  } catch {
    /* storage unavailable (private mode) — toggle still works in-session */
  }
}

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
   * Marks the document for hosted-only CSS (titlebar drag region, OS-style
   * scrollbars). It does NOT affect sidebar visibility — the sidebar renders
   * identically in browser and hosted modes (user decision 2026-07-25).
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
  // Desktop sidebar collapse — default visible, persisted across launches.
  const [sidebarCollapsed, setSidebarCollapsed] = useState(readSidebarCollapsed);

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

  const toggleSidebarCollapsed = useCallback(() => {
    setSidebarCollapsed((prev) => {
      const next = !prev;
      writeSidebarCollapsed(next);
      return next;
    });
  }, []);

  const handleSelectionClose = useCallback(() => {
    setDrawerOpen(false);
  }, []);

  const handleCancelClose = useCallback(() => {
    setDrawerOpen(false);
  }, []);

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
        data-sidebar-collapsed={sidebarCollapsed ? "true" : "false"}
        data-hosted={hosted ? "true" : "false"}
      >
        {/* banner: spans full width */}
        <div className="app-banner">{banner}</div>

        {/* header: HeaderBar + hamburger (left on mobile, FR-013) */}
        <div className="app-header-area app-drag-region">
          <div className="app-header-inner">
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
            {/* Desktop (>=768px) counterpart of the hamburger: collapses the
                sidebar column in place. Visibility is swapped with the
                hamburger purely via CSS breakpoints. */}
            <button
              type="button"
              className="hamburger-toggle sidebar-toggle app-no-drag"
              data-role="sidebar-toggle"
              aria-label="Toggle sidebar"
              aria-expanded={sidebarCollapsed ? "false" : "true"}
              onClick={toggleSidebarCollapsed}
            >
              ☰
            </button>
            <div className="app-no-drag app-header-content">{header}</div>
          </div>
        </div>

        {/* sidebar: hidden on <768px via CSS; shown as drawer via SessionDrawer.
            Rendered in hosted mode too — visibility is owned solely by the
            collapse toggle + breakpoints. */}
        <div
          className="app-sidebar"
          data-drawer-open={drawerOpen ? "true" : "false"}
          aria-hidden={drawerOpen ? "false" : undefined}
        >
          {sidebar}
        </div>

        {/* main content — id used by SessionDrawer for three-layer guard */}
        <div id="main-content" className="app-main">
          {main}
        </div>
      </div>

      {/* SessionDrawer — off-canvas drawer on mobile (FR-DRAWER-001/002/ADR-0060) */}
      <SessionDrawer
        open={drawerOpen}
        onSelectionClose={handleSelectionClose}
        onCancelClose={handleCancelClose}
      >
        {sidebar}
      </SessionDrawer>

      {/* Portal-mounted overlays (palette, toasts) */}
      {overlays}
    </ThemeProvider>
  );
}
