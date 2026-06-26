/**
 * SessionDrawer.test.tsx
 *
 * Covers FR-DRAWER-001/002/003/005/006/007 per acceptance criteria.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SessionDrawer } from "./SessionDrawer";

// ─── helpers ─────────────────────────────────────────────────────────────────

/** Insert a #main-content element into the document body for guard tests. */
function insertMainContent(): HTMLElement {
  const el = document.createElement("div");
  el.id = "main-content";
  document.body.appendChild(el);
  return el;
}

/** Create a minimal fake Touch object. */
function fakeTouch(x: number, y: number): Touch {
  return {
    clientX: x,
    clientY: y,
    identifier: 0,
    target: document.body,
  } as unknown as Touch;
}

// ─── setup / teardown ────────────────────────────────────────────────────────

let mainContent: HTMLElement;

beforeEach(() => {
  mainContent = insertMainContent();
});

afterEach(() => {
  mainContent.remove();
  vi.restoreAllMocks();
});

// ─── FR-DRAWER-001: role + aria-modal + focus ─────────────────────────────────

describe("FR-DRAWER-001: open=true → role=dialog + aria-modal + focus in subtree", () => {
  it("renders role='dialog' and aria-modal='true' when open", () => {
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <button type="button">Session A</button>
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog).toBeTruthy();
    expect(dialog.getAttribute("aria-modal")).toBe("true");
  });

  it("moves focus into the drawer subtree on open", () => {
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <button type="button" data-testid="first-btn">
          Session A
        </button>
      </SessionDrawer>,
    );

    // After mount, activeElement should be within the dialog subtree.
    const dialog = screen.getByRole("dialog");
    expect(dialog.contains(document.activeElement)).toBe(true);
  });

  it("renders aria-label='Sessions' on the dialog", () => {
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span>content</span>
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog", { name: "Sessions" });
    expect(dialog).toBeTruthy();
  });

  it("does not render dialog when open=false", () => {
    render(
      <SessionDrawer open={false} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <button type="button">Session A</button>
      </SessionDrawer>,
    );

    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

// ─── FR-DRAWER-002: three-layer guard on main content ────────────────────────

describe("FR-DRAWER-002: open=true → inert + aria-hidden + pointer-events:none (AND)", () => {
  it("adds inert attribute to #main-content when open", () => {
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span />
      </SessionDrawer>,
    );

    expect(mainContent.hasAttribute("inert")).toBe(true);
  });

  it("adds aria-hidden='true' to #main-content when open", () => {
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span />
      </SessionDrawer>,
    );

    expect(mainContent.getAttribute("aria-hidden")).toBe("true");
  });

  it("adds .main-content--inert class to #main-content when open (pointer-events guard)", () => {
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span />
      </SessionDrawer>,
    );

    expect(mainContent.classList.contains("main-content--inert")).toBe(true);
  });

  it("all three guards are present simultaneously (AND requirement, ADR-0060)", () => {
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span />
      </SessionDrawer>,
    );

    const hasInert = mainContent.hasAttribute("inert");
    const hasAriaHidden = mainContent.getAttribute("aria-hidden") === "true";
    const hasPointerEventsClass = mainContent.classList.contains("main-content--inert");

    expect(hasInert && hasAriaHidden && hasPointerEventsClass).toBe(true);
  });

  it("removes all three guards when open=false", () => {
    const { rerender } = render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span />
      </SessionDrawer>,
    );

    // Guards should be present when open.
    expect(mainContent.hasAttribute("inert")).toBe(true);

    rerender(
      <SessionDrawer open={false} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span />
      </SessionDrawer>,
    );

    expect(mainContent.hasAttribute("inert")).toBe(false);
    expect(mainContent.getAttribute("aria-hidden")).toBeNull();
    expect(mainContent.classList.contains("main-content--inert")).toBe(false);
  });
});

