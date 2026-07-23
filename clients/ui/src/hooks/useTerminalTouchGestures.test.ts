// useTerminalTouchGestures.test.ts — ADR 0077 swipe-to-arrow arbitration.
//
// Layer 1: gestureReducer — pure transition table over idle/swipe/dwell/
// longpress-drag, with the horizontal-swipe arrow emission discipline and the
// 2-finger collapse contract.
//
// Layer 2: useTerminalTouchGestures — hook driven through the chunk-01 TouchEvent
// shim (swipeFromTo / longPressAndDrag / pinchByRatio + fake timers), asserting
// the input-mode gate (FR-MOB-SWIPE-ARROW-001/002), the 2-finger collapse
// (FR-MOB-SWIPE-ARROW-003), and the long-press selection regression (UAC-010/011).

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

/** Construct a fresh swipe-phase state at (x, y) for reducer tests. */
function freshSwipe(x: number, y: number): Extract<GestureState, { phase: "swipe" }> {
  const start = gestureReducer(INITIAL_GESTURE_STATE, {
    kind: "start",
    touches: [{ x, y }],
    cell: CELL,
  }).state;
  if (start.phase !== "swipe") throw new Error("expected swipe after 1-finger start");
  return start;
}

// ---------------------------------------------------------------------------
// 1. Pure reducer — transition table
// ---------------------------------------------------------------------------

