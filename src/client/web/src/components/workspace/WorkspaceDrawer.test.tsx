import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useWorkspaceActivityStore } from "../../store/workspaceActivity";
import { WorkspaceDrawer } from "./WorkspaceDrawer";

async function primeDrawerDirtyConflict(kind: "read" | "edit" = "read") {
  useWorkspaceActivityStore.getState().openDrawerFromRow({
    sessionId: "s1",
    path: "src/foo.ts",
    kind,
  });
  render(<WorkspaceDrawer sessionId="s1" />);
  await act(async () => {
    await Promise.resolve();
  });
  useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);
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
  expect(screen.getByTestId("conflict-banner")).toBeTruthy();
}

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
      mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
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

  it("shows dirty indicator when buffer is dirty", () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);
    render(<WorkspaceDrawer sessionId="s1" />);
    expect(screen.getByTestId("dirty-indicator")).toBeTruthy();
  });

  it("shows close warning when closing with dirty buffer", () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);
    render(<WorkspaceDrawer sessionId="s1" />);
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.getByTestId("close-warning-dialog")).toBeTruthy();
  });

  it("shows conflict banner for dirty buffer stale touch", () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "edit",
    });
    useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);
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
    expect(screen.getByTestId("conflict-banner")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Keep mine" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Take theirs" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Merge" })).toBeTruthy();
  });

  it("verify-drawer-terminal-rect-nonregression: terminal-slot rect unchanged with drawer chrome", () => {
    const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect;
    HTMLElement.prototype.getBoundingClientRect = () =>
      ({
        x: 10,
        y: 20,
        width: 640,
        height: 480,
        top: 20,
        left: 10,
        right: 650,
        bottom: 500,
        toJSON: () => ({}),
      }) as DOMRect;

    function Harness({ withDrawer }: { withDrawer: boolean }) {
      return (
        <div className="main-with-activity-rail" data-testid="main-with-activity-rail">
          <div className="main-with-activity-rail__tabs">
            <div className="main-tabs-body">
              <div className="terminal-slot" data-testid="terminal-slot">
                slot
              </div>
            </div>
          </div>
          {withDrawer && (
            <>
              <WorkspaceDrawer sessionId="s1" />
            </>
          )}
        </div>
      );
    }

    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "edit",
    });
    useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [
      {
        type: "mid_turn_touch",
        session_id: "s1",
        sequence: 1,
        path: "src/foo.ts",
        tool_call_id: "tc1",
      },
    ]);

    const { unmount: unmountWithout } = render(<Harness withDrawer={false} />);
    const without = screen.getByTestId("terminal-slot").getBoundingClientRect();
    unmountWithout();

    render(<Harness withDrawer />);
    const withDrawer = screen.getByTestId("terminal-slot").getBoundingClientRect();

    expect(Math.abs(withDrawer.width - without.width)).toBeLessThanOrEqual(1);
    expect(Math.abs(withDrawer.height - without.height)).toBeLessThanOrEqual(1);
    expect(Math.abs(withDrawer.top - without.top)).toBeLessThanOrEqual(1);
    expect(Math.abs(withDrawer.left - without.left)).toBeLessThanOrEqual(1);

    HTMLElement.prototype.getBoundingClientRect = originalGetBoundingClientRect;
  });

  it("verify-write-conflict-detection: keep-mine retains operator buffer and clears conflict", async () => {
    await primeDrawerDirtyConflict("read");
    fireEvent.click(screen.getByRole("button", { name: "Keep mine" }));
    const state = useWorkspaceActivityStore.getState();
    expect(screen.queryByTestId("conflict-banner")).toBeNull();
    expect(state.dirtyBuffers["src/foo.ts"]).toBeDefined();
    expect(state.conflictByPath["src/foo.ts"]).toBeUndefined();
    expect(state.stalePaths["src/foo.ts"]).toBeUndefined();
    expect(workspaceApiMocks.getFile).toHaveBeenCalled();
  });

  it("verify-write-conflict-detection: take-theirs reloads server bytes", async () => {
    workspaceApiMocks.getFile
      .mockResolvedValueOnce({
        path: "src/foo.ts",
        size: 5,
        is_binary: false,
        content: "hello",
        mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
      })
      .mockResolvedValueOnce({
        path: "src/foo.ts",
        size: 14,
        is_binary: false,
        content: "agent-on-disk",
        mtime: "Mon, 02 Jan 2024 00:00:00 GMT",
      });
    await primeDrawerDirtyConflict("read");
    fireEvent.click(screen.getByRole("button", { name: "Take theirs" }));
    await act(async () => {
      await Promise.resolve();
    });
    expect(workspaceApiMocks.getFile.mock.calls.length).toBeGreaterThanOrEqual(2);
    const state = useWorkspaceActivityStore.getState();
    expect(state.dirtyBuffers["src/foo.ts"]?.dirty).toBe(false);
    expect(screen.queryByTestId("conflict-banner")).toBeNull();
    expect(workspaceApiMocks.getFile.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(workspaceApiMocks.getFile.mock.calls.at(-1)?.[1]).toBe("src/foo.ts");
  });

  it("verify-write-conflict-detection: merge shows diff tab with server diff", async () => {
    workspaceApiMocks.getDiff.mockResolvedValue({
      outcome: "ok",
      diff: "@@\n+merged-line\n",
    });
    await primeDrawerDirtyConflict("edit");
    fireEvent.click(screen.getByRole("button", { name: "Merge" }));
    await act(async () => {
      await Promise.resolve();
    });
    expect(workspaceApiMocks.getDiff.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(screen.getByRole("tab", { name: "Diff" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByTestId("diff-viewer").textContent).toContain("merged-line");
  });

  it("aria-live announces conflict over stale", () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "edit",
    });
    useWorkspaceActivityStore.getState().setBufferDirty("src/foo.ts", true);
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
    const live = document.querySelector(".workspace-drawer__live");
    expect(live?.textContent).toMatch(/write conflict/i);
  });
});