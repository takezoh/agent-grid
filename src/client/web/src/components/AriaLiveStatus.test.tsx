// AriaLiveStatus.test.tsx — the terminal's single visually-hidden aria-live slot
// (ADR 0073, role-separated from the palette per ADR 0057).

import { act, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AnnouncerProvider, useAnnouncer } from "../hooks/useAnnouncer";
import { AriaLiveStatus } from "./AriaLiveStatus";

let capturedAnnounce: ((text: string) => void) | null = null;
function Capture() {
  capturedAnnounce = useAnnouncer().announce;
  return null;
}

function renderWithProvider() {
  return render(
    <AnnouncerProvider>
      <AriaLiveStatus />
      <Capture />
    </AnnouncerProvider>,
  );
}

describe("AriaLiveStatus", () => {
  it("renders exactly one aria-live='polite' region that starts empty", () => {
    const { container } = renderWithProvider();
    const live = screen.getByTestId("terminal-aria-live");
    expect(live.getAttribute("aria-live")).toBe("polite");
    expect(live.getAttribute("aria-atomic")).toBe("true");
    expect(live.textContent).toBe("");
    // Role separation (ADR 0057): one aria-live element, and it carries no role.
    expect(container.querySelectorAll("[aria-live]")).toHaveLength(1);
    expect(live.hasAttribute("role")).toBe(false);
  });

  it("displays announced text via useAnnouncer", () => {
    renderWithProvider();
    const live = screen.getByTestId("terminal-aria-live");
    act(() => capturedAnnounce?.("閲覧モードに戻りました"));
    expect(live.textContent).toBe("閲覧モードに戻りました");
  });
});
