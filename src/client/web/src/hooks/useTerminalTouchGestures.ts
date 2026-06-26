// useTerminalTouchGestures — one touch listener over `.xterm-viewport` that
// arbitrates swipe (scroll) / long-press (selection) / pinch (fontSize) through a
// single pure state machine (ADR 0071, FR-MOB-SCROLL-001..003 / FR-MOB-SELECT-001/002
// / FR-MOB-PINCH-001/003).
//
// Why one machine: swipe, long-press and pinch all read the same touch source.
// Splitting them into separate listeners re-introduces ordering bugs and double
// `preventDefault` (ADR 0071 rejected alternative). Collapsing them into one
// reducer makes the arbitration total and provable:
//
//   idle ──touchstart(1)──▶ swipe ──dwell(500ms,<8px)──▶ dwell ──move──▶ longpress-drag
//     │                       │                                              (term.select)
//     │                       └─move>8px─▶ swipe (scroll, no preventDefault)
//     └──touchstart(2)/1→2────▶ pinch (onPinchFontSize(ratio) + scheduleFit)
//
// `preventDefault` is emitted by the reducer in exactly two places — after a dwell
// is realised (longpress-drag) and on every pinch move. Plain swipe never
// preventDefaults, so the CSS `touch-action: pan-y` lane (terminal-gestures.css)
// owns native scroll (FR-MOB-SCROLL-001). Pinch only calls `onPinchFontSize(ratio)`
// + `scheduleFit()`; it never enters input mode (FR-MOB-PINCH-003) — the hook has
// no path that focuses the helper textarea or flips data-input-active.

import { type RefObject, useEffect, useRef } from "react";
import "../css/terminal-gestures.css";

/** Stationary tolerance before a hold is rejected as a swipe (px). */
export const MOVE_THRESHOLD = 8;
/** Dwell time before a stationary hold becomes a selection (ms). */
export const DWELL_MS = 500;

/** A point in viewport-relative client coordinates. */
export interface Pt {
  x: number;
  y: number;
}

/** Cell metrics used to map pixels → xterm cells for `term.select`. */
export interface Cell {
  width: number;
  height: number;
}

/** Normalised input to the reducer, derived from a TouchEvent (or a fake timer). */
export type GestureInput =
  | { kind: "start"; touches: Pt[]; cell: Cell }
  | { kind: "move"; touches: Pt[]; cell: Cell }
  | { kind: "end" }
  | { kind: "dwellElapsed" };

/** The five arbitration states (ADR 0071). */
export type GestureState =
  | { phase: "idle" }
  | { phase: "swipe"; start: Pt; cell: Cell; moved: boolean }
  | { phase: "dwell"; start: Pt; cell: Cell }
  | { phase: "longpress-drag"; start: Pt; cell: Cell }
  | { phase: "pinch"; startDistance: number };

/** Side effects the reducer asks the host to run (kept out of the pure core). */
export type GestureEffect =
  | { kind: "preventDefault" }
  | { kind: "select"; col: number; row: number; length: number }
  | { kind: "pinch"; ratio: number }
  | { kind: "scheduleFit" };

export interface ReduceResult {
  state: GestureState;
  effects: GestureEffect[];
}

export const INITIAL_GESTURE_STATE: GestureState = { phase: "idle" };

function dist(a: Pt, b: Pt): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

function touchDistance(t: Pt[]): number {
  const [a, b] = t;
  return a && b ? Math.hypot(a.x - b.x, a.y - b.y) : 0;
}

/** Start position → cell coordinate; drag distance → selection length (≥1 cell). */
function selectFromDrag(start: Pt, current: Pt, cell: Cell): GestureEffect {
  const col = Math.max(0, Math.floor(start.x / cell.width));
  const row = Math.max(0, Math.floor(start.y / cell.height));
  const length = Math.max(1, Math.round(dist(start, current) / cell.width));
  return { kind: "select", col, row, length };
}

