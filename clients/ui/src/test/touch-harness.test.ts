// touch-harness.test.ts — self-test for the synthetic touch / matchMedia /
// visualViewport shims (chunk-01). Verifies each helper produces the expected
// event sequence, touch-list cardinality, and coordinates, and documents the
// happy-dom limits that the real-device checklist must cover instead.

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type GestureTrace,
  type SyntheticTouchEvent,
  longPressAndDrag,
  mockMatchMedia,
  mockVisualViewport,
  pinchByRatio,
  swipeFromTo,
  tapAt,
  touchDistance,
} from "./touch-harness";

/** Attach typed touch listeners to `el` and collect the events fired. */
function recordTouches(el: Element): SyntheticTouchEvent[] {
  const seen: SyntheticTouchEvent[] = [];
  for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"]) {
    el.addEventListener(type, (e) => seen.push(e as SyntheticTouchEvent));
  }
  return seen;
}

describe("touch-harness: tapAt", () => {
  it("emits touchstart then touchend at (x, y) with empty touches on end", () => {
    const el = document.createElement("div");
    const seen = recordTouches(el);
    const trace = tapAt(el, 12, 34);

    expect(seen.map((e) => e.type)).toEqual(["touchstart", "touchend"]);
    // touchstart carries one active touch at the tap coordinates.
    expect(seen[0].touches).toHaveLength(1);
    expect(seen[0].changedTouches[0].clientX).toBe(12);
    expect(seen[0].changedTouches[0].clientY).toBe(34);
    // touchend has no remaining active touches (finger lifted).
    expect(seen[1].touches).toHaveLength(0);
    expect(seen[1].changedTouches[0].clientX).toBe(12);

    expect(trace.map((t: GestureTrace) => t.type)).toEqual(["touchstart", "touchend"]);
  });
});

describe("touch-harness: swipeFromTo", () => {
  it("emits touchstart, >=1 touchmove, touchend with interpolated coordinates", () => {
    const el = document.createElement("div");
    const seen = recordTouches(el);
    swipeFromTo(el, { clientX: 0, clientY: 200 }, { clientX: 0, clientY: 50 }, 64);

    const types = seen.map((e) => e.type);
    expect(types[0]).toBe("touchstart");
    expect(types[types.length - 1]).toBe("touchend");
    const moves = seen.filter((e) => e.type === "touchmove");
    expect(moves.length).toBeGreaterThanOrEqual(1);

    // Every move stays within the start->end Y span (monotonic upward swipe).
    for (const m of moves) {
      const y = m.changedTouches[0].clientY;
      expect(y).toBeLessThanOrEqual(200);
      expect(y).toBeGreaterThanOrEqual(50);
    }
    // Final move lands on the end point.
    const lastMove = moves[moves.length - 1];
    expect(lastMove.changedTouches[0].clientY).toBeCloseTo(50, 5);
  });
});

describe("touch-harness: pinchByRatio", () => {
  it("synthesises a 2-finger gesture (touches.length === 2) scaled by ratio", () => {
    const el = document.createElement("div");
    const seen = recordTouches(el);
    // ratio 2 -> fingers spread apart (zoom-in); start half-gap 40 -> distances
    // 80 (start) -> 160 (move).
    pinchByRatio(el, 2, { cx: 100, cy: 100 }, 40);

    const start = seen.find((e) => e.type === "touchstart");
    const move = seen.find((e) => e.type === "touchmove");
    if (!start || !move) throw new Error("expected touchstart + touchmove");
    // Open Question 2: TouchEvent touches.length=2 synthesis is reproducible
    // in happy-dom via the plain-object shim.
    expect(start.touches).toHaveLength(2);
    expect(move.touches).toHaveLength(2);

    const startDist = touchDistance(start.touches[0], start.touches[1]);
    const moveDist = touchDistance(move.touches[0], move.touches[1]);
    expect(startDist).toBeCloseTo(80, 5);
    expect(moveDist).toBeCloseTo(160, 5);
    expect(moveDist / startDist).toBeCloseTo(2, 5);
  });

  it("ratio < 1 contracts the inter-touch distance (pinch-in)", () => {
    const el = document.createElement("div");
    const seen = recordTouches(el);
    pinchByRatio(el, 0.5, { cx: 100, cy: 100 }, 40);
    const start = seen.find((e) => e.type === "touchstart");
    const move = seen.find((e) => e.type === "touchmove");
    if (!start || !move) throw new Error("expected touchstart + touchmove");
    const startDist = touchDistance(start.touches[0], start.touches[1]);
    const moveDist = touchDistance(move.touches[0], move.touches[1]);
    expect(moveDist).toBeLessThan(startDist);
    expect(moveDist / startDist).toBeCloseTo(0.5, 5);
  });
});

