import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SessionTerminateButton } from "./SessionTerminateButton";

describe("SessionTerminateButton — basic render (FR-024 / UAC-011)", () => {
  it("renders icon-only stop button with session label in aria-label", () => {
    render(
      <SessionTerminateButton
        sessionId="s1"
        sessionLabel="My Session"
        onRequestTerminate={vi.fn()}
      />,
    );
    const btn = screen.getByRole("button");
    expect(btn.tagName).toBe("BUTTON");
    expect(btn.getAttribute("aria-label")).toBe('Stop "My Session"');
    expect(btn.className).toContain("session-terminate-button");
    expect(btn.textContent?.trim()).toBe("");
  });

  it("disabled prop is forwarded to the button", () => {
    render(
      <SessionTerminateButton
        sessionId="s1"
        sessionLabel="X"
        onRequestTerminate={vi.fn()}
        disabled
      />,
    );
    const btn = screen.getByRole("button") as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });
});

describe("SessionTerminateButton — onRequestTerminate contract", () => {
  it("click calls onRequestTerminate(sessionId, opener)", () => {
    const onRequest = vi.fn();
    render(
      <SessionTerminateButton sessionId="s42" sessionLabel="X" onRequestTerminate={onRequest} />,
    );
    const btn = screen.getByRole("button");
    fireEvent.click(btn);
    expect(onRequest).toHaveBeenCalledTimes(1);
    expect(onRequest.mock.calls[0]?.[0]).toBe("s42");
    expect(onRequest.mock.calls[0]?.[1]).toBe(btn);
  });
});
