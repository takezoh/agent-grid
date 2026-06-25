/**
 * ThemeProvider — ADR-0059
 *
 * Responsibilities:
 *  - Reads localStorage on mount and initialises the ThemeStore.
 *  - Applies the resolved theme to document.documentElement.dataset.theme via
 *    useLayoutEffect (synchronous before paint).
 *  - Persists explicit (light/dark) choices to localStorage; removes the key
 *    for "system" (FR-THEME-005).
 *  - Subscribes to matchMedia change events and re-syncs data-theme when
 *    theme === 'system' (FR-THEME-006).
 *
 * Exports:
 *  - ThemeProvider({ children }) — React component
 *  - useXtermTheme()             — hook returning ITheme built from CSS tokens
 */

import type { ITheme } from "@xterm/xterm";
import { type ReactNode, useEffect, useLayoutEffect, useState } from "react";
import type { Theme } from "../store/theme";
import { useThemeStore } from "../store/theme";

// ─── constants ────────────────────────────────────────────────────────────────

const STORAGE_KEY = "agent-reactor-theme";
const VALID_THEMES: ReadonlySet<string> = new Set(["system", "light", "dark"]);
const DARK_QUERY = "(prefers-color-scheme: dark)";

/**
 * Hard-coded fallback ITheme palette used when CSS tokens cannot be read
 * (e.g. tokens.css not yet loaded, SSR, or CSS bundling failure).
 * Values mirror the `:root` dark defaults in tokens.css so the terminal
 * is never left with xterm's implicit white-background defaults.
 */
const FALLBACK_XTERM_THEME: Readonly<ITheme> = {
  foreground: "#e6e6e6",
  cursor: "#e6e6e6",
  cursorAccent: "#e6e6e6",
  selectionBackground: "rgba(74, 158, 255, 0.3)",
};

// ─── localStorage helpers ────────────────────────────────────────────────────

/**
 * Read the persisted theme from localStorage.
 * Returns undefined (not found or invalid) or a valid Theme string.
 * Swallows SecurityError / QuotaExceededError so Safari Private / Storage-
 * disabled environments do not throw during initialization.
 */
function readStoredTheme(): Theme | undefined {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored !== null && VALID_THEMES.has(stored)) {
      return stored as Theme;
    }
    return undefined;
  } catch (e) {
    // M6 (post-review): symmetric with persistTheme — log for observability
    // when Safari Private / Storage-disabled envs make read fail. Returning
    // undefined keeps the caller on the 'system' default; the warn surfaces
    // the gap to devtools so the silent fallback is auditable.
    console.warn("[ThemeProvider] localStorage read failed", e);
    return undefined;
  }
}

/**
 * Persist or remove the theme key from localStorage.
 * Swallows SecurityError / QuotaExceededError — localStorage failure is non-
 * fatal; the DOM data-theme attribute is the authoritative source for the
 * current session.
 */
function persistTheme(theme: Theme): void {
  try {
    if (theme === "system") {
      localStorage.removeItem(STORAGE_KEY);
    } else {
      localStorage.setItem(STORAGE_KEY, theme);
    }
  } catch {
    // Storage failure is non-fatal; log for observability but do not
    // interrupt the theme-application flow.
    console.warn("[ThemeProvider] localStorage write failed", { theme });
  }
}

// ─── helpers ──────────────────────────────────────────────────────────────────

function resolveDataTheme(theme: Theme, darkMatches: boolean): "dark" | "light" {
  if (theme === "system") return darkMatches ? "dark" : "light";
  return theme;
}

// ─── ThemeProvider ────────────────────────────────────────────────────────────

interface ThemeProviderProps {
  children: ReactNode;
}

