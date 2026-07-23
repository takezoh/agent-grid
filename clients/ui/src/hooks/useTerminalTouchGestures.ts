// useTerminalTouchGestures — one touch listener over `.xterm-viewport` that
// arbitrates swipe (horizontal → arrow key spam, vertical → native scroll) and
// long-press (selection) through a single pure state machine.
//
// ADR 0077 supersedes the original ADR 0071 pinch path: 2-finger touches are
// collapsed to idle (no fontSize effect, no PinchIndicator), and the swipe
// phase now carries an axis lock + a `lastArrowX` cursor so a horizontal-
// locked swipe emits `{kind:"arrow"}` effects in cell-width increments. The
// host (`TerminalMobileOverlay`) gates the apply layer with `isInputActive()`
// and forms the VT100 byte sequence (`"\x1b[C"` / `"\x1b[D"`) outside the
// reducer.
//
//   idle ──touchstart(1)──▶ swipe(axis=undecided) ──dwell(500ms,<8px)──▶ dwell ──▶ longpress-drag
//     │                       │                                                       (term.select)
//     │                       └─move>8px─▶ axis locked horizontal | vertical
//     │                                       │                       │
//     │                                       ▼                       ▼
//     │                              arrow effects (1 per           native pan-y
//     │                              touchmove, count cells)        scroll
//     └──touchstart(≥2)/1→2────▶ idle (collapsed; no pinch)
//
// `preventDefault` is emitted by the reducer in exactly one place — after a
// dwell is realised (longpress-drag). Plain swipe never preventDefaults, so
// the CSS `touch-action: pan-y` lane (terminal-gestures.css) owns native
// scroll (FR-MOB-SCROLL-001) and horizontal swipe has no native xterm
// behaviour to suppress. 2-finger collapse is total (`FR-MOB-SWIPE-ARROW-003`).

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

/** Cell metrics used to map pixels → xterm cells for `term.select` and arrow chunking. */
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

/** Axis lock for the swipe phase — undecided until the first move past MOVE_THRESHOLD. */
export type SwipeAxis = "undecided" | "horizontal" | "vertical";

/** Arbitration states. `pinch` is removed (ADR 0077); 2-finger touches collapse to idle. */
export type GestureState =
  | { phase: "idle" }
  | {
      phase: "swipe";
      start: Pt;
      cell: Cell;
      moved: boolean;
      axis: SwipeAxis;
      /** x-coordinate of the most recent arrow emission; residuals < cell.width carry over. */
      lastArrowX: number;
    }
  | { phase: "dwell"; start: Pt; cell: Cell }
  | { phase: "longpress-drag"; start: Pt; cell: Cell };

/** Side effects the reducer asks the host to run (kept out of the pure core). */
export type GestureEffect =
  | { kind: "preventDefault" }
  | { kind: "select"; col: number; row: number; length: number }
  | { kind: "arrow"; direction: "left" | "right"; count: number };

export interface ReduceResult {
  state: GestureState;
  effects: GestureEffect[];
}

export const INITIAL_GESTURE_STATE: GestureState = { phase: "idle" };

function dist(a: Pt, b: Pt): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

/** Start position → cell coordinate; drag distance → selection length (≥1 cell). */
function selectFromDrag(start: Pt, current: Pt, cell: Cell): GestureEffect {
  const col = Math.max(0, Math.floor(start.x / cell.width));
  const row = Math.max(0, Math.floor(start.y / cell.height));
  const length = Math.max(1, Math.round(dist(start, current) / cell.width));
  return { kind: "select", col, row, length };
}

function reduceStart(input: Extract<GestureInput, { kind: "start" }>): ReduceResult {
  // 2+ fingers down at once → collapse to idle (ADR 0077, FR-MOB-SWIPE-ARROW-003).
  // No effect, no preventDefault — the gesture is fully ignored.
  if (input.touches.length >= 2) {
    return { state: INITIAL_GESTURE_STATE, effects: [] };
  }
  const start = input.touches[0] ?? { x: 0, y: 0 };
  return {
    state: {
      phase: "swipe",
      start,
      cell: input.cell,
      moved: false,
      axis: "undecided",
      lastArrowX: start.x,
    },
    effects: [],
  };
}

