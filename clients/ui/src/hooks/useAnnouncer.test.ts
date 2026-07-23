// useAnnouncer.test.ts — identical-text 1.5s debounce (FR-MOB-MODE-006, ADR 0073).
// Discriminating against the 'no debounce' counterexample: an identical string
// inside the window must NOT re-emit (seq frozen), a different string emits
// immediately, and the same string after the window emits again. Verified with
// vitest fake timers so the 1.5s clock is deterministic.

import { act, renderHook } from "@testing-library/react";
import { type ReactNode, createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ANNOUNCER_DEBOUNCE_MS, AnnouncerProvider, useAnnouncer } from "./useAnnouncer";

const MSG = "Returned to view mode";

function wrapper({ children }: { children: ReactNode }) {
  return createElement(AnnouncerProvider, null, children);
}

describe("useAnnouncer — identical-text 1.5s debounce", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("emits the first announcement and exposes its text", () => {
    const { result } = renderHook(() => useAnnouncer(), { wrapper });
    expect(result.current.text).toBe("");
    act(() => result.current.announce(MSG));
    expect(result.current.text).toBe(MSG);
    expect(result.current.seq).toBe(1);
  });

  it("suppresses an identical announcement within the 1.5s window", () => {
    const { result } = renderHook(() => useAnnouncer(), { wrapper });
    act(() => result.current.announce(MSG));
    const seqAfterFirst = result.current.seq;

    act(() => vi.advanceTimersByTime(ANNOUNCER_DEBOUNCE_MS - 1));
    act(() => result.current.announce(MSG));
    // Still within the window → no re-emit.
    expect(result.current.seq).toBe(seqAfterFirst);
  });

  it("re-emits the same text once the 1.5s window elapses", () => {
    const { result } = renderHook(() => useAnnouncer(), { wrapper });
    act(() => result.current.announce(MSG));
    const seqAfterFirst = result.current.seq;

    act(() => vi.advanceTimersByTime(ANNOUNCER_DEBOUNCE_MS));
    act(() => result.current.announce(MSG));
    expect(result.current.seq).toBe(seqAfterFirst + 1);
  });

  it("emits a different text immediately, even inside the window", () => {
    const { result } = renderHook(() => useAnnouncer(), { wrapper });
    act(() => result.current.announce("A"));
    const seqA = result.current.seq;
    act(() => result.current.announce("B"));
    expect(result.current.seq).toBe(seqA + 1);
    expect(result.current.text).toBe("B");
  });

  it("throws when used outside an AnnouncerProvider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useAnnouncer())).toThrow(/AnnouncerProvider/);
    spy.mockRestore();
  });
});
