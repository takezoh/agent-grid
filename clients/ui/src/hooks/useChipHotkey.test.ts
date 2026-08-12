// UAC-009 / FR-018 / FR-023 / NFR-006
import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePaletteStore } from "../store/palette";
import { type ChipHotkeyOptions, useChipHotkey } from "./useChipHotkey";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// dispatchKey fires a capture-visible keydown event on document.body so the
// capture-phase listener on document sees it before any bubble-phase consumer.
function dispatchKey(init: KeyboardEventInit): KeyboardEvent {
  const ev = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  document.body.dispatchEvent(ev);
  return ev;
}

// defaultOpts returns options that put the hook in the "fully active" state.
function defaultOpts(): ChipHotkeyOptions {
  return {
    worktreeChipVisible: true,
  };
}

// installSpies replaces toggleWorktree with a vi.fn() spy via setState so call
// sites are observable without triggering real reducer paths.
function installSpies(): {
  toggleWorktree: ReturnType<typeof vi.fn>;
} {
  const toggleWorktree = vi.fn();
  usePaletteStore.setState({ toggleWorktree });
  return { toggleWorktree };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useChipHotkey", () => {
  const realToggleWorktree = usePaletteStore.getState().toggleWorktree;

  beforeEach(() => {
    // Reset to a clean open paramSelect state with real actions restored.
    // Individual tests call installSpies() to swap in vi.fn() replacements.
    usePaletteStore.setState({
      open: true,
      phase: "paramSelect",
      composing: false,
      toggleWorktree: realToggleWorktree,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // -------------------------------------------------------------------------
  // Happy-path: Alt+W toggles worktree chip
  // -------------------------------------------------------------------------

  it("Alt+W (KeyW + altKey) calls toggleWorktree and calls preventDefault (FR-018 / UAC-009)", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).toHaveBeenCalledTimes(1);
    expect(ev.defaultPrevented).toBe(true);

    unmount();
  });

  // -------------------------------------------------------------------------
  // composing=true is no-op (FR-023)
  // -------------------------------------------------------------------------

  it("composing=true suppresses Alt+W (FR-023)", () => {
    // UAC-009 / FR-023
    const { toggleWorktree } = installSpies();
    usePaletteStore.setState({ composing: true });
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // phase !== 'paramSelect' is no-op
  // -------------------------------------------------------------------------

  it("phase='toolSelect' still toggles Alt+W when worktree chip is visible (FR-030)", () => {
    const { toggleWorktree } = installSpies();
    usePaletteStore.setState({ phase: "toolSelect" });
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).toHaveBeenCalledTimes(1);
    expect(ev.defaultPrevented).toBe(true);

    unmount();
  });

  // -------------------------------------------------------------------------
  // open=false is no-op
  // -------------------------------------------------------------------------

  it("open=false suppresses Alt+W", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    usePaletteStore.setState({ open: false });
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // worktreeChipVisible=false suppresses Alt+W
  // -------------------------------------------------------------------------

  it("worktreeChipVisible=false suppresses Alt+W", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const opts: ChipHotkeyOptions = { worktreeChipVisible: false };
    const { unmount } = renderHook(() => useChipHotkey(opts));

    const ev = dispatchKey({ code: "KeyW", altKey: true });
    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // altKey=false is no-op
  // -------------------------------------------------------------------------

  it("altKey=false does not fire toggleWorktree for KeyW", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyW", altKey: false });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // Unrelated key codes with altKey are no-op
  // -------------------------------------------------------------------------

  it("event.code='KeyA' + altKey=true does not fire any action", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const ev = dispatchKey({ code: "KeyA", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(false);

    unmount();
  });

  // -------------------------------------------------------------------------
  // Capture-phase registration
  // -------------------------------------------------------------------------

  it("registers listener on document with capture=true and removes on unmount (NFR-006)", () => {
    // NFR-006 / FR-018
    const addSpy = vi.spyOn(document, "addEventListener");
    const removeSpy = vi.spyOn(document, "removeEventListener");

    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));

    const addCalls = addSpy.mock.calls.filter((c) => c[0] === "keydown");
    expect(addCalls).toHaveLength(1);
    // The third argument must be the boolean `true` (capture phase).
    expect(addCalls[0]?.[2]).toBe(true);

    unmount();

    const removeCalls = removeSpy.mock.calls.filter((c) => c[0] === "keydown");
    expect(removeCalls).toHaveLength(1);
    expect(removeCalls[0]?.[2]).toBe(true);
  });

  // -------------------------------------------------------------------------
  // Unmount removes the listener — subsequent keydown is silently ignored
  // -------------------------------------------------------------------------

  it("after unmount, Alt+W no longer calls toggleWorktree", () => {
    // UAC-009 / FR-018
    const { toggleWorktree } = installSpies();
    const { unmount } = renderHook(() => useChipHotkey(defaultOpts()));
    unmount();

    dispatchKey({ code: "KeyW", altKey: true });

    expect(toggleWorktree).not.toHaveBeenCalled();
  });
});
