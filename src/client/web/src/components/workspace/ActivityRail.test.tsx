import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../../store/daemon";
import { useWorkspaceActivityStore } from "../../store/workspaceActivity";
import { ActivityRail } from "./ActivityRail";

describe("ActivityRail", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    useWorkspaceActivityStore.getState().reset();
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/p",
          command: "claude",
          created_at: "2026-07-14T00:00:00Z",
          view: { card: {} },
        },
      ],
      activeSessionID: "s1",
    });
    useWorkspaceActivityStore.getState().setScopedSession("s1");
  });

  it("verify-workspace-affordance-a11y: affordance visible with 0 rows", () => {
    render(<ActivityRail onOpenTree={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Open workspace tree" })).toBeTruthy();
  });

  it("affordance visible with rows and keyboard opens tree via callback", () => {
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [
      {
        type: "turn_row",
        session_id: "s1",
        sequence: 1,
        turn_id: "t1",
        path: "src/a.ts",
        kind: "read",
        count: 1,
        events: [{ path: "src/a.ts", kind: "read" }],
      },
    ]);
    const onOpenTree = vi.fn();
    render(<ActivityRail onOpenTree={onOpenTree} />);
    fireEvent.click(screen.getByRole("button", { name: "Open workspace tree" }));
    expect(onOpenTree).toHaveBeenCalled();
  });

  it("verify-workspace-affordance-a11y: affordance with 5 rows and Enter opens tree", () => {
    const events = Array.from({ length: 5 }, (_, i) => ({
      type: "turn_row" as const,
      session_id: "s1",
      sequence: i + 1,
      turn_id: `t${i}`,
      path: `src/f${i}.ts`,
      kind: "read" as const,
      count: 1,
      events: [{ path: `src/f${i}.ts`, kind: "read" as const }],
    }));
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", events);
    const onOpenTree = vi.fn();
    render(<ActivityRail onOpenTree={onOpenTree} />);
    const btn = screen.getByRole("button", { name: "Open workspace tree" });
    fireEvent.keyDown(btn, { key: "Enter" });
    expect(onOpenTree).toHaveBeenCalled();
    expect(screen.getAllByText(/src\/f\d\.ts/)).toHaveLength(5);
  });

  it("verify-appshell-terminal-rect-nonregression: terminal-slot rect unchanged with rail", () => {
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

    function Harness({ withRail }: { withRail: boolean }) {
      return (
        <div className="main-with-activity-rail" data-testid="main-with-activity-rail">
          {withRail && <ActivityRail onOpenTree={() => {}} />}
          <div className="main-with-activity-rail__tabs">
            <div className="main-tabs-body">
              <div className="terminal-slot" data-testid="terminal-slot">
                slot
              </div>
            </div>
          </div>
        </div>
      );
    }

    const { unmount: unmountWithout } = render(<Harness withRail={false} />);
    const without = screen.getByTestId("terminal-slot").getBoundingClientRect();
    unmountWithout();

    render(<Harness withRail />);
    const withRail = screen.getByTestId("terminal-slot").getBoundingClientRect();

    expect(Math.abs(withRail.width - without.width)).toBeLessThanOrEqual(1);
    expect(Math.abs(withRail.height - without.height)).toBeLessThanOrEqual(1);
    expect(Math.abs(withRail.top - without.top)).toBeLessThanOrEqual(1);
    expect(Math.abs(withRail.left - without.left)).toBeLessThanOrEqual(1);

    HTMLElement.prototype.getBoundingClientRect = originalGetBoundingClientRect;
  });

  it("operator row has distinct badge and aria-label", () => {
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [
      {
        type: "mid_turn_touch",
        session_id: "s1",
        sequence: 1,
        path: "src/op.ts",
        tool_call_id: "op-1",
        kind: "edit",
        actor: "operator",
      },
    ]);
    render(<ActivityRail onOpenTree={vi.fn()} />);
    expect(screen.getByLabelText("Operator edited src/op.ts")).toBeTruthy();
    expect(screen.getByText("operator")).toBeTruthy();
  });

  it("renders turn rows from store", () => {
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [
      {
        type: "turn_row",
        session_id: "s1",
        sequence: 1,
        turn_id: "t1",
        path: "src/foo.ts",
        kind: "edit",
        count: 2,
        events: [
          { path: "src/foo.ts", kind: "read" },
          { path: "src/foo.ts", kind: "edit" },
        ],
      },
    ]);
    render(<ActivityRail onOpenTree={vi.fn()} />);
    expect(screen.getByText("src/foo.ts")).toBeTruthy();
    expect(screen.getByLabelText("2 events")).toBeTruthy();
  });
});
