import * as fs from "node:fs";
import * as path from "node:path";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { contrastRatio, parseColor } from "../util/contrast";
import type { View } from "../wire/server";
import { DriverViewPanel, resolveTagPillStyle } from "./DriverViewPanel";

function makeView(overrides: Partial<View> = {}): View {
  return {
    card: {},
    ...overrides,
  };
}

describe("DriverViewPanel", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders card title", () => {
    const view = makeView({ card: { title: "My Title" } });
    render(<DriverViewPanel view={view} />);
    expect(screen.getByText("My Title")).toBeTruthy();
  });

  it("renders only the title row — the legacy .driver-view-subtitle row never appears", () => {
    const view = makeView({ card: { title: "My Title" } });
    const { container } = render(<DriverViewPanel view={view} />);
    expect(screen.getByText("My Title")).toBeTruthy();
    expect(container.querySelector(".driver-view-subtitle")).toBeNull();
  });

  it("falls back to New Session in the header title slot when card.title is absent", () => {
    const view = makeView({ card: {} });
    const { container } = render(<DriverViewPanel view={view} />);
    expect(container.querySelector(".driver-view-title")?.textContent).toBe("New Session");
  });

  it("ADR-0076: preserves the full title text in the DOM (no JS truncation)", () => {
    const longTitle = "T".repeat(140);
    const view = makeView({ card: { title: longTitle } });
    render(<DriverViewPanel view={view} />);
    expect(screen.getByText(longTitle)).toBeTruthy();
  });

  it("ADR-0076: session-list.css clamps driver-view-title width with text-overflow", () => {
    const cssDir = path.resolve(__dirname, "../css");
    // The clamp declarations are co-located in session-list.css so view.css
    // stays under its 500-line FR-FRAMEWORK-001 cap. Cascade order is unaffected.
    const css = fs.readFileSync(path.join(cssDir, "session-list.css"), "utf-8");
    expect(css).toContain(".driver-view-title");
    expect(css).not.toContain(".driver-view-subtitle");
    expect(css).toMatch(/max-width:\s*100ch/);
    expect(css).toMatch(/text-overflow:\s*ellipsis/);
    expect(css).toMatch(/white-space:\s*nowrap/);
  });

  it("renders tags", () => {
    const view = makeView({
      card: {
        tags: [
          { text: "alpha", fg: "#fff" },
          { text: "beta", bg: "#333" },
        ],
      },
    });
    render(<DriverViewPanel view={view} />);
    expect(screen.getByText("alpha")).toBeTruthy();
    expect(screen.getByText("beta")).toBeTruthy();
  });

  it("renders header metadata pills for model and effort", () => {
    const view = makeView({ card: { title: "My Title" }, model: "gpt-5", effort: "high" });
    const { container } = render(<DriverViewPanel view={view} />);
    const row = container.querySelector(".driver-view-metadata");
    expect(row?.textContent).toContain("gpt-5");
    expect(row?.textContent).toContain("high");
  });

  it("hides missing metadata values in the header row", () => {
    const view = makeView({ card: { title: "My Title" }, model: "gpt-5" });
    const { container } = render(<DriverViewPanel view={view} />);
    const pills = [...container.querySelectorAll(".driver-view-meta-pill")].map((el) => el.textContent);
    expect(pills).toEqual(["gpt-5"]);
  });

  it("renders RunStateBadge for view.status", () => {
    const view = makeView({ status: "running" });
    render(<DriverViewPanel view={view} />);
    expect(screen.getByLabelText("status: running")).toBeTruthy();
  });

  it("renders status_line and ticking elapsed", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));

    const view = makeView({
      status_line: "Running task",
      status_changed_at: "2026-06-19T23:59:55Z",
    });
    render(<DriverViewPanel view={view} />);

    // Initial render: 5 seconds elapsed
    expect(screen.getByLabelText("elapsed").textContent).toBe("5s");

    // Advance 2 seconds — hook fires twice → elapsed becomes 7s
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByLabelText("elapsed").textContent).toBe("7s");
  });

  it("hides border row when all border fields are empty", () => {
    const view = makeView({ card: { title: "T" } });
    const { container } = render(<DriverViewPanel view={view} />);
    const borderRow = container.querySelector(".driver-view-border");
    expect(borderRow).toBeNull();
  });

  it("suppresses status_line when absent", () => {
    const view = makeView({ card: { title: "T" } });
    const { container } = render(<DriverViewPanel view={view} />);
    const footer = container.querySelector(".driver-view-footer");
    expect(footer).toBeNull();
  });
});

