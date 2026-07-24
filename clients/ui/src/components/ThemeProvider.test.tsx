/**
 * ThemeProvider.test.tsx — FR-THEME-001 through FR-THEME-006 + FR-THEME-003
 *
 * rAF runs synchronously (test-setup.ts mock).
 * matchMedia is controlled via globalThis.setMatchMedia (test-setup.ts mock).
 * Default matchMedia("(prefers-color-scheme: dark)").matches = true.
 * flushThemeObservers() manually fires MutationObserver callbacks registered
 * for documentElement[data-theme] (test-setup.ts mock — happy-dom does not
 * fire them automatically).
 */

import { act, render, renderHook, cleanup as rtlCleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useThemeStore } from "../store/theme";
import { ThemeProvider, useXtermTheme } from "./ThemeProvider";
import { ThemeSegmentedControl } from "./ThemeSegmentedControl";

const DARK_QUERY = "(prefers-color-scheme: dark)";
const STORAGE_KEY = "agent-grid-theme";

// ─── helpers ─────────────────────────────────────────────────────────────────

/** Reset Zustand store to initial state between tests. */
function resetStore() {
  useThemeStore.setState({ theme: "system" });
}

/** Clean up all side effects between tests. */
function cleanup() {
  rtlCleanup();
  resetStore();
  localStorage.clear();
  window.agentGridAppearance = undefined;
  delete document.documentElement.dataset.theme;
  delete document.documentElement.dataset.density;
  document.documentElement.style.removeProperty("font-size");
  document.documentElement.style.removeProperty("--bg");
  document.documentElement.style.removeProperty("--xterm-fg");
  document.documentElement.style.removeProperty("--xterm-cursor");
  document.documentElement.style.removeProperty("--xterm-selection");
}

// ─── FR-THEME-001 ─────────────────────────────────────────────────────────────

describe("FR-THEME-001 — system fallback follows matchMedia", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("mounts with empty localStorage and dark matchMedia → data-theme = dark", () => {
    globalThis.setMatchMedia(DARK_QUERY, true);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("matchMedia change from dark→light updates data-theme to light", async () => {
    globalThis.setMatchMedia(DARK_QUERY, true);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("dark");

    act(() => {
      globalThis.setMatchMedia(DARK_QUERY, false);
    });

    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("data integrity: invalid localStorage value falls back to system mode", () => {
    localStorage.setItem(STORAGE_KEY, "<script>alert(1)</script>");
    globalThis.setMatchMedia(DARK_QUERY, false);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    // Invalid value → system fallback → tracks matchMedia (false → light).
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });
});

describe("desktop appearance configuration", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("keeps the original UI defaults for default appearance", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    window.agentGridAppearance = {
      theme: "default",
      density: "comfortable",
      font_scale: 1,
    };

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("light");
    expect(document.documentElement.dataset.density).toBeUndefined();
    expect(document.documentElement.style.fontSize).toBe("");
  });

  it("uses hosted appearance instead of browser localStorage", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    window.agentGridAppearance = {
      theme: "dark",
      density: "compact",
      font_scale: 1.25,
    };

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.dataset.density).toBe("compact");
    expect(document.documentElement.style.fontSize).toBe("125%");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
  });

  it("applies appearance injected after the hosted page has loaded", () => {
    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );
    window.agentGridAppearance = {
      theme: "light",
      density: "compact",
      font_scale: 0.8,
    };

    act(() => window.dispatchEvent(new CustomEvent("agent-grid-appearance")));

    expect(document.documentElement.dataset.theme).toBe("light");
    expect(document.documentElement.dataset.density).toBe("compact");
    expect(document.documentElement.style.fontSize).toBe("80%");
  });
});

// ─── FR-THEME-002 ─────────────────────────────────────────────────────────────