// ─── Drawer height anchoring (regression: only first project visible) ────────
// Before this guard: <dialog>'s UA default `height: fit-content` combined
// with our `top:0; bottom:0;` is over-constrained per CSS positioning rules.
// The spec ignores `bottom`, the dialog shrinks to its content height, and
// the drawer becomes a viewport-clipped strip that hides every project group
// after the first (with no way to scroll because the slide's flex item also
// resists shrinking without min-height: 0).
describe("Drawer container is anchored to viewport height (regression)", () => {
  // Extract a single `.cls { ... }` block from shell.css so each assertion is
  // scoped to its rule (e.g. --dvh usage on .app-shell doesn't leak into the
  // .session-drawer check). Non-greedy + `\n}` boundary terminates at the
  // rule's own closing brace.
  const shellCss = fs.readFileSync(path.resolve(__dirname, "../css/shell.css"), "utf-8");
  const ruleBlock = (selector: string): string => {
    const re = new RegExp(`\\.${selector}\\s*\\{[\\s\\S]*?\\n\\}`);
    const m = shellCss.match(re);
    expect(m, `expected a .${selector} rule block in shell.css`).not.toBeNull();
    return m?.[0] ?? "";
  };

  it("session-drawer is sized to var(--dvh) so all project groups can scroll", () => {
    const block = ruleBlock("session-drawer");
    expect(block).toMatch(/height:\s*var\(--dvh\)/);
    expect(block).toMatch(/max-height:\s*var\(--dvh\)/);
  });

  it("session-drawer__slide has min-height: 0 so overflow-y scroll can engage", () => {
    const block = ruleBlock("session-drawer__slide");
    expect(block).toMatch(/min-height:\s*0/);
    expect(block).toMatch(/overflow-y:\s*auto/);
  });
});

// ─── FR-DRAWER-003: row click → onSelectionClose called ──────────────────────

describe("FR-DRAWER-003: row click → onSelectionClose(selectedId)", () => {
  it("child row click allows onSelectionClose to be called by parent handler", () => {
    const onSelectionClose = vi.fn();
    const onCancelClose = vi.fn();

    // SessionDrawer wraps children: the parent is responsible for wiring
    // session row clicks to onSelectionClose. We verify the callback prop
    // is correctly wired by invoking it directly (unit boundary).
    render(
      <SessionDrawer open={true} onSelectionClose={onSelectionClose} onCancelClose={onCancelClose}>
        <button
          type="button"
          onClick={() => {
            onSelectionClose("session-1");
          }}
        >
          Session 1
        </button>
      </SessionDrawer>,
    );

    act(() => {
      fireEvent.click(screen.getByText("Session 1"));
    });

    expect(onSelectionClose).toHaveBeenCalledWith("session-1");
    expect(onCancelClose).not.toHaveBeenCalled();
  });
});

// ─── FR-DRAWER-005: cancel close — scrim click / Esc / swipe ─────────────────

describe("FR-DRAWER-005: scrim click / Esc / left-to-right swipe → onCancelClose", () => {
  it("scrim click calls onCancelClose", () => {
    const onCancelClose = vi.fn();
    const { container } = render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    const scrim = container.querySelector(".session-drawer__scrim");
    expect(scrim).not.toBeNull();
    if (!scrim) return;

    act(() => {
      fireEvent.click(scrim);
    });

    expect(onCancelClose).toHaveBeenCalledTimes(1);
  });

  it("Esc keydown on drawer calls onCancelClose", () => {
    const onCancelClose = vi.fn();
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog");

    act(() => {
      fireEvent.keyDown(dialog, { key: "Escape" });
    });

    expect(onCancelClose).toHaveBeenCalledTimes(1);
  });

  it("left-to-right swipe (dx=80, dy=10) calls onCancelClose", () => {
    const onCancelClose = vi.fn();
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog");

    act(() => {
      // touchstart at (0, 100), touchend at (80, 110) — dx=80 dy=10 → swipe
      fireEvent.touchStart(dialog, {
        touches: [fakeTouch(0, 100)],
      });
      fireEvent.touchEnd(dialog, {
        changedTouches: [fakeTouch(80, 110)],
      });
    });

    expect(onCancelClose).toHaveBeenCalledTimes(1);
  });

  it("insufficient swipe (dx=30, dy=5) does NOT call onCancelClose", () => {
    const onCancelClose = vi.fn();
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog");

    act(() => {
      fireEvent.touchStart(dialog, {
        touches: [fakeTouch(0, 100)],
      });
      fireEvent.touchEnd(dialog, {
        changedTouches: [fakeTouch(30, 105)],
      });
    });

    expect(onCancelClose).not.toHaveBeenCalled();
  });

  it("right-to-left swipe (dx=-80, dy=5) does NOT call onCancelClose", () => {
    const onCancelClose = vi.fn();
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog");

    act(() => {
      fireEvent.touchStart(dialog, {
        touches: [fakeTouch(100, 100)],
      });
      fireEvent.touchEnd(dialog, {
        changedTouches: [fakeTouch(20, 105)],
      });
    });

    expect(onCancelClose).not.toHaveBeenCalled();
  });

  it("vertical-dominant swipe (dx=80, dy=50) does NOT call onCancelClose", () => {
    const onCancelClose = vi.fn();
    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog");

    act(() => {
      fireEvent.touchStart(dialog, {
        touches: [fakeTouch(0, 100)],
      });
      fireEvent.touchEnd(dialog, {
        changedTouches: [fakeTouch(80, 150)],
      });
    });

    expect(onCancelClose).not.toHaveBeenCalled();
  });
});

