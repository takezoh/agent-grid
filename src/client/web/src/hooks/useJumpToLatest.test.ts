// useJumpToLatest.test.ts — viewport-scroll → FAB visibility + seed gate +
// edge-triggered polite announcement (FR-MOB-JUMP-001/004/005/006, ADR 0073).
//
// Discriminating against:
//   - UAC-012/013: the ±2px tail boundary toggles shouldShowFab (a strict ==
//     tail check would chatter; a wide margin would never show in scrollback).
//   - FR-MOB-JUMP-005: seedReady=false forces shouldShowFab=false, and even after
//     the seed completes the FAB stays absent until the first scroll lands.
//   - FR-MOB-JUMP-004 + ADR 0073: announce fires on the false→true edge only, and
//     kinetic-scroll oscillation collapses to a single emit through the
//     useAnnouncer 1.5s identical-text debounce.
//   - FR-MOB-JUMP-006: jumpToBottom is instant under reduced-motion, smooth else.

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mockMatchMedia } from "../test/touch-harness";
import { AnnouncerProvider, useAnnouncer } from "./useAnnouncer";
import {
  JUMP_FAB_ANNOUNCEMENT,
  type JumpBehavior,
  isAtTail,
  useJumpToLatest,
} from "./useJumpToLatest";

// ---------------------------------------------------------------------------
// Viewport stub: happy-dom has no layout, so scrollTop / scrollHeight /
// clientHeight are backed by manual properties (chunk-01 harness note).
// ---------------------------------------------------------------------------

function makeViewport(scrollHeight: number, clientHeight: number): HTMLDivElement {
  const el = document.createElement("div");
  el.className = "xterm-viewport";
  let top = scrollHeight - clientHeight; // start exactly at tail
  Object.defineProperty(el, "scrollHeight", { value: scrollHeight, configurable: true });
  Object.defineProperty(el, "clientHeight", { value: clientHeight, configurable: true });
  Object.defineProperty(el, "scrollTop", {
    configurable: true,
    get: () => top,
    set: (v: number) => {
      top = v;
    },
  });
  document.body.appendChild(el);
  return el;
}

/** Set scrollTop and dispatch a scroll event (what the real viewport emits). */
function scrollTo(el: HTMLElement, value: number): void {
  el.scrollTop = value;
  el.dispatchEvent(new Event("scroll"));
}

describe("isAtTail — ±2px boundary (pure)", () => {
  // tail = scrollHeight - clientHeight = 1000 - 200 = 800.
  it("is at tail within ±2px and not at tail at 3px", () => {
    expect(isAtTail(800, 1000, 200)).toBe(true); // diff 0
    expect(isAtTail(798, 1000, 200)).toBe(true); // diff 2 (boundary, inclusive)
    expect(isAtTail(802, 1000, 200)).toBe(true); // diff 2 the other way
    expect(isAtTail(797, 1000, 200)).toBe(false); // diff 3 → scrollback
  });
});

describe("useJumpToLatest — tail detection toggles shouldShowFab", () => {
  let el: HTMLDivElement;

  beforeEach(() => {
    el = makeViewport(1000, 200); // tail offset = 800
  });
  afterEach(() => {
    el.remove();
  });

  function setup(seedReady = true, announce?: (t: string) => void) {
    const viewportRef = { current: el };
    const scrollToBottom = vi.fn();
    const view = renderHook(() =>
      useJumpToLatest({ viewportRef, scrollToBottom, seedReady, announce }),
    );
    return { ...view, scrollToBottom };
  }

  it("UAC-012: starts absent (false) and stays false while at tail", () => {
    const { result } = setup();
    expect(result.current.shouldShowFab).toBe(false);
    act(() => scrollTo(el, 798)); // diff 2 — still tail
    expect(result.current.shouldShowFab).toBe(false);
  });

  it("UAC-013: crossing the ±2px boundary flips false→true and back", () => {
    const { result } = setup();
    act(() => scrollTo(el, 797)); // diff 3 — scrollback
    expect(result.current.shouldShowFab).toBe(true);
    act(() => scrollTo(el, 500)); // far from tail
    expect(result.current.shouldShowFab).toBe(true);
    act(() => scrollTo(el, 799)); // diff 1 — back at tail
    expect(result.current.shouldShowFab).toBe(false);
  });
});