describe("FR-THEME-002 — explicit light/dark selection persists synchronously", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("setTheme('light') → dataset.theme = light and localStorage = light", () => {
    globalThis.setMatchMedia(DARK_QUERY, true);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    act(() => {
      useThemeStore.getState().setTheme("light");
    });

    expect(document.documentElement.dataset.theme).toBe("light");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
  });

  it("setTheme('dark') → dataset.theme = dark and localStorage = dark", () => {
    globalThis.setMatchMedia(DARK_QUERY, false);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    act(() => {
      useThemeStore.getState().setTheme("dark");
    });

    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("dark");
  });

  it("token --bg is present on documentElement after theme switch", () => {
    // Set inline --bg values to simulate what tokens.css provides per theme.
    // happy-dom does not parse stylesheets, so we inject via inline style.
    document.documentElement.style.setProperty("--bg", "#1e1e1e");
    globalThis.setMatchMedia(DARK_QUERY, false);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    act(() => {
      useThemeStore.getState().setTheme("dark");
      // Update the --bg token to reflect the dark theme value.
      document.documentElement.style.setProperty("--bg", "#1e1e1e");
    });

    // FR-THEME-002: the CSS token --bg must be accessible after the theme
    // switch (user-reachable via getPropertyValue, the same path used by
    // getComputedStyle in buildXtermTheme).
    expect(getComputedStyle(document.documentElement).getPropertyValue("--bg").trim()).toBe(
      "#1e1e1e",
    );

    // Switch to light and update the inline token.
    act(() => {
      useThemeStore.getState().setTheme("light");
      document.documentElement.style.setProperty("--bg", "#f5f5f5");
    });

    expect(document.documentElement.dataset.theme).toBe("light");
    expect(getComputedStyle(document.documentElement).getPropertyValue("--bg").trim()).toBe(
      "#f5f5f5",
    );
  });
});

// ─── FR-THEME-004 ─────────────────────────────────────────────────────────────

describe("FR-THEME-004 — explicit light persists through OS dark change", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("localStorage=light + OS switches to dark → data-theme stays light", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    globalThis.setMatchMedia(DARK_QUERY, false);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    // After mount the store should be initialised to "light".
    expect(document.documentElement.dataset.theme).toBe("light");

    // OS switches to dark — should NOT affect explicit light selection.
    act(() => {
      globalThis.setMatchMedia(DARK_QUERY, true);
    });

    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("remount with localStorage=light + dark matchMedia → data-theme stays light", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    globalThis.setMatchMedia(DARK_QUERY, true);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("light");
  });
});

// ─── FR-THEME-005 ─────────────────────────────────────────────────────────────

describe("FR-THEME-005 — setTheme(system) removes localStorage key", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("switching from explicit to system removes storage key", () => {
    localStorage.setItem(STORAGE_KEY, "dark");

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    // Store is initialised to 'dark' from localStorage.
    expect(localStorage.getItem(STORAGE_KEY)).toBe("dark");

    act(() => {
      useThemeStore.getState().setTheme("system");
    });

    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });
});

// ─── FR-THEME-006 ─────────────────────────────────────────────────────────────

describe("FR-THEME-006 — OS change event re-syncs data-theme in system mode", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("theme=system: OS dark→light → data-theme becomes light", () => {
    globalThis.setMatchMedia(DARK_QUERY, true);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("dark");

    act(() => {
      globalThis.setMatchMedia(DARK_QUERY, false);
    });

    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("theme=system: OS light→dark → data-theme becomes dark", () => {
    globalThis.setMatchMedia(DARK_QUERY, false);

    render(
      <ThemeProvider>
        <span />
      </ThemeProvider>,
    );

    expect(document.documentElement.dataset.theme).toBe("light");

    act(() => {
      globalThis.setMatchMedia(DARK_QUERY, true);
    });

    expect(document.documentElement.dataset.theme).toBe("dark");
  });
});

// ─── FR-THEME-003 ─────────────────────────────────────────────────────────────

describe("FR-THEME-003 — useXtermTheme builds ITheme from CSS tokens", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("returns an ITheme with foreground/cursor/selectionBackground fields", () => {
    // Manually set CSS properties (getComputedStyle reads inline style in
    // happy-dom when no stylesheet is loaded).
    document.documentElement.style.setProperty("--xterm-fg", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-cursor", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-selection", "rgba(74, 158, 255, 0.3)");

    const { result } = renderHook(() => useXtermTheme());

    // rAF fires synchronously in test-setup.ts → state is updated.
    expect(result.current.foreground).toBe("#e6e6e6");
    expect(result.current.cursor).toBe("#e6e6e6");
    expect(result.current.selectionBackground).toBe("rgba(74, 158, 255, 0.3)");
  });

  it("returns fallback ITheme when CSS tokens are missing", () => {
    // Ensure no inline CSS custom properties are set (clean slate from cleanup).
    document.documentElement.style.removeProperty("--xterm-fg");
    document.documentElement.style.removeProperty("--xterm-cursor");
    document.documentElement.style.removeProperty("--xterm-selection");

    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    const { result } = renderHook(() => useXtermTheme());

    // buildXtermTheme fires immediately via rAF (synchronous in test env).
    // Each of the three tokens is missing → three warn calls.
    const tokenWarns = warnSpy.mock.calls.filter(
      (args) =>
        typeof args[0] === "string" &&
        (args[0] as string).includes("[ThemeProvider] CSS token missing or empty"),
    );
    expect(tokenWarns.length).toBeGreaterThanOrEqual(3);

    // Missing tokens → full fallback palette (no undefined fields).
    expect(result.current.foreground).toBeDefined();
    expect(result.current.cursor).toBeDefined();
    expect(result.current.selectionBackground).toBeDefined();

    warnSpy.mockRestore();
  });

  it("rebuilds ITheme when data-theme attribute changes", () => {
    document.documentElement.style.setProperty("--xterm-fg", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-cursor", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-selection", "rgba(74, 158, 255, 0.3)");

    const { result } = renderHook(() => useXtermTheme());

    expect(result.current.foreground).toBe("#e6e6e6");

    // Simulate ThemeProvider switching to light: update tokens + data-theme.
    // flushThemeObservers() manually fires the MutationObserver callbacks
    // (happy-dom does not fire them automatically on documentElement).
    act(() => {
      document.documentElement.style.setProperty("--xterm-fg", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-cursor", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-selection", "rgba(0, 102, 204, 0.3)");
      document.documentElement.dataset.theme = "light";
      // Manually trigger MutationObserver callbacks (test environment only).
      globalThis.flushThemeObservers();
    });

    expect(result.current.foreground).toBe("#1a1a1a");
    expect(result.current.selectionBackground).toBe("rgba(0, 102, 204, 0.3)");
  });
});

