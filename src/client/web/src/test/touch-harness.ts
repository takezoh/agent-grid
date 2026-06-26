// touch-harness.ts — synthetic touch / matchMedia / visualViewport shims for
// happy-dom (vitest). chunk-01 foundation: shared by the mobile gesture / gate
// tests added in chunk-02..07. happy-dom ships no Touch / TouchEvent constructor
// and no visualViewport, so this module synthesises them from plain objects.
//
// Design:
//   - A TouchEvent is built as `new Event(type)` with touches / changedTouches /
//     targetTouches attached afterwards (no dependency on a Touch constructor).
//   - matchMedia / visualViewport are swap-in stubs that return a handle with a
//     restore(). Because this overrides the global matchMedia mock from
//     test-setup.ts, each test must call handle.restore() in afterEach.
//
// happy-dom limits (the Open Question 2 demonstration lives in
// touch-harness.test.ts):
//   - No layout engine, so getBoundingClientRect returns 0 → 44x44 hit-area
//     checks must stub the rect in the test (size assertions move to the
//     real-device checklist).
//   - scrollTop / scrollHeight / clientHeight only exist via manual assignment.

import { vi } from "vitest";

// ---------------------------------------------------------------------------
// Touch event synthesis
// ---------------------------------------------------------------------------

/** A single touch point in client coordinates. */
export interface TouchPoint {
  clientX: number;
  clientY: number;
  /** Stable finger id; defaults to the array index when omitted. */
  identifier?: number;
}

/** Minimal Touch-like record. happy-dom lacks the Touch constructor, so this
 *  is a plain object carrying the fields gesture code reads. */
export interface SyntheticTouch {
  identifier: number;
  target: EventTarget;
  clientX: number;
  clientY: number;
  pageX: number;
  pageY: number;
  screenX: number;
  screenY: number;
  radiusX: number;
  radiusY: number;
  rotationAngle: number;
  force: number;
}

/** A dispatched touch event augmented with the touch lists. */
export interface SyntheticTouchEvent extends Event {
  touches: SyntheticTouch[];
  changedTouches: SyntheticTouch[];
  targetTouches: SyntheticTouch[];
}

function makeTouch(target: EventTarget, p: TouchPoint, index: number): SyntheticTouch {
  return {
    identifier: p.identifier ?? index,
    target,
    clientX: p.clientX,
    clientY: p.clientY,
    pageX: p.clientX,
    pageY: p.clientY,
    screenX: p.clientX,
    screenY: p.clientY,
    radiusX: 1,
    radiusY: 1,
    rotationAngle: 0,
    force: 1,
  };
}

/**
 * dispatchTouchEvent synthesises and dispatches a touch event of `type` on
 * `el`, attaching `touches` / `changedTouches` / `targetTouches`. The returned
 * event can be inspected (e.g. `defaultPrevented`).
 */
export function dispatchTouchEvent(
  el: Element,
  type: "touchstart" | "touchmove" | "touchend" | "touchcancel",
  touches: SyntheticTouch[],
  changedTouches: SyntheticTouch[],
): SyntheticTouchEvent {
  const ev = new Event(type, { bubbles: true, cancelable: true }) as SyntheticTouchEvent;
  ev.touches = touches;
  ev.changedTouches = changedTouches;
  ev.targetTouches = touches;
  el.dispatchEvent(ev);
  return ev;
}

/** Record of events produced by a gesture helper, for assertion in tests. */
export interface GestureTrace {
  type: string;
  touches: SyntheticTouch[];
  changedTouches: SyntheticTouch[];
}

/**
 * tapAt — touchstart → touchend at (x, y) with zero dwell. Returns the ordered
 * event trace.
 */
export function tapAt(el: Element, x: number, y: number): GestureTrace[] {
  const trace: GestureTrace[] = [];
  const start = makeTouch(el, { clientX: x, clientY: y }, 0);
  const down = dispatchTouchEvent(el, "touchstart", [start], [start]);
  trace.push({ type: down.type, touches: down.touches, changedTouches: down.changedTouches });
  const up = dispatchTouchEvent(el, "touchend", [], [start]);
  trace.push({ type: up.type, touches: up.touches, changedTouches: up.changedTouches });
  return trace;
}

/**
 * swipeFromTo — touchstart → N touchmove → touchend, interpolating linearly
 * from `start` to `end`. `durationMs` only seeds the step count (>= 1 move).
 */
