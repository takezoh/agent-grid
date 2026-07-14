import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { WorkspaceTree } from "./WorkspaceTree";

const getTree = vi.fn();

vi.mock("../../api/workspace", () => ({
  makeWorkspaceApi: () => ({ getTree }),
}));

describe("WorkspaceTree", () => {
  beforeEach(() => {
    getTree.mockReset();
  });

  it("verify-tree-refresh-outcomes: normal listing", async () => {
    getTree.mockResolvedValue({
      outcome: "ok",
      entries: [{ name: "new.txt", path: "new.txt", type: "file" }],
    });
    render(
      <WorkspaceTree
        sessionId="s1"
        pinned={{ frameGeneration: 1, resolvedRootPath: "/ws" }}
        onSelectFile={vi.fn()}
      />,
    );
    await waitFor(() => expect(screen.getByRole("button", { name: /new\.txt/ })).toBeTruthy());
  });

  it("root_unreachable shows banner", async () => {
    getTree.mockResolvedValue({ outcome: "root_unreachable" });
    render(
      <WorkspaceTree
        sessionId="s1"
        pinned={{ frameGeneration: 1, resolvedRootPath: "/ws" }}
        onSelectFile={vi.fn()}
      />,
    );
    await waitFor(() => expect(screen.getByText(/unreachable/i)).toBeTruthy());
  });

  it("refresh_failed on transport error", async () => {
    getTree.mockRejectedValueOnce(new Error("network"));
    getTree.mockResolvedValue({
      outcome: "ok",
      entries: [{ name: "after.txt", path: "after.txt", type: "file" }],
    });
    render(
      <WorkspaceTree
        sessionId="s1"
        pinned={{ frameGeneration: 1, resolvedRootPath: "/ws" }}
        onSelectFile={vi.fn()}
      />,
    );
    await waitFor(() => expect(screen.getByText(/network/i)).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: "Refresh tree" }));
    expect(await screen.findByRole("button", { name: /after\.txt/ })).toBeTruthy();
  });
});
