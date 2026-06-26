// SessionTerminateButton.test.tsx — stopPropagation 契約 / aria-label / opener 引数 /
// disabled pass-through を検証. UnifiedListbox の row activation 抑制が load-bearing
// なので、特に pointerdown と click 両方の stopPropagation を pin する.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SessionTerminateButton } from "./SessionTerminateButton";

describe("SessionTerminateButton — basic render", () => {
  it("× アイコン + 「<label>」を終了 の aria-label を持つ <button> を render", () => {
    render(
      <SessionTerminateButton
        sessionId="s1"
        sessionLabel="My Session"
        onRequestTerminate={vi.fn()}
      />,
    );
    const btn = screen.getByRole("button");
    expect(btn.tagName).toBe("BUTTON");
    expect(btn.getAttribute("aria-label")).toBe("「My Session」を終了");
    expect(btn.textContent).toContain("×");
  });

  it("disabled prop が button に伝わる", () => {
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
  it("click で onRequestTerminate(sessionId, opener) が呼ばれる", () => {
    const onRequest = vi.fn();
    render(
      <SessionTerminateButton sessionId="s42" sessionLabel="X" onRequestTerminate={onRequest} />,
    );
    const btn = screen.getByRole("button");
    fireEvent.click(btn);
    expect(onRequest).toHaveBeenCalledTimes(1);
    // 第 1 引数 sessionId, 第 2 引数 opener element (currentTarget = button 自身)
    expect(onRequest.mock.calls[0]?.[0]).toBe("s42");
    expect(onRequest.mock.calls[0]?.[1]).toBe(btn);
  });
});

describe("SessionTerminateButton — UnifiedListbox 親への伝播抑制", () => {
  it("click は parent に伝播しない (stopPropagation 契約)", () => {
    const parentClick = vi.fn();
    const onRequest = vi.fn();
    render(
      // biome-ignore lint/a11y/useKeyWithClickEvents: test harness for stopPropagation pin
      <div onClick={parentClick}>
        <SessionTerminateButton sessionId="s1" sessionLabel="X" onRequestTerminate={onRequest} />
      </div>,
    );
    fireEvent.click(screen.getByRole("button"));
    expect(onRequest).toHaveBeenCalledTimes(1);
    expect(parentClick).not.toHaveBeenCalled();
  });

  it("pointerdown は parent に伝播しない (UnifiedListbox の row activate 抑制)", () => {
    const parentPointerDown = vi.fn();
    render(
      <div onPointerDown={parentPointerDown}>
        <SessionTerminateButton sessionId="s1" sessionLabel="X" onRequestTerminate={vi.fn()} />
      </div>,
    );
    const btn = screen.getByRole("button");
    fireEvent.pointerDown(btn);
    expect(parentPointerDown).not.toHaveBeenCalled();
  });
});