export function swipeFromTo(
  el: Element,
  start: TouchPoint,
  end: TouchPoint,
  durationMs = 100,
): GestureTrace[] {
  const trace: GestureTrace[] = [];
  const steps = Math.max(1, Math.round(durationMs / 16)); // ~16ms/frame
  const startTouch = makeTouch(el, start, 0);
  const down = dispatchTouchEvent(el, "touchstart", [startTouch], [startTouch]);
  trace.push({ type: down.type, touches: down.touches, changedTouches: down.changedTouches });

  for (let i = 1; i <= steps; i++) {
    const t = i / steps;
    const p: TouchPoint = {
      clientX: start.clientX + (end.clientX - start.clientX) * t,
      clientY: start.clientY + (end.clientY - start.clientY) * t,
    };
    const moveTouch = makeTouch(el, p, 0);
    const move = dispatchTouchEvent(el, "touchmove", [moveTouch], [moveTouch]);
    trace.push({ type: move.type, touches: move.touches, changedTouches: move.changedTouches });
  }

  const endTouch = makeTouch(el, end, 0);
  const up = dispatchTouchEvent(el, "touchend", [], [endTouch]);
  trace.push({ type: up.type, touches: up.touches, changedTouches: up.changedTouches });
  return trace;
}

/**
 * pinchByRatio — two-finger touchstart / touchmove / touchend where the
 * inter-touch distance is scaled by `ratio` (>1 = spread/zoom-in, <1 = pinch).
 * Touches are placed horizontally around the element-relative center (cx, cy).
 */
export function pinchByRatio(
  el: Element,
  ratio: number,
  center: { cx: number; cy: number } = { cx: 100, cy: 100 },
  startHalfGap = 40,
): GestureTrace[] {
  const trace: GestureTrace[] = [];
  const { cx, cy } = center;
  const a0 = makeTouch(el, { clientX: cx - startHalfGap, clientY: cy, identifier: 0 }, 0);
  const b0 = makeTouch(el, { clientX: cx + startHalfGap, clientY: cy, identifier: 1 }, 1);
  const down = dispatchTouchEvent(el, "touchstart", [a0, b0], [a0, b0]);
  trace.push({ type: down.type, touches: down.touches, changedTouches: down.changedTouches });

  const endHalfGap = startHalfGap * ratio;
  const a1 = makeTouch(el, { clientX: cx - endHalfGap, clientY: cy, identifier: 0 }, 0);
  const b1 = makeTouch(el, { clientX: cx + endHalfGap, clientY: cy, identifier: 1 }, 1);
  const move = dispatchTouchEvent(el, "touchmove", [a1, b1], [a1, b1]);
  trace.push({ type: move.type, touches: move.touches, changedTouches: move.changedTouches });

  const up = dispatchTouchEvent(el, "touchend", [], [a1, b1]);
  trace.push({ type: up.type, touches: up.touches, changedTouches: up.changedTouches });
  return trace;
}

/** Euclidean distance between two synthetic touches (helper for pinch asserts). */
export function touchDistance(a: SyntheticTouch, b: SyntheticTouch): number {
  return Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);
}

/**
 * longPressAndDrag — touchstart at (x, y), a `dwellMs` stationary hold advanced
 * via vitest fake timers, then a drag by (dx, dy) and touchend. Requires
 * `vi.useFakeTimers()` to be active in the caller; the dwell is realised by
 * `vi.advanceTimersByTime(dwellMs)` so any long-press timer in production code
 * fires before the drag.
 */
export function longPressAndDrag(
  el: Element,
  x: number,
  y: number,
  dx: number,
  dy: number,
  dwellMs = 500,
): GestureTrace[] {
  const trace: GestureTrace[] = [];
  const start = makeTouch(el, { clientX: x, clientY: y }, 0);
  const down = dispatchTouchEvent(el, "touchstart", [start], [start]);
  trace.push({ type: down.type, touches: down.touches, changedTouches: down.changedTouches });

  // Stationary dwell: advance fake timers so long-press detection fires.
  vi.advanceTimersByTime(dwellMs);

  const end: TouchPoint = { clientX: x + dx, clientY: y + dy };
  const moveTouch = makeTouch(el, end, 0);
  const move = dispatchTouchEvent(el, "touchmove", [moveTouch], [moveTouch]);
  trace.push({ type: move.type, touches: move.touches, changedTouches: move.changedTouches });

  const up = dispatchTouchEvent(el, "touchend", [], [moveTouch]);
  trace.push({ type: up.type, touches: up.touches, changedTouches: up.changedTouches });
  return trace;
}

// ---------------------------------------------------------------------------
// matchMedia shim
// ---------------------------------------------------------------------------

export interface MatchMediaHandle {
  /** Update a query's match value and synchronously fire its change listeners. */
  setMatches(query: string, matches: boolean): void;
  /** Restore the previous window.matchMedia. */
  restore(): void;
}

type MQLListener = (event: MediaQueryListEvent) => void;

/**
 * mockMatchMedia replaces `window.matchMedia` with a map-backed stub. Queries
 * absent from `map` resolve to `false`. The returned handle can flip values and
 * dispatch `change` events, and must be `restore()`d in afterEach.
 */
