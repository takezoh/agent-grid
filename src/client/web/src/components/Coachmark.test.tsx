// Coachmark.test.tsx — ADR 0072 surface contract (FR-MOB-COACH-001/002).
//
// The once-gate / persistence / timer live in useCoachmarkOnce (tested
// separately). Here we freeze the *surface* contract ADR 0072 (3) mandates:
//   - role='status' with NO explicit aria-live (the live region is AriaLiveStatus).
//   - data-overlay (host pointer interceptor treats the tap as an overlay tap).
//   - tap forwards to onDismiss.
//   - the rendered copy is the spec hint string.

import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Coachmark } from "./Coachmark";

// Spec hint copy, \u-escaped (ADR-0049 english-only source).
const HINT =
  "\u30BF\u30C3\u30D7\u3067\u5165\u529B / 2 \u672C\u6307\u3067\u6587\u5B57\u30B5\u30A4\u30BA";

describe("Coachmark — ADR 0072 surface", () => {
  it("renders role='status' WITHOUT an aria-live attribute (orthogonal to AriaLiveStatus)", () => {
    const { container } = render(<Coachmark onDismiss={vi.fn()} />);
    const node = container.querySelector('[role="status"]') as HTMLElement;
    expect(node).not.toBeNull();
    // The terminal live region is AriaLiveStatus; the coachmark must not be one.
    expect(node.hasAttribute("aria-live")).toBe(false);
  });

  it("carries data-coachmark and data-overlay hooks", () => {
    const { container } = render(<Coachmark onDismiss={vi.fn()} />);
    expect(container.querySelector("[data-coachmark]")).not.toBeNull();
    expect(container.querySelector("[data-overlay]")).not.toBeNull();
  });

  it("renders the spec hint copy", () => {
    const { container } = render(<Coachmark onDismiss={vi.fn()} />);
    expect(container.textContent).toContain(HINT);
  });

  it("tap forwards to onDismiss", () => {
    const onDismiss = vi.fn();
    const { container } = render(<Coachmark onDismiss={onDismiss} />);
    fireEvent.click(container.querySelector("[data-coachmark]") as HTMLElement);
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
});
