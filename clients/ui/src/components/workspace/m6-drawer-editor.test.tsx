import * as fs from "node:fs";
import * as path from "node:path";
import { EditorView } from "@codemirror/view";
import { Vim, getCM } from "@replit/codemirror-vim";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useWorkspaceActivityStore } from "../../store/workspaceActivity";
import { FileViewer } from "./FileViewer";
import { WorkspaceDrawer } from "./WorkspaceDrawer";
import { WorkspaceTree } from "./WorkspaceTree";

const saveMock = vi.hoisted(() => vi.fn());

const workspaceApiMocks = vi.hoisted(() => ({
  getRootHandle: vi.fn(),
  getFile: vi.fn(),
  getDiff: vi.fn(),
  getTree: vi.fn(),
  save: saveMock,
}));

vi.mock("../../api/workspace", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../api/workspace")>();
  return {
    ...actual,
    makeWorkspaceApi: () => workspaceApiMocks,
  };
});

function readWorkspaceCss(): string {
  return fs.readFileSync(path.resolve(__dirname, "../../css/workspace.css"), "utf-8");
}

describe("m6 — workspace is a main-area mode layer (not an overlay)", () => {
  it("workspace-view is a flex mode layer with a persistent tree panel; no fixed overlay", () => {
    const css = readWorkspaceCss();
    expect(css).toMatch(/\.main-modes__layer\[data-active="false"\][\s\S]*visibility:\s*hidden/);
    expect(css).toMatch(/\.workspace-view__tree[\s\S]*border-left/);
    expect(css).not.toMatch(/\.workspace-drawer__panel[\s\S]*position:\s*fixed/);
  });

  it("workspace view is non-modal: no focus trap, controls focus normally", async () => {
    useWorkspaceActivityStore.getState().reset();
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    render(<WorkspaceDrawer sessionId="s1" />);
    const panel = document.querySelector(".workspace-drawer__panel");
    expect(panel).toBeTruthy();
    const close = screen.getByRole("button", { name: "Close" });
    close.focus();
    expect(document.activeElement).toBe(close);
  });
});

describe("m6 UAC-017 — WorkspaceTree hierarchy chrome", () => {
  beforeEach(() => {
    workspaceApiMocks.getTree.mockReset();
    workspaceApiMocks.getTree.mockImplementation(async (_sid, treePath) => {
      if (treePath === "") {
        return { outcome: "ok", entries: [{ name: "src", path: "src", type: "dir" }] };
      }
      if (treePath === "src") {
        return {
          outcome: "ok",
          entries: [{ name: "foo.ts", path: "src/foo.ts", type: "file" }],
        };
      }
      return { outcome: "ok", entries: [] };
    });
  });

  it("renders indentation guides, chevron rotation, and dir/file icons when expanding src/", async () => {
    render(
      <WorkspaceTree
        sessionId="s1"
        pinned={{ sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/ws" }}
        onSelectFile={vi.fn()}
      />,
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "src" })).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: "src" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "foo.ts" })).toBeTruthy());
    expect(document.querySelector(".workspace-tree__group")).toBeTruthy();
    expect(document.querySelector(".workspace-tree__chevron--expanded")).toBeTruthy();
    expect(document.querySelector(".icon--folder")).toBeTruthy();
    expect(document.querySelector(".icon--file")).toBeTruthy();
  });

  it("keeps expansion state while drawer stays open across tab switches", async () => {
    useWorkspaceActivityStore.getState().reset();
    workspaceApiMocks.getRootHandle.mockResolvedValue({
      session_id: "s1",
      frame_generation: 3,
      resolved_root_path: "/workspace",
    });
    useWorkspaceActivityStore.getState().openDrawerTree("s1");
    render(<WorkspaceDrawer sessionId="s1" />);
    await waitFor(() => expect(screen.getByRole("button", { name: "src" })).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: "src" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "foo.ts" })).toBeTruthy());
    // The tree is a persistent side panel now: switching content tabs
    // (Viewer/Diff) must not unmount it or drop expansion state.
    fireEvent.click(screen.getByRole("tab", { name: "Viewer" }));
    fireEvent.click(screen.getByRole("tab", { name: "Viewer" }));
    expect(screen.getByRole("button", { name: "foo.ts" })).toBeTruthy();
    expect(document.querySelector(".workspace-tree__chevron--expanded")).toBeTruthy();
  });
});

