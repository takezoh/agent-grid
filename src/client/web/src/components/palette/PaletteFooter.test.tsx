// PaletteFooter tests — FR-026 (footer context + hints). The worktree toggle
// moved to the ParamSelectPhase actions row; the footer carries no chrome
// buttons besides Back.

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePaletteStore } from "../../store/palette";
import type { ActiveContextSnapshot } from "../../store/palette_active_context";
import { PaletteFooter } from "./PaletteFooter";

function renderFooter(overrides: Partial<React.ComponentProps<typeof PaletteFooter>> = {}) {
  const onBack = vi.fn();
  const onClose = vi.fn();
  const utils = render(
    <PaletteFooter
      phase="toolSelect"
      snapshot={{ kind: "none" }}
      flashSeq={0}
      statusText={null}
      submitting={false}
      composing={false}
      onBack={onBack}
      onClose={onClose}
      {...overrides}
    />,
  );
  return { onBack, onClose, ...utils };
}

describe("PaletteFooter", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    usePaletteStore.setState({ flashSeq: 0 });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders context label for kind=none", () => {
    renderFooter({ snapshot: { kind: "none" } });
    expect(screen.getByTestId("palette-active-context").textContent).toContain("No active session");
  });

  it("renders projBase / sid8 for kind=resolved", () => {
    const snap: ActiveContextSnapshot = {
      kind: "resolved",
      projBase: "bar",
      sid8: "abcd1234",
      fullPath: "/home/foo/bar",
      fullSessionId: "abcd1234efgh",
    };
    renderFooter({ snapshot: snap });
    const el = screen.getByTestId("palette-active-context");
    expect(el.textContent).toContain("bar");
    expect(el.textContent).toContain("abcd1234");
    expect(el.getAttribute("title")).toBe("/home/foo/bar\nabcd1234efgh");
  });

  it("adds flash class when flashSeq changes (600ms)", () => {
    const { rerender } = renderFooter({ snapshot: { kind: "none" }, flashSeq: 1 });
    rerender(
      <PaletteFooter
        phase="toolSelect"
        snapshot={{ kind: "none" }}
        flashSeq={2}
        statusText={null}
        submitting={false}
        composing={false}
        onBack={() => {}}
        onClose={() => {}}
      />,
    );
    const el = screen.getByTestId("palette-active-context");
    expect(el.className).toContain("palette-footer__context--flash");
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(el.className).not.toContain("palette-footer__context--flash");
  });

  it("renders no worktree chip and no close button (options live in the param phase)", () => {
    renderFooter();
    expect(document.querySelector("[data-toggle='worktree']")).toBeNull();
    expect(screen.queryByTestId("palette-close")).toBeNull();
  });

  it("shows back button only in paramSelect phase", () => {
    const { unmount } = renderFooter({ phase: "toolSelect" });
    expect(screen.queryByTestId("palette-back")).toBeNull();
    unmount();
    renderFooter({ phase: "paramSelect" });
    expect(screen.getByTestId("palette-back")).toBeDefined();
  });
});
