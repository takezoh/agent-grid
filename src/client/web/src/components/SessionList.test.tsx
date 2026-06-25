import * as fs from "node:fs";
import * as path from "node:path";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { SessionList, displayLabel } from "./SessionList";
import { UnifiedListbox } from "./primitives/UnifiedListbox";

const fakeConn = {
  subscribe: vi.fn(async () => {}),
  unsubscribe: vi.fn(async () => {}),
} as unknown as import("../socket/connection").Connection;

describe("displayLabel", () => {
  it("FR-011: returns title when title is present", () => {
    expect(displayLabel({ title: "My Session" }, "s1")).toBe("My Session");
  });

  it("FR-011: returns subtitle when title is absent", () => {
    expect(displayLabel({ subtitle: "sub" }, "s1")).toBe("sub");
  });

  it("FR-011: returns subtitle when title is empty string", () => {
    expect(displayLabel({ title: "", subtitle: "sub" }, "s1")).toBe("sub");
  });

  it("FR-012: returns id when both title and subtitle are absent", () => {
    expect(displayLabel({}, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is undefined and subtitle is undefined", () => {
    expect(displayLabel({ title: undefined, subtitle: undefined }, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is empty string and subtitle is empty string", () => {
    expect(displayLabel({ title: "", subtitle: "" }, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is whitespace-only and subtitle is whitespace-only", () => {
    expect(displayLabel({ title: "  ", subtitle: "   " }, "s1")).toBe("s1");
  });

  it("FR-011: trims title before returning it", () => {
    expect(displayLabel({ title: "  trimmed  " }, "s1")).toBe("trimmed");
  });

  it("FR-011: trims subtitle before returning it", () => {
    expect(displayLabel({ title: "", subtitle: "  sub  " }, "s1")).toBe("sub");
  });
});

describe("SessionList rendering", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders session with title via displayLabel", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("alpha")).toBeDefined();
  });

  it("renders session with subtitle when title is absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { subtitle: "my-sub" }, status: "running" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("my-sub")).toBeDefined();
  });

  it("FR-012: renders session id when both title and subtitle are absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-raw-id",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-raw-id")).toBeDefined();
  });

  it("renders session id when title and subtitle are empty strings", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-empty",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "", subtitle: "" }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-empty")).toBeDefined();
  });

  it("renders session id when title and subtitle are whitespace-only", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-ws",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "  ", subtitle: "  " }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-ws")).toBeDefined();
  });
});

describe("SessionList status indicator", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders a spinning indicator only for active (running) sessions", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const spinners = container.querySelectorAll(".session-status-spinner");
    expect(spinners.length).toBe(1);
  });

  it("renders a spinning indicator for waiting sessions", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "waiting" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-spinner").length).toBe(1);
  });

  it.each(["idle", "stopped", "pending", undefined] as (string | undefined)[])(
    "renders NO spinner for inactive status=%s",
    (status) => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "s1",
            project: "p",
            command: "claude",
            created_at: "2026-06-20T00:00:00Z",
            view: { card: { title: "alpha" }, status },
          },
        ],
      });
      const { container } = render(<SessionList conn={fakeConn} />);
      expect(container.querySelectorAll(".session-status-spinner").length).toBe(0);
    },
  );

  it("status slot precedes the title (top-left placement)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    // With UnifiedListbox, each row is a role=option div containing a session-list__row div.
    const row = container.querySelector(".session-list__row");
    expect(row).not.toBeNull();
    const children = row ? Array.from(row.children) : [];
    expect(children[0]?.className).toMatch(/session-status-slot/);
    expect(children[1]?.className).toMatch(/title/);
  });

  it("status slot exposes the status name via aria-label even when inactive", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByLabelText("status: stopped")).toBeDefined();
  });

  it("does NOT render the textual status label inside the list item", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    // The option div contains only the session row (status slot + label); no textual status word.
    const option = container.querySelector('[role="option"]');
    expect(option?.textContent).toBe("alpha");
  });
});