export function mockMatchMedia(map: Record<string, boolean>): MatchMediaHandle {
  const state = new Map<string, boolean>(Object.entries(map));
  const listeners = new Map<string, Set<MQLListener>>();
  const prev = window.matchMedia;

  function getListeners(query: string): Set<MQLListener> {
    let set = listeners.get(query);
    if (!set) {
      set = new Set();
      listeners.set(query, set);
    }
    return set;
  }

  const impl = (query: string): MediaQueryList => {
    const mql: MediaQueryList = {
      get matches() {
        return state.get(query) ?? false;
      },
      get media() {
        return query;
      },
      onchange: null,
      addEventListener(_type: string, l: EventListenerOrEventListenerObject) {
        if (typeof l === "function") getListeners(query).add(l as MQLListener);
      },
      removeEventListener(_type: string, l: EventListenerOrEventListenerObject) {
        if (typeof l === "function") getListeners(query).delete(l as MQLListener);
      },
      dispatchEvent() {
        return true;
      },
      addListener(l: ((this: MediaQueryList, ev: MediaQueryListEvent) => unknown) | null) {
        if (l) getListeners(query).add(l as unknown as MQLListener);
      },
      removeListener(l: ((this: MediaQueryList, ev: MediaQueryListEvent) => unknown) | null) {
        if (l) getListeners(query).delete(l as unknown as MQLListener);
      },
    };
    return mql;
  };

  window.matchMedia = impl as unknown as typeof window.matchMedia;

  return {
    setMatches(query: string, matches: boolean) {
      state.set(query, matches);
      const event = {
        matches,
        media: query,
        type: "change",
        bubbles: false,
        cancelable: false,
      } as unknown as MediaQueryListEvent;
      for (const l of getListeners(query)) l(event);
    },
    restore() {
      window.matchMedia = prev;
    },
  };
}

// ---------------------------------------------------------------------------
// visualViewport shim
// ---------------------------------------------------------------------------

export interface VisualViewportState {
  height: number;
  width: number;
  offsetTop: number;
  offsetLeft: number;
  pageTop: number;
  pageLeft: number;
  scale: number;
}

export interface VisualViewportHandle {
  /** Mutate the stubbed metrics (does NOT auto-fire events). */
  set(props: Partial<VisualViewportState>): void;
  /** Fire all registered `resize` listeners. */
  fireResize(): void;
  /** Fire all registered `scroll` listeners. */
  fireScroll(): void;
  /** Restore the previous window.visualViewport. */
  restore(): void;
}

/**
 * mockVisualViewport stubs `window.visualViewport` (absent in happy-dom). The
 * returned handle drives `resize` / `scroll` listeners and updates metrics so
 * keyboard-inset code (visualViewport.height shrink) can be exercised.
 */
export function mockVisualViewport(init: {
  height: number;
  offsetTop?: number;
  width?: number;
}): VisualViewportHandle {
  const resizeListeners = new Set<EventListener>();
  const scrollListeners = new Set<EventListener>();
  const state: VisualViewportState = {
    height: init.height,
    width: init.width ?? window.innerWidth,
    offsetTop: init.offsetTop ?? 0,
    offsetLeft: 0,
    pageTop: init.offsetTop ?? 0,
    pageLeft: 0,
    scale: 1,
  };
  const prev = (window as { visualViewport?: VisualViewport }).visualViewport;

  const vv = {
    get height() {
      return state.height;
    },
    get width() {
      return state.width;
    },
    get offsetTop() {
      return state.offsetTop;
    },
    get offsetLeft() {
      return state.offsetLeft;
    },
    get pageTop() {
      return state.pageTop;
    },
    get pageLeft() {
      return state.pageLeft;
    },
    get scale() {
      return state.scale;
    },
    onresize: null,
    onscroll: null,
    addEventListener(type: string, l: EventListenerOrEventListenerObject) {
      if (typeof l !== "function") return;
      if (type === "resize") resizeListeners.add(l as EventListener);
      else if (type === "scroll") scrollListeners.add(l as EventListener);
    },
    removeEventListener(type: string, l: EventListenerOrEventListenerObject) {
      if (typeof l !== "function") return;
      if (type === "resize") resizeListeners.delete(l as EventListener);
      else if (type === "scroll") scrollListeners.delete(l as EventListener);
    },
    dispatchEvent() {
      return true;
    },
  };

  Object.defineProperty(window, "visualViewport", {
    value: vv,
    writable: true,
    configurable: true,
  });

  return {
    set(props: Partial<VisualViewportState>) {
      Object.assign(state, props);
    },
    fireResize() {
      const ev = new Event("resize");
      for (const l of resizeListeners) l(ev);
    },
    fireScroll() {
      const ev = new Event("scroll");
      for (const l of scrollListeners) l(ev);
    },
    restore() {
      Object.defineProperty(window, "visualViewport", {
        value: prev,
        writable: true,
        configurable: true,
      });
    },
  };
}
