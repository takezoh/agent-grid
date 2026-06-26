// useCoachmarkOnce.test.ts — FR-MOB-COACH-001/002 (ADR 0072).
//
// Discriminators:
//   - FR-MOB-COACH-001 counterexample ("write hintSeen on dismiss, not on
//     render"): asserted dead by checking the Map adapter holds '1' immediately
//     after the first render, BEFORE any tap or timer — a dismiss-time write
//     would leave it absent here.
//   - FR-MOB-COACH-002: tap and the 5s fake timer each unmount the coachmark,
//     whichever is first; a second mount (seen='1') never re-shows it.

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { COACHMARK_AUTO_DISMISS_MS, HINT_SEEN_KEY, useCoachmarkOnce } from "./useCoachmarkOnce";
import type { StorageLike } from "./usePersistedValue";

function mapStorage(initial?: Record<string, string>): StorageLike & { map: Map<string, string> } {
  const map = new Map<string, string>(initial ? Object.entries(initial) : []);
  return {
    map,
    getItem: (k) => (map.has(k) ? (map.get(k) as string) : null),
    setItem: (k, v) => {
      map.set(k, v);
    },
  };
}

describe("useCoachmarkOnce — idempotent first-render write (FR-MOB-COACH-001)", () => {
  it("shows the coachmark and writes hintSeen='1' on the first active render", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useCoachmarkOnce({ active: true, storage }));

    expect(result.current.showCoachmark).toBe(true);
    // Written up-front, before any tap / timer (kills the dismiss-time-write bug).
    expect(storage.map.get(HINT_SEEN_KEY)).toBe("1");
  });

  it("does not show or write again when hintSeen is already '1' (2nd+ session)", () => {
    const storage = mapStorage({ [HINT_SEEN_KEY]: "1" });
    const { result } = renderHook(() => useCoachmarkOnce({ active: true, storage }));

    expect(result.current.showCoachmark).toBe(false);
    expect(storage.map.get(HINT_SEEN_KEY)).toBe("1");
  });

  it("stays hidden while inactive and shows only once active flips true", () => {
    const storage = mapStorage();
    const { result, rerender } = renderHook(({ active }) => useCoachmarkOnce({ active, storage }), {
      initialProps: { active: false },
    });

    expect(result.current.showCoachmark).toBe(false);
    expect(storage.map.get(HINT_SEEN_KEY)).toBeUndefined();

    rerender({ active: true });
    expect(result.current.showCoachmark).toBe(true);
    expect(storage.map.get(HINT_SEEN_KEY)).toBe("1");
  });
});

describe("useCoachmarkOnce — dismiss is tap or 5s (FR-MOB-COACH-002)", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("auto-dismisses after exactly 5s", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useCoachmarkOnce({ active: true, storage }));
    expect(result.current.showCoachmark).toBe(true);

    act(() => {
      vi.advanceTimersByTime(COACHMARK_AUTO_DISMISS_MS - 1);
    });
    expect(result.current.showCoachmark).toBe(true); // not yet

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current.showCoachmark).toBe(false); // 5s elapsed → gone
  });

  it("tap dismisses earlier than 5s and the timer does not resurrect it", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useCoachmarkOnce({ active: true, storage }));

    act(() => {
      result.current.dismiss();
    });
    expect(result.current.showCoachmark).toBe(false);

    // Advancing past 5s must not bring it back.
    act(() => {
      vi.advanceTimersByTime(COACHMARK_AUTO_DISMISS_MS * 2);
    });
    expect(result.current.showCoachmark).toBe(false);
  });
});
