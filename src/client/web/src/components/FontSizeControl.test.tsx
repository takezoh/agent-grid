// FontSizeControl.test.tsx — FR-MOB-STEPPER-001, ADR 0070 / 0075, WCAG 2.5.5 / 4.1.2.
//
// Discriminating coverage of the UAC-020 counterexample ("fontSize is only
// reachable through the two-finger pinch handler, no non-pinch alternative"):
// the +/-/Reset buttons must EXIST in the DOM, be real role=button controls with
// non-empty aria-labels, inherit the 44×44 icon-button target, and actually drive
// ±2px / reset-to-14 plus a scheduleFit on every activate.
//
// The buttons are wired to a real useFontSize instance through a small harness so
// the ±2px math and scheduleFit fan-out are observed end-to-end (not just "a
// callback fired").

import { promises as fs } from "node:fs";
import path from "node:path";
import { fireEvent, render, screen } from "@testing-library/react";
import { type JSX, useMemo } from "react";
import { describe, expect, it, vi } from "vitest";
import { useFontSize } from "../hooks/useFontSize";
import type { StorageLike } from "../hooks/usePersistedValue";
import { FontSizeControl } from "./FontSizeControl";

function mapStorage(): StorageLike {
  const map = new Map<string, string>();
  return {
    getItem: (k) => (map.has(k) ? (map.get(k) as string) : null),
    setItem: (k, v) => {
      map.set(k, v);
    },
  };
}

function Harness({ scheduleFit }: { scheduleFit: () => void }): JSX.Element {
  const storage = useMemo(mapStorage, []);
  const fontSizeApi = useFontSize({ scheduleFit, storage });
  return (
    <FontSizeControl
      fontSize={fontSizeApi.fontSize}
      onIncrease={fontSizeApi.increase}
      onDecrease={fontSizeApi.decrease}
      onReset={() => fontSizeApi.reset(14)}
    />
  );
}

/** Open the disclosure popover by activating the "文字サイズ" trigger. */
function openPopover(): void {
  fireEvent.click(screen.getByRole("button", { name: "文字サイズ" }));
}

describe("FontSizeControl — disclosure popover a11y (UAC-020)", () => {
  it("the Aa trigger exposes the '文字サイズ' accessible name and is collapsed initially", () => {
    render(<Harness scheduleFit={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "文字サイズ" });
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    // Popover closed → no stepper buttons in the tree yet.
    expect(screen.queryByRole("button", { name: "文字を大きく" })).toBeNull();
  });

  it("UAC-020: opening the popover exposes +, -, Reset as role=button with non-empty aria-labels", () => {
    render(<Harness scheduleFit={vi.fn()} />);
    openPopover();

    for (const name of ["文字を大きく", "文字を小さく", "文字サイズを既定に戻す"]) {
      const btn = screen.getByRole("button", { name });
      expect(btn.tagName).toBe("BUTTON");
      expect((btn.getAttribute("aria-label") ?? "").trim().length).toBeGreaterThan(0);
      // 44×44 target is inherited from the IconButton primitive (icon-button.css).
      expect(btn.classList.contains("icon-button")).toBe(true);
    }
  });

  it("the stepper buttons inherit the 44×44 floor from icon-button.css (WCAG 2.5.5)", async () => {
    // happy-dom has no layout engine, so the size contract lives in CSS and is
    // read from disk (same pattern as IconButton.test).
    const cssPath = path.join(import.meta.dirname ?? __dirname, "..", "css", "icon-button.css");
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/min-width:\s*44px/);
    expect(source).toMatch(/min-height:\s*44px/);
    expect(source).not.toMatch(/min-(?:width|height):\s*32px/);
  });
});

describe("FontSizeControl — stepper semantics (FR-MOB-STEPPER-001)", () => {
  it("+ increases by 2px and schedules a fit", () => {
    const scheduleFit = vi.fn();
    render(<Harness scheduleFit={scheduleFit} />);
    openPopover();
    expect(screen.getByText("14px")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "文字を大きく" }));

    expect(screen.getByText("16px")).toBeTruthy();
    expect(scheduleFit).toHaveBeenCalledTimes(1);
  });

  it("- decreases by 2px and schedules a fit", () => {
    const scheduleFit = vi.fn();
    render(<Harness scheduleFit={scheduleFit} />);
    openPopover();

    fireEvent.click(screen.getByRole("button", { name: "文字を小さく" }));

    expect(screen.getByText("12px")).toBeTruthy();
    expect(scheduleFit).toHaveBeenCalledTimes(1);
  });

  it("Reset returns to 14px and schedules a fit", () => {
    const scheduleFit = vi.fn();
    render(<Harness scheduleFit={scheduleFit} />);
    openPopover();

    // Bump up twice → 18px, then Reset back to 14px.
    fireEvent.click(screen.getByRole("button", { name: "文字を大きく" }));
    fireEvent.click(screen.getByRole("button", { name: "文字を大きく" }));
    expect(screen.getByText("18px")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "文字サイズを既定に戻す" }));

    expect(screen.getByText("14px")).toBeTruthy();
    // 2 increases + 1 reset = 3 scheduleFit invocations.
    expect(scheduleFit).toHaveBeenCalledTimes(3);
  });
});
