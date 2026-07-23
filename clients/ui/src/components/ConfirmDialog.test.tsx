// ConfirmDialog.test.tsx — open/close, scrim, Esc, focus restore, pending state.

import { fireEvent, render, screen } from "@testing-library/react";
import { useRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { ConfirmDialog } from "./ConfirmDialog";

function Harness({
  open,
  onCancel,
  onConfirm,
  pending = false,
}: {
  open: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  pending?: boolean;
}) {
  const openerRef = useRef<HTMLButtonElement>(null);
  return (
    <>
      <button ref={openerRef} type="button">
        opener
      </button>
      <ConfirmDialog
        open={open}
        title="セッションを終了"
        body="「test」を終了します。"
        confirmLabel="終了する"
        cancelLabel="キャンセル"
        destructive
        pending={pending}
        pendingLabel="終了中…"
        onConfirm={onConfirm}
        onCancel={onCancel}
        openerRef={openerRef}
      />
    </>
  );
}

describe("ConfirmDialog — basic render & action", () => {
  it("open=false なら何も render しない", () => {
    const { container } = render(<Harness open={false} onCancel={vi.fn()} onConfirm={vi.fn()} />);
    // opener button だけ存在.
    expect(container.querySelectorAll("button").length).toBe(1);
  });

  it("open=true で title / body / 2 button が render される", () => {
    render(<Harness open={true} onCancel={vi.fn()} onConfirm={vi.fn()} />);
    expect(screen.getByText("セッションを終了")).toBeTruthy();
    expect(screen.getByText("「test」を終了します。")).toBeTruthy();
    expect(screen.getByText("キャンセル")).toBeTruthy();
    expect(screen.getByText("終了する")).toBeTruthy();
  });

  it("Cancel button で onCancel", () => {
    const onCancel = vi.fn();
    render(<Harness open={true} onCancel={onCancel} onConfirm={vi.fn()} />);
    fireEvent.click(screen.getByText("キャンセル"));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("Confirm button で onConfirm", () => {
    const onConfirm = vi.fn();
    render(<Harness open={true} onCancel={vi.fn()} onConfirm={onConfirm} />);
    fireEvent.click(screen.getByText("終了する"));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});

describe("ConfirmDialog — dismiss paths", () => {
  it("scrim click で onCancel", () => {
    const onCancel = vi.fn();
    render(<Harness open={true} onCancel={onCancel} onConfirm={vi.fn()} />);
    const scrim = screen.getByLabelText("Close dialog");
    fireEvent.click(scrim);
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("Esc で onCancel", () => {
    const onCancel = vi.fn();
    render(<Harness open={true} onCancel={onCancel} onConfirm={vi.fn()} />);
    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});

describe("ConfirmDialog — pending state", () => {
  it("pending=true で両 button が disabled, confirm ラベルが pendingLabel", () => {
    render(<Harness open={true} onCancel={vi.fn()} onConfirm={vi.fn()} pending={true} />);
    const cancel = screen.getByText("キャンセル") as HTMLButtonElement;
    const confirm = screen.getByText("終了中…") as HTMLButtonElement;
    expect(cancel.disabled).toBe(true);
    expect(confirm.disabled).toBe(true);
  });

  it("pending=true で Esc は onCancel を呼ばない (cancel 不可)", () => {
    const onCancel = vi.fn();
    render(<Harness open={true} onCancel={onCancel} onConfirm={vi.fn()} pending={true} />);
    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onCancel).not.toHaveBeenCalled();
  });
});

describe("ConfirmDialog — focus", () => {
  it("open 時に cancel button に focus が当たる (誤押下防止)", () => {
    render(<Harness open={true} onCancel={vi.fn()} onConfirm={vi.fn()} />);
    expect(document.activeElement?.textContent).toBe("キャンセル");
  });
});
