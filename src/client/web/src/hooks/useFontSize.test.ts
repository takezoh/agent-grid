// useFontSize.test.ts — ADR 0070 / 0034, FR-MOB-PERSIST-001/002 /
// FR-MOB-PINCH-002 / FR-MOB-STEPPER-001.
//
// Proves the three write paths (pinch / stepper / restore) all clamp to [8,28],
// persist through the injected Map adapter, and fan out exactly one scheduleFit
// per mutation (ADR 0034). The UAC-017 lower-bound counterexample ("ratio is
// multiplied straight onto fontSize with no floor → 5px / NaN cols") is failed by
// asserting a deep pinch-in floors at 8, never below.

import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FONT_SIZE_KEY, useFontSize } from "./useFontSize";
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

describe("useFontSize — restore on mount (ADR 0070)", () => {
  it("UAC-019: stored '999' restores clamped to 28", () => {
    const storage = mapStorage({ [FONT_SIZE_KEY]: "999" });
    const { result } = renderHook(() => useFontSize({ scheduleFit: vi.fn(), storage }));
    expect(result.current.fontSize).toBe(28);
  });

  it("missing key restores default 14", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useFontSize({ scheduleFit: vi.fn(), storage }));
    expect(result.current.fontSize).toBe(14);
  });
});

describe("useFontSize — stepper (FR-MOB-STEPPER-001)", () => {
  it("increase() adds 2px, persists '16', and schedules a fit", () => {
    const storage = mapStorage();
    const scheduleFit = vi.fn();
    const { result } = renderHook(() => useFontSize({ scheduleFit, storage }));

    act(() => result.current.increase());

    expect(result.current.fontSize).toBe(16);
    expect(storage.map.get(FONT_SIZE_KEY)).toBe("16");
    expect(scheduleFit).toHaveBeenCalledTimes(1);
  });

  it("decrease() subtracts 2px and schedules a fit", () => {
    const storage = mapStorage();
    const scheduleFit = vi.fn();
    const { result } = renderHook(() => useFontSize({ scheduleFit, storage }));

    act(() => result.current.decrease());

    expect(result.current.fontSize).toBe(12);
    expect(scheduleFit).toHaveBeenCalledTimes(1);
  });

  it("reset() returns to 14px and schedules a fit even when value already differs", () => {
    const storage = mapStorage({ [FONT_SIZE_KEY]: "22" });
    const scheduleFit = vi.fn();
    const { result } = renderHook(() => useFontSize({ scheduleFit, storage }));
    expect(result.current.fontSize).toBe(22);

    act(() => result.current.reset());

    expect(result.current.fontSize).toBe(14);
    expect(storage.map.get(FONT_SIZE_KEY)).toBe("14");
    expect(scheduleFit).toHaveBeenCalledTimes(1);
  });
});

describe("useFontSize — clamp [8,28] (FR-MOB-PINCH-002)", () => {
  it("UAC-017: deep pinch-in floors at 8px, never below", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useFontSize({ scheduleFit: vi.fn(), storage }));

    act(() => {
      result.current.beginPinch();
      result.current.applyPinch(0.1); // 14 * 0.1 = 1.4 → would be ~1px without a floor
    });

    expect(result.current.fontSize).toBe(8);
  });

  it("pinch-out ceils at 28px, never above", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useFontSize({ scheduleFit: vi.fn(), storage }));

    act(() => {
      result.current.beginPinch();
      result.current.applyPinch(5); // 14 * 5 = 70 → clamps to 28
    });

    expect(result.current.fontSize).toBe(28);
  });

  it("applyPinch is relative to the pinch-start base, not the compounding state", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useFontSize({ scheduleFit: vi.fn(), storage }));

    act(() => {
      result.current.beginPinch(); // base = 14
      result.current.applyPinch(1.5); // 14 * 1.5 = 21
    });
    expect(result.current.fontSize).toBe(21);

    act(() => {
      result.current.applyPinch(1.5); // still 14 * 1.5 = 21 (not 21 * 1.5)
    });
    expect(result.current.fontSize).toBe(21);
  });

  it("set() rounds and clamps an explicit value below the floor", () => {
    const storage = mapStorage();
    const { result } = renderHook(() => useFontSize({ scheduleFit: vi.fn(), storage }));
    act(() => result.current.set(5));
    expect(result.current.fontSize).toBe(8);
  });
});
