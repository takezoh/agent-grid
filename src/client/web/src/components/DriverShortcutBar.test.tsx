// DriverShortcutBar.test.tsx — driver 別表示 / 未対応 driver hide /
// sendInput 呼び出し / data-overlay 属性 / inputActive gate のテスト.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BYTES_CTRL_C, BYTES_ESC, BYTES_SHIFT_TAB } from "../lib/driverShortcuts";
import { DriverShortcutBar } from "./DriverShortcutBar";

describe("DriverShortcutBar — visibility gates", () => {
  it("inputActive=false なら何も render しない", () => {
    const { container } = render(
      <DriverShortcutBar driver="claude" inputActive={false} sendInput={vi.fn()} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("driver が null / 未対応 (shell) なら何も render しない", () => {
    const { container: c1 } = render(
      <DriverShortcutBar driver={null} inputActive={true} sendInput={vi.fn()} />,
    );
    expect(c1.firstChild).toBeNull();

    const { container: c2 } = render(
      <DriverShortcutBar driver="shell" inputActive={true} sendInput={vi.fn()} />,
    );
    expect(c2.firstChild).toBeNull();
  });

  it.each(["claude", "codex"])("%s driver で 3 button を render する", (driver) => {
    render(<DriverShortcutBar driver={driver} inputActive={true} sendInput={vi.fn()} />);
    const bar = screen.getByRole("toolbar");
    expect(bar).toBeTruthy();
    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBe(3);
  });
});

describe("DriverShortcutBar — sendInput contract", () => {
  it("Mode tap で SHIFT_TAB bytes を sendInput に渡す", () => {
    const sendInput = vi.fn();
    render(<DriverShortcutBar driver="claude" inputActive={true} sendInput={sendInput} />);
    const [mode] = screen.getAllByRole("button");
    if (!mode) throw new Error("Mode button missing");
    fireEvent.click(mode);
    expect(sendInput).toHaveBeenCalledWith(BYTES_SHIFT_TAB);
  });

  it("Esc tap で ESC bytes を渡す", () => {
    const sendInput = vi.fn();
    render(<DriverShortcutBar driver="codex" inputActive={true} sendInput={sendInput} />);
    const [, esc] = screen.getAllByRole("button");
    if (!esc) throw new Error("Esc button missing");
    fireEvent.click(esc);
    expect(sendInput).toHaveBeenCalledWith(BYTES_ESC);
  });

  it("Ctrl-C tap で CTRL_C bytes を渡す", () => {
    const sendInput = vi.fn();
    render(<DriverShortcutBar driver="claude" inputActive={true} sendInput={sendInput} />);
    const [, , ctrlc] = screen.getAllByRole("button");
    if (!ctrlc) throw new Error("Ctrl-C button missing");
    fireEvent.click(ctrlc);
    expect(sendInput).toHaveBeenCalledWith(BYTES_CTRL_C);
  });
});

describe("DriverShortcutBar — host pointer interceptor 連携", () => {
  it("bar と各 button に data-overlay 属性を持つ (KeyboardFAB と同じ pattern)", () => {
    render(<DriverShortcutBar driver="claude" inputActive={true} sendInput={vi.fn()} />);
    const bar = screen.getByRole("toolbar");
    expect(bar.hasAttribute("data-overlay")).toBe(true);
    for (const btn of screen.getAllByRole("button")) {
      expect(btn.hasAttribute("data-overlay")).toBe(true);
    }
  });

  it("button の pointerdown は preventDefault される (focus-steal 防止)", () => {
    render(<DriverShortcutBar driver="claude" inputActive={true} sendInput={vi.fn()} />);
    const [btn] = screen.getAllByRole("button");
    if (!btn) throw new Error("button missing");
    const ev = new Event("pointerdown", { bubbles: true, cancelable: true });
    btn.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
  });
});
