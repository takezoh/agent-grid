// useTerminalTouchGestures.test.ts — the swipe / long-press / pinch arbitration
// state machine (ADR 0071). Tests are split into:
//   1. gestureReducer — the pure transition table over the 5 states, exercised
//      directly so every edge is named (idle/swipe/dwell/longpress-drag/pinch).
//   2. useTerminalTouchGestures — the hook driven through the chunk-01 TouchEvent
//      shim (swipeFromTo / longPressAndDrag / pinchByRatio + fake timers),
//      asserting the discriminating contracts behind UAC-009/010/011/016 and the
//      "preventDefault only after dwell + during pinch" discipline.

import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  type SyntheticTouch,
  dispatchTouchEvent,
  longPressAndDrag,
  pinchByRatio,
  swipeFromTo,
} from "../test/touch-harness";
import {
  type Cell,
  type GestureState,
  INITIAL_GESTURE_STATE,
  type TerminalLike,
  gestureReducer,
  useTerminalTouchGestures,
} from "./useTerminalTouchGestures";

const CELL: Cell = { width: 10, height: 20 };

// ---------------------------------------------------------------------------
// 1. Pure reducer — transition table
// ---------------------------------------------------------------------------

describe("gestureReducer (pure transition table)", () => {
  it("idle + 1-finger touchstart → swipe (no effects)", () => {
    const r = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [{ x: 100, y: 100 }],
      cell: CELL,
    });
    expect(r.state.phase).toBe("swipe");
    expect(r.effects).toEqual([]);
  });

  it("swipe + move past 8px → stays swipe, marks moved, no preventDefault", () => {
    const swipe = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [{ x: 100, y: 100 }],
      cell: CELL,
    }).state;
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 100, y: 160 }], cell: CELL });
    expect(r.state).toEqual({ phase: "swipe", start: { x: 100, y: 100 }, cell: CELL, moved: true });
    expect(r.effects).toEqual([]);
  });

  it("swipe + dwellElapsed while stationary → dwell", () => {
    const swipe = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [{ x: 100, y: 100 }],
      cell: CELL,
    }).state;
    const r = gestureReducer(swipe, { kind: "dwellElapsed" });
    expect(r.state.phase).toBe("dwell");
  });

  it("swipe + dwellElapsed AFTER moving past 8px → stays swipe (no false dwell)", () => {
    const swipe = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [{ x: 100, y: 100 }],
      cell: CELL,
    }).state;
    const moved = gestureReducer(swipe, {
      kind: "move",
      touches: [{ x: 100, y: 160 }],
      cell: CELL,
    }).state;
    const r = gestureReducer(moved, { kind: "dwellElapsed" });
    expect(r.state.phase).toBe("swipe");
  });

  it("dwell + move → longpress-drag, emits preventDefault + select", () => {
    const dwell: GestureState = { phase: "dwell", start: { x: 100, y: 200 }, cell: CELL };
    const r = gestureReducer(dwell, { kind: "move", touches: [{ x: 130, y: 200 }], cell: CELL });
    expect(r.state.phase).toBe("longpress-drag");
    expect(r.effects).toEqual([
      { kind: "preventDefault" },
      { kind: "select", col: 10, row: 10, length: 3 },
    ]);
  });

  it("longpress-drag + move → keeps selecting (preventDefault + select)", () => {
    const drag: GestureState = { phase: "longpress-drag", start: { x: 100, y: 200 }, cell: CELL };
    const r = gestureReducer(drag, { kind: "move", touches: [{ x: 150, y: 200 }], cell: CELL });
    expect(r.state.phase).toBe("longpress-drag");
    expect(r.effects.some((e) => e.kind === "preventDefault")).toBe(true);
    expect(r.effects.some((e) => e.kind === "select")).toBe(true);
  });

  it("idle + 2-finger touchstart → pinch (records baseline, no preventDefault yet)", () => {
    const r = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [
        { x: 60, y: 100 },
        { x: 140, y: 100 },
      ],
      cell: CELL,
    });
    expect(r.state).toEqual({ phase: "pinch", startDistance: 80 });
    expect(r.effects).toEqual([]);
  });

  it("swipe + 2-finger move (1→2 finger) → interrupts into pinch, preventDefault only on first frame", () => {
    const swipe = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [{ x: 100, y: 100 }],
      cell: CELL,
    }).state;
    const r = gestureReducer(swipe, {
      kind: "move",
      touches: [
        { x: 60, y: 100 },
        { x: 140, y: 100 },
      ],
      cell: CELL,
    });
    expect(r.state.phase).toBe("pinch");
    expect(r.effects).toEqual([{ kind: "preventDefault" }]);
  });

  it("pinch + 2-finger move → ratio follows d_now/d_start, with preventDefault + scheduleFit", () => {
    const pinch: GestureState = { phase: "pinch", startDistance: 80 };
    const r = gestureReducer(pinch, {
      kind: "move",
      touches: [
        { x: 40, y: 100 },
        { x: 160, y: 100 },
      ],
      cell: CELL,
    });
    expect(r.effects).toEqual([
      { kind: "preventDefault" },
      { kind: "pinch", ratio: 1.5 },
      { kind: "scheduleFit" },
    ]);
  });

  it("any phase + touchend → idle", () => {
    const drag: GestureState = { phase: "longpress-drag", start: { x: 0, y: 0 }, cell: CELL };
    expect(gestureReducer(drag, { kind: "end" }).state).toEqual(INITIAL_GESTURE_STATE);
  });
});

