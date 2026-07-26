import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { SessionStatus } from "../wire/server";
import { RunStateBadge } from "./RunStateBadge";
import { type StatusKind, statusLabel } from "./StatusIcon";

const cases: [SessionStatus | undefined, StatusKind][] = [
  [{ lifecycle: "active", human_input: false, failure: "none" }, "running"],
  [{ lifecycle: "active", human_input: true, failure: "none" }, "action"],
  [{ lifecycle: "idle", human_input: false, failure: "none" }, "idle"],
  [{ lifecycle: "stopped", human_input: false, failure: "none" }, "stopped"],
  [{ lifecycle: "idle", human_input: false, failure: "failed" }, "failed"],
  [undefined, "unknown"],
];

describe("RunStateBadge", () => {
  it.each(cases)("renders the composite visual %s", (status, want) => {
    const { container } = render(<RunStateBadge status={status} />);
    const badge = screen.getByLabelText(`status: ${statusLabel(want)}`);
    expect(badge.className).toContain(`run-state-${want}`);
    expect(badge.textContent).toContain(want);
    expect(container.querySelector(`.status-icon--${want}`)).not.toBeNull();
  });

  it("renders bell + ! without a spinner", () => {
    const status: SessionStatus = {
      lifecycle: "active",
      human_input: true,
      failure: "none",
      signal: { kind: "bell", epoch: "e1", sequence: 1 },
    };
    const { container } = render(<RunStateBadge status={status} />);
    expect(screen.getByLabelText("status: Action required; Notification")).toBeDefined();
    expect(container.querySelector(".status-icon--action")).not.toBeNull();
    expect(container.querySelector(".status-icon__signal")).not.toBeNull();
    expect(container.querySelector(".run-state-spinner")).toBeNull();
  });
});
