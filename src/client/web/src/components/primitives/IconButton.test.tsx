// IconButton.test.tsx — ADR 0075, WCAG 2.5.5 (44×44) / 4.1.2 (name).
//
// Discriminating coverage of the UAC-024 counterexample ("32px + aria-label
// 省略"): a non-empty aria-label is enforced (render throws on empty), and the
// 44×44 contract is asserted at the CSS layer (happy-dom has no layout engine,
// per the chunk-01 harness note, so size lives in icon-button.css and is read
// from disk here). The focus-steal-suppression mechanism (pointerdown →
// preventDefault) is verified by spying on the dispatched event.

import { promises as fs } from "node:fs";
import path from "node:path";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { IconButton } from "./IconButton";

describe("IconButton — element + a11y contract", () => {
  it("renders a real <button type='button'>", () => {
    render(<IconButton aria-label="Close panel">×</IconButton>);
    const btn = screen.getByRole("button", { name: "Close panel" });
    expect(btn.tagName).toBe("BUTTON");
    expect(btn.getAttribute("type")).toBe("button");
  });

  it("exposes a non-empty accessible name from aria-label", () => {
    render(<IconButton aria-label="Toggle keyboard">⌨</IconButton>);
    const btn = screen.getByRole("button");
    expect(btn.getAttribute("aria-label")).toBe("Toggle keyboard");
    expect((btn.getAttribute("aria-label") ?? "").trim().length).toBeGreaterThan(0);
  });

  it("UAC-024 counterexample: an empty aria-label is rejected (throws on render)", () => {
    // Suppress React's error-boundary console noise for this expected throw.
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<IconButton aria-label="">×</IconButton>)).toThrow(/non-empty aria-label/i);
    expect(() => render(<IconButton aria-label={"   "}>×</IconButton>)).toThrow(
      /non-empty aria-label/i,
    );
    spy.mockRestore();
  });

  it("reflects aria-pressed when provided, and omits it otherwise", () => {
    const { rerender } = render(
      <IconButton aria-label="Mode" aria-pressed={true}>
        m
      </IconButton>,
    );
    expect(screen.getByRole("button").getAttribute("aria-pressed")).toBe("true");

    rerender(
      <IconButton aria-label="Mode" aria-pressed={false}>
        m
      </IconButton>,
    );
    expect(screen.getByRole("button").getAttribute("aria-pressed")).toBe("false");

    rerender(<IconButton aria-label="Mode">m</IconButton>);
    expect(screen.getByRole("button").hasAttribute("aria-pressed")).toBe(false);
  });
});

describe("IconButton — pointerdown focus-steal suppression (FR-MOB-FAB-PD-001)", () => {
  it("calls preventDefault on the pointerdown event", () => {
    render(<IconButton aria-label="FAB">f</IconButton>);
    const btn = screen.getByRole("button");

    const ev = new Event("pointerdown", { bubbles: true, cancelable: true });
    const preventSpy = vi.spyOn(ev, "preventDefault");
    btn.dispatchEvent(ev);

    expect(preventSpy).toHaveBeenCalled();
    expect(ev.defaultPrevented).toBe(true);
  });

  it("still forwards a caller-supplied onPointerDown handler", () => {
    const onPointerDown = vi.fn();
    render(
      <IconButton aria-label="FAB" onPointerDown={onPointerDown}>
        f
      </IconButton>,
    );
    const btn = screen.getByRole("button");
    fireEvent.pointerDown(btn);
    expect(onPointerDown).toHaveBeenCalledTimes(1);
  });

  it("invokes onClick when activated", () => {
    const onClick = vi.fn();
    render(
      <IconButton aria-label="FAB" onClick={onClick}>
        f
      </IconButton>,
    );
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("merges a caller className with the icon-button base class", () => {
    render(
      <IconButton aria-label="FAB" className="keyboard-fab">
        f
      </IconButton>,
    );
    const btn = screen.getByRole("button");
    expect(btn.classList.contains("icon-button")).toBe(true);
    expect(btn.classList.contains("keyboard-fab")).toBe(true);
  });
});

describe("IconButton — 44×44 touch target lives in icon-button.css (WCAG 2.5.5)", () => {
  it("icon-button.css declares min-width and min-height of 44px (not 32px)", async () => {
    const cssPath = path.join(
      import.meta.dirname ?? __dirname,
      "..",
      "..",
      "css",
      "icon-button.css",
    );
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/min-width:\s*44px/);
    expect(source).toMatch(/min-height:\s*44px/);
    // UAC-024 counterexample guard: the 32px target must not be the size.
    expect(source).not.toMatch(/min-(?:width|height):\s*32px/);
  });

  it("icon-button.css uses theme tokens (--accent / --surface-*), not ad-hoc colors", async () => {
    const cssPath = path.join(
      import.meta.dirname ?? __dirname,
      "..",
      "..",
      "css",
      "icon-button.css",
    );
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/var\(--accent\)/);
    expect(source).toMatch(/var\(--surface-/);
  });
});