describe("gestureReducer (pure transition table)", () => {
  it("idle + 1-finger touchstart → swipe (axis=undecided, lastArrowX=start.x)", () => {
    const r = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [{ x: 100, y: 100 }],
      cell: CELL,
    });
    expect(r.state).toEqual({
      phase: "swipe",
      start: { x: 100, y: 100 },
      cell: CELL,
      moved: false,
      axis: "undecided",
      lastArrowX: 100,
    });
    expect(r.effects).toEqual([]);
  });

  it("swipe + move <8px → axis undecided, moved false, no effects", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 105, y: 100 }], cell: CELL });
    expect(r.state.phase).toBe("swipe");
    if (r.state.phase !== "swipe") throw new Error("unreachable");
    expect(r.state.moved).toBe(false);
    expect(r.state.axis).toBe("undecided");
    expect(r.effects).toEqual([]);
  });

  it("swipe + first move past 8px with |dx|>|dy| → axis horizontal locked", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 115, y: 102 }], cell: CELL });
    expect(r.state.phase).toBe("swipe");
    if (r.state.phase !== "swipe") throw new Error("unreachable");
    expect(r.state.axis).toBe("horizontal");
    expect(r.state.moved).toBe(true);
  });

  it("swipe + first move past 8px with |dy|>=|dx| → axis vertical locked (never arrow)", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 102, y: 130 }], cell: CELL });
    expect(r.state.phase).toBe("swipe");
    if (r.state.phase !== "swipe") throw new Error("unreachable");
    expect(r.state.axis).toBe("vertical");
    expect(r.effects).toEqual([]);
  });

  it("horizontal swipe: 18px right (cell.width=10) → arrow{right,1}, lastArrowX advances 10", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 118, y: 100 }], cell: CELL });
    expect(r.effects).toEqual([{ kind: "arrow", direction: "right", count: 1 }]);
    if (r.state.phase !== "swipe") throw new Error("unreachable");
    expect(r.state.lastArrowX).toBe(110);
  });

  it("horizontal swipe: 25px right → arrow{right,2}, lastArrowX advances 20", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 125, y: 100 }], cell: CELL });
    expect(r.effects).toEqual([{ kind: "arrow", direction: "right", count: 2 }]);
    if (r.state.phase !== "swipe") throw new Error("unreachable");
    expect(r.state.lastArrowX).toBe(120);
  });

  it("horizontal swipe: residuals carry over — 13px → 1, 15px more (28 total) → 1 more", () => {
    let s: GestureState = freshSwipe(100, 100);
    const r1 = gestureReducer(s, { kind: "move", touches: [{ x: 113, y: 100 }], cell: CELL });
    expect(r1.effects).toEqual([{ kind: "arrow", direction: "right", count: 1 }]);
    s = r1.state;
    // total absolute 128 → delta from lastArrowX(110) is 18 → trunc(18/10)=1
    const r2 = gestureReducer(s, { kind: "move", touches: [{ x: 128, y: 100 }], cell: CELL });
    expect(r2.effects).toEqual([{ kind: "arrow", direction: "right", count: 1 }]);
    if (r2.state.phase !== "swipe") throw new Error("unreachable");
    expect(r2.state.lastArrowX).toBe(120);
  });

  it("horizontal swipe: sub-cell residual stays in state, no emission", () => {
    const swipe = freshSwipe(100, 100);
    // 9px > MOVE_THRESHOLD (8) so axis locks, but |Δx|/cell.width = 0.9 → trunc 0
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 109, y: 100 }], cell: CELL });
    expect(r.effects).toEqual([]);
    if (r.state.phase !== "swipe") throw new Error("unreachable");
    expect(r.state.axis).toBe("horizontal");
    expect(r.state.lastArrowX).toBe(100); // residual preserved
  });

  it("horizontal swipe: leftward 18px → arrow{left,1}", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, { kind: "move", touches: [{ x: 82, y: 100 }], cell: CELL });
    expect(r.effects).toEqual([{ kind: "arrow", direction: "left", count: 1 }]);
    if (r.state.phase !== "swipe") throw new Error("unreachable");
    expect(r.state.lastArrowX).toBe(90);
  });

  it("vertical-locked swipe: any horizontal residual never emits arrow", () => {
    const swipe = freshSwipe(100, 100);
    // Vertical lock first (dy dominates)
    const locked = gestureReducer(swipe, {
      kind: "move",
      touches: [{ x: 102, y: 130 }],
      cell: CELL,
    }).state;
    // Now drift sideways — must still be silent because the axis is vertical.
    const r = gestureReducer(locked, { kind: "move", touches: [{ x: 150, y: 132 }], cell: CELL });
    expect(r.effects).toEqual([]);
  });

  it("2-finger touchstart → idle collapse (FR-MOB-SWIPE-ARROW-003)", () => {
    const r = gestureReducer(INITIAL_GESTURE_STATE, {
      kind: "start",
      touches: [
        { x: 60, y: 100 },
        { x: 140, y: 100 },
      ],
      cell: CELL,
    });
    expect(r.state).toEqual(INITIAL_GESTURE_STATE);
    expect(r.effects).toEqual([]);
  });

  it("1-finger swipe + 2-finger move (1→2 finger transition) → idle collapse, no effects", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, {
      kind: "move",
      touches: [
        { x: 60, y: 100 },
        { x: 140, y: 100 },
      ],
      cell: CELL,
    });
    expect(r.state).toEqual(INITIAL_GESTURE_STATE);
    expect(r.effects).toEqual([]);
  });

  it("swipe + dwellElapsed while stationary → dwell", () => {
    const swipe = freshSwipe(100, 100);
    const r = gestureReducer(swipe, { kind: "dwellElapsed" });
    expect(r.state.phase).toBe("dwell");
  });

  it("swipe + dwellElapsed AFTER moving past 8px → stays swipe (no false dwell)", () => {
    const swipe = freshSwipe(100, 100);
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
  let onArrowKey: ReturnType<typeof vi.fn>;
  let isInputActiveValue: boolean;

  function mount(): void {
    renderHook(() =>
      useTerminalTouchGestures({
        viewportRef: { current: viewport },
        termRef: { current: term },
        onArrowKey,
        isInputActive: () => isInputActiveValue,
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
    onArrowKey = vi.fn();
    isInputActiveValue = true; // default to input mode for arrow tests
  });

  afterEach(() => {
    host.remove();
    vi.useRealTimers();
  });

  // FR-MOB-SWIPE-ARROW-001 — input mode + horizontal swipe → arrow callbacks.
  it("FR-MOB-SWIPE-ARROW-001: input mode + 100px right swipe → arrow{right} sum count = 10", () => {
    mount();
    swipeFromTo(viewport, { clientX: 100, clientY: 200 }, { clientX: 200, clientY: 200 }, 100);

    expect(onArrowKey).toHaveBeenCalled();
    let totalRight = 0;
    let totalLeft = 0;
    for (const call of onArrowKey.mock.calls) {
      const [direction, count] = call as ["left" | "right", number];
      if (direction === "right") totalRight += count;
      else totalLeft += count;
    }
    expect(totalRight).toBe(10);
    expect(totalLeft).toBe(0);
  });

  // FR-MOB-SWIPE-ARROW-001 — leftward swipe direction is honoured.
  it("FR-MOB-SWIPE-ARROW-001: input mode + 100px left swipe → arrow{left} sum count = 10", () => {
    mount();
    swipeFromTo(viewport, { clientX: 200, clientY: 200 }, { clientX: 100, clientY: 200 }, 100);

    let totalRight = 0;
    let totalLeft = 0;
    for (const call of onArrowKey.mock.calls) {
      const [direction, count] = call as ["left" | "right", number];
      if (direction === "right") totalRight += count;
      else totalLeft += count;
    }
    expect(totalLeft).toBe(10);
    expect(totalRight).toBe(0);
  });

  // FR-MOB-SWIPE-ARROW-002 — view mode: arrow callbacks must not fire.
  it("FR-MOB-SWIPE-ARROW-002: view mode + horizontal swipe → onArrowKey never fires", () => {
    isInputActiveValue = false;
    mount();
    swipeFromTo(viewport, { clientX: 100, clientY: 200 }, { clientX: 200, clientY: 200 }, 100);

    expect(onArrowKey).not.toHaveBeenCalled();
  });

  // FR-MOB-SWIPE-ARROW-002 — vertical swipe is silent even in input mode.
  it("FR-MOB-SWIPE-ARROW-002: vertical swipe in input mode → onArrowKey never fires", () => {
    mount();
    swipeFromTo(viewport, { clientX: 100, clientY: 400 }, { clientX: 100, clientY: 80 }, 200);

    expect(onArrowKey).not.toHaveBeenCalled();
  });

  // FR-MOB-SWIPE-ARROW-003 — 2-finger pinch is fully ignored: no arrow, no select.
  it("FR-MOB-SWIPE-ARROW-003: 2-finger pinch → no arrow, no select, no input-mode flip", () => {
    mount();
    pinchByRatio(viewport, 1.5, { cx: 100, cy: 100 }, 40);

    expect(onArrowKey).not.toHaveBeenCalled();
    expect(term.selectCalls).toHaveLength(0);
    expect(document.activeElement).not.toBe(textarea);
    expect(host.getAttribute("data-input-active")).toBe("false");
  });

  // FR-MOB-SWIPE-ARROW-003 — 1→2 finger mid-swipe collapses to idle.
  it("FR-MOB-SWIPE-ARROW-003: second finger mid-swipe → collapse, no further arrows", () => {
    mount();
    dispatchTouchEvent(viewport, "touchstart", [makeT(100, 100, 0)], [makeT(100, 100, 0)]);
    dispatchTouchEvent(viewport, "touchmove", [makeT(120, 100, 0)], [makeT(120, 100, 0)]);
    onArrowKey.mockClear();
    // Second finger lands → idle collapse.
    dispatchTouchEvent(
      viewport,
      "touchmove",
      [makeT(140, 100, 0), makeT(200, 100, 1)],
      [makeT(140, 100, 0), makeT(200, 100, 1)],
    );
    // Subsequent 2-finger move stays idle.
    dispatchTouchEvent(
      viewport,
      "touchmove",
      [makeT(160, 100, 0), makeT(220, 100, 1)],
      [makeT(160, 100, 0), makeT(220, 100, 1)],
    );
    expect(onArrowKey).not.toHaveBeenCalled();
  });

  // UAC-010 — 500ms dwell + drag → term.select non-empty, no focus, view mode kept.
  it("UAC-010: long-press dwell + drag selects via term.select and never enters input mode", () => {
    isInputActiveValue = false; // view mode is the canonical context for long-press
    mount();
    longPressAndDrag(viewport, 100, 200, 30, 0, 500);

    expect(term.selectCalls.length).toBeGreaterThan(0);
    expect(term.selectCalls[0]).toEqual([10, 10, 3]); // floor(100/10), floor(200/20), round(30/10)
    expect(term.getSelection()).not.toBe("");
    expect(document.activeElement).not.toBe(textarea);
    expect(host.getAttribute("data-input-active")).toBe("false");
  });

  // UAC-011 — dwell-absent swipe scrolls only: no selection.
  it("UAC-011: a swipe without dwell never selects (getSelection stays empty)", () => {
    mount();
    swipeFromTo(viewport, { clientX: 100, clientY: 300 }, { clientX: 100, clientY: 100 }, 100);

    expect(term.selectCalls).toHaveLength(0);
    expect(term.getSelection()).toBe("");
  });

  // UAC-009 — continuous swipe must not focus the helper textarea.
  it("UAC-009: continuous swipe dispatches zero focus events and keeps view mode", () => {
    mount();
    const focusSpy = vi.fn();
    textarea.addEventListener("focus", focusSpy);

    swipeFromTo(viewport, { clientX: 100, clientY: 400 }, { clientX: 100, clientY: 80 }, 200);

    expect(focusSpy).not.toHaveBeenCalled();
    expect(document.activeElement).not.toBe(textarea);
    expect(host.getAttribute("data-input-active")).toBe("false");
  });

  // preventDefault discipline — only after dwell + during longpress-drag.
  describe("preventDefault is reserved for dwell-drag (pinch path is gone)", () => {
    it("plain horizontal swipe move does NOT preventDefault (no native xterm behaviour to suppress)", () => {
      mount();
      dispatchTouchEvent(viewport, "touchstart", [makeT(100, 200, 0)], [makeT(100, 200, 0)]);
      const move = dispatchTouchEvent(
        viewport,
        "touchmove",
        [makeT(200, 200, 0)],
        [makeT(200, 200, 0)],
      );
      expect(move.defaultPrevented).toBe(false);
    });

    it("plain vertical swipe move does NOT preventDefault (pan-y owns the lane)", () => {
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
      expect(start.defaultPrevented).toBe(false);
      vi.advanceTimersByTime(500); // dwell fires
      const drag = dispatchTouchEvent(
        viewport,
        "touchmove",
        [makeT(120, 200, 0)],
        [makeT(120, 200, 0)],
      );
      expect(drag.defaultPrevented).toBe(true);
    });
  });

  it("attaches a non-passive touchmove listener so preventDefault is honoured", () => {
    const addSpy = vi.spyOn(viewport, "addEventListener");
    mount();
    const moveCall = addSpy.mock.calls.find((c) => c[0] === "touchmove");
    expect(moveCall?.[2]).toEqual({ passive: false });
  });
});