describe("useJumpToLatest — seed gating (FR-MOB-JUMP-005)", () => {
  let el: HTMLDivElement;
  beforeEach(() => {
    el = makeViewport(1000, 200);
  });
  afterEach(() => {
    el.remove();
  });

  it("forces shouldShowFab=false while seedReady=false even when scrolled away", () => {
    const viewportRef = { current: el };
    const scrollToBottom = vi.fn();
    const { result, rerender } = renderHook(
      (props: { seedReady: boolean }) =>
        useJumpToLatest({ viewportRef, scrollToBottom, seedReady: props.seedReady }),
      { initialProps: { seedReady: false } },
    );

    act(() => scrollTo(el, 100)); // deep scrollback, but seed not ready
    expect(result.current.shouldShowFab).toBe(false);

    // Seed completes: still false until the FIRST post-seed scroll arrives.
    act(() => rerender({ seedReady: true }));
    expect(result.current.shouldShowFab).toBe(false);

    act(() => scrollTo(el, 100)); // first real scroll after seed → now visible
    expect(result.current.shouldShowFab).toBe(true);
  });
});

describe("useJumpToLatest — polite announcement (FR-MOB-JUMP-004)", () => {
  let el: HTMLDivElement;
  beforeEach(() => {
    el = makeViewport(1000, 200);
  });
  afterEach(() => {
    el.remove();
  });

  it("announces once per false→true edge (not on every away-scroll)", () => {
    const announce = vi.fn();
    const viewportRef = { current: el };
    renderHook(() =>
      useJumpToLatest({ viewportRef, scrollToBottom: vi.fn(), seedReady: true, announce }),
    );

    act(() => scrollTo(el, 500)); // tail → away (edge): announce #1
    expect(announce).toHaveBeenCalledTimes(1);
    expect(announce).toHaveBeenCalledWith(JUMP_FAB_ANNOUNCEMENT);

    act(() => scrollTo(el, 300)); // stays away: NO new announce
    expect(announce).toHaveBeenCalledTimes(1);

    act(() => scrollTo(el, 800)); // back to tail (no announce)
    act(() => scrollTo(el, 300)); // away again (new edge): announce #2
    expect(announce).toHaveBeenCalledTimes(2);
  });

  it("ADR 0073: kinetic mount/unmount oscillation collapses to ONE emit via 1.5s debounce", () => {
    vi.useFakeTimers();
    try {
      const viewportRef = { current: el };
      // Wire the REAL useAnnouncer so its identical-text debounce is exercised.
      const { result } = renderHook(
        () => {
          const ann = useAnnouncer();
          const jump = useJumpToLatest({
            viewportRef,
            scrollToBottom: vi.fn(),
            seedReady: true,
            announce: ann.announce,
          });
          return { ann, jump };
        },
        { wrapper: AnnouncerProvider },
      );

      // Kinetic scroll oscillates across the tail boundary repeatedly within the
      // 1.5s window: each away-edge attempts an announce, all identical.
      for (let i = 0; i < 3; i++) {
        act(() => scrollTo(el, 300)); // away (announce attempt)
        act(() => scrollTo(el, 800)); // tail
      }

      // Three away-edges, but the announcer accepted exactly one (seq bumped once).
      expect(result.current.ann.seq).toBe(1);
      expect(result.current.ann.text).toBe(JUMP_FAB_ANNOUNCEMENT);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("useJumpToLatest — jumpToBottom reduced-motion (FR-MOB-JUMP-006)", () => {
  let el: HTMLDivElement;
  let media: ReturnType<typeof mockMatchMedia>;

  beforeEach(() => {
    el = makeViewport(1000, 200);
  });
  afterEach(() => {
    media?.restore();
    el.remove();
  });

  function behaviorFor(reduce: boolean): JumpBehavior {
    media = mockMatchMedia({ "(prefers-reduced-motion: reduce)": reduce });
    const viewportRef = { current: el };
    const scrollToBottom = vi.fn<(b: JumpBehavior) => void>();
    const { result } = renderHook(() =>
      useJumpToLatest({ viewportRef, scrollToBottom, seedReady: true }),
    );
    act(() => result.current.jumpToBottom());
    expect(scrollToBottom).toHaveBeenCalledTimes(1);
    return scrollToBottom.mock.calls[0][0];
  }

  it("jumps instantly when prefers-reduced-motion:reduce", () => {
    expect(behaviorFor(true)).toBe("instant");
  });

  it("jumps smoothly otherwise", () => {
    expect(behaviorFor(false)).toBe("smooth");
  });
});
