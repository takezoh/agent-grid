// useInputMode.test.ts — pure reducer + DOM-wiring coverage for the mode machine
// (FR-MOB-MODE-001..006, ADR 0068). Discriminating against:
//   - UAC-001: 'false' invariant at rest.
//   - UAC-003 counterexample B: enter must survive 200ms with no exit race.
//   - UAC-004: a re-toggle returns to false (toggle, not one-shot).
//   - UAC-005 / UAC-006: outside-tap is silent; blur / Esc announce exactly once.

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  INITIAL_INPUT_MODE_STATE,
  type InputModeState,
  VIEW_MODE_ANNOUNCEMENT,
  inputModeReducer,
  useInputMode,
} from "./useInputMode";

describe("inputModeReducer — pure transitions", () => {
  it("UAC-001: the initial state is the 'false' invariant with no message", () => {
    expect(INITIAL_INPUT_MODE_STATE).toEqual({ active: false, lastMessage: null });
  });

  it("enter activates input mode silently", () => {
    expect(inputModeReducer(INITIAL_INPUT_MODE_STATE, { type: "enter" })).toEqual({
      active: true,
      lastMessage: null,
    });
  });

  it("enter while already active is a no-op (same reference)", () => {
    const active: InputModeState = { active: true, lastMessage: null };
    expect(inputModeReducer(active, { type: "enter" })).toBe(active);
  });

  it("UAC-004: toggle flips false→true→false (a real toggle, not one-shot)", () => {
    const on = inputModeReducer(INITIAL_INPUT_MODE_STATE, { type: "toggle" });
    expect(on).toEqual({ active: true, lastMessage: null });
    const off = inputModeReducer(on, { type: "toggle" });
    expect(off).toEqual({ active: false, lastMessage: null });
  });

  it("UAC-006: exit('blur') and exit('esc') announce 'Returned to view mode'", () => {
    const active: InputModeState = { active: true, lastMessage: null };
    expect(inputModeReducer(active, { type: "exit", reason: "blur" })).toEqual({
      active: false,
      lastMessage: VIEW_MODE_ANNOUNCEMENT,
    });
    expect(inputModeReducer(active, { type: "exit", reason: "esc" })).toEqual({
      active: false,
      lastMessage: VIEW_MODE_ANNOUNCEMENT,
    });
  });

  it("UAC-005: exit('outside-tap') / 'fab' / 'gate-false' exit silently", () => {
    const active: InputModeState = { active: true, lastMessage: null };
    for (const reason of ["outside-tap", "fab", "gate-false"] as const) {
      expect(inputModeReducer(active, { type: "exit", reason })).toEqual({
        active: false,
        lastMessage: null,
      });
    }
  });

  it("counterexample B: exit while already inactive is an idempotent no-op (no re-announce)", () => {
    const view: InputModeState = { active: false, lastMessage: null };
    // A stray blur from the FAB stealing focus AFTER the user already left input
    // mode must not produce a phantom transition or a duplicate announcement.
    expect(inputModeReducer(view, { type: "exit", reason: "blur" })).toBe(view);
  });
});

describe("useInputMode — DOM wiring (data-input-active / readonly / focus / announce)", () => {
  let host: HTMLElement;
  let ta: HTMLTextAreaElement;

  beforeEach(() => {
    vi.useFakeTimers();
    host = document.createElement("div");
    host.className = "terminal-host";
    ta = document.createElement("textarea");
    ta.className = "xterm-helper-textarea";
    document.body.append(host, ta);
  });

  afterEach(() => {
    vi.useRealTimers();
    host.remove();
    ta.remove();
  });

  function setup(announce?: (text: string) => void) {
    const hostRef = { current: host };
    const textareaRef = { current: ta };
    return renderHook(() => useInputMode({ hostRef, textareaRef, announce }));
  }

  it("UAC-001: at rest data-input-active='false', textarea readonly, activeElement≠textarea", () => {
    setup();
    expect(host.getAttribute("data-input-active")).toBe("false");
    expect(ta.hasAttribute("readonly")).toBe(true);
    expect(document.activeElement).not.toBe(ta);
  });

  it("UAC-003: enter focuses the textarea and stays active 200ms later (no exit race)", () => {
    const focusSpy = vi.spyOn(ta, "focus");
    const { result } = setup();

    act(() => result.current.enter());
    expect(host.getAttribute("data-input-active")).toBe("true");
    expect(ta.hasAttribute("readonly")).toBe(false);
    expect(focusSpy).toHaveBeenCalled();

    // No timer-driven blur-exit may flip us back within the dwell window.
    act(() => vi.advanceTimersByTime(200));
    expect(host.getAttribute("data-input-active")).toBe("true");
    expect(result.current.active).toBe(true);
  });

  it("UAC-004: toggling twice returns to view mode and re-adds readonly", () => {
    const { result } = setup();
    act(() => result.current.toggle());
    expect(result.current.active).toBe(true);
    act(() => result.current.toggle());
    expect(result.current.active).toBe(false);
    expect(host.getAttribute("data-input-active")).toBe("false");
    expect(ta.hasAttribute("readonly")).toBe(true);
  });

  it("UAC-006: exit('blur') announces exactly once; outside-tap is silent", () => {
    const announce = vi.fn();
    const { result } = setup(announce);

    act(() => result.current.enter());
    act(() => result.current.exit("outside-tap"));
    expect(announce).not.toHaveBeenCalled();

    act(() => result.current.enter());
    act(() => result.current.exit("blur"));
    expect(announce).toHaveBeenCalledTimes(1);
    expect(announce).toHaveBeenCalledWith(VIEW_MODE_ANNOUNCEMENT);
  });

  it("UAC-006: a textarea blur event exits input mode and announces", () => {
    const announce = vi.fn();
    const { result } = setup(announce);
    act(() => result.current.enter());
    act(() => ta.dispatchEvent(new Event("blur")));
    expect(result.current.active).toBe(false);
    expect(announce).toHaveBeenCalledWith(VIEW_MODE_ANNOUNCEMENT);
  });

  it("UAC-006: an Escape keydown on document exits input mode and announces", () => {
    const announce = vi.fn();
    const { result } = setup(announce);
    act(() => result.current.enter());
    act(() => document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" })));
    expect(result.current.active).toBe(false);
    expect(announce).toHaveBeenCalledWith(VIEW_MODE_ANNOUNCEMENT);
  });
});
