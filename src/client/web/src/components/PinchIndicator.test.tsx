// PinchIndicator.test.tsx — FR-MOB-PINCH-004, ADR 0063 / 0070.
//
// Two load-bearing contracts:
//   1. NO new toast primitive (ADR 0063): PinchIndicator must reuse
//      NotificationToast with ariaHidden=true. NotificationToast is mocked so we
//      can assert, by props, that it is invoked with `ariaHidden === true` — a
//      bespoke <div> readout would fail this assertion.
//   2. Lifecycle: visible while pinching, shows the live fontSize, fades out ~800ms
//      after the pinch ends, and a tap fires the reset callback.

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PINCH_FADE_MS, PinchIndicator } from "./PinchIndicator";

// Record every props object NotificationToast is rendered with, and still render
// the children so the readout text is observable.
const toastProps: Array<{ ariaHidden?: boolean }> = [];
vi.mock("./NotificationToast", () => ({
  NotificationToast: (props: { ariaHidden?: boolean; children?: React.ReactNode }) => {
    toastProps.push({ ariaHidden: props.ariaHidden });
    return <div data-testid="mock-toast">{props.children}</div>;
  },
}));

describe("PinchIndicator — reuses NotificationToast (ADR 0063)", () => {
  beforeEach(() => {
    toastProps.length = 0;
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders the NotificationToast primitive with ariaHidden=true (props assertion)", () => {
    render(<PinchIndicator fontSize={20} active onReset={vi.fn()} />);
    expect(toastProps.length).toBeGreaterThan(0);
    // Every render of the primitive must be in the aria-hidden (visual-only) mode.
    for (const p of toastProps) {
      expect(p.ariaHidden).toBe(true);
    }
  });

  it("shows the live fontSize inside the reused primitive", () => {
    render(<PinchIndicator fontSize={20} active onReset={vi.fn()} />);
    const toast = screen.getByTestId("mock-toast");
    expect(toast.textContent).toContain("20px");
  });

  it("tapping the indicator fires onReset (caller wires reset(14)+scheduleFit)", () => {
    const onReset = vi.fn();
    render(<PinchIndicator fontSize={20} active onReset={onReset} />);
    fireEvent.click(screen.getByText("20px"));
    expect(onReset).toHaveBeenCalledTimes(1);
  });
});

describe("PinchIndicator — fade lifecycle (~800ms after touchend)", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("stays visible immediately after the pinch ends, then unmounts after the fade delay", () => {
    vi.useFakeTimers();
    const { rerender } = render(<PinchIndicator fontSize={18} active onReset={vi.fn()} />);
    expect(screen.queryByText("18px")).toBeTruthy();

    // touchend: active flips false — still shown right away (fading).
    rerender(<PinchIndicator fontSize={18} active={false} onReset={vi.fn()} />);
    expect(screen.queryByText("18px")).toBeTruthy();

    // After the fade delay it is gone.
    act(() => {
      vi.advanceTimersByTime(PINCH_FADE_MS);
    });
    expect(screen.queryByText("18px")).toBeNull();
  });

  it("does not render anything when inactive from the start", () => {
    vi.useFakeTimers();
    render(<PinchIndicator fontSize={14} active={false} onReset={vi.fn()} />);
    expect(screen.queryByText("14px")).toBeNull();
  });
});
