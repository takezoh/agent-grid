import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RunStateBadge } from "./RunStateBadge";

describe("RunStateBadge", () => {
  // FR-010: existing textContent / aria-label contract preserved for all statuses
  it.each([
    ["running", "running"],
    ["waiting", "waiting"],
    ["idle", "idle"],
    ["stopped", "stopped"],
    ["pending", "pending"],
    [undefined, "unknown"],
  ] as [string | undefined, string][])(
    "status=%s renders class run-state-%s and text %s",
    (status, want) => {
      render(<RunStateBadge status={status} />);
      const el = screen.getByLabelText(/status:/);
      expect(el.className).toContain(`run-state-${want}`);
      expect(el.textContent).toBe(want);
    },
  );

  // FR-009: running and waiting render aria-hidden spinner
  it.each(["running", "waiting"] as string[])(
    "active status=%s renders one aria-hidden .run-state-spinner",
    (status) => {
      const { container } = render(<RunStateBadge status={status} />);
      const spinners = container.querySelectorAll("[aria-hidden=true].run-state-spinner");
      expect(spinners.length).toBe(1);
    },
  );

  // FR-009 negative: idle / stopped / pending / unknown render no spinner
  it.each(["idle", "stopped", "pending", undefined] as (string | undefined)[])(
    "inactive status=%s renders no .run-state-spinner",
    (status) => {
      const { container } = render(<RunStateBadge status={status} />);
      const spinners = container.querySelectorAll(".run-state-spinner");
      expect(spinners.length).toBe(0);
    },
  );
});
