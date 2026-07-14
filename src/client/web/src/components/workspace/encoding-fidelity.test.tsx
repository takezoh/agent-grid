import { EditorView } from "@codemirror/view";
import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FileViewer } from "./FileViewer";

function getEditorView(): EditorView | null {
  const el = screen.getByTestId("codemirror-editor");
  return EditorView.findFromDOM(el) ?? null;
}

describe("encoding-fidelity", () => {
  it("preserves CRLF line endings via EditorState.lineSeparator", async () => {
    const crlf = "line1\r\nline2\r\n";
    render(
      <FileViewer
        file={{
          path: "win.txt",
          size: crlf.length,
          is_binary: false,
          content: crlf,
        }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("codemirror-editor")).toBeTruthy());
    expect(screen.getByTestId("codemirror-editor").getAttribute("data-line-separator")).toBe(
      "crlf",
    );
    const view = getEditorView();
    expect(view?.state.lineBreak).toBe("\r\n");
    expect(view?.state.sliceDoc(0, view.state.doc.length)).toBe(crlf);
  });

  it("uses LF separator when content has no CRLF", async () => {
    const lf = "line1\nline2\n";
    render(
      <FileViewer
        file={{
          path: "unix.txt",
          size: lf.length,
          is_binary: false,
          content: lf,
        }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("codemirror-editor")).toBeTruthy());
    expect(screen.getByTestId("codemirror-editor").getAttribute("data-line-separator")).toBe("lf");
    const view = getEditorView();
    expect(view?.state.lineBreak).toBe("\n");
    expect(view?.state.sliceDoc(0, view.state.doc.length)).toBe(lf);
  });
});