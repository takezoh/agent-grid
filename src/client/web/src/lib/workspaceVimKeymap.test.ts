import { getCM, Vim, vim } from "@replit/codemirror-vim";
import { history } from "@codemirror/commands";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createVimMotionState, dispatchVimKey } from "./workspaceVimKeymap";

function mountVimDoc(content: string): { view: EditorView; cm: ReturnType<typeof getCM> } {
  const parent = document.createElement("div");
  document.body.appendChild(parent);
  const state = EditorState.create({
    doc: content,
    extensions: [history(), vim()],
  });
  const view = new EditorView({ state, parent });
  const cm = getCM(view);
  if (!cm) throw new Error("vim cm missing");
  return { view, cm };
}

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

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("verify-vim-undo-locality: CodeMirror undo stack is session-local", () => {
    const { view, cm } = mountVimDoc("abc");
    const insertAt = (pos: number, text: string) => {
      act(() => {
        view.focus();
        Vim.handleKey(cm, "i");
        view.dispatch({ changes: { from: pos, to: pos, insert: text }, userEvent: "input.type" });
        Vim.handleKey(cm, "<Esc>");
      });
    };
    insertAt(0, "A");
    insertAt(1, "B");
    insertAt(2, "C");
    expect(view.state.doc.toString()).toBe("ABCabc");
    act(() => {
      Vim.handleKey(cm, "u");
      Vim.handleKey(cm, "u");
    });
    expect(view.state.doc.toString()).toBe("Aabc");
    view.destroy();
  });

  it("verify-vim-undo-locality: reload clears undo stack", () => {
    const first = mountVimDoc("abc");
    act(() => {
      Vim.handleKey(first.cm, "i");
      first.view.dispatch({ changes: { from: 0, to: 0, insert: "Z" }, userEvent: "input.type" });
      Vim.handleKey(first.cm, "<Esc>");
    });
    first.view.destroy();

    const second = mountVimDoc("abc");
    act(() => {
      Vim.handleKey(second.cm, "u");
    });
    expect(second.view.state.doc.toString()).toBe("abc");
    second.view.destroy();
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
