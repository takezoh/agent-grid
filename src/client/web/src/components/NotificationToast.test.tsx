import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useNotificationsStore } from "../store/notifications";
import { NotificationToast } from "./NotificationToast";

describe("NotificationToast", () => {
  beforeEach(() => {
    useNotificationsStore.getState().clear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // ── existing behaviour ────────────────────────────────────────────────────

  it("TestRendersUpTo3Items: shows only the latest 3 of 5 items", () => {
    for (let i = 1; i <= 5; i++) {
      useNotificationsStore.getState().add({ level: "info", message: `msg${i}` });
    }
    render(<NotificationToast />);
    // Latest 3 are msg3, msg4, msg5
    expect(screen.queryByText("msg1")).toBeNull();
    expect(screen.queryByText("msg2")).toBeNull();
    expect(screen.getByText("msg3")).toBeTruthy();
    expect(screen.getByText("msg4")).toBeTruthy();
    expect(screen.getByText("msg5")).toBeTruthy();
  });

  it("TestClickDismisses: clicking a toast removes it from the store", () => {
    useNotificationsStore.getState().add({ level: "info", message: "click-me" });
    render(<NotificationToast />);
    const toast = screen.getByText("click-me");
    fireEvent.click(toast);
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  it("TestAutoDismissAfter5s: auto-dismisses after 5000ms", () => {
    vi.useFakeTimers();
    useNotificationsStore.getState().add({ level: "info", message: "auto" });
    render(<NotificationToast />);
    expect(screen.getByText("auto")).toBeTruthy();
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  // ── FR-TOAST-001: single aria-live container, no per-item aria-live ───────

  it("FR-TOAST-001: container has aria-live=polite and role=status", () => {
    useNotificationsStore.getState().add({ level: "info", message: "aria-test" });
    render(<NotificationToast />);

    // Container has both aria-live and role=status
    const container = screen.getByRole("status");
    expect(container.getAttribute("aria-live")).toBe("polite");
    expect(container.getAttribute("aria-label")).toBe("notifications");
  });

  it("FR-TOAST-001: individual toast items do NOT have aria-live (prevents double announcement)", () => {
    useNotificationsStore.getState().add({ level: "info", message: "no-live" });
    render(<NotificationToast />);

    // Items have class notification-toast__item — none should carry aria-live
    const container = screen.getByRole("status");
    const items = container.querySelectorAll(".notification-toast__item");
    expect(items.length).toBeGreaterThan(0);
    for (const item of items) {
      expect(item.getAttribute("aria-live")).toBeNull();
    }
  });

  // ── FR-TOAST-002: no inline color, token CSS class per level ─────────────

  it("FR-TOAST-002: toast items have no inline background color (uses CSS class)", () => {
    useNotificationsStore.getState().add({ level: "info", message: "info-msg" });
    useNotificationsStore.getState().add({ level: "warn", message: "warn-msg" });
    useNotificationsStore.getState().add({ level: "error", message: "error-msg" });
    render(<NotificationToast />);

    const container = screen.getByRole("status");
    const items = container.querySelectorAll(".notification-toast__item");
    for (const item of items) {
      // No inline background color set — color comes from CSS token class
      expect((item as HTMLElement).style.background).toBe("");
    }
  });

  it("FR-TOAST-002: toast items carry the correct type CSS class per level", () => {
    useNotificationsStore.getState().add({ level: "info", message: "info-msg" });
    useNotificationsStore.getState().add({ level: "warn", message: "warn-msg" });
    useNotificationsStore.getState().add({ level: "error", message: "error-msg" });
    render(<NotificationToast />);

    const container = screen.getByRole("status");
    const infoItem = Array.from(container.querySelectorAll(".notification-toast__item")).find(
      (el) => el.textContent?.includes("info-msg"),
    );
    const warnItem = Array.from(container.querySelectorAll(".notification-toast__item")).find(
      (el) => el.textContent?.includes("warn-msg"),
    );
    const errorItem = Array.from(container.querySelectorAll(".notification-toast__item")).find(
      (el) => el.textContent?.includes("error-msg"),
    );

    expect(infoItem?.classList.contains("notification-toast__item--info")).toBe(true);
    expect(warnItem?.classList.contains("notification-toast__item--warn")).toBe(true);
    expect(errorItem?.classList.contains("notification-toast__item--error")).toBe(true);
  });

  it("FR-TOAST-002: app.css declares bottom safe-area rule for mobile viewport", () => {
    // happy-dom does not process CSS files; verify the source rule exists.
    const cssPath = resolve(__dirname, "../css/app.css");
    const css = readFileSync(cssPath, "utf-8");

    // Mobile media query with env(safe-area-inset-bottom)
    expect(css).toMatch(/@media\s*\(max-width:\s*767px\)/);
    expect(css).toMatch(/env\(safe-area-inset-bottom\)/);
    // Desktop fallback
    expect(css).toMatch(/@media\s*\(min-width:\s*768px\)/);
    // Token variable references — no hardcoded hex colors for bg
    expect(css).toMatch(/--toast-bg-info/);
    expect(css).toMatch(/--toast-bg-success/);
    expect(css).toMatch(/--toast-bg-warn/);
    expect(css).toMatch(/--toast-bg-error/);
  });

  // ── FR-TOAST-003: undosnackbar slot exists; 3-stream independence ─────────

  it("FR-TOAST-003: container includes notification-toast__undosnackbar-slot", () => {
    render(<NotificationToast />);
    const container = screen.getByRole("status");
    const slot = container.querySelector(".notification-toast__undosnackbar-slot");
    expect(slot).not.toBeNull();
  });

  it("FR-TOAST-003: undosnackbar slot is a sibling of toast items (3-stream independent)", () => {
    useNotificationsStore.getState().add({ level: "info", message: "passive" });
    render(<NotificationToast />);

    const container = screen.getByRole("status");
    const children = Array.from(container.children);

    // At least one toast item and exactly one undosnackbar-slot as children
    const itemChildren = children.filter((el) => el.classList.contains("notification-toast__item"));
    const slotChildren = children.filter((el) =>
      el.classList.contains("notification-toast__undosnackbar-slot"),
    );

    expect(itemChildren.length).toBeGreaterThan(0);
    expect(slotChildren).toHaveLength(1);

    // Verify they are siblings (both direct children of container)
    for (const item of itemChildren) {
      expect(item.parentElement).toBe(container);
    }
    const slotEl = slotChildren[0];
    expect(slotEl).toBeDefined();
    expect(slotEl?.parentElement).toBe(container);
  });

  // m1: FR-A11Y-001 44x44px floor — each toast item is the dismiss target on
  // tap, so the CSS contract must enforce min-height: 44px (width is already
  // 240px+). happy-dom cannot resolve layout, so we read app.css directly and
  // assert the literal min-height declaration on .notification-toast__item.
  it("FR-A11Y-001 (m1): .notification-toast__item declares min-height: 44px in app.css", () => {
    const cssPath = resolve(__dirname, "..", "css", "app.css");
    const css = readFileSync(cssPath, "utf-8");
    // Capture the .notification-toast__item rule block (no descendant selector
    // like '__item--info' should be matched).
    const blockMatch = css.match(/\.notification-toast__item \{([^}]+)\}/);
    expect(blockMatch).not.toBeNull();
    const block = blockMatch?.[1] ?? "";
    expect(block).toMatch(/min-height:\s*44px/);
    // width floor: the existing min-width: 240px already exceeds 44px.
    expect(block).toMatch(/min-width:\s*240px/);
  });

  // ── ariaHidden prop (additive — ADR 0063 non-breaking, FR-MOB-PINCH-004) ───
  // PinchIndicator reuses this primitive in a visual-only, screen-reader-silent
  // mode. The prop must be purely additive: default behaviour is unchanged
  // (covered by the cases above), and ariaHidden=true flips the container to an
  // aria-hidden surface that renders ONLY its children.

  it("ariaHidden=true: container is aria-hidden and carries no role=status / aria-live", () => {
    render(
      <NotificationToast ariaHidden>
        <span>22px</span>
      </NotificationToast>,
    );
    // Not announced: no status role at all.
    expect(screen.queryByRole("status")).toBeNull();
    const surface = document.querySelector(".notification-toast");
    expect(surface).not.toBeNull();
    expect(surface?.getAttribute("aria-hidden")).toBe("true");
    expect(surface?.getAttribute("aria-live")).toBeNull();
  });

  it("ariaHidden=true: renders children and does NOT duplicate the passive store items", () => {
    useNotificationsStore.getState().add({ level: "info", message: "passive-toast" });
    render(
      <NotificationToast ariaHidden>
        <span>22px</span>
      </NotificationToast>,
    );
    // The visual readout is shown…
    expect(screen.getByText("22px")).toBeTruthy();
    // …but passive store notifications are NOT mirrored into the indicator,
    // nor is the undosnackbar slot (visual-only surface).
    expect(screen.queryByText("passive-toast")).toBeNull();
    expect(document.querySelector(".notification-toast__undosnackbar-slot")).toBeNull();
  });

  it("default (no ariaHidden) still exposes role=status (additive, unchanged)", () => {
    render(<NotificationToast />);
    const container = screen.getByRole("status");
    expect(container.getAttribute("aria-hidden")).toBeNull();
    expect(container.getAttribute("aria-live")).toBe("polite");
  });
});
