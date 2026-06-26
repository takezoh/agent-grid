// useMobileGate.test.ts — FR-MOB-GATE-001/002, ADR 0067.
//
// The gate is an AND contract: max-width:767px AND pointer:coarse. The four
// width×pointer combinations are exhaustively covered so the UAC-022
// counterexample ("width-only gate") is FAILED by a discriminating assertion:
// narrow×fine MUST return false. A width-only implementation queries only
// '(max-width: 767px)' (mapped true below) and would wrongly report true.
//
// happy-dom has no real matchMedia AND-evaluation, so we drive the chunk-01
// mockMatchMedia harness with all three relevant query strings (the width
// fragment, the pointer fragment, and the combined AND string). The combined
// string is what the AND implementation reads; the fragment maps exist solely
// so a width-only / pointer-only mis-implementation resolves true and gets
// caught.

import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type MatchMediaHandle, mockMatchMedia } from "../test/touch-harness";
import { MOBILE_GATE_QUERY, useMobileGate } from "./useMobileGate";

const WIDTH_Q = "(max-width: 767px)";
const POINTER_Q = "(pointer: coarse)";

let mm: MatchMediaHandle | null = null;

/** Build a matchMedia map for a width×pointer combination. The combined AND
 *  query resolves to width && pointer (the real-browser semantics). */
function mountGate(narrow: boolean, coarse: boolean): Record<string, boolean> {
  return {
    [WIDTH_Q]: narrow,
    [POINTER_Q]: coarse,
    [MOBILE_GATE_QUERY]: narrow && coarse,
  };
}

afterEach(() => {
  mm?.restore();
  mm = null;
});

describe("useMobileGate — AND contract (ADR 0067)", () => {
  it("UAC-001: narrow × coarse → gate true on mount", () => {
    mm = mockMatchMedia(mountGate(true, true));
    const { result } = renderHook(() => useMobileGate());
    expect(result.current).toBe(true);
  });

  it("UAC-022 counterexample: narrow × fine → gate FALSE (width-only gate would be true)", () => {
    mm = mockMatchMedia(mountGate(true, false));
    const { result } = renderHook(() => useMobileGate());
    expect(result.current).toBe(false);
  });

  it("wide × coarse → gate false (pointer-only gate would be true)", () => {
    mm = mockMatchMedia(mountGate(false, true));
    const { result } = renderHook(() => useMobileGate());
    expect(result.current).toBe(false);
  });

  it("wide × fine → gate false", () => {
    mm = mockMatchMedia(mountGate(false, false));
    const { result } = renderHook(() => useMobileGate());
    expect(result.current).toBe(false);
  });
});

describe("useMobileGate — change subscription + transition notification", () => {
  it("reacts to a change event flipping the combined query true→false", () => {
    mm = mockMatchMedia(mountGate(true, true));
    const { result } = renderHook(() => useMobileGate());
    expect(result.current).toBe(true);

    act(() => {
      mm!.setMatches(MOBILE_GATE_QUERY, false);
    });
    expect(result.current).toBe(false);
  });

  it("fires onLeaveMobile exactly on the true→false edge", () => {
    mm = mockMatchMedia(mountGate(true, true));
    const onLeaveMobile = vi.fn();
    const { result } = renderHook(() => useMobileGate({ onLeaveMobile }));
    expect(result.current).toBe(true);
    // Not fired on mount.
    expect(onLeaveMobile).not.toHaveBeenCalled();

    act(() => {
      mm!.setMatches(MOBILE_GATE_QUERY, false);
    });
    expect(result.current).toBe(false);
    expect(onLeaveMobile).toHaveBeenCalledTimes(1);
  });

  it("does NOT fire onLeaveMobile on a false→true edge", () => {
    mm = mockMatchMedia(mountGate(false, false));
    const onLeaveMobile = vi.fn();
    const { result } = renderHook(() => useMobileGate({ onLeaveMobile }));
    expect(result.current).toBe(false);

    act(() => {
      mm!.setMatches(MOBILE_GATE_QUERY, true);
    });
    expect(result.current).toBe(true);
    expect(onLeaveMobile).not.toHaveBeenCalled();
  });

  it("unsubscribes the change listener on unmount", () => {
    mm = mockMatchMedia(mountGate(true, true));
    const onLeaveMobile = vi.fn();
    const { result, unmount } = renderHook(() => useMobileGate({ onLeaveMobile }));
    expect(result.current).toBe(true);

    unmount();
    // After unmount the listener is gone: a transition must not call back.
    act(() => {
      mm!.setMatches(MOBILE_GATE_QUERY, false);
    });
    expect(onLeaveMobile).not.toHaveBeenCalled();
  });
});

describe("useMobileGate — SSR / matchMedia absence (FR-PC-PRESERVE)", () => {
  it("returns false when window.matchMedia is unavailable", () => {
    const original = window.matchMedia;
    // Simulate a legacy / SSR environment with no matchMedia.
    (window as { matchMedia?: typeof window.matchMedia }).matchMedia =
      undefined as unknown as typeof window.matchMedia;
    try {
      const { result } = renderHook(() => useMobileGate());
      expect(result.current).toBe(false);
    } finally {
      window.matchMedia = original;
    }
  });

  it("UAC-021: gate is the boolean truth source for conditional render (no DOM/CSS hiding)", () => {
    // The hook itself emits ONLY a boolean — it renders nothing and touches no
    // DOM. gate=false therefore means "do not render overlays" (conditional
    // render), never "render then display:none" (the rejected counterexample).
    mm = mockMatchMedia(mountGate(false, true)); // wide × coarse → false
    const { result } = renderHook(() => useMobileGate());
    expect(result.current).toBe(false);
    expect(typeof result.current).toBe("boolean");
  });
});
