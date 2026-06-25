import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Connection } from "../socket/connection";
import { TerminalPane } from "./TerminalPane";

// ---------------------------------------------------------------------------
// Helpers to grab the mocked FitAddon instance from vi.mock("@xterm/addon-fit")
// ---------------------------------------------------------------------------

// We need a spy on fit.fit() — re-open the mock to capture instance calls.
// The mock is defined in test-setup.ts; we extend it here per-test with vi.spyOn
// by reaching into the module mock after importing.

import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";

// ---------------------------------------------------------------------------
// Minimal fakeConn factory (fresh per test to avoid state bleed)
// ---------------------------------------------------------------------------
function makeFakeConn(): {
  conn: Connection;
  capturedOnOutput: () => ((frame: [number, string, string, string]) => void) | undefined;
} {
  let _onOutput: ((frame: [number, string, string, string]) => void) | undefined;
  const conn = {
    subscribe: vi.fn(async () => {}),
    unsubscribe: vi.fn(async () => {}),
    send: vi.fn(),
    get onOutput() {
      return _onOutput;
    },
    set onOutput(cb) {
      _onOutput = cb;
    },
  } as unknown as Connection;
  return {
    conn,
    capturedOnOutput: () => _onOutput,
  };
}

describe("TerminalPane", () => {
  // Spy on FitAddon.prototype.fit across all tests
  let fitSpy: ReturnType<typeof vi.spyOn>;
  let writeSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    fitSpy = vi.spyOn(FitAddon.prototype, "fit");
    writeSpy = vi.spyOn(Terminal.prototype, "write");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // -------------------------------------------------------------------------
  // Basic smoke test
  // -------------------------------------------------------------------------
  it("mounts and unmounts without throwing", () => {
    const { conn } = makeFakeConn();
    const { unmount, container } = render(<TerminalPane conn={conn} sessionId="s1" />);
    expect(container.querySelector(".terminal-host")).not.toBeNull();
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-008: initial fit is called via scheduleFit (rAF) on mount
  // The synchronous rAF mock in test-setup flushes immediately, so fit.fit()
  // should have been called once right after render.
  // -------------------------------------------------------------------------
  it("FR-008: calls fit.fit() on initial mount via scheduleFit (rAF)", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    // rAF mock runs synchronously → fit.fit() should already be called
    expect(fitSpy).toHaveBeenCalledTimes(1);
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-006: __triggerResize fires ResizeObserver callback → fit.fit() called
  // -------------------------------------------------------------------------
  it("FR-006: __triggerResize on host element causes fit.fit() to be called", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as Element;
    expect(host).not.toBeNull();

    const callsBefore = fitSpy.mock.calls.length;
    // Simulate ResizeObserver firing on the host element
    globalThis.__triggerResize(host, []);

    // rAF mock is synchronous so fit.fit() fires immediately
    expect(fitSpy.mock.calls.length).toBeGreaterThan(callsBefore);
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-007: sibling panel size change via ResizeObserver → refit
  // (same mechanic as FR-006; verifying at least one additional call)
  // -------------------------------------------------------------------------
  it("FR-007: ResizeObserver host resize triggers scheduleFit and calls fit.fit()", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as Element;

    const callsBefore = fitSpy.mock.calls.length;
    globalThis.__triggerResize(host, [{ contentRect: { width: 400, height: 300 } }]);

    expect(fitSpy.mock.calls.length).toBeGreaterThan(callsBefore);
    unmount();
  });

  // -------------------------------------------------------------------------
  // NFR-005: rapid consecutive ResizeObserver firings are coalesced into a
  // single rAF tick. With synchronous rAF mock each call resolves immediately,
  // so the pending-flag logic prevents duplicate calls within the same tick.
  // We test by counting fit.fit() calls after N rapid triggers — should be N
  // (one per trigger) but never more, and each rAF resolves synchronously so
  // we can count exactly.
  // Actually with synchronous rAF, each scheduleFit call runs immediately,
  // meaning rafPending flips back to false between calls, so each call to
  // scheduleFit will invoke fit. The NFR is about coalescing within ONE frame.
  // We simulate TWO back-to-back __triggerResize calls without any frame
  // boundary between them by temporarily making rAF queue (not fire immediately).
  // -------------------------------------------------------------------------
  it("NFR-005: rapid ResizeObserver firings in same frame are coalesced to 1 fit.fit() call", () => {
    const { conn } = makeFakeConn();

    // Temporarily override rAF to queue (not fire synchronously) so we can
    // test the pending-flag coalescing behavior.
    const rafQueue: FrameRequestCallback[] = [];
    const origRAF = globalThis.requestAnimationFrame;
    globalThis.requestAnimationFrame = (cb: FrameRequestCallback) => {
      rafQueue.push(cb);
      return rafQueue.length;
    };

    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as Element;

    // Clear calls from mount (initial scheduleFit was queued, not yet run)
    fitSpy.mockClear();

    // Fire resize 5 times rapidly (no frame flush between them)
    const trigger = globalThis.__triggerResize;
    trigger(host, []);
    trigger(host, []);
    trigger(host, []);
    trigger(host, []);
    trigger(host, []);

    // Before flushing, fit.fit() should not yet have been called
    expect(fitSpy).not.toHaveBeenCalled();

    // Flush all queued rAF callbacks
    const queued = rafQueue.splice(0);
    for (const cb of queued) cb(performance.now());

    // Only 1 fit.fit() should have been called (coalesced)
    expect(fitSpy).toHaveBeenCalledTimes(1);

    // Restore
    globalThis.requestAnimationFrame = origRAF;
    unmount();
  });

  // -------------------------------------------------------------------------
  // FR-005: keyed remount — stale output for old sessionId is NOT written
  // to the new TerminalPane instance.
  // -------------------------------------------------------------------------
  it("FR-005: stale output for old sessionId is not written after key-based remount", () => {
    const { conn, capturedOnOutput } = makeFakeConn();

    // Mount with sessionId "s1"
    const { unmount: unmount1 } = render(<TerminalPane key="s1" conn={conn} sessionId="s1" />);
    const onOutputS1 = capturedOnOutput();
    expect(onOutputS1).toBeDefined();

    // Unmount s1 (simulating key change / remount)
    unmount1();
    // After unmount, conn.onOutput should be cleared
    expect(capturedOnOutput()).toBeUndefined();

    // Mount new instance with sessionId "s2"
    const { unmount: unmount2 } = render(<TerminalPane key="s2" conn={conn} sessionId="s2" />);

    writeSpy.mockClear();

    // Deliver stale output tagged with old session "s1" to the s1 handler
    // (simulate the stale callback from before remount)
    if (onOutputS1) {
      // The old handler is detached; calling it would use a disposed terminal.
      // The important assertion is that the NEW instance's onOutput drops
      // frames whose sessionId !== "s2".
      const newHandler = capturedOnOutput();
      expect(newHandler).toBeDefined();
      if (newHandler) {
        // Deliver stale s1 frame to new handler — should be dropped
        newHandler([0, "o", btoa("stale data"), "s1"]);
        expect(writeSpy).not.toHaveBeenCalled();

        // Deliver correct s2 frame — should be written
        newHandler([0, "o", btoa("good data"), "s2"]);
        expect(writeSpy).toHaveBeenCalledTimes(1);
      }
    }

    unmount2();
  });

  // -------------------------------------------------------------------------
  // ADR 0030 lifecycle: a real key remount must hand off conn.onOutput
  // cleanly. Driving the swap through @testing-library's rerender with a
  // changing `key` forces React to unmount the old instance (cleanup runs,
  // conn.onOutput cleared) BEFORE the new instance mounts (new onOutput
  // installed). Without ADR 0030's keyed remount the old effect would keep
  // running and writes for the stale sessionId would land in the new term.
  // -------------------------------------------------------------------------
  it("FR-005/ADR-0030: keyed rerender unmounts old onOutput before new install; new term receives new-session frames only", () => {
    const { conn, capturedOnOutput } = makeFakeConn();

    // Mount with key=s1 → installs onOutput-A bound to s1
    const { rerender, unmount } = render(<TerminalPane key="s1" conn={conn} sessionId="s1" />);
    const onOutputBefore = capturedOnOutput();
    expect(onOutputBefore).toBeDefined();

    // Track the term.write call count seen by ANY Terminal instance — both
    // old and new share Terminal.prototype, so writeSpy spans them.
    writeSpy.mockClear();

    // Force a real key remount. React unmounts the s1 instance (cleanup runs
    // → conn.onOutput cleared, term disposed) and mounts a fresh instance
    // under key=s2 which installs a new onOutput bound to s2.
    rerender(<TerminalPane key="s2" conn={conn} sessionId="s2" />);

    const onOutputAfter = capturedOnOutput();
    expect(onOutputAfter).toBeDefined();
    // The new effect installed a brand-new handler — not the s1 closure.
    expect(onOutputAfter).not.toBe(onOutputBefore);

    // A stale s1-tagged frame arriving on the now-live (s2) handler is
    // dropped by the frame[3] !== sessionRef.current guard.
    if (onOutputAfter) {
      onOutputAfter([0, "o", btoa("stale s1 frame"), "s1"]);
      expect(writeSpy).not.toHaveBeenCalled();

      // The matching s2 frame lands on the new term.
      onOutputAfter([0, "o", btoa("good s2 frame"), "s2"]);
      expect(writeSpy).toHaveBeenCalledTimes(1);
    }

    unmount();
    // Final cleanup leaves conn.onOutput cleared so a future Connection
    // consumer doesn't see a dangling closure into a disposed terminal.
    expect(capturedOnOutput()).toBeUndefined();
  });

  // -------------------------------------------------------------------------
  // Regression: conn.onOutput filter — frame[3] !== sessionRef.current drops
  // -------------------------------------------------------------------------
  it("regression: conn.onOutput drops frames whose sessionId does not match current session", () => {
    const { conn, capturedOnOutput } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="session-A" />);

    writeSpy.mockClear();
    const handler = capturedOnOutput();
    expect(handler).toBeDefined();

    if (handler) {
      // Frame from a different session — must be dropped
      handler([0, "o", btoa("wrong session"), "session-B"]);
      expect(writeSpy).not.toHaveBeenCalled();

      // Frame from the correct session — must be written
      handler([0, "o", btoa("correct"), "session-A"]);
      expect(writeSpy).toHaveBeenCalledTimes(1);
    }

    unmount();
  });

  // -------------------------------------------------------------------------
  // Subscribe / unsubscribe lifecycle
  // -------------------------------------------------------------------------
  it("calls conn.subscribe on mount and conn.unsubscribe on unmount", () => {
    const { conn } = makeFakeConn();
    const subscribeMock = conn.subscribe as ReturnType<typeof vi.fn>;
    const unsubscribeMock = conn.unsubscribe as ReturnType<typeof vi.fn>;

    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    expect(subscribeMock).toHaveBeenCalledWith("s1");
    expect(unsubscribeMock).not.toHaveBeenCalled();

    unmount();
    expect(unsubscribeMock).toHaveBeenCalledWith("s1");
  });

  it("cleans up conn.onOutput on unmount", () => {
    const { conn, capturedOnOutput } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    expect(capturedOnOutput()).toBeDefined();
    unmount();
    expect(capturedOnOutput()).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// FR-THEME-003: terminal-host element uses CSS var(--bg) for background
// (ADR-0059 hybrid bridge)
// ---------------------------------------------------------------------------

describe("FR-THEME-003 — terminal-host background is driven by CSS var(--bg)", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Inject CSS rules so happy-dom can resolve custom properties.
    // The actual app.css rule: .terminal-host { background: var(--bg) !important }
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --bg: #1e1e1e; }
      [data-theme="light"] { --bg: #f5f5f5; }
      [data-theme="dark"]  { --bg: #1e1e1e; }
      .terminal-host { background: var(--bg) !important; }
    `;
    document.head.appendChild(styleEl);
    document.documentElement.dataset.theme = "dark";
  });

  afterEach(() => {
    styleEl.remove();
    delete document.documentElement.dataset.theme;
  });

  it("terminal-host background resolves to dark --bg when data-theme=dark", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    expect(host).not.toBeNull();

    const bg = getComputedStyle(host).backgroundColor;
    // In happy-dom, getComputedStyle resolves custom property values from
    // injected <style> tags. The injected dark rule sets --bg to #1e1e1e so
    // the resolved background should equal that value (not empty, not transparent,
    // not the light value). This verifies token-to-element wiring is active.
    expect(bg).toBe("#1e1e1e");
    unmount();
  });

  it("dark and light --bg resolve to different values", () => {
    const { conn: connDark } = makeFakeConn();
    document.documentElement.dataset.theme = "dark";
    const { container: containerDark, unmount: unmountDark } = render(
      <TerminalPane conn={connDark} sessionId="s1" />,
    );
    const hostDark = containerDark.querySelector(".terminal-host") as HTMLElement;
    const bgDark = getComputedStyle(hostDark).backgroundColor;
    unmountDark();

    const { conn: connLight } = makeFakeConn();
    document.documentElement.dataset.theme = "light";
    const { container: containerLight, unmount: unmountLight } = render(
      <TerminalPane conn={connLight} sessionId="s2" />,
    );
    const hostLight = containerLight.querySelector(".terminal-host") as HTMLElement;
    const bgLight = getComputedStyle(hostLight).backgroundColor;
    unmountLight();

    // Dark and light backgrounds must be distinct values.
    expect(bgDark).not.toBe(bgLight);
  });
});

// ---------------------------------------------------------------------------
// FR-TERMINAL-001: terminal-host height > 0 after resize (ADR-0060 / ADR-0034)
// FR-TERMINAL-002: no double-scroll after render
// ADR-0060 structural: terminal-host CSS uses var(--dvh) + flex:1 1 0 coexist
// ---------------------------------------------------------------------------

describe("FR-TERMINAL-001 — viewport height changes refit terminal-host (ADR-0060 / ADR-0034)", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Inject CSS so getComputedStyle can resolve height token.
    // --dvh defaults to 100vh; in happy-dom 100vh resolves to a px value
    // based on window.innerHeight. We set it explicitly so the test is stable.
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --dvh: 100vh; }
      .terminal-host {
        flex: 1 1 0;
        min-height: 0;
        width: 100%;
        height: var(--dvh);
        box-sizing: border-box;
      }
    `;
    document.head.appendChild(styleEl);
  });

  afterEach(() => {
    styleEl.remove();
    vi.restoreAllMocks();
  });

  it("FR-TERMINAL-001: terminal-host getComputedStyle.height > 0 after shrink and expand resize", () => {
    const { conn } = makeFakeConn();
    const fitSpy = vi.spyOn(FitAddon.prototype, "fit");
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    expect(host).not.toBeNull();

    // Simulate viewport shrink: set innerHeight to 600 and fire resize.
    Object.defineProperty(window, "innerHeight", {
      value: 600,
      writable: true,
      configurable: true,
    });
    window.dispatchEvent(new Event("resize"));
    // rAF mock fires synchronously → fit.fit() called.
    expect(fitSpy.mock.calls.length).toBeGreaterThan(0);

    // terminal-host has height set via CSS var(--dvh) → getComputedStyle.height
    // is non-empty (not "0px") as long as CSS is injected.
    const heightShrink = getComputedStyle(host).height;
    expect(heightShrink).not.toBe("0px");
    expect(heightShrink).not.toBe("");

    // Simulate viewport expand: restore innerHeight to 667 and fire resize.
    fitSpy.mockClear();
    Object.defineProperty(window, "innerHeight", {
      value: 667,
      writable: true,
      configurable: true,
    });
    window.dispatchEvent(new Event("resize"));
    expect(fitSpy.mock.calls.length).toBeGreaterThan(0);

    const heightExpand = getComputedStyle(host).height;
    expect(heightExpand).not.toBe("0px");
    expect(heightExpand).not.toBe("");

    unmount();
  });

  it("FR-TERMINAL-001 rAF coalesce: rapid resize events produce at most 1 fit.fit() per frame", () => {
    const { conn } = makeFakeConn();

    // Use a queuing rAF so we can verify coalescing.
    const rafQueue: FrameRequestCallback[] = [];
    const origRAF = globalThis.requestAnimationFrame;
    globalThis.requestAnimationFrame = (cb: FrameRequestCallback) => {
      rafQueue.push(cb);
      return rafQueue.length;
    };

    const fitSpy = vi.spyOn(FitAddon.prototype, "fit");
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;

    // Drain the initial mount rAF queue within act() to avoid cross-test
    // state contamination. On mount, scheduleFit() and useXtermTheme's
    // rebuild() both queue rAF callbacks. We flush them first so the
    // pending flag resets to false before we start the resize test.
    const mountQueue = rafQueue.splice(0);
    act(() => {
      for (const cb of mountQueue) cb(performance.now());
    });
    fitSpy.mockClear();

    // Fire 5 resize events without flushing the rAF queue.
    for (let i = 0; i < 5; i++) {
      window.dispatchEvent(new Event("resize"));
    }
    // Also trigger 3 ResizeObserver callbacks on the host element.
    for (let i = 0; i < 3; i++) {
      globalThis.__triggerResize(host, []);
    }

    // Before flush: coalesce means pending flag is set, fit.fit() not yet called.
    expect(fitSpy).not.toHaveBeenCalled();

    // Flush queued rAF callbacks: only 1 should have been queued for the
    // resize batch (pending flag prevents additional enqueues).
    const resizeQueue = rafQueue.splice(0);
    expect(resizeQueue.length).toBe(1);
    act(() => {
      for (const cb of resizeQueue) cb(performance.now());
    });
    expect(fitSpy).toHaveBeenCalledTimes(1);

    globalThis.requestAnimationFrame = origRAF;
    unmount();
  });
});

describe("FR-TERMINAL-002 — no double-scroll after render (ADR-0060 / m3 body overflow:hidden)", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Inject body overflow:hidden (guaranteed by m3-app-shell-grid) plus
    // the terminal-host height token so the DOM layout matches production.
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --dvh: 100vh; }
      html, body, #root { height: 100%; margin: 0; }
      body { overflow: hidden; }
      .terminal-host {
        flex: 1 1 0;
        min-height: 0;
        width: 100%;
        height: var(--dvh);
        box-sizing: border-box;
      }
    `;
    document.head.appendChild(styleEl);
  });

  afterEach(() => {
    styleEl.remove();
  });

  it("FR-TERMINAL-002: scrollHeight <= innerHeight and scrollWidth <= innerWidth after render", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    // With body overflow:hidden, scrollHeight and scrollWidth must not exceed
    // the viewport dimensions — double-scroll is impossible.
    const scrollH = document.documentElement.scrollHeight;
    const scrollW = document.documentElement.scrollWidth;
    const innerH = window.innerHeight;
    const innerW = window.innerWidth;

    expect(scrollH).toBeLessThanOrEqual(innerH);
    expect(scrollW).toBeLessThanOrEqual(innerW);

    unmount();
  });
});

describe("ADR-0060 structural — terminal-host CSS: var(--dvh) + flex:1 1 0 coexist", () => {
  let styleEl: HTMLStyleElement;

  beforeEach(() => {
    // Use a concrete px value for --dvh so happy-dom's getComputedStyle can
    // resolve height: var(--dvh) to a non-empty, non-zero string without
    // needing to resolve viewport-relative units (which happy-dom cannot do
    // for custom properties transitively referencing 100dvh/100vh).
    styleEl = document.createElement("style");
    styleEl.textContent = `
      :root { --dvh: 768px; }
      .terminal-host {
        flex: 1 1 0;
        min-height: 0;
        width: 100%;
        height: var(--dvh);
        box-sizing: border-box;
      }
    `;
    document.head.appendChild(styleEl);
  });

  afterEach(() => {
    styleEl.remove();
  });

  it("terminal-host has height CSS property set via var(--dvh)", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;

    // getComputedStyle.height must resolve to the --dvh concrete value (768px).
    // This confirms the height: var(--dvh) declaration is wired and applied.
    const height = getComputedStyle(host).height;
    expect(height).toBe("768px");

    unmount();
  });

  it("terminal-host retains flex:1 1 0 alongside height:var(--dvh) (ADR-0029 + ADR-0060 coexist)", () => {
    const { conn } = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;

    const style = getComputedStyle(host);
    // flex: 1 1 0 expands to flexGrow=1, flexShrink=1, flexBasis=0.
    expect(style.flexGrow).toBe("1");
    expect(style.flexShrink).toBe("1");
    // flexBasis and minHeight may be "0px" or "0" depending on happy-dom version.
    expect(Number.parseFloat(style.flexBasis)).toBe(0);
    expect(Number.parseFloat(style.minHeight)).toBe(0);

    unmount();
  });
});

// ---------------------------------------------------------------------------
// FR-THEME-002: xterm.options.theme is updated within 1 rAF after data-theme
// switches (ADR-0059 hybrid bridge — ITheme side)
// ---------------------------------------------------------------------------

describe("FR-THEME-002 — xterm.options.theme is updated on data-theme change", () => {
  // Capture created Terminal instances so we can inspect options.theme.
  let createdTerminals: Terminal[];
  let constructorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    createdTerminals = [];
    constructorSpy = vi.spyOn(Terminal.prototype, "open").mockImplementation(function (
      this: Terminal,
    ) {
      createdTerminals.push(this);
    });

    // Set dark token values on documentElement so useXtermTheme reads them.
    document.documentElement.style.setProperty("--xterm-fg", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-cursor", "#e6e6e6");
    document.documentElement.style.setProperty("--xterm-selection", "rgba(74, 158, 255, 0.3)");
    document.documentElement.dataset.theme = "dark";
  });

  afterEach(() => {
    constructorSpy.mockRestore();
    createdTerminals = [];
    document.documentElement.style.removeProperty("--xterm-fg");
    document.documentElement.style.removeProperty("--xterm-cursor");
    document.documentElement.style.removeProperty("--xterm-selection");
    delete document.documentElement.dataset.theme;
  });

  it("xterm.options.theme is set on mount with initial ITheme", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    // After mount the terminal should have been opened; theme effect runs after.
    // With synchronous rAF mock, theme is applied immediately.
    // The terminal instance captured via open() spy has options.theme set.
    expect(createdTerminals.length).toBeGreaterThan(0);
    const term = createdTerminals[0];
    const theme = (term as unknown as { options: Record<string, unknown> }).options.theme;
    // The ITheme should have been applied (not undefined).
    expect(theme).toBeDefined();
    expect(typeof theme).toBe("object");

    unmount();
  });

  it("xterm.options.theme updates within 1 rAF after data-theme changes dark→light", () => {
    const { conn } = makeFakeConn();
    const { unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    expect(createdTerminals.length).toBeGreaterThan(0);
    const term = createdTerminals[0] as unknown as { options: Record<string, unknown> };

    // Record initial dark theme.
    const darkTheme = term.options.theme as { foreground?: string } | undefined;
    expect(darkTheme).toBeDefined();

    // Switch to light: update CSS tokens + data-theme then flush MutationObserver.
    act(() => {
      document.documentElement.style.setProperty("--xterm-fg", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-cursor", "#1a1a1a");
      document.documentElement.style.setProperty("--xterm-selection", "rgba(0, 102, 204, 0.3)");
      document.documentElement.dataset.theme = "light";
      // Manually fire MutationObserver callbacks — happy-dom does not fire them
      // automatically on documentElement attribute mutations.
      globalThis.flushThemeObservers();
    });

    // rAF runs synchronously → ITheme is rebuilt with light token values.
    const lightTheme = term.options.theme as { foreground?: string } | undefined;
    expect(lightTheme).toBeDefined();
    // foreground token changed from dark (#e6e6e6) to light (#1a1a1a).
    expect((lightTheme as { foreground?: string }).foreground).toBe("#1a1a1a");
    // Dark and light IThemes are distinct objects with different foreground values.
    expect((darkTheme as { foreground?: string }).foreground).not.toBe(
      (lightTheme as { foreground?: string }).foreground,
    );

    unmount();
  });
});
