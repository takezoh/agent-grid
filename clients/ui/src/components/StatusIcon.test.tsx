import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { SessionStatus } from "../wire/server";
import { StatusIcon, isActiveStatus, normalizeStatus } from "./StatusIcon";

const status = (
  lifecycle: SessionStatus["lifecycle"],
  human_input = false,
  failure: SessionStatus["failure"] = "none",
): SessionStatus => ({ lifecycle, human_input, failure });

describe("normalizeStatus", () => {
  it.each([
    [status("active"), "running"],
    [status("active", true), "action"],
    [status("idle"), "idle"],
    [status("stopped"), "stopped"],
    [status("idle", false, "failed"), "failed"],
  ] as const)("maps the composite status by shared precedence", (raw, want) => {
    expect(normalizeStatus(raw)).toBe(want);
  });

  it("does not fall back when the composite status is absent", () => {
    expect(normalizeStatus(undefined)).toBe("unknown");
  });

  it("human input suppresses the active spinner", () => {
    expect(normalizeStatus(status("active", true))).toBe("action");
  });
});

describe("StatusIcon", () => {
  it.each(["running", "action", "idle", "stopped", "failed", "unknown"] as const)(
    "renders the %s visual",
    (kind) => {
      const { container } = render(<StatusIcon status={kind} />);
      expect(container.querySelector(`.status-icon--${kind}`)).not.toBeNull();
      expect(container.querySelectorAll("svg").length).toBe(1);
    },
  );

  it("only the running visual is active", () => {
    expect(isActiveStatus("running")).toBe(true);
    for (const kind of ["action", "idle", "stopped", "failed", "unknown"] as const) {
      expect(isActiveStatus(kind)).toBe(false);
    }
  });

  it("renders ! for human input", () => {
    const { container } = render(<StatusIcon status="action" />);
    expect(container.querySelector(".status-icon__action")?.textContent).toBe("!");
  });

  it("renders bell independently of !", () => {
    const { container } = render(<StatusIcon status="action" signal />);
    expect(container.querySelector(".status-icon__action")).not.toBeNull();
    expect(container.querySelector(".status-icon__signal")?.textContent).toBe("🔔");
  });

  it("puts the spinner class only on running", () => {
    const running = render(
      <StatusIcon status="running" activeClass="spinner" inactiveClass="static" />,
    ).container;
    const action = render(
      <StatusIcon status="action" activeClass="spinner" inactiveClass="static" />,
    ).container;
    expect(running.querySelector(".spinner")).not.toBeNull();
    expect(action.querySelector(".spinner")).toBeNull();
    expect(action.querySelector(".static")).not.toBeNull();
  });
});
