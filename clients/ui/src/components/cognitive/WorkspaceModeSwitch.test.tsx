import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
  WorkspaceModeSwitch,
  parseWorkspaceUiMode,
  validateWorkspaceUiMode,
} from "./WorkspaceModeSwitch";

describe("WorkspaceModeSwitch", () => {
  it("renders both modes as a radiogroup and reflects the current value", () => {
    render(<WorkspaceModeSwitch value="classic" onChange={() => {}} />);
    const group = screen.getByRole("radiogroup", { name: "Workspace mode" });
    expect(group).not.toBeNull();
    expect(screen.getByRole("radio", { name: "Classic" }).getAttribute("aria-checked")).toBe(
      "true",
    );
    expect(screen.getByRole("radio", { name: "Cognitive" }).getAttribute("aria-checked")).toBe(
      "false",
    );
  });

  it("fires onChange with the selected mode", () => {
    const onChange = vi.fn();
    render(<WorkspaceModeSwitch value="classic" onChange={onChange} />);

    fireEvent.click(screen.getByRole("radio", { name: "Cognitive" }));

    expect(onChange).toHaveBeenCalledWith("cognitive");
  });

  it("validates persisted values strictly", () => {
    expect(validateWorkspaceUiMode(parseWorkspaceUiMode("classic"))).toBe("classic");
    expect(validateWorkspaceUiMode(parseWorkspaceUiMode("cognitive"))).toBe("cognitive");
    expect(validateWorkspaceUiMode(parseWorkspaceUiMode("garbage"))).toBeNull();
  });
});