// ---------------------------------------------------------------------------
// 2. Hook — driven through the TouchEvent shim
// ---------------------------------------------------------------------------

interface MockTerm extends TerminalLike {
  selectCalls: Array<[number, number, number]>;
  /** Mock-only mirror of the active selection; verified by UAC-010/011 assertions. */
  getSelection(): string;
  /** Mock-only stand-in for the live xterm options; mutated by the onPinch handler. */
  options: { fontSize: number };
}

function makeTerm(): MockTerm {
  let selection = "";
  const calls: Array<[number, number, number]> = [];
  return {
    selectCalls: calls,
    select(col, row, length) {
      calls.push([col, row, length]);
      selection = `${col},${row}+${length}`;
    },
    getSelection() {
      return selection;
    },
    options: { fontSize: 14 },
  };
}

function makeT(x: number, y: number, id: number): SyntheticTouch {
  return { clientX: x, clientY: y, identifier: id } as unknown as SyntheticTouch;
}

describe("useTerminalTouchGestures (hook over the TouchEvent shim)", () => {
  let host: HTMLElement;
  let viewport: HTMLElement;
  let textarea: HTMLTextAreaElement;
  let term: MockTerm;
  let onPinch: ReturnType<typeof vi.fn>;
  let scheduleFit: ReturnType<typeof vi.fn>;

  function mount(): void {
    renderHook(() =>
      useTerminalTouchGestures({
        viewportRef: { current: viewport },
        termRef: { current: term },
        onPinchFontSize: onPinch,
        scheduleFit,
        cellSize: () => CELL,
      }),
    );
  }

  beforeEach(() => {
    vi.useFakeTimers();
    host = document.createElement("div");
    host.className = "terminal-host";
    host.setAttribute("data-input-active", "false");
    viewport = document.createElement("div");
    viewport.className = "xterm-viewport";
    textarea = document.createElement("textarea");
    textarea.className = "xterm-helper-textarea";
    host.append(viewport, textarea);
    document.body.appendChild(host);
    term = makeTerm();
    // chunk-05 useFontSize stand-in: multiply the 14px base by the ratio, clamp [8,28].
    onPinch = vi.fn((ratio: number) => {
      term.options.fontSize = Math.min(28, Math.max(8, Math.round(14 * ratio)));
    });
    scheduleFit = vi.fn();
  });

  afterEach(() => {
    host.remove();
    vi.useRealTimers();
  });

  // UAC-010 — 500ms dwell + drag → term.select non-empty, no focus, view mode kept.
  it("UAC-010: long-press dwell + drag selects via term.select and never enters input mode", () => {
    mount();
    longPressAndDrag(viewport, 100, 200, 30, 0, 500);

    expect(term.selectCalls.length).toBeGreaterThan(0);
    expect(term.selectCalls[0]).toEqual([10, 10, 3]); // floor(100/10), floor(200/20), round(30/10)
    expect(term.getSelection()).not.toBe(""); // non-empty selection (counterexample: tap→focus would leave empty)
    expect(document.activeElement).not.toBe(textarea);
    expect(host.getAttribute("data-input-active")).toBe("false");
  });

  // UAC-011 — dwell-absent swipe scrolls only: no selection, no preventDefault hijack.
  it("UAC-011: a swipe without dwell never selects (getSelection stays empty)", () => {
    mount();
    // No timer advance → dwell never fires; this is a pure swipe.
    swipeFromTo(viewport, { clientX: 100, clientY: 300 }, { clientX: 100, clientY: 100 }, 100);

    expect(term.selectCalls).toHaveLength(0);
    expect(term.getSelection()).toBe("");
  });

  // UAC-009 — continuous swipe must not focus the helper textarea (focus count 0).
  it("UAC-009: continuous swipe dispatches zero focus events and keeps view mode", () => {
    mount();
    const focusSpy = vi.fn();
    textarea.addEventListener("focus", focusSpy);

    swipeFromTo(viewport, { clientX: 100, clientY: 400 }, { clientX: 100, clientY: 80 }, 200);

    expect(focusSpy).not.toHaveBeenCalled();
    expect(document.activeElement).not.toBe(textarea);
    expect(host.getAttribute("data-input-active")).toBe("false");
  });

  // UAC-016 — pinch follows the d_now/d_start ratio (counterexample A: ±2px step)
  // and refits (counterexample B: missing fit()).
  it("UAC-016: pinch out 1.5x drives fontSize ≥18px (ratio-faithful) and calls scheduleFit", () => {
    mount();
    pinchByRatio(viewport, 1.5, { cx: 100, cy: 100 }, 40);

    expect(onPinch).toHaveBeenCalled();
    const ratio = onPinch.mock.calls.at(-1)?.[0] as number;
    expect(ratio).toBeCloseTo(1.5, 5); // not a fixed ±2px step
    expect(term.options.fontSize).toBeGreaterThanOrEqual(18); // 14*1.5≈21 > 18
    expect(term.options.fontSize).toBeLessThanOrEqual(28); // clamp ceiling honoured
    expect(scheduleFit).toHaveBeenCalled();
    expect(term.selectCalls).toHaveLength(0); // pinch must not select
  });

  // FR-MOB-PINCH-003 — a 1→2 finger transition interrupts the swipe and never
  // flips into input mode.
  it("FR-MOB-PINCH-003: second finger mid-swipe interrupts into pinch without input-mode transition", () => {
    mount();
    // 1-finger swipe in progress …
    dispatchTouchEvent(viewport, "touchstart", [makeT(100, 100, 0)], [makeT(100, 100, 0)]);
    dispatchTouchEvent(viewport, "touchmove", [makeT(100, 130, 0)], [makeT(100, 130, 0)]);
    // … a second finger lands → interrupt into pinch (baseline frame only suppresses).
    dispatchTouchEvent(
      viewport,
      "touchmove",
      [makeT(60, 100, 0), makeT(160, 100, 1)],
      [makeT(60, 100, 0), makeT(160, 100, 1)],
    );
    // … a subsequent pinch frame delivers the ratio.
    dispatchTouchEvent(
      viewport,
      "touchmove",
      [makeT(40, 100, 0), makeT(160, 100, 1)],
      [makeT(40, 100, 0), makeT(160, 100, 1)],
    );

    expect(onPinch).toHaveBeenCalled(); // pinch took over
    expect(term.selectCalls).toHaveLength(0); // swipe was interrupted, not turned into a selection
    expect(document.activeElement).not.toBe(textarea);
    expect(host.getAttribute("data-input-active")).toBe("false");
  });

  // preventDefault discipline — exactly: not on plain swipe, yes after dwell, yes on pinch.
  describe("preventDefault is reserved for dwell-drag + pinch", () => {
    it("plain swipe move does NOT preventDefault (pan-y native scroll owns it)", () => {
      mount();
      dispatchTouchEvent(viewport, "touchstart", [makeT(100, 300, 0)], [makeT(100, 300, 0)]);
      const move = dispatchTouchEvent(
        viewport,
        "touchmove",
        [makeT(100, 200, 0)],
        [makeT(100, 200, 0)],
      );
      expect(move.defaultPrevented).toBe(false);
    });

    it("dwell-drag move DOES preventDefault; the preceding touchstart does not", () => {
      mount();
      const start = dispatchTouchEvent(
        viewport,
        "touchstart",
        [makeT(100, 200, 0)],
        [makeT(100, 200, 0)],
      );
      expect(start.defaultPrevented).toBe(false); // touchstart is never suppressed
      vi.advanceTimersByTime(500); // dwell fires
      const drag = dispatchTouchEvent(
        viewport,
        "touchmove",
        [makeT(120, 200, 0)],
        [makeT(120, 200, 0)],
      );
      expect(drag.defaultPrevented).toBe(true);
    });

    it("pinch move DOES preventDefault", () => {
      mount();
      dispatchTouchEvent(
        viewport,
        "touchstart",
        [makeT(60, 100, 0), makeT(140, 100, 1)],
        [makeT(60, 100, 0), makeT(140, 100, 1)],
      );
      const move = dispatchTouchEvent(
        viewport,
        "touchmove",
        [makeT(40, 100, 0), makeT(160, 100, 1)],
        [makeT(40, 100, 0), makeT(160, 100, 1)],
      );
      expect(move.defaultPrevented).toBe(true);
    });
  });

  it("attaches a non-passive touchmove listener so preventDefault is honoured", () => {
    const addSpy = vi.spyOn(viewport, "addEventListener");
    mount();
    const moveCall = addSpy.mock.calls.find((c) => c[0] === "touchmove");
    expect(moveCall?.[2]).toEqual({ passive: false });
  });
});