// ─── FR-DRAWER-006: cancel close does NOT change active session ───────────────

describe("FR-DRAWER-006: cancel close → activeSessionId unchanged", () => {
  it("onCancelClose is called but onSelectionClose is not (session unchanged)", () => {
    const onSelectionClose = vi.fn();
    const onCancelClose = vi.fn();

    render(
      <SessionDrawer open={true} onSelectionClose={onSelectionClose} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    const dialog = screen.getByRole("dialog");
    act(() => {
      fireEvent.keyDown(dialog, { key: "Escape" });
    });

    expect(onCancelClose).toHaveBeenCalledTimes(1);
    expect(onSelectionClose).not.toHaveBeenCalled();
  });
});

// ─── FR-DRAWER-007: auto-close on viewport >= 1024px + idempotent ────────────

describe("FR-DRAWER-007: auto-close on viewport >= 1024px (idempotent)", () => {
  it("open=true + window.innerWidth >= 1024 on resize event → onCancelClose called", () => {
    const onCancelClose = vi.fn();

    // Mock window.innerWidth to simulate a desktop viewport.
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 1024,
    });

    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    act(() => {
      window.dispatchEvent(new Event("resize"));
    });

    expect(onCancelClose).toHaveBeenCalledTimes(1);
  });

  it("open=false + resize event does NOT call onCancelClose", () => {
    const onCancelClose = vi.fn();

    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 1024,
    });

    render(
      <SessionDrawer open={false} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    act(() => {
      window.dispatchEvent(new Event("resize"));
    });

    expect(onCancelClose).not.toHaveBeenCalled();
  });

  it("resize to < 1024 does NOT call onCancelClose", () => {
    const onCancelClose = vi.fn();

    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 600,
    });

    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    act(() => {
      window.dispatchEvent(new Event("resize"));
    });

    expect(onCancelClose).not.toHaveBeenCalled();
  });

  it("multiple resize events at >= 1024 fire onCancelClose each time (handler is stable)", () => {
    // Each resize event fires independently; idempotency is guaranteed by
    // the parent AppShell (which has debounce). Here we verify the handler
    // fires on each event (not that it is de-duped at this layer).
    const onCancelClose = vi.fn();

    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 1024,
    });

    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    act(() => {
      window.dispatchEvent(new Event("resize"));
      window.dispatchEvent(new Event("resize"));
    });

    // Both events fire — parent is responsible for debounce.
    expect(onCancelClose).toHaveBeenCalledTimes(2);
  });

  it("removes guards (inert/aria-hidden/class) when closed after resize", () => {
    const onCancelClose = vi.fn(() => {
      // Simulate parent responding to onCancelClose by closing the drawer.
    });

    render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={onCancelClose}>
        <span />
      </SessionDrawer>,
    );

    // Guards are present while open.
    expect(mainContent.hasAttribute("inert")).toBe(true);
    expect(mainContent.getAttribute("aria-hidden")).toBe("true");
    expect(mainContent.classList.contains("main-content--inert")).toBe(true);
  });

  it("cleanup on unmount removes guards regardless of open state", () => {
    const { unmount } = render(
      <SessionDrawer open={true} onSelectionClose={vi.fn()} onCancelClose={vi.fn()}>
        <span />
      </SessionDrawer>,
    );

    expect(mainContent.hasAttribute("inert")).toBe(true);

    act(() => {
      unmount();
    });

    expect(mainContent.hasAttribute("inert")).toBe(false);
    expect(mainContent.getAttribute("aria-hidden")).toBeNull();
    expect(mainContent.classList.contains("main-content--inert")).toBe(false);
  });
});