function reduceStart(input: Extract<GestureInput, { kind: "start" }>): ReduceResult {
  // Two (or more) fingers down at once → pinch immediately; record the baseline
  // gap. No preventDefault yet (no movement to suppress).
  if (input.touches.length >= 2) {
    return { state: { phase: "pinch", startDistance: touchDistance(input.touches) }, effects: [] };
  }
  const start = input.touches[0] ?? { x: 0, y: 0 };
  return { state: { phase: "swipe", start, cell: input.cell, moved: false }, effects: [] };
}

function reducePinchMove(state: GestureState, current: Pt[]): ReduceResult {
  // 1→2 finger transition mid-swipe: interrupt the swipe and seed the baseline.
  // First pinch frame only preventDefaults (ratio would be 1, no font change yet).
  if (state.phase !== "pinch") {
    return {
      state: { phase: "pinch", startDistance: touchDistance(current) },
      effects: [{ kind: "preventDefault" }],
    };
  }
  const ratio = state.startDistance > 0 ? touchDistance(current) / state.startDistance : 1;
  return {
    state,
    effects: [{ kind: "preventDefault" }, { kind: "pinch", ratio }, { kind: "scheduleFit" }],
  };
}

function reduceMove(
  state: GestureState,
  input: Extract<GestureInput, { kind: "move" }>,
): ReduceResult {
  if (input.touches.length >= 2) return reducePinchMove(state, input.touches);
  const cur = input.touches[0] ?? { x: 0, y: 0 };

  switch (state.phase) {
    case "swipe": {
      // Plain swipe: defer to pan-y native scroll. Crossing the threshold locks
      // out the dwell (the dwellElapsed guard reads `moved`). No preventDefault.
      const moved = state.moved || dist(state.start, cur) > MOVE_THRESHOLD;
      return { state: { ...state, moved }, effects: [] };
    }
    case "dwell": {
      // Dwell realised, first drag → start selecting. First and only place a
      // single-finger move preventDefaults.
      return {
        state: { phase: "longpress-drag", start: state.start, cell: state.cell },
        effects: [{ kind: "preventDefault" }, selectFromDrag(state.start, cur, state.cell)],
      };
    }
    case "longpress-drag":
      return {
        state,
        effects: [{ kind: "preventDefault" }, selectFromDrag(state.start, cur, state.cell)],
      };
    default:
      return { state, effects: [] };
  }
}

/**
 * gestureReducer — the pure arbitration core. `(state, input) → { state, effects }`
 * with no DOM, timers or term access, so the whole transition table is unit-tested
 * against the TouchEvent shim and the counterexamples.
 */
export function gestureReducer(state: GestureState, input: GestureInput): ReduceResult {
  switch (input.kind) {
    case "start":
      return reduceStart(input);
    case "move":
      return reduceMove(state, input);
    case "dwellElapsed":
      // Promote a stationary swipe to a dwell only if it never moved past 8px.
      if (state.phase === "swipe" && !state.moved) {
        return { state: { phase: "dwell", start: state.start, cell: state.cell }, effects: [] };
      }
      return { state, effects: [] };
    case "end":
      return { state: INITIAL_GESTURE_STATE, effects: [] };
    default:
      return { state, effects: [] };
  }
}

/** Minimal xterm `Terminal` surface this hook touches (mocked in tests, wired in chunk-07).
 *  Only `select(c, r, l)` is consumed — the broader xterm Terminal type extends this
 *  trivially, and tests carry any additional spy state on their own mock object. */
export interface TerminalLike {
  select(column: number, row: number, length: number): void;
}

export interface UseTerminalTouchGesturesOptions {
  /** `.xterm-viewport` the single listener attaches to. */
  viewportRef: RefObject<HTMLElement | null>;
  /** xterm Terminal (or mock) for programmatic `select` (chunk-07 wires the real one). */
  termRef: RefObject<TerminalLike | null>;
  /** Called with `d_now / d_start` on each pinch move; chunk-05 useFontSize clamps + applies. */
  onPinchFontSize: (ratio: number) => void;
  /** ADR 0034 rAF-coalesced refit, invoked once per pinch move. */
  scheduleFit: () => void;
  /** Pixel size of one xterm cell; defaults to a coarse constant when unavailable. */
  cellSize?: () => Cell | null;
}

