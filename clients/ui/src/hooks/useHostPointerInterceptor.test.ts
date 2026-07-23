// useHostPointerInterceptor.test.ts — the single capture-phase pointerdown
// listener (FR-MOB-MODE-002 / 005, ADR 0068). Discriminating against:
//   - UAC-002 / UAC-009 counterexample ('touchend → term.focus() チラ見せ'):
//     in 閲覧 mode a pointerdown is preventDefault()'d and dispatches focus 0
//     times / leaves activeElement unchanged.
//   - the '1 系統だけ attach' contract: addEventListener('pointerdown', …,
//     {capture:true}) is called exactly once.

import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useHostPointerInterceptor } from "./useHostPointerInterceptor";

describe("useHostPointerInterceptor", () => {
  let host: HTMLElement;
  let content: HTMLElement;
  let overlay: HTMLElement;
  let ta: HTMLTextAreaElement;

  beforeEach(() => {
    host = document.createElement("div");
    host.className = "terminal-host";
    content = document.createElement("div"); // a plain xterm content cell
    overlay = document.createElement("div");
    overlay.setAttribute("data-overlay", ""); // a FAB / popover root
    const overlayChild = document.createElement("button");
    overlay.appendChild(overlayChild);
    ta = document.createElement("textarea");
    ta.className = "xterm-helper-textarea";
    host.append(content, overlay, ta);
    document.body.appendChild(host);
  });

  afterEach(() => {
    host.remove();
  });

  function pointerdownOn(el: Element): Event {
    const ev = new Event("pointerdown", { bubbles: true, cancelable: true });
    el.dispatchEvent(ev);
    return ev;
  }

  it("attaches exactly one capture-phase pointerdown listener to the host", () => {
    const addSpy = vi.spyOn(host, "addEventListener");
    const { rerender } = renderHook(() =>
      useHostPointerInterceptor({
        hostRef: { current: host },
        textareaRef: { current: ta },
        isActive: () => false,
        onOutsideTap: () => {},
      }),
    );

    const pointerdownCalls = addSpy.mock.calls.filter((c) => c[0] === "pointerdown");
    expect(pointerdownCalls).toHaveLength(1);
    expect(pointerdownCalls[0][2]).toEqual({ capture: true });

    rerender();
    const after = addSpy.mock.calls.filter((c) => c[0] === "pointerdown");
    expect(after).toHaveLength(1); // no re-subscribe on rerender
  });

  it("UAC-002/009: in 閲覧 mode a content tap is preventDefault'd, fires focus 0×, activeElement unchanged", () => {
    const focusSpy = vi.spyOn(ta, "focus");
    let focusEvents = 0;
    ta.addEventListener("focus", () => {
      focusEvents += 1;
    });
    const before = document.activeElement;

    renderHook(() =>
      useHostPointerInterceptor({
        hostRef: { current: host },
        textareaRef: { current: ta },
        isActive: () => false,
        onOutsideTap: () => {},
      }),
    );

    const ev = pointerdownOn(content);
    expect(ev.defaultPrevented).toBe(true);
    expect(focusSpy).not.toHaveBeenCalled();
    expect(focusEvents).toBe(0);
    expect(document.activeElement).toBe(before);
  });

  it("UAC-009: a swipe (multiple content pointerdowns) keeps focus dispatch at 0", () => {
    let focusEvents = 0;
    ta.addEventListener("focus", () => {
      focusEvents += 1;
    });
    renderHook(() =>
      useHostPointerInterceptor({
        hostRef: { current: host },
        textareaRef: { current: ta },
        isActive: () => false,
        onOutsideTap: () => {},
      }),
    );

    for (let i = 0; i < 4; i++) {
      const ev = pointerdownOn(content);
      expect(ev.defaultPrevented).toBe(true);
    }
    expect(focusEvents).toBe(0);
  });

  it("UAC-005: in 入力 mode a content tap triggers outside-tap (and does NOT block)", () => {
    const onOutsideTap = vi.fn();
    renderHook(() =>
      useHostPointerInterceptor({
        hostRef: { current: host },
        textareaRef: { current: ta },
        isActive: () => true,
        onOutsideTap,
      }),
    );

    const ev = pointerdownOn(content);
    expect(onOutsideTap).toHaveBeenCalledTimes(1);
    expect(ev.defaultPrevented).toBe(false);
  });

  it("in 入力 mode a tap on the helper textarea keeps input mode (no outside-tap)", () => {
    const onOutsideTap = vi.fn();
    renderHook(() =>
      useHostPointerInterceptor({
        hostRef: { current: host },
        textareaRef: { current: ta },
        isActive: () => true,
        onOutsideTap,
      }),
    );
    pointerdownOn(ta);
    expect(onOutsideTap).not.toHaveBeenCalled();
  });

  it("in 入力 mode a tap inside a [data-overlay] (FAB) is excluded from outside-tap", () => {
    const onOutsideTap = vi.fn();
    renderHook(() =>
      useHostPointerInterceptor({
        hostRef: { current: host },
        textareaRef: { current: ta },
        isActive: () => true,
        onOutsideTap,
      }),
    );
    const overlayChild = overlay.querySelector("button");
    if (!overlayChild) throw new Error("overlay child missing");
    pointerdownOn(overlayChild);
    expect(onOutsideTap).not.toHaveBeenCalled();
  });
});