/** Compute arrow effect from horizontal residual; mutates `state.lastArrowX`. */
function reduceHorizontalArrow(
  state: Extract<GestureState, { phase: "swipe" }>,
  curX: number,
): { state: GestureState; effects: GestureEffect[] } {
  const delta = curX - state.lastArrowX;
  const n = Math.trunc(delta / state.cell.width);
  if (n === 0) return { state, effects: [] };
  const advanced = n * state.cell.width;
  return {
    state: { ...state, lastArrowX: state.lastArrowX + advanced },
    effects: [{ kind: "arrow", direction: n > 0 ? "right" : "left", count: Math.abs(n) }],
  };
}

function reduceSwipeMove(state: Extract<GestureState, { phase: "swipe" }>, cur: Pt): ReduceResult {
  const dx = cur.x - state.start.x;
  const dy = cur.y - state.start.y;
  const moved = state.moved || Math.hypot(dx, dy) > MOVE_THRESHOLD;
  // Lock the axis exactly once, at the first frame that crosses MOVE_THRESHOLD.
  // Per-frame dominance would flicker on diagonal swipes; one-shot lock is stable.
  let axis = state.axis;
  if (axis === "undecided" && moved) {
    axis = Math.abs(dx) > Math.abs(dy) ? "horizontal" : "vertical";
  }
  const moved_state: Extract<GestureState, { phase: "swipe" }> = { ...state, moved, axis };
  if (axis !== "horizontal") {
    return { state: moved_state, effects: [] };
  }
  return reduceHorizontalArrow(moved_state, cur.x);
}

function reduceMove(
  state: GestureState,
  input: Extract<GestureInput, { kind: "move" }>,
): ReduceResult {
  // 2+ fingers during any phase → collapse (ADR 0077, FR-MOB-SWIPE-ARROW-003).
  // Mirrors the `start` discipline so a 1→2 finger transition can't sneak past.
  if (input.touches.length >= 2) {
    return { state: INITIAL_GESTURE_STATE, effects: [] };
  }
  const cur = input.touches[0] ?? { x: 0, y: 0 };

  switch (state.phase) {
    case "swipe":
      return reduceSwipeMove(state, cur);
    case "dwell":
      // Dwell realised, first drag → start selecting. First and only place a
      // single-finger move preventDefaults.
      return {
        state: { phase: "longpress-drag", start: state.start, cell: state.cell },
        effects: [{ kind: "preventDefault" }, selectFromDrag(state.start, cur, state.cell)],
      };
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
  /** xterm Terminal (or mock) for programmatic `select`. */
  termRef: RefObject<TerminalLike | null>;
  /**
   * Called whenever a horizontal swipe accumulates ≥ 1 cell of motion in either
   * direction (ADR 0077, FR-MOB-SWIPE-ARROW-001). Host turns this into
   * `"\x1b[C".repeat(count)` / `"\x1b[D".repeat(count)` and sends it as one
   * wire frame. Effect is dropped silently when `isInputActive?()` is false.
   */
  onArrowKey?: (direction: "left" | "right", count: number) => void;
  /**
   * Apply-layer gate (ADR 0077). When omitted or returning false, the reducer
   * still computes arrow effects but the hook drops them — view mode keeps the
   * legacy scroll-only behaviour byte-identical (FR-MOB-SWIPE-ARROW-002).
   */
  isInputActive?: () => boolean;
  /** Pixel size of one xterm cell; defaults to a coarse constant when unavailable. */
  cellSize?: () => Cell | null;
}

const DEFAULT_CELL: Cell = { width: 9, height: 17 };

/**
 * useTerminalTouchGestures attaches one handler to `touchstart/touchmove/touchend`
 * on `viewportRef.current` and drives `gestureReducer`. State lives in a ref (no
 * re-render), the dwell timer fires `dwellElapsed`, and effects are applied here:
 * preventDefault on the live event, `term.select(...)`, and `onArrowKey(...)`
 * (gated by `isInputActive?()`).
 */
export function useTerminalTouchGestures(options: UseTerminalTouchGesturesOptions): void {
  const { viewportRef, termRef, onArrowKey, isInputActive, cellSize } = options;

  // Volatile callbacks via refs so the listeners attach once and never re-subscribe.
  const onArrowRef = useRef(onArrowKey);
  onArrowRef.current = onArrowKey;
  const isInputActiveRef = useRef(isInputActive);
  isInputActiveRef.current = isInputActive;
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
          case "arrow":
            // ADR 0077: gate at apply, not in the reducer. View mode silently
            // drops the effect so the legacy scroll-only path is preserved.
            if (isInputActiveRef.current?.() ?? false) {
              onArrowRef.current?.(effect.direction, effect.count);
            }
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

    // passive:false is load-bearing — preventDefault on touchmove (dwell-drag)
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