describe("touch-harness: longPressAndDrag", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("advances fake timers through the dwell before dragging", () => {
    vi.useFakeTimers();
    const el = document.createElement("div");
    const seen = recordTouches(el);

    // A production-style long-press timer: must fire during the dwell.
    let longPressFired = false;
    setTimeout(() => {
      longPressFired = true;
    }, 500);

    longPressAndDrag(el, 10, 10, 30, 0, 500);

    // The dwell advanced fake timers -> the 500ms long-press timer fired.
    expect(longPressFired).toBe(true);

    const types = seen.map((e) => e.type);
    expect(types[0]).toBe("touchstart");
    expect(types).toContain("touchmove");
    expect(types[types.length - 1]).toBe("touchend");
    // Drag delta applied to the move.
    const move = seen.find((e) => e.type === "touchmove");
    expect(move?.changedTouches[0].clientX).toBe(40); // 10 + 30
  });
});

describe("touch-harness: mockMatchMedia (Open Question 2 — pointer media eval)", () => {
  let handle: ReturnType<typeof mockMatchMedia> | null = null;

  afterEach(() => {
    handle?.restore();
    handle = null;
  });

  it("evaluates (pointer: coarse) / (pointer: fine) from the supplied map", () => {
    handle = mockMatchMedia({
      "(pointer: coarse)": true,
      "(pointer: fine)": false,
    });
    expect(window.matchMedia("(pointer: coarse)").matches).toBe(true);
    expect(window.matchMedia("(pointer: fine)").matches).toBe(false);
    // Unmapped query defaults to false.
    expect(window.matchMedia("(max-width: 767px)").matches).toBe(false);
  });

  it("setMatches flips a query and fires its change listeners", () => {
    handle = mockMatchMedia({ "(pointer: coarse)": false });
    const mql = window.matchMedia("(pointer: coarse)");
    const events: boolean[] = [];
    mql.addEventListener("change", (e) => events.push(e.matches));

    handle.setMatches("(pointer: coarse)", true);
    expect(mql.matches).toBe(true);
    expect(events).toEqual([true]);
  });
});

describe("touch-harness: mockVisualViewport", () => {
  let handle: ReturnType<typeof mockVisualViewport> | null = null;

  afterEach(() => {
    handle?.restore();
    handle = null;
  });

  it("stubs window.visualViewport metrics and fires resize/scroll", () => {
    handle = mockVisualViewport({ height: 800, offsetTop: 0 });
    expect(window.visualViewport?.height).toBe(800);

    let resizeCount = 0;
    let scrollCount = 0;
    window.visualViewport?.addEventListener("resize", () => resizeCount++);
    window.visualViewport?.addEventListener("scroll", () => scrollCount++);

    // Simulate a soft-keyboard opening: viewport shrinks + offsets down.
    handle.set({ height: 520, offsetTop: 120 });
    handle.fireResize();
    handle.fireScroll();

    expect(window.visualViewport?.height).toBe(520);
    expect(window.visualViewport?.offsetTop).toBe(120);
    expect(resizeCount).toBe(1);
    expect(scrollCount).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// Open Question 2 — happy-dom limits, explicitly demonstrated.
// Items happy-dom CAN reproduce are asserted above (matchMedia pointer eval,
// 2-finger touches.length, scrollTop manual read/write below). Items it CANNOT
// reproduce are documented here and moved to the real-device checklist.
// ---------------------------------------------------------------------------

describe("touch-harness: happy-dom layout limits (move to real-device checklist)", () => {
  it("scrollTop / scrollHeight / clientHeight are expressible via manual assignment", () => {
    // happy-dom has no layout engine, so scroll metrics never populate from
    // content. They are plain assignable properties — gesture/scroll tests must
    // seed them manually (legacy scroll behaviour is then emulated by the test).
    const vp = document.createElement("div");
    vp.className = "xterm-viewport";
    vp.scrollTop = 1000;
    expect(vp.scrollTop).toBe(1000);
    vp.scrollTop = 880;
    expect(vp.scrollTop).toBe(880);
    // scrollHeight / clientHeight default to 0 (no layout) — assignment is the
    // only way to give them meaning under happy-dom.
    Object.defineProperty(vp, "scrollHeight", { value: 5000, configurable: true });
    Object.defineProperty(vp, "clientHeight", { value: 400, configurable: true });
    expect(vp.scrollHeight).toBe(5000);
    expect(vp.clientHeight).toBe(400);
  });

  it("getBoundingClientRect returns 0x0 (no layout): 44x44 check needs a rect stub / device test", () => {
    // CONSEQUENCE: the 44x44px hit area (FR-A11Y-001) behind UAC-020/024 cannot
    // be auto-verified in happy-dom. Chunks that write size assertions must stub
    // the rect (Object.defineProperty(el, 'getBoundingClientRect', ...)) or move
    // the check to the real-device checklist (iOS Safari / Android Chrome).
    const btn = document.createElement("button");
    document.body.appendChild(btn);
    const rect = btn.getBoundingClientRect();
    expect(rect.width).toBe(0);
    expect(rect.height).toBe(0);
    btn.remove();

    // Demonstrate the rect-stub workaround the size-assertion chunks must use.
    const stubbed = document.createElement("button");
    Object.defineProperty(stubbed, "getBoundingClientRect", {
      value: () => ({ width: 44, height: 44, top: 0, left: 0, right: 44, bottom: 44 }),
      configurable: true,
    });
    expect(stubbed.getBoundingClientRect().width).toBe(44);
  });
});
