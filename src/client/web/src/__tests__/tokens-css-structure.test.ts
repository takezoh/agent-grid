/**
 * tokens-css-structure.test.ts
 *
 * FR-TOKEN-001: semantic CSS custom property hierarchy and --row-* sharing.
 * FR-FRAMEWORK-001: tokens.css / app.css / view.css file separation, each <= 500 lines.
 *
 * Observes CSS source structure via fs.readFile + regex so that visual regressions
 * in the token layer are caught without a full browser runtime.
 *
 * Also includes a happy-dom getComputedStyle test to lock-in --row-* token resolution
 * values (FR-TOKEN-001 acceptance: computed pixel values match between legacy and token paths).
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { beforeAll, describe, expect, it } from "vitest";

// Resolve the css directory relative to this test file's location.
// __dirname is available in vitest's Node environment.
const cssDir = path.resolve(__dirname, "../css");

function readCss(filename: string): string {
  return fs.readFileSync(path.join(cssDir, filename), "utf-8");
}

function countLines(content: string): number {
  return content.split("\n").length;
}

// ─── FR-FRAMEWORK-001: file line count ───────────────────────────────────────
describe("FR-FRAMEWORK-001: CSS file structure (3-file split, each <= 500 lines)", () => {
  const files = ["tokens.css", "app.css", "view.css"];

  for (const filename of files) {
    it(`${filename} exists and is <= 500 lines`, () => {
      const content = readCss(filename);
      const lines = countLines(content);
      expect(lines, `${filename} line count`).toBeLessThanOrEqual(500);
    });
  }

  it("tokens.css is a distinct file from app.css and view.css", () => {
    const tokensContent = readCss("tokens.css");
    const appContent = readCss("app.css");
    const viewContent = readCss("view.css");
    expect(tokensContent).not.toEqual(appContent);
    expect(tokensContent).not.toEqual(viewContent);
  });
});

// ─── FR-TOKEN-001: tokens.css declares required semantic tokens ───────────────
describe("FR-TOKEN-001: tokens.css declares required semantic tokens", () => {
  let tokens: string;

  beforeAll(() => {
    tokens = readCss("tokens.css");
  });

  const requiredTokens = [
    "--fg",
    "--fg-muted",
    "--bg",
    "--bg-elevated",
    "--accent",
    "--status-running",
    "--status-waiting",
    "--status-idle",
    "--status-stopped",
    "--status-unknown",
    "--border-color",
    "--focus-ring",
    "--row-radius",
    "--row-padding-y",
    "--row-padding-x",
    "--row-font-size",
    "--row-line-height",
    "--row-min-height",
    "--xterm-fg",
    "--xterm-cursor",
    "--xterm-selection",
    "--toast-bg-info",
    "--toast-bg-success",
    "--toast-bg-warn",
    "--toast-bg-error",
    "--dvh",
    "--bp-mobile-max",
    "--bp-tablet-max",
  ];

  for (const token of requiredTokens) {
    it(`declares ${token}`, () => {
      expect(tokens).toContain(`${token}:`);
    });
  }

  it("has :root block", () => {
    expect(tokens).toMatch(/:root\s*\{/);
  });

  it("has [data-theme='light'] block", () => {
    expect(tokens).toMatch(/\[data-theme=['"]light['"]\]\s*\{/);
  });

  it("has [data-theme='dark'] block", () => {
    expect(tokens).toMatch(/\[data-theme=['"]dark['"]\]\s*\{/);
  });

  it("has @supports not (height: 100dvh) fallback block", () => {
    expect(tokens).toMatch(/@supports not \(height: 100dvh\)/);
    expect(tokens).toMatch(/--dvh: 100vh/);
  });
});

// ─── FR-TOKEN-001: --row-* tokens are referenced in app.css ──────────────────
describe("FR-TOKEN-001: --row-* tokens are referenced in app.css (SessionList rows)", () => {
  let appCss: string;

  beforeAll(() => {
    appCss = readCss("app.css");
  });

  const rowTokens = [
    "--row-padding-y",
    "--row-padding-x",
    "--row-font-size",
    "--row-line-height",
    "--row-min-height",
    "--row-radius",
  ];

  for (const token of rowTokens) {
    it(`app.css references var(${token})`, () => {
      expect(appCss).toContain(`var(${token})`);
    });
  }
});

// ─── No hardcoded hex colors in app.css / view.css ───────────────────────────
describe("No hardcoded hex colors in app.css (all replaced with var(--*))", () => {
  let appCss: string;

  beforeAll(() => {
    appCss = readCss("app.css");
  });

  it("app.css does not contain bare 6-digit hex colors (like #1e1e1e, #45475a)", () => {
    // Match #RRGGBB patterns that are NOT inside comments.
    // Strip comment lines first for a simple check.
    const withoutComments = appCss
      .split("\n")
      .filter((line) => !line.trimStart().startsWith("/*") && !line.trimStart().startsWith("*"))
      .join("\n");

    // Allow #000 and #fff only in run-state-* color role (contrast guarantee for badges)
    // and rgba() references. These are intentional accessibility choices for badge text.
    const sixDigitHex = withoutComments.match(/#[0-9a-fA-F]{6}/g) ?? [];
    expect(sixDigitHex, `Found 6-digit hex in app.css: ${sixDigitHex.join(", ")}`).toHaveLength(0);
  });

  it("app.css does not reference 3-digit shorthand hex colors (like #333, #444)", () => {
    const withoutComments = appCss
      .split("\n")
      .filter((line) => !line.trimStart().startsWith("/*") && !line.trimStart().startsWith("*"))
      .join("\n");
    // Only 3-digit hex: #XYZ not followed by more hex digits
    const threeDigitHex = withoutComments.match(/#[0-9a-fA-F]{3}(?![0-9a-fA-F])/g) ?? [];
    expect(threeDigitHex, `Found 3-digit hex in app.css: ${threeDigitHex.join(", ")}`).toHaveLength(
      0,
    );
  });
});

describe("No hardcoded hex colors in view.css (all replaced with var(--*))", () => {
  let viewCss: string;

  beforeAll(() => {
    viewCss = readCss("view.css");
  });

  it("view.css replaces status bg/fg colors with var(--status-*) tokens", () => {
    // The run-state badges use var(--status-running) etc instead of #2c7a4d
    expect(viewCss).toContain("var(--status-running)");
    expect(viewCss).toContain("var(--status-waiting)");
    expect(viewCss).toContain("var(--status-idle)");
    expect(viewCss).toContain("var(--status-stopped)");
    expect(viewCss).toContain("var(--status-unknown)");
  });

  it("view.css uses var(--fg-muted) instead of hardcoded #888", () => {
    expect(viewCss).toContain("var(--fg-muted)");
    expect(viewCss).not.toContain("#888");
  });

  it("view.css uses var(--input-border) instead of hardcoded #333", () => {
    expect(viewCss).toContain("var(--input-border)");
    expect(viewCss).not.toContain("#333");
  });

  it("view.css has no bare 6-digit hex colors", () => {
    const withoutComments = viewCss
      .split("\n")
      .filter((line) => !line.trimStart().startsWith("/*") && !line.trimStart().startsWith("*"))
      .join("\n");
    const sixDigitHex = withoutComments.match(/#[0-9a-fA-F]{6}/g) ?? [];
    expect(sixDigitHex, `Found 6-digit hex in view.css: ${sixDigitHex.join(", ")}`).toHaveLength(0);
  });

  it("view.css has no bare 3-digit shorthand hex colors", () => {
    const withoutComments = viewCss
      .split("\n")
      .filter((line) => !line.trimStart().startsWith("/*") && !line.trimStart().startsWith("*"))
      .join("\n");
    const threeDigitHex = withoutComments.match(/#[0-9a-fA-F]{3}(?![0-9a-fA-F])/g) ?? [];
    expect(
      threeDigitHex,
      `Found 3-digit hex in view.css: ${threeDigitHex.join(", ")}`,
    ).toHaveLength(0);
  });

  it("view.css uses var(--session-status-*) for session status colors", () => {
    expect(viewCss).toContain("var(--session-status-running)");
    expect(viewCss).toContain("var(--session-status-waiting)");
    expect(viewCss).toContain("var(--session-status-stopped)");
    expect(viewCss).toContain("var(--session-status-pending)");
  });

  it("view.css uses var(--status-pending) for run-state-pending background", () => {
    expect(viewCss).toContain("var(--status-pending)");
  });

  it("view.css uses var(--badge-text-on-dark) and var(--badge-text-on-bright) for badge text", () => {
    expect(viewCss).toContain("var(--badge-text-on-dark)");
    expect(viewCss).toContain("var(--badge-text-on-bright)");
  });
});

// ─── NotificationToast uses CSS classes, not inline hex ───────────────────────
describe("NotificationToast.tsx: no inline hex color values", () => {
  it("NotificationToast.tsx has no inline hex color values", () => {
    const componentsDir = path.resolve(__dirname, "../components");
    const src = fs.readFileSync(path.join(componentsDir, "NotificationToast.tsx"), "utf-8");
    const hexMatches = src.match(/#[0-9a-fA-F]{3,6}/g) ?? [];
    expect(
      hexMatches,
      `Found inline hex in NotificationToast.tsx: ${hexMatches.join(", ")}`,
    ).toHaveLength(0);
  });
});

// ─── FR-TOKEN-001: --row-* token computed value lock-in (happy-dom) ──────────
// Observes that --row-* tokens resolve to their expected literal values when
// the tokens.css :root block is active. This is the "computed pixel values must
// match legacy" acceptance criterion from the task spec.
describe("FR-TOKEN-001: --row-* token resolution via getComputedStyle (happy-dom)", () => {
  it("resolves --row-* tokens to their declared literal values", () => {
    // Inject tokens.css :root declarations into a <style> tag
    const tokensCss = readCss("tokens.css");

    const style = document.createElement("style");
    style.textContent = tokensCss;
    document.head.appendChild(style);

    const root = document.documentElement;
    const computed = getComputedStyle(root);

    // These are the canonical values declared in tokens.css :root
    expect(computed.getPropertyValue("--row-radius").trim()).toBe("4px");
    expect(computed.getPropertyValue("--row-padding-y").trim()).toBe("6px");
    expect(computed.getPropertyValue("--row-padding-x").trim()).toBe("8px");
    expect(computed.getPropertyValue("--row-font-size").trim()).toBe("0.875rem");
    expect(computed.getPropertyValue("--row-line-height").trim()).toBe("1.4");
    expect(computed.getPropertyValue("--row-min-height").trim()).toBe("2rem");

    document.head.removeChild(style);
  });

  it("resolves --fg and --bg to dark theme defaults", () => {
    const tokensCss = readCss("tokens.css");

    const style = document.createElement("style");
    style.textContent = tokensCss;
    document.head.appendChild(style);

    const root = document.documentElement;
    const computed = getComputedStyle(root);

    expect(computed.getPropertyValue("--fg").trim()).toBe("#e6e6e6");
    expect(computed.getPropertyValue("--bg").trim()).toBe("#1e1e1e");

    document.head.removeChild(style);
  });
});

// ─── tokens.css declares --dvh and @supports fallback ───────────────────────
describe("FR-LAYOUT-004: --dvh token and @supports legacy fallback", () => {
  let tokens: string;

  beforeAll(() => {
    tokens = readCss("tokens.css");
  });

  it("tokens.css declares --dvh: 100dvh in :root", () => {
    expect(tokens).toContain("--dvh: 100dvh");
  });

  it("tokens.css has @supports not (height: 100dvh) { :root { --dvh: 100vh; } }", () => {
    expect(tokens).toMatch(/@supports not \(height: 100dvh\)[\s\S]*--dvh: 100vh/);
  });
});

// ─── ADR-0060 structural: app.css .terminal-host declares height:var(--dvh) ──
// Reads the *production* app.css text so that removing the declaration causes
// test failure — assertions against dynamically-injected CSS in happy-dom do
// not catch accidental deletions from the real file.
describe("ADR-0060 structural: app.css .terminal-host declares height:var(--dvh) (regression gate)", () => {
  let appCss: string;

  beforeAll(() => {
    appCss = readCss("app.css");
  });

  it("app.css .terminal-host block contains height: var(--dvh)", () => {
    // Match the declaration inside a .terminal-host { ... } block.
    // The regex scans for the rule set and verifies the height property is present.
    expect(appCss).toMatch(/\.terminal-host\s*\{[^}]*height\s*:\s*var\(--dvh\)/s);
  });

  it("app.css .terminal-host block contains flex: 1 1 0 (ADR-0029 coexistence)", () => {
    expect(appCss).toMatch(/\.terminal-host\s*\{[^}]*flex\s*:\s*1\s+1\s+0/s);
  });

  it("app.css .terminal-host block does NOT re-apply safe-area-inset-bottom (already in .app-shell via shell.css)", () => {
    // Locate the first .terminal-host { ... } block only (exclude the
    // multi-selector .terminal-host, .terminal-host .xterm { ... } block that
    // follows). Extract up to the first closing brace after '.terminal-host {'.
    const match = appCss.match(/\.terminal-host\s*\{([^}]*)\}/s);
    const block = match ? match[1] : "";
    expect(block).not.toContain("safe-area-inset-bottom");
  });
});