const DEFAULT_CELL: Cell = { width: 9, height: 17 };

/**
 * useTerminalTouchGestures attaches one handler to `touchstart/touchmove/touchend`
 * on `viewportRef.current` and drives `gestureReducer`. State lives in a ref (no
 * re-render), the dwell timer fires `dwellElapsed`, and effects are applied here:
 * preventDefault on the live event, `term.select(...)`, `onPinchFontSize(ratio)`,
 * `scheduleFit()`.
 */
export function useTerminalTouchGestures(options: UseTerminalTouchGesturesOptions): void {
  const { viewportRef, termRef, onPinchFontSize, scheduleFit, cellSize } = options;

  // Volatile callbacks via refs so the listeners attach once and never re-subscribe.
  const onPinchRef = useRef(onPinchFontSize);
  onPinchRef.current = onPinchFontSize;
  const scheduleFitRef = useRef(scheduleFit);
  scheduleFitRef.current = scheduleFit;
  const cellSizeRef = useRef(cellSize);
  cellSizeRef.current = cellSize;

  // biome-ignore lint/correctness/useExhaustiveDependencies: attach-once listener; refs are stable.
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;

    let state: GestureState = INITIAL_GESTURE_STATE;
    let dwellTimer: ReturnType<typeof setTimeout> | null = null;

    const clearDwell = (): void => {
      if (dwellTimer !== null) {
        clearTimeout(dwellTimer);
        dwellTimer = null;
      }
    };

    const apply = (effects: GestureEffect[], event: Event | null): void => {
      for (const effect of effects) {
        switch (effect.kind) {
          case "preventDefault":
            event?.preventDefault();
            break;
          case "select":
            termRef.current?.select(effect.col, effect.row, effect.length);
            break;
          case "pinch":
            onPinchRef.current(effect.ratio);
            break;
          case "scheduleFit":
            scheduleFitRef.current();
            break;
        }
      }
    };

    const dispatch = (input: GestureInput, event: Event | null): void => {
      const result = gestureReducer(state, input);
      state = result.state;
      apply(result.effects, event);
      // Re-arm / cancel the dwell timer purely from the resulting phase.
      if (state.phase !== "swipe") clearDwell();
    };

    const cell = (): Cell => cellSizeRef.current?.() ?? DEFAULT_CELL;
    const rectPoints = (touches: ArrayLike<Touch>): Pt[] => {
      const rect = viewport.getBoundingClientRect();
      return Array.from(touches).map((t) => ({
        x: t.clientX - rect.left,
        y: t.clientY - rect.top,
      }));
    };

    const onStart = (event: Event): void => {
      const te = event as TouchEvent;
      clearDwell();
      dispatch({ kind: "start", touches: rectPoints(te.touches), cell: cell() }, event);
      // Arm the dwell only while a single finger rests (we just entered swipe).
      if (state.phase === "swipe") {
        dwellTimer = setTimeout(() => dispatch({ kind: "dwellElapsed" }, null), DWELL_MS);
      }
    };

    const onMove = (event: Event): void => {
      const te = event as TouchEvent;
      dispatch({ kind: "move", touches: rectPoints(te.touches), cell: cell() }, event);
    };

    const onEnd = (event: Event): void => {
      clearDwell();
      dispatch({ kind: "end" }, event);
    };

    // passive:false is load-bearing — preventDefault on touchmove (dwell/pinch)
    // must be honoured by the browser.
    viewport.addEventListener("touchstart", onStart, { passive: false });
    viewport.addEventListener("touchmove", onMove, { passive: false });
    viewport.addEventListener("touchend", onEnd, { passive: false });
    return () => {
      clearDwell();
      viewport.removeEventListener("touchstart", onStart);
      viewport.removeEventListener("touchmove", onMove);
      viewport.removeEventListener("touchend", onEnd);
    };
  }, []);
}
