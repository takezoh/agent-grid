// KeyboardFAB.test.tsx — FR-MOB-FAB-001 / FR-MOB-FAB-PD-001, ADR 0068 / 0075.
// Discriminating against:
//   - UAC-024: an unlabeled / non-button control (IconButton enforces a real
//     <button> + non-empty aria-label).
//   - UAC-026: aria-pressed / aria-label must track useInputMode state.
//   - counterexample B: pointerdown must preventDefault so the FAB never steals
//     focus from the helper textarea.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { KeyboardFAB } from "./KeyboardFAB";

describe("KeyboardFAB — a11y + toggle sync", () => {
  it("UAC-024: renders a real <button> with a non-empty aria-label", () => {
    render(<KeyboardFAB active={false} onToggle={() => {}} />);
    const btn = screen.getByRole("button");
    expect(btn.tagName).toBe("BUTTON");
    expect((btn.getAttribute("aria-label") ?? "").trim().length).toBeGreaterThan(0);
  });

  it("UAC-026: aria-label / aria-pressed track the active state", () => {
    const { rerender } = render(<KeyboardFAB active={false} onToggle={() => {}} />);
    const closed = screen.getByRole("button");
    expect(closed.getAttribute("aria-label")).toBe("Open keyboard");
    expect(closed.getAttribute("aria-pressed")).toBe("false");

    rerender(<KeyboardFAB active={true} onToggle={() => {}} />);
    const open = screen.getByRole("button");
    expect(open.getAttribute("aria-label")).toBe("Close keyboard");
    expect(open.getAttribute("aria-pressed")).toBe("true");
  });

  it("UAC-003/004: click invokes onToggle", () => {
    const onToggle = vi.fn();
    render(<KeyboardFAB active={false} onToggle={onToggle} />);
    fireEvent.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("FR-MOB-FAB-PD-001: pointerdown is preventDefault'd (no focus-steal)", () => {
    render(<KeyboardFAB active={false} onToggle={() => {}} />);
    const btn = screen.getByRole("button");
    const ev = new Event("pointerdown", { bubbles: true, cancelable: true });
    btn.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
  });

  it("carries data-overlay so the host interceptor excludes it from outside-tap", () => {
    render(<KeyboardFAB active={true} onToggle={() => {}} />);
    const btn = screen.getByRole("button");
    expect(btn.hasAttribute("data-overlay")).toBe(true);
  });
});
