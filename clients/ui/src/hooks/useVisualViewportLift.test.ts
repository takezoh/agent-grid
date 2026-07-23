// useVisualViewportLift.test.ts — FR-MOB-VVP-001/002/003 (ADR 0069).
//
// Proves the hook (1) mirrors the keyboard inset into the `.terminal-fab-layer`
// inline custom property via getComputedStyle, (2) is a strict no-op when
// `window.visualViewport` is absent, and (3) subscribes on input-mode enter and
// unsubscribes (resize + scroll) on exit / rotation, in the order required to
// avoid a listener leak.

import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockVisualViewport } from "../test/touch-harness";
import {
  FAB_OFFSET_BASE_PX,
  FAB_OFFSET_PROP,
  useVisualViewportLift,
} from "./useVisualViewportLift";

function makeLayer(): { ref: { current: HTMLElement | null }; el: HTMLElement } {
  const el = document.createElement("div");
  el.className = "terminal-fab-layer";
  document.body.appendChild(el);
  return { ref: { current: el }, el };
}

afterEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
});

describe("useVisualViewportLift — offset update (FR-MOB-VVP-001)", () => {
  it("stamps --terminal-fab-offset from innerHeight - vv.height - vv.offsetTop + 16", () => {
    Object.defineProperty(window, "innerHeight", { value: 800, configurable: true });
    // keyboard open: visualViewport shrinks to 500, offsetTop 0 → 800-500-0+16 = 316
    const vv = mockVisualViewport({ height: 500, offsetTop: 0 });
    const { ref, el } = makeLayer();

    renderHook(() => useVisualViewportLift({ layerRef: ref, active: true }));

    expect(getComputedStyle(el).getPropertyValue(FAB_OFFSET_PROP).trim()).toBe("316px");
    vv.restore();
  });

  it("re-stamps on a visualViewport resize event (keyboard height change)", () => {
    Object.defineProperty(window, "innerHeight", { value: 800, configurable: true });
    const vv = mockVisualViewport({ height: 800, offsetTop: 0 });
    const { ref, el } = makeLayer();

    renderHook(() => useVisualViewportLift({ layerRef: ref, active: true }));
    // keyboard closed → exactly the 16px base.
    expect(getComputedStyle(el).getPropertyValue(FAB_OFFSET_PROP).trim()).toBe(
      `${FAB_OFFSET_BASE_PX}px`,
    );

    act(() => {
      vv.set({ height: 360 }); // keyboard opened → 800-360+16 = 456
      vv.fireResize();
    });
    expect(getComputedStyle(el).getPropertyValue(FAB_OFFSET_PROP).trim()).toBe("456px");
    vv.restore();
  });
});

describe("useVisualViewportLift — absent visualViewport (FR-MOB-VVP-002)", () => {
  it("writes nothing when window.visualViewport is undefined (CSS default fallback)", () => {
    const original = (window as { visualViewport?: unknown }).visualViewport;
    Object.defineProperty(window, "visualViewport", {
      value: undefined,
      configurable: true,
      writable: true,
    });
    const { ref, el } = makeLayer();

    renderHook(() => useVisualViewportLift({ layerRef: ref, active: true }));

    // No inline write at all → the property is empty (CSS default 16px applies).
    expect(el.style.getPropertyValue(FAB_OFFSET_PROP)).toBe("");
    Object.defineProperty(window, "visualViewport", {
      value: original,
      configurable: true,
      writable: true,
    });
  });
});

describe("useVisualViewportLift — subscribe/unsubscribe order (FR-MOB-VVP-003)", () => {
  it("subscribes resize+scroll on enter and removes both on exit, sub before unsub", () => {
    Object.defineProperty(window, "innerHeight", { value: 800, configurable: true });
    const vv = mockVisualViewport({ height: 800, offsetTop: 0 });
    const target = window.visualViewport as VisualViewport;
    const addSpy = vi.spyOn(target, "addEventListener");
    const removeSpy = vi.spyOn(target, "removeEventListener");
    const { ref } = makeLayer();

    const { rerender, unmount } = renderHook(
      ({ active }) => useVisualViewportLift({ layerRef: ref, active }),
      { initialProps: { active: true } },
    );

    const addedTypes = addSpy.mock.calls.map((c) => c[0]);
    expect(addedTypes).toContain("resize");
    expect(addedTypes).toContain("scroll");
    expect(removeSpy).not.toHaveBeenCalled();

    // Exit input mode (active false): teardown must remove both listeners.
    rerender({ active: false });
    const removedTypes = removeSpy.mock.calls.map((c) => c[0]);
    expect(removedTypes).toContain("resize");
    expect(removedTypes).toContain("scroll");

    // Ordering: every subscribe happened before any unsubscribe.
    expect(addSpy.mock.invocationCallOrder[0]).toBeLessThan(removeSpy.mock.invocationCallOrder[0]);

    unmount();
    vv.restore();
  });

  it("does not subscribe while inactive (view mode never touches listeners)", () => {
    const vv = mockVisualViewport({ height: 800 });
    const addSpy = vi.spyOn(window.visualViewport as VisualViewport, "addEventListener");
    const { ref } = makeLayer();

    renderHook(() => useVisualViewportLift({ layerRef: ref, active: false }));

    expect(addSpy).not.toHaveBeenCalled();
    vv.restore();
  });
});
