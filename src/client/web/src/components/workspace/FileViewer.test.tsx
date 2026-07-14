import { getCM, Vim } from "@replit/codemirror-vim";
import { EditorView } from "@codemirror/view";
import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FileViewer } from "./FileViewer";

const saveMock = vi.hoisted(() => vi.fn());

vi.mock("../../api/workspace", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../api/workspace")>();
  return {
    ...actual,
    makeWorkspaceApi: () => ({
      ...actual.makeWorkspaceApi(),
      save: saveMock,
    }),
  };
});

function getEditorView(): EditorView | null {
  const el = screen.getByTestId("codemirror-editor");
  return EditorView.findFromDOM(el) ?? null;
}

describe("FileViewer", () => {
  beforeEach(() => {
    saveMock.mockReset();
    saveMock.mockResolvedValue({ updated_mtime: "Mon, 01 Jan 2024 00:00:00 GMT", path: "a.txt" });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows metadata placeholder for binary files", () => {
    render(
      <FileViewer
        file={{
          path: "img.png",
          size: 1024,
          is_binary: true,
          content_type: "image/png",
        }}
      />,
    );
    expect(screen.getByTestId("metadata-placeholder")).toBeTruthy();
    expect(screen.getByText("image/png")).toBeTruthy();
  });

  it("verify-vim-mutation-integration: mutation keys change buffer content", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 20,
          is_binary: false,
          content: "line1\nline2\nline3\nline4\nline5\n",
        }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView();
    const cm = view ? getCM(view) : null;
    expect(cm).toBeTruthy();
    act(() => {
      Vim.handleKey(cm!, "d");
      Vim.handleKey(cm!, "d");
    });
    await waitFor(() => {
      const text = view?.state.doc.toString() ?? "";
      expect(text).not.toContain("line1");
    });
  });

  it("i+text+Esc changes content", async () => {
    const onDirtyChange = vi.fn();
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 3,
          is_binary: false,
          content: "abc",
        }}
        onDirtyChange={onDirtyChange}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView();
    const cm = view ? getCM(view) : null;
    act(() => {
      view?.focus();
      Vim.handleKey(cm!, "i");
    });
    expect(cm?.state.vim?.insertMode).toBe(true);
    act(() => {
      view?.dispatch({
        changes: { from: 0, to: 0, insert: "X" },
        userEvent: true,
      });
    });
    act(() => {
      Vim.handleKey(cm!, "<Esc>");
    });
    await waitFor(() => {
      expect(view?.state.sliceDoc(0, view.state.doc.length)).toBe("Xabc");
      expect(onDirtyChange).toHaveBeenCalledWith(true);
    });
  });

  it("renders .env content verbatim without masking", async () => {
    const secret = "API_KEY=super-secret-value\nDB_PASS=hunter2";
    render(
      <FileViewer
        file={{
          path: ".env",
          size: secret.length,
          is_binary: false,
          content: secret,
        }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("codemirror-editor")).toBeTruthy());
    const view = getEditorView();
    expect(view?.state.doc.toString()).toContain("super-secret-value");
    expect(view?.state.doc.toString()).toContain("hunter2");
  });

  it("uses CodeMirror for large files", async () => {
    const big = `${"x".repeat(100)}\n`.repeat(20_000);
    render(
      <FileViewer
        file={{
          path: "big.txt",
          size: big.length,
          is_binary: false,
          content: big,
        }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("codemirror-editor")).toBeTruthy());
    expect(screen.queryByTestId("virtualized-source")).toBeNull();
  });

  it("verify-vim-mutation-editing: ciw changes inner word", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 20,
          is_binary: false,
          content: "foo bar baz\n",
        }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView()!;
    const cm = getCM(view)!;
    act(() => {
      view.dispatch({ selection: { anchor: 4 } });
      Vim.handleKey(cm, "c");
      Vim.handleKey(cm, "i");
      Vim.handleKey(cm, "w");
    });
    act(() => {
      view.dispatch({
        changes: { from: view.state.selection.main.head, to: view.state.selection.main.head, insert: "QUX" },
        userEvent: "input.type",
      });
      Vim.handleKey(cm, "<Esc>");
    });
    expect(view.state.doc.toString()).toBe("foo QUX baz\n");
  });

  it("verify-vim-mutation-editing: 3dd deletes three lines", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 30,
          is_binary: false,
          content: "line1\nline2\nline3\nline4\nline5\n",
        }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView()!;
    const cm = getCM(view)!;
    act(() => {
      Vim.handleKey(cm, "3");
      Vim.handleKey(cm, "d");
      Vim.handleKey(cm, "d");
    });
    expect(view.state.doc.toString()).toBe("line4\nline5\n");
  });

  it("verify-vim-mutation-editing: yy/p duplicates a line", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 12,
          is_binary: false,
          content: "alpha\nbeta\n",
        }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView()!;
    const cm = getCM(view)!;
    act(() => {
      view.dispatch({ selection: { anchor: 0 } });
      Vim.handleKey(cm, "y");
      Vim.handleKey(cm, "y");
      Vim.handleKey(cm, "p");
    });
    expect(view.state.doc.toString()).toBe("alpha\nalpha\nbeta\n");
  });

  it("verify-vim-undo-locality: u reverts sequential edits", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 3,
          is_binary: false,
          content: "abc",
        }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView()!;
    const cm = getCM(view)!;
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
  });

  it("verify-vim-motion-unit: / search then n advances matches without xterm leak", async () => {
    const xtermKeydown = vi.fn();
    window.addEventListener("keydown", xtermKeydown);
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 24,
          is_binary: false,
          content: "alpha\nbeta\nalpha\n",
        }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView()!;
    const cm = getCM(view)!;
    act(() => {
      view.dispatch({ selection: { anchor: 0 } });
      Vim.handleKey(cm, "/");
      for (const ch of "alpha") {
        Vim.handleKey(cm, ch);
      }
      Vim.handleKey(cm, "<Enter>");
      Vim.handleKey(cm, "n");
    });
    const head = view.state.selection.main.head;
    expect(head).toBeGreaterThan(0);
    expect(xtermKeydown).not.toHaveBeenCalled();
    window.removeEventListener("keydown", xtermKeydown);
  });

  it("verify-vim-motion-unit: j/k and gg/G move cursor without xterm leak", async () => {
    const xtermKeydown = vi.fn();
    window.addEventListener("keydown", xtermKeydown);
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 30,
          is_binary: false,
          content: "line1\nline2\nline3\nline4\nline5\n",
        }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView()!;
    const cm = getCM(view)!;
    act(() => {
      Vim.handleKey(cm, "g");
      Vim.handleKey(cm, "g");
      Vim.handleKey(cm, "G");
    });
    const sel = view.state.selection.main.head;
    const line = view.state.doc.lineAt(sel).number;
    expect(line).toBe(view.state.doc.lines);
    expect(xtermKeydown).not.toHaveBeenCalled();
    window.removeEventListener("keydown", xtermKeydown);
  });

  it(":w invokes save when wired with session context", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 3,
          is_binary: false,
          content: "abc",
          mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
        }}
        sessionId="s1"
        pinnedHandle={{ frameGeneration: 1, resolvedRootPath: "/workspace" }}
      />,
    );
    await waitFor(() => expect(getEditorView()).toBeTruthy());
    const view = getEditorView();
    const cm = view ? getCM(view) : null;
    act(() => {
      Vim.handleEx(cm!, "w");
    });
    await waitFor(() => {
      expect(saveMock).toHaveBeenCalledTimes(1);
      expect(saveMock).toHaveBeenCalledWith(
        "s1",
        "a.txt",
        { frameGeneration: 1, resolvedRootPath: "/workspace" },
        "abc",
        "Mon, 01 Jan 2024 00:00:00 GMT",
      );
    });
  });
});