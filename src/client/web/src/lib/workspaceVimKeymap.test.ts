import { describe, expect, it, vi } from "vitest";
import { createVimMotionState, dispatchVimKey } from "./workspaceVimKeymap";

describe("workspaceVimKeymap", () => {
  const cb = {
    getLineCount: () => 10,
    scrollToLine: vi.fn(),
    getSearchableText: () => "alpha\nbeta\nalpha\n",
  };

  it("j moves down one line", () => {
    let state = createVimMotionState();
    const r = dispatchVimKey("j", state, cb);
    state = r.state;
    expect(state.line).toBe(1);
    expect(cb.scrollToLine).toHaveBeenCalledWith(1);
  });

  it("k moves up one line", () => {
    const state = { ...createVimMotionState(), line: 3 };
    const r = dispatchVimKey("k", state, cb);
    expect(r.state.line).toBe(2);
  });

  it("gg moves to top", () => {
    const state = { ...createVimMotionState(), line: 5, pendingG: true };
    const r = dispatchVimKey("g", state, cb);
    expect(r.state.line).toBe(0);
  });

  it("G moves to bottom", () => {
    const state = { ...createVimMotionState(), line: 0 };
    const r = dispatchVimKey("G", state, cb);
    expect(r.state.line).toBe(9);
  });

  it("/ search then n advances matches", () => {
    let state = createVimMotionState();
    state = dispatchVimKey("/", state, cb).state;
    state = dispatchVimKey("a", state, cb).state;
    state = dispatchVimKey("l", state, cb).state;
    state = dispatchVimKey("Enter", state, cb).state;
    expect(state.matchIndices.length).toBeGreaterThan(0);
    const next = dispatchVimKey("n", state, cb);
    expect(next.state.matchCursor).toBe(1);
  });
});