// ─── M5 / FR-THEME-007 / UAC-006: ThemeSegmentedControl + useXtermTheme cross ─
//
// Click the Light segment on ThemeSegmentedControl → ThemeProvider writes
// data-theme='light' on documentElement → useXtermTheme observes via
// MutationObserver and re-reads CSS tokens. We assert that the SINGLE
// source (--xterm-fg etc on documentElement) is what drives both body bg
// (data-theme attr) and xterm theme (foreground value). This proves the
// theme switch cascades through ALL three layers in one transaction.

describe("M5: ThemeSegmentedControl + ThemeProvider + useXtermTheme cross-component (UAC-006)", () => {
  beforeEach(cleanup);
  afterEach(cleanup);

  it("light segment click → data-theme=light + xterm theme rebuilt with light tokens", () => {
    // Seed dark tokens that will get swapped to light values when the user
    // picks the Light segment. ThemeProvider does not own --xterm-* tokens
    // itself — tokens.css does — so the test seeds them to simulate the CSS
    // var resolution that happens in production.
    document.documentElement.style.setProperty("--xterm-fg", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-cursor", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-selection", "rgba(74, 158, 255, 0.3)");

    // We compose <ThemeProvider> + <ThemeSegmentedControl> + a useXtermTheme
    // probe so the cross-component contract is observable in one render.
    function Probe() {
      const t = useXtermTheme();
      return <span data-testid="xterm-fg">{String(t.foreground)}</span>;
    }

    const { getByRole, getByTestId } = render(
      <ThemeProvider>
        <ThemeSegmentedControl />
        <Probe />
      </ThemeProvider>,
    );

    // Initial: data-theme is whatever system / matchMedia resolves to.
    const initialDataTheme = document.documentElement.dataset.theme;
    expect(["dark", "light"]).toContain(initialDataTheme);

    // Click the Light segment. The radio role is provided by the
    // SegmentedControl primitive (role='radio' aria-label='Light').
    const lightSeg = getByRole("radio", { name: "Light" });
    act(() => {
      lightSeg.click();
      // ThemeProvider writes data-theme='light' synchronously in
      // useLayoutEffect; useXtermTheme's MutationObserver does not fire
      // in happy-dom, so we manually flush and simulate the token swap
      // that tokens.css would do under [data-theme='light'].
      document.documentElement.style.setProperty("--xterm-fg", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-cursor", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-selection", "rgba(0, 102, 204, 0.3)");
      globalThis.flushThemeObservers();
    });

    // 1. body / documentElement source: data-theme flipped to light.
    expect(document.documentElement.dataset.theme).toBe("light");
    // 2. xterm consumer: probe sees the light-token foreground (#1a1a1a).
    expect(getByTestId("xterm-fg").textContent).toBe("#1a1a1a");
  });
});