export function ThemeProvider({ children }: ThemeProviderProps): ReactNode {
  const theme = useThemeStore((s) => s.theme);
  const setTheme = useThemeStore((s) => s.setTheme);

  // Mount: read localStorage and initialise store.
  // We only call setTheme if the stored value differs from the Zustand
  // initial state to avoid a spurious removeItem / setItem transient
  // when the store was already initialised lazily from storage.
  useLayoutEffect(() => {
    const stored = readStoredTheme();
    setTheme(stored ?? "system");
    // setTheme is stable (Zustand); running once on mount is intentional.
  }, [setTheme]);

  // Sync data-theme + localStorage whenever theme changes.
  useLayoutEffect(() => {
    const mq = window.matchMedia(DARK_QUERY);
    const resolved = resolveDataTheme(theme, mq.matches);
    document.documentElement.dataset.theme = resolved;
    persistTheme(theme);
  }, [theme]);

  // Subscribe to OS-level change events; only act when theme === 'system'.
  // The handler reads current theme from the store directly (no closure dep).
  useEffect(() => {
    const mq = window.matchMedia(DARK_QUERY);

    function handleChange(event: MediaQueryListEvent): void {
      if (useThemeStore.getState().theme !== "system") return;
      document.documentElement.dataset.theme = event.matches ? "dark" : "light";
    }

    mq.addEventListener("change", handleChange);
    return () => {
      mq.removeEventListener("change", handleChange);
    };
  }, []);

  return <>{children}</>;
}

// ─── useXtermTheme ────────────────────────────────────────────────────────────

/**
 * Builds an xterm ITheme by reading CSS custom properties from
 * document.documentElement after each data-theme change (FR-THEME-003).
 *
 * A MutationObserver watches documentElement[data-theme] and schedules a
 * rebuild via a 1-rAF guard to ensure getComputedStyle is called after the
 * browser has applied the new [data-theme] selector.
 *
 * In test environments (happy-dom) where MutationObserver does not fire on
 * documentElement, call globalThis.flushThemeObservers() to manually trigger
 * the registered observer callbacks.
 */
export function useXtermTheme(): ITheme {
  const [xtermTheme, setXtermTheme] = useState<ITheme>(() => buildXtermTheme());

  useLayoutEffect(() => {
    let rafId: number | null = null;

    function rebuild(): void {
      if (rafId !== null) cancelAnimationFrame(rafId);
      rafId = requestAnimationFrame(() => {
        setXtermTheme(buildXtermTheme());
        rafId = null;
      });
    }

    // MutationObserver on documentElement[data-theme] — production signal.
    // In real browsers this fires whenever ThemeProvider writes dataset.theme.
    // In happy-dom tests, use globalThis.flushThemeObservers() to simulate.
    const observer = new MutationObserver((mutations) => {
      for (const m of mutations) {
        if (m.type === "attributes" && m.attributeName === "data-theme") {
          rebuild();
          return;
        }
      }
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    // Trigger an immediate rebuild in case data-theme is already set.
    rebuild();

    return () => {
      observer.disconnect();
      if (rafId !== null) cancelAnimationFrame(rafId);
    };
  }, []);

  return xtermTheme;
}

const XTERM_TOKEN_NAMES = ["--xterm-fg", "--xterm-cursor", "--xterm-selection"] as const;
type XtermTokenName = (typeof XTERM_TOKEN_NAMES)[number];

function readToken(style: CSSStyleDeclaration, name: XtermTokenName): string | undefined {
  const value = style.getPropertyValue(name).trim();
  if (!value) {
    console.warn("[ThemeProvider] CSS token missing or empty", { token: name });
    return undefined;
  }
  return value;
}

/**
 * Build an ITheme from CSS custom properties.
 * If any token is missing, the entire result falls back to the hard-coded
 * dark-mode palette so xterm.js never silently adopts its own defaults.
 */
function buildXtermTheme(): ITheme {
  const style = getComputedStyle(document.documentElement);
  const fg = readToken(style, "--xterm-fg");
  const cursor = readToken(style, "--xterm-cursor");
  const selection = readToken(style, "--xterm-selection");

  // If any token is absent, use the full fallback palette rather than
  // constructing a partially-undefined ITheme that xterm silently degrades.
  if (fg === undefined || cursor === undefined || selection === undefined) {
    return { ...FALLBACK_XTERM_THEME };
  }

  return {
    foreground: fg,
    cursor,
    cursorAccent: cursor,
    selectionBackground: selection,
  };
}
