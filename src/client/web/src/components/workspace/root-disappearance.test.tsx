import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WorkspaceApiError } from "../../api/workspace";
import { useWorkspaceActivityStore } from "../../store/workspaceActivity";
import { WorkspaceDrawer } from "./WorkspaceDrawer";

const workspaceApiMocks = vi.hoisted(() => ({
  getRootHandle: vi.fn(),
  getFile: vi.fn(),
  getDiff: vi.fn(),
  getTree: vi.fn(),
  save: vi.fn(),
}));

vi.mock("../../api/workspace", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../api/workspace")>();
  return {
    ...actual,
    makeWorkspaceApi: () => workspaceApiMocks,
  };
});

describe("root-disappearance", () => {
  beforeEach(() => {
    useWorkspaceActivityStore.getState().reset();
    useWorkspaceActivityStore.getState().setScopedSession("s1");
    workspaceApiMocks.getRootHandle.mockResolvedValue({
      session_id: "s1",
      frame_generation: 3,
      resolved_root_path: "/workspace",
    });
    workspaceApiMocks.getFile.mockResolvedValue({
      path: "src/foo.ts",
      size: 12,
      is_binary: false,
      content: "hello world",
      mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
    });
    workspaceApiMocks.getDiff.mockResolvedValue({ outcome: "ok", diff: "" });
    workspaceApiMocks.getTree.mockResolvedValue({ outcome: "ok", entries: [] });
    vi.stubGlobal("navigator", {
      ...navigator,
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("shows root_disappeared banner with clipboard export when handle_stale on dirty buffer", async () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    useWorkspaceActivityStore.getState().setPinnedHandle({
      sessionId: "s1",
      frameGeneration: 3,
      resolvedRootPath: "/workspace",
    });
    useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);

    workspaceApiMocks.getFile.mockRejectedValue(
      new WorkspaceApiError(409, "handle_stale", "stale", { error: "handle_stale" }),
    );

    render(<WorkspaceDrawer sessionId="s1" />);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    await act(async () => {
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(screen.getByTestId("root-disappeared-banner")).toBeTruthy();
    });
    expect(screen.getByText(/Copy buffer to clipboard/i)).toBeTruthy();
    expect(useWorkspaceActivityStore.getState().rootDisappeared).toBe(true);
  });

  it("does not close drawer silently on root disappearance", async () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    useWorkspaceActivityStore.getState().setPinnedHandle({
      sessionId: "s1",
      frameGeneration: 3,
      resolvedRootPath: "/workspace",
    });
    useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);
    useWorkspaceActivityStore.getState().setRootDisappeared(true);

    render(<WorkspaceDrawer sessionId="s1" />);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(screen.getByTestId("workspace-drawer")).toBeTruthy();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Copy buffer to clipboard" }));
      await Promise.resolve();
    });
    expect(navigator.clipboard.writeText).toHaveBeenCalled();
  });
});