// ─── SessionTerminateButton placement (旧 SessionList row 配置からの移設) ──
describe("DriverViewPanel — terminate button placement", () => {
  it("sessionId と onRequestTerminate が両方与えられた時に終了ボタンを header に出す", () => {
    const view = makeView({ card: { title: "alpha" }, status: "running" });
    const { container } = render(
      <DriverViewPanel view={view} sessionId="s1" onRequestTerminate={vi.fn()} />,
    );
    const header = container.querySelector(".driver-view-header");
    expect(header).not.toBeNull();
    const btn = header?.querySelector(".session-terminate-button");
    expect(btn).not.toBeNull();
    // RunStateBadge と一緒の actions cluster に置く.
    const actions = header?.querySelector(".driver-view-actions");
    expect(actions).not.toBeNull();
    expect(actions?.querySelector(".run-state-badge")).not.toBeNull();
    expect(actions?.querySelector(".session-terminate-button")).not.toBeNull();
  });

  it("onRequestTerminate が未指定なら button を出さない (旧 API 互換)", () => {
    const view = makeView({ card: { title: "alpha" } });
    const { container } = render(<DriverViewPanel view={view} sessionId="s1" />);
    expect(container.querySelector(".session-terminate-button")).toBeNull();
  });

  it("sessionId が未指定なら button を出さない", () => {
    const view = makeView({ card: { title: "alpha" } });
    const { container } = render(<DriverViewPanel view={view} onRequestTerminate={vi.fn()} />);
    expect(container.querySelector(".session-terminate-button")).toBeNull();
  });

  it("click で onRequestTerminate(id, label, opener) が呼ばれる", () => {
    const onRequest = vi.fn();
    const view = makeView({ card: { title: "alpha" } });
    render(<DriverViewPanel view={view} sessionId="s-id-42" onRequestTerminate={onRequest} />);
    const btn = screen.getByRole("button", { name: "「alpha」を終了" });
    fireEvent.click(btn);
    expect(onRequest).toHaveBeenCalledTimes(1);
    expect(onRequest.mock.calls[0]?.[0]).toBe("s-id-42");
    expect(onRequest.mock.calls[0]?.[1]).toBe("alpha");
    expect(onRequest.mock.calls[0]?.[2]).toBe(btn);
  });

  it("card.title が空の時は 'New Session' placeholder を label として使う", () => {
    const onRequest = vi.fn();
    const view = makeView({ card: {} });
    render(<DriverViewPanel view={view} sessionId="s-empty" onRequestTerminate={onRequest} />);
    const btn = screen.getByRole("button", { name: "「New Session」を終了" });
    fireEvent.click(btn);
    expect(onRequest.mock.calls[0]?.[1]).toBe("New Session");
  });
});