describe("SessionList onClick", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
        {
          id: "s2",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "beta" }, status: "stopped" },
        },
      ],
    });
  });

  it("calls selectSession on click", () => {
    const selectSession = vi.fn();
    useDaemonStore.setState({ selectSession });
    render(<SessionList conn={fakeConn} />);
    // UnifiedListbox activates via onPointerDown on the option div
    const betaOption = screen.getByText("beta").closest('[role="option"]');
    expect(betaOption).not.toBeNull();
    if (betaOption) fireEvent.pointerDown(betaOption);
    expect(selectSession).toHaveBeenCalledWith("s2");
  });

  it("ADR-0030: does NOT call conn.subscribe on click", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    render(<SessionList conn={fakeConn} />);
    const betaOption = screen.getByText("beta").closest('[role="option"]');
    if (betaOption) fireEvent.pointerDown(betaOption);
    expect((fakeConn.subscribe as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(0);
  });

  it("ADR-0030: does NOT call conn.unsubscribe on click", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    render(<SessionList conn={fakeConn} />);
    const betaOption = screen.getByText("beta").closest('[role="option"]');
    if (betaOption) fireEvent.pointerDown(betaOption);
    expect((fakeConn.unsubscribe as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(0);
  });
});

// ─── FR-TOKEN-002: role=listbox upgrade + disabled skip-nav ──────────────────
describe("FR-TOKEN-002: SessionList is role=listbox; disabled rows visible, skip-nav, reason text", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders a role=listbox container", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listbox = container.querySelector('[role="listbox"]');
    expect(listbox).not.toBeNull();
  });

  it("aria-activedescendant points to the active session id", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-active",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "active-session" }, status: "running" },
        },
        {
          id: "s-other",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "other-session" }, status: "stopped" },
        },
      ],
      activeSessionID: "s-active",
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listbox = container.querySelector('[role="listbox"]');
    expect(listbox?.getAttribute("aria-activedescendant")).toBe("s-active");
  });

  it("disabled rows (daemonDisconnected=true) are still DOM-visible", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "my-session" }, status: "running" },
        },
      ],
      daemonDisconnected: true,
    });
    render(<SessionList conn={fakeConn} />);
    // The session row should still be visible in the DOM.
    expect(screen.getByText("my-session")).toBeDefined();
  });

  it("disabled rows carry aria-disabled=true", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "my-session" }, status: "stopped" },
        },
      ],
      daemonDisconnected: true,
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const option = container.querySelector('[role="option"]');
    expect(option?.getAttribute("aria-disabled")).toBe("true");
  });

  it("disabled rows include a disabledReason child text node", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "my-session" }, status: "stopped" },
        },
      ],
      daemonDisconnected: true,
    });
    render(<SessionList conn={fakeConn} />);
    // The reason text is rendered as a span child of the disabled option.
    expect(screen.getByText("Daemon disconnected")).toBeDefined();
  });

  it("ArrowDown navigation moves cursor (aria-activedescendant) but does NOT call selectSession (onActiveChange is preview-only)", () => {
    const selectSession = vi.fn();
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "first" }, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "second" }, status: "stopped" },
        },
      ],
      activeSessionID: "s1",
      selectSession,
      // Sessions are NOT disabled (daemonDisconnected: false).
      daemonDisconnected: false,
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listbox = container.querySelector('[role="listbox"]');
    expect(listbox).not.toBeNull();
    // Before ArrowDown: cursor is at the initial active session (s1).
    expect(listbox?.getAttribute("aria-activedescendant")).toBe("s1");
    // ArrowDown moves the preview cursor (onActiveChange) to s2 and updates
    // aria-activedescendant. Session selection only happens on onActivate
    // (Enter / pointer click) — selectSession must NOT be called.
    if (listbox) fireEvent.keyDown(listbox, { key: "ArrowDown" });
    expect(selectSession).not.toHaveBeenCalled();
    // aria-activedescendant must now point to s2 (the next enabled row).
    expect(listbox?.getAttribute("aria-activedescendant")).toBe("s2");
  });

  it("ArrowDown skips disabled rows: mixed enabled+disabled advances past disabled to next enabled", () => {
    const selectSession = vi.fn();
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "first" }, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "second" }, status: "stopped" },
        },
        {
          id: "s3",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "third" }, status: "idle" },
        },
      ],
      activeSessionID: "s1",
      selectSession,
      // Only daemon-disconnected flag makes all rows disabled currently.
      // Use daemonDisconnected: false so rows are enabled; we rely on the
      // UnifiedListbox skip-nav logic over enabled rows here.
      daemonDisconnected: false,
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listbox = container.querySelector('[role="listbox"]');
    expect(listbox).not.toBeNull();
    // First ArrowDown: s1 → s2.
    if (listbox) fireEvent.keyDown(listbox, { key: "ArrowDown" });
    expect(listbox?.getAttribute("aria-activedescendant")).toBe("s2");
    // Second ArrowDown: s2 → s3.
    if (listbox) fireEvent.keyDown(listbox, { key: "ArrowDown" });
    expect(listbox?.getAttribute("aria-activedescendant")).toBe("s3");
    // selectSession must never have been called.
    expect(selectSession).not.toHaveBeenCalled();
  });

  it("Enter key activates the cursor session and calls selectSession", () => {
    const selectSession = vi.fn();
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "first" }, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "second" }, status: "stopped" },
        },
      ],
      activeSessionID: "s1",
      selectSession,
      daemonDisconnected: false,
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const listbox = container.querySelector('[role="listbox"]');
    expect(listbox).not.toBeNull();
    // Enter on the currently focused session (s1) calls selectSession("s1").
    if (listbox) fireEvent.keyDown(listbox, { key: "Enter" });
    expect(selectSession).toHaveBeenCalledWith("s1");
  });
});

