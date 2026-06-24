import { render } from "@testing-library/react";
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