describe("resolveTagPillStyle — unit (FR-TAGPILL-001)", () => {
  it("low-contrast fg=#777 bg=#fff: flips fg to #000 and adds border", () => {
    // #777 on #fff ratio ≈ 4.48 < 4.5 → should flip to black
    const style = resolveTagPillStyle("#777777", "#ffffff");
    expect(style.color).toBe("#000000");
    expect(style.backgroundColor).toBe("#ffffff");
    expect(style.border).toBe("1px solid currentColor");

    // Double-observation: verify the resulting ratio meets WCAG AA
    const actualFg = parseColor(style.color);
    const actualBg = parseColor(style.backgroundColor);
    if (!actualFg || !actualBg) throw new Error("parseColor returned null for resolved style");
    const ratio = contrastRatio(actualFg, actualBg);
    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });

  it("high-contrast fg=#000 bg=#fff: keeps original colors, no border", () => {
    // #000 on #fff ratio = 21.0 ≥ 4.5 → no change
    const style = resolveTagPillStyle("#000000", "#ffffff");
    expect(style.color).toBe("#000000");
    expect(style.backgroundColor).toBe("#ffffff");
    expect(style.border).toBeUndefined();
  });

  it("high-contrast fg=#fff bg=#000: keeps original colors, no border", () => {
    // #fff on #000 ratio = 21.0 ≥ 4.5 → no change
    const style = resolveTagPillStyle("#ffffff", "#000000");
    expect(style.color).toBe("#ffffff");
    expect(style.backgroundColor).toBe("#000000");
    expect(style.border).toBeUndefined();
  });

  it("low-contrast dark bg: flips to white fg", () => {
    // Dark bg — white should win over black
    const style = resolveTagPillStyle("#555555", "#111111");
    // #555 on #111: contrast ratio is low, white gives ratio ~10.7 vs black ~1.9
    expect(style.color).toBe("#ffffff");
    expect(style.border).toBe("1px solid currentColor");

    const actualFg = parseColor(style.color);
    const actualBg = parseColor(style.backgroundColor);
    if (!actualFg || !actualBg) throw new Error("parseColor returned null for resolved style");
    const ratio = contrastRatio(actualFg, actualBg);
    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });

  it("invalid fg falls back to token default, ratio still computed", () => {
    // Provide a valid bg so we can verify token-default fg is applied
    // token default fg = #e6e6e6 on #ffffff → ratio ≈ 1.26 < 4.5 → flip
    const style = resolveTagPillStyle("not-a-color", "#ffffff");
    // With invalid fg, parseColor returns null → uses TOKEN_DEFAULT_FG (#e6e6e6)
    // #e6e6e6 on #fff is low contrast → flips to black
    expect(style.color).toBe("#000000");
    // backgroundColor must be the valid bg input (not the invalid fg)
    expect(style.backgroundColor).toBe("#ffffff");
    expect(style.border).toBe("1px solid currentColor");
  });

  it("invalid bg falls back to token default", () => {
    // With invalid bg, uses TOKEN_DEFAULT_BG (#333)
    // #000 on #333 → ratio ≈ 3.24 < 4.5 → flip to white
    const style = resolveTagPillStyle("#000000", "bad-color");
    // black on dark-grey → low contrast → flip to white
    expect(style.color).toBe("#ffffff");
    // backgroundColor must be the token-default bg string (rgb(51,51,51)),
    // NOT the raw invalid 'bad-color' string that the browser would reject.
    expect(style.backgroundColor).toBe("rgb(51,51,51)");
    expect(style.border).toBe("1px solid currentColor");
  });
});

describe("TagPill DOM — FR-TAGPILL-001 computed style", () => {
  it("low-contrast pill has border and high-contrast fg in computed style", () => {
    const view = makeView({
      card: {
        tags: [{ text: "LowContrast", fg: "#777777", bg: "#ffffff" }],
      },
    });
    const { container } = render(<DriverViewPanel view={view} />);
    const pill = container.querySelector(".driver-tag-pill");
    expect(pill).not.toBeNull();

    // Check inline style properties applied to DOM element
    const el = pill as HTMLElement;
    // color should be black (#000 / rgb(0,0,0))
    expect(el.style.color).toBeTruthy();
    // border should be set (happy-dom normalises currentColor → currentcolor)
    expect(el.style.border.toLowerCase()).toBe("1px solid currentcolor");

    // Double-observation via contrast util
    const fgStr = el.style.color;
    const bgStr = el.style.backgroundColor;
    const fg = parseColor(fgStr) ?? { r: 0, g: 0, b: 0 };
    const bg = parseColor(bgStr) ?? { r: 255, g: 255, b: 255 };
    const ratio = contrastRatio(fg, bg);
    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });

  it("high-contrast pill has no border in DOM", () => {
    const view = makeView({
      card: {
        tags: [{ text: "HighContrast", fg: "#000000", bg: "#ffffff" }],
      },
    });
    const { container } = render(<DriverViewPanel view={view} />);
    const pill = container.querySelector(".driver-tag-pill") as HTMLElement;
    expect(pill).not.toBeNull();
    // border should not be set (empty string in happy-dom)
    expect(pill.style.border).toBeFalsy();
  });
});