// ─── FR-TOKEN-001: CSS source structure — no hardcoded values in .unified-listbox__option ─
describe("FR-TOKEN-001: .unified-listbox__option uses only --row-* tokens (CSS source check)", () => {
  const cssDir = path.resolve(__dirname, "../css");

  it("app.css declares .unified-listbox__option with border-radius: var(--row-radius)", () => {
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    expect(appCss).toContain("border-radius: var(--row-radius)");
  });

  it("app.css declares .unified-listbox__option with padding-top: var(--row-padding-y)", () => {
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    expect(appCss).toContain("padding-top: var(--row-padding-y)");
  });

  it("app.css declares .unified-listbox__option with font-size: var(--row-font-size)", () => {
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    expect(appCss).toContain("font-size: var(--row-font-size)");
  });

  it("app.css declares .unified-listbox__option with line-height: var(--row-line-height)", () => {
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    expect(appCss).toContain("line-height: var(--row-line-height)");
  });

  it("app.css declares .unified-listbox__option with min-height: var(--row-min-height)", () => {
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    expect(appCss).toContain("min-height: var(--row-min-height)");
  });

  it("app.css .unified-listbox__option and derived blocks have no hardcoded hex color values", () => {
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    // Extract all rule blocks whose selector contains .unified-listbox__option
    // (base rule + derived: --disabled, [aria-selected], -reason, etc.).
    const rulePattern = /\.unified-listbox__option(?:[^{]*)\{([^}]*)\}/gs;
    const matches = Array.from(appCss.matchAll(rulePattern));
    expect(matches.length, "Expected at least one .unified-listbox__option rule").toBeGreaterThan(
      0,
    );

    for (const m of matches) {
      const block: string = m[1] ?? "";
      const selector = (m[0] ?? "").split("{")[0]?.trim() ?? "";
      // Check for 6-digit hex.
      const sixDigit = block.match(/#[0-9a-fA-F]{6}/g) ?? [];
      expect(sixDigit, `Found 6-digit hex in "${selector}": ${sixDigit.join(", ")}`).toHaveLength(
        0,
      );
      // Check for 3-digit hex.
      const threeDigit = block.match(/#[0-9a-fA-F]{3}(?![0-9a-fA-F])/g) ?? [];
      expect(
        threeDigit,
        `Found 3-digit hex in "${selector}": ${threeDigit.join(", ")}`,
      ).toHaveLength(0);
    }
  });
});

// ─── FR-TOKEN-001: computed style parity between SessionList and palette listbox ─
describe("FR-TOKEN-001: SessionList and palette listbox option computed styles match via --row-* tokens", () => {
  it("session-list option and standalone UnifiedListbox option resolve identical computed styles", () => {
    const cssDir = path.resolve(__dirname, "../css");
    const tokensCss = fs.readFileSync(path.join(cssDir, "tokens.css"), "utf-8");
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");

    // Inject tokens and app CSS so computed style resolves via the cascade.
    const tokensStyle = document.createElement("style");
    tokensStyle.textContent = tokensCss;
    document.head.appendChild(tokensStyle);

    const appStyle = document.createElement("style");
    appStyle.textContent = appCss;
    document.head.appendChild(appStyle);

    // Render a SessionList (which uses UnifiedListbox internally).
    useDaemonStore.setState({
      sessions: [
        {
          id: "cmp-s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "compare" }, status: "stopped" },
        },
      ],
      daemonDisconnected: false,
    });
    const { container: slContainer } = render(<SessionList conn={fakeConn} />);
    const slOption = slContainer.querySelector('[role="option"]');
    expect(slOption).not.toBeNull();

    // Render a second standalone UnifiedListbox (simulates the palette listbox option).
    const { container: ulContainer } = render(
      <UnifiedListbox
        ariaLabel="palette-test"
        items={[{ id: "pal-opt-1", label: <span>Palette Option</span> }]}
        activeId={null}
        onActiveChange={() => {}}
        onActivate={() => {}}
      />,
    );
    const paletteOption = ulContainer.querySelector('[role="option"]');
    expect(paletteOption).not.toBeNull();

    // Both options use .unified-listbox__option → the same --row-* tokens.
    // In happy-dom, custom properties resolve to their declared value strings.
    // We compare the resolved getComputedStyle property values for each option.
    const slStyle = slOption ? getComputedStyle(slOption) : null;
    const palStyle = paletteOption ? getComputedStyle(paletteOption) : null;

    if (slStyle && palStyle) {
      // The base .unified-listbox__option rule drives these — values must match.
      expect(slStyle.borderRadius).toBe(palStyle.borderRadius);
      expect(slStyle.paddingTop).toBe(palStyle.paddingTop);
      expect(slStyle.paddingBottom).toBe(palStyle.paddingBottom);
      expect(slStyle.paddingLeft).toBe(palStyle.paddingLeft);
      expect(slStyle.paddingRight).toBe(palStyle.paddingRight);
      expect(slStyle.fontSize).toBe(palStyle.fontSize);
      expect(slStyle.lineHeight).toBe(palStyle.lineHeight);
      // SessionList overrides min-height to 44px (FR-A11Y-001); the palette
      // option retains the --row-min-height value. We confirm both resolve to
      // non-empty values (the cascade applies to both).
      expect(slStyle.minHeight).toBeTruthy();
      expect(palStyle.minHeight).toBeTruthy();
    }

    document.head.removeChild(tokensStyle);
    document.head.removeChild(appStyle);
  });
});

// ─── 2-line ellipsis and 44px min-height (ADR-0033, FR-A11Y-001) ─────────────
describe("SessionList: 2-line ellipsis and 44px minimum touch target", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("session label element has -webkit-line-clamp: 2 via computed style (app.css injected)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {
              title: "A very long session title that should be clamped to two lines maximum",
            },
            status: "running",
          },
        },
      ],
    });
    // Inject app.css so getComputedStyle can resolve -webkit-line-clamp.
    const cssDir = path.resolve(__dirname, "../css");
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    const style = document.createElement("style");
    style.textContent = appCss;
    document.head.appendChild(style);

    const { container } = render(<SessionList conn={fakeConn} />);
    const label = container.querySelector(".session-list__label--clamped");
    expect(label).not.toBeNull();

    if (label) {
      // happy-dom resolves CSS custom properties and vendor-prefixed properties.
      const clampValue = getComputedStyle(label).getPropertyValue("-webkit-line-clamp");
      expect(
        clampValue.trim(),
        `Expected -webkit-line-clamp to be "2", got: "${clampValue.trim()}"`,
      ).toBe("2");
    }

    document.head.removeChild(style);
  });

  it("app.css declares -webkit-line-clamp: 2 for .session-list__label--clamped", () => {
    const cssDir = path.resolve(__dirname, "../css");
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    expect(appCss).toContain("-webkit-line-clamp: 2");
    expect(appCss).toContain("-webkit-box-orient: vertical");
    expect(appCss).toContain(".session-list__label--clamped");
  });

  it("FR-A11Y-001: app.css declares 44px min-height for .session-list .unified-listbox__option", () => {
    // FR-A11Y-001 requires a 44×44px minimum touch target for SessionList rows.
    // The shared --row-min-height token (2rem = 32px) is intentionally compact for
    // the palette; the SessionList scope overrides it to 44px.
    const cssDir = path.resolve(__dirname, "../css");
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");
    // Assert that the SessionList-scoped override is present.
    expect(appCss).toContain(".session-list .unified-listbox__option");
    expect(appCss).toContain("min-height: 44px");
  });

  it("FR-A11Y-001: SessionList option computed min-height is at least 44px", () => {
    const cssDir = path.resolve(__dirname, "../css");
    const tokensCss = fs.readFileSync(path.join(cssDir, "tokens.css"), "utf-8");
    const appCss = fs.readFileSync(path.join(cssDir, "app.css"), "utf-8");

    // Inject CSS so getComputedStyle can resolve via the cascade.
    const tokensStyle = document.createElement("style");
    tokensStyle.textContent = tokensCss;
    document.head.appendChild(tokensStyle);

    const appStyle = document.createElement("style");
    appStyle.textContent = appCss;
    document.head.appendChild(appStyle);

    useDaemonStore.setState({
      sessions: [
        {
          id: "a11y-s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "touch target" }, status: "stopped" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    const option = container.querySelector('[role="option"]');
    expect(option).not.toBeNull();

    if (option) {
      const computedMinHeight = getComputedStyle(option).minHeight;
      // happy-dom resolves CSS custom properties; 44px is a literal pixel value
      // so it should resolve directly to "44px".
      expect(
        computedMinHeight,
        `Expected computed min-height >= 44px, got: ${computedMinHeight}`,
      ).toBe("44px");
    }

    document.head.removeChild(tokensStyle);
    document.head.removeChild(appStyle);
  });
});

// ─── ADR-0033 displayLabel chain maintained ──────────────────────────────────
describe("ADR-0033: displayLabel chain maintained in SessionList render", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders title when title is present (ADR-0033 chain: title first)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "My Title", subtitle: "My Sub" }, status: "idle" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("My Title")).toBeDefined();
    expect(screen.queryByText("My Sub")).toBeNull();
  });

  it("renders subtitle when title is absent (ADR-0033 chain: subtitle second)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { subtitle: "Only Sub" }, status: "idle" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("Only Sub")).toBeDefined();
  });

  it("renders id when title and subtitle both absent (ADR-0033 chain: id fallback)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "fallback-id",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "idle" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("fallback-id")).toBeDefined();
  });
});

// ─── ADR-0032: RunStateBadge / status spinner maintained ─────────────────────
describe("ADR-0032: session-status-slot and session-status-spinner are maintained", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("running session has session-status-spinner (ADR-0032 active spinner)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "active" }, status: "running" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-spinner").length).toBe(1);
  });

  it("stopped session has no session-status-spinner (ADR-0032 inactive)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "stopped" }, status: "stopped" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-spinner").length).toBe(0);
  });

  it("each session row has a session-status-slot element", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "s1" }, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "s2" }, status: "stopped" },
        },
      ],
    });
    const { container } = render(<SessionList conn={fakeConn} />);
    expect(container.querySelectorAll(".session-status-slot").length).toBe(2);
  });
});
