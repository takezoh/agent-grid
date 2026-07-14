import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useWorkspaceActivityStore } from "../../store/workspaceActivity";
import { WorkspaceDrawer } from "./WorkspaceDrawer";

const workspaceApiMocks = vi.hoisted(() => ({
  getRootHandle: vi.fn(),
  getFile: vi.fn(),
  getDiff: vi.fn(),
  getTree: vi.fn(),
}));

vi.mock("../../api/workspace", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../api/workspace")>();
  return {
    ...actual,
    makeWorkspaceApi: () => workspaceApiMocks,
  };
});

describe("WorkspaceDrawer", () => {
  beforeEach(() => {
    vi.useFakeTimers();
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
      content: "hello",
    });
    workspaceApiMocks.getDiff.mockResolvedValue({ outcome: "ok", diff: "+added\n" });
    workspaceApiMocks.getTree.mockResolvedValue({
      outcome: "ok",
      entries: [{ name: "src", path: "src", type: "dir" }],
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("verify-stale-render-latency: stale banner within 500ms", async () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "edit",
    });
    render(<WorkspaceDrawer sessionId="s1" />);

    act(() => {
      useWorkspaceActivityStore.getState().applyActivityEvents("s1", [
        {
          type: "mid_turn_touch",
          session_id: "s1",
          sequence: 1,
          path: "src/foo.ts",
          tool_call_id: "tc1",
        },
      ]);
    });

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(document.querySelector(".workspace-drawer__stale")).toBeTruthy();
  });

  it("reload clears stale banner", async () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [
      {
        type: "mid_turn_touch",
        session_id: "s1",
        sequence: 1,
        path: "src/foo.ts",
        tool_call_id: "tc1",
      },
    ]);
    render(<WorkspaceDrawer sessionId="s1" />);
    fireEvent.click(screen.getByRole("button", { name: "Reload" }));
    expect(screen.queryByText(/may be stale/i)).toBeNull();
  });

  it("verify-workspace-root-handle-pinning: shows banner on handle_stale", async () => {
    const { WorkspaceApiError } = await import("../../api/workspace");
    workspaceApiMocks.getFile.mockRejectedValue(
      new WorkspaceApiError(409, "handle_stale", "stale", {
        error: "handle_stale",
        frame_generation: 4,
      }),
    );
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    useWorkspaceActivityStore.getState().setPinnedHandle({
      frameGeneration: 3,
      resolvedRootPath: "/workspace",
    });
    render(<WorkspaceDrawer sessionId="s1" />);
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.getByText(/Workspace root changed/i)).toBeTruthy();
  });

  it("pins root handle at open", async () => {
    useWorkspaceActivityStore.getState().openDrawerTree("s1");
    render(<WorkspaceDrawer sessionId="s1" />);
    await act(async () => {
      await Promise.resolve();
    });
    const handle = useWorkspaceActivityStore.getState().pinnedHandle;
    expect(handle?.frameGeneration).toBe(3);
    expect(handle?.resolvedRootPath).toBe("/workspace");
  });
});
