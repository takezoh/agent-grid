// SessionTerminateButton.test.tsx — DriverViewPanel header に置く outline danger
// button の契約を pin する. 旧 SessionRow 配置時代の stopPropagation 契約は
// 移設に伴い不要となったため削除した (header は clickable な親を持たない).

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SessionTerminateButton } from "./SessionTerminateButton";

describe("SessionTerminateButton — basic render", () => {
  it("「終了」テキスト + stop icon + 「<label>」を終了 の aria-label を持つ <button> を render", () => {
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
    // Visible text is the explicit "終了" label rather than a bare ✕ glyph.
    expect(btn.textContent).toContain("終了");
    // Stop-square SVG glyph is rendered (aria-hidden) for visual affordance.
    const glyph = btn.querySelector(".session-terminate-button__glyph");
    expect(glyph).not.toBeNull();
    expect(glyph?.getAttribute("aria-hidden")).toBe("true");
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