describe("m6 UAC-018 — editor Save button and Cmd/Ctrl+S scope", () => {
  beforeEach(() => {
    saveMock.mockReset();
    saveMock.mockResolvedValue({ updated_mtime: "Mon, 01 Jan 2024 00:00:00 GMT", path: "a.txt" });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Save button transitions saved → dirty → saving → saved", async () => {
    const onDirtyChange = vi.fn();
    const onSaveSuccess = vi.fn();
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 3,
          is_binary: false,
          content: "abc",
          mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
        }}
        eventKind="edit"
        sessionId="s1"
        pinnedHandle={{ sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/workspace" }}
        onDirtyChange={onDirtyChange}
        onSaveSuccess={onSaveSuccess}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("codemirror-editor")).toBeTruthy());
    const saveBtn = screen.getByTestId("editor-save-button");
    expect(saveBtn.getAttribute("data-save-state")).toBe("saved");
    expect((saveBtn as HTMLButtonElement).disabled).toBe(true);

    const el = screen.getByTestId("codemirror-editor");
    const view = EditorView.findFromDOM(el);
    if (!view) throw new Error("missing editor");
    act(() => {
      view.dispatch({ changes: { from: 0, to: 0, insert: "X" }, userEvent: true });
    });
    await waitFor(() => expect(saveBtn.getAttribute("data-save-state")).toBe("dirty"));
    expect(screen.getByTestId("save-dirty-dot")).toBeTruthy();

    fireEvent.click(saveBtn);
    await waitFor(() => expect(saveMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(onSaveSuccess).toHaveBeenCalled());
    expect(saveBtn.getAttribute("data-save-state")).toBe("saved");
  });

  it("Cmd/Ctrl+S saves only when editor is focused", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 3,
          is_binary: false,
          content: "abc",
          mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
        }}
        eventKind="edit"
        sessionId="s1"
        pinnedHandle={{ sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/workspace" }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("codemirror-editor")).toBeTruthy());

    const outside = document.createElement("button");
    outside.textContent = "outside";
    document.body.appendChild(outside);
    outside.focus();
    fireEvent.keyDown(document, { key: "s", ctrlKey: true });
    expect(saveMock).not.toHaveBeenCalled();

    const editor = screen.getByTestId("codemirror-editor");
    const content = editor.querySelector(".cm-content") as HTMLElement;
    content.focus();
    fireEvent.keyDown(editor, { key: "s", ctrlKey: true });
    await waitFor(() => expect(saveMock).toHaveBeenCalledTimes(1));
    outside.remove();
  });

  it("vim :w still invokes save", async () => {
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 3,
          is_binary: false,
          content: "abc",
          mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
        }}
        eventKind="edit"
        sessionId="s1"
        pinnedHandle={{ sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/workspace" }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("codemirror-editor")).toBeTruthy());
    const el = screen.getByTestId("codemirror-editor");
    const view = EditorView.findFromDOM(el);
    if (!view) throw new Error("missing editor");
    const cm = getCM(view);
    if (!cm) throw new Error("missing vim");
    act(() => {
      Vim.handleEx(cm, "w");
    });
    await waitFor(() => expect(saveMock).toHaveBeenCalledTimes(1));
  });
});

describe("m6 UAC-019 — read-only degradation banner and Save tooltip", () => {
  it("shows read-only banner and disables Save with reason when saveDisabled", async () => {
    const reason = "Workspace root disappeared. Buffer kept in memory; save is disabled.";
    render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 3,
          is_binary: false,
          content: "abc",
          mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
        }}
        eventKind="edit"
        sessionId="s1"
        pinnedHandle={{ sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/workspace" }}
        saveDisabled
        readOnlyReason={reason}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("read-only-banner")).toBeTruthy());
    expect(screen.getByTestId("read-only-banner").textContent).toContain("root disappeared");
    const saveBtn = screen.getByTestId("editor-save-button");
    expect(saveBtn.getAttribute("data-save-state")).toBe("read-only");
    expect((saveBtn as HTMLButtonElement).disabled).toBe(true);
  });
});
