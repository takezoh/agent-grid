/**
 * tokens-css-structure.test.ts
 *
 * FR-001/FR-002: two-layer token system (primitive + semantic alias).
 * FR-TOKEN-001: semantic CSS custom property hierarchy and --row-* sharing.
 * FR-FRAMEWORK-001: tokens.css / app.css / view.css file separation, each <= 500 lines.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { beforeAll, describe, expect, it } from "vitest";

const cssDir = path.resolve(__dirname, "../css");

function readCss(filename: string): string {
  return fs.readFileSync(path.join(cssDir, filename), "utf-8");
}

function countLines(content: string): number {
  return content.split("\n").length;
}

function extractRootBlock(tokens: string): string {
  const match = tokens.match(/:root\s*\{([\s\S]*?)\n\}/);
  return match ? match[1] : "";
}

function extractSemanticBlock(tokens: string): string {
  const root = extractRootBlock(tokens);
  const semanticStart = root.indexOf("/* ── Semantic:");
  return semanticStart === -1 ? root : root.slice(semanticStart);
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

// ─── FR-001: primitive layer exists ──────────────────────────────────────────
describe("FR-001: tokens.css declares primitive layer", () => {
  let tokens: string;
  let rootBlock: string;

  beforeAll(() => {
    tokens = readCss("tokens.css");
    rootBlock = extractRootBlock(tokens);
  });

  const primitiveTokens = [
    "--space-1",
    "--space-2",
    "--space-3",
    "--space-4",
    "--space-5",
    "--space-6",
    "--space-7",
    "--space-8",
    "--text-xs",
    "--text-sm",
    "--text-base",
    "--text-md",
    "--text-lg",
    "--text-xl",
    "--radius-1",
    "--radius-2",
    "--radius-3",
    "--radius-full",
    "--motion-1",
    "--motion-2",
    "--motion-3",
    "--motion-ease",
    "--font-ui",
    "--font-mono",
    "--color-neutral-250",
    "--color-accent-500",
    "--color-status-running",
  ];

  for (const token of primitiveTokens) {
    it(`declares primitive ${token}`, () => {
      expect(rootBlock).toContain(`${token}:`);
    });
  }

  it("motion-1 is 120ms", () => {
    expect(rootBlock).toMatch(/--motion-1:\s*120ms/);
  });

  it("motion-2 is 180ms", () => {
    expect(rootBlock).toMatch(/--motion-2:\s*180ms/);
  });

  it("motion-3 is 240ms", () => {
    expect(rootBlock).toMatch(/--motion-3:\s*240ms/);
  });

  it("motion-ease is cubic-bezier(.2,.7,.3,1)", () => {
    expect(rootBlock).toMatch(/--motion-ease:\s*cubic-bezier\(0\.2,\s*0\.7,\s*0\.3,\s*1\)/);
  });
});

// ─── FR-001/FR-002: semantic layer uses var() only ───────────────────────────
describe("FR-001/FR-002: semantic tokens resolve only to primitives via var()", () => {
  let semanticBlock: string;

  beforeAll(() => {
    const tokens = readCss("tokens.css");
    semanticBlock = extractSemanticBlock(tokens);
  });

  const requiredSemanticTokens = [
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
    "--status-pending",
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

  for (const token of requiredSemanticTokens) {
    it(`declares semantic ${token}`, () => {
      expect(semanticBlock).toContain(`${token}:`);
    });
  }

  it("semantic tokens do not contain hex/rgb/rgba/hsl literals", () => {
    const declarations = semanticBlock.match(/--[\w-]+:\s*[^;]+;/g) ?? [];
    const colorLiteral = /#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(/;
    const offenders = declarations.filter((d) => colorLiteral.test(d));
    expect(offenders, `Semantic literals found: ${offenders.join("; ")}`).toHaveLength(0);
  });

  it("semantic tokens use var() references", () => {
    const declarations = semanticBlock.match(/--[\w-]+:\s*[^;]+;/g) ?? [];
    const withoutVar = declarations.filter((d) => !d.includes("var(") && !d.includes("100dvh"));
    // --dvh and --row-line-height are non-var exceptions (layout literals)
    const allowed = new Set([
      "--dvh:",
      "--row-line-height:",
      "--row-min-height:",
      "--header-height:",
      "--bp-mobile-max:",
      "--bp-tablet-max:",
    ]);
    const offenders = withoutVar.filter((d) => !Array.from(allowed).some((a) => d.startsWith(a)));
    expect(offenders, `Semantic without var(): ${offenders.join("; ")}`).toHaveLength(0);
  });

  it("does NOT declare --session-status-* tokens (FR-003)", () => {
    const tokens = readCss("tokens.css");
    expect(tokens).not.toMatch(/--session-status-/);
  });

  it("has :root block", () => {
    expect(readCss("tokens.css")).toMatch(/:root\s*\{/);
  });

  it("has [data-theme='light'] block", () => {
    expect(readCss("tokens.css")).toMatch(/\[data-theme=['"]light['"]\]\s*\{/);
  });

  it("has [data-theme='dark'] block", () => {
    expect(readCss("tokens.css")).toMatch(/\[data-theme=['"]dark['"]\]\s*\{/);
  });

  it("has @supports not (height: 100dvh) fallback block", () => {
    const tokens = readCss("tokens.css");
    expect(tokens).toMatch(/@supports not \(height: 100dvh\)/);
    expect(tokens).toMatch(/--dvh: 100vh/);
  });
});

// ─── FR-007: reduced-motion zeros motion tokens in tokens.css ────────────────
describe("FR-007: motion token zeroing in tokens.css", () => {
  let tokens: string;

  beforeAll(() => {
    tokens = readCss("tokens.css");
  });

  it("tokens.css contains exactly 1 @media (prefers-reduced-motion: reduce) block", () => {
    const matches = tokens.match(/^@media\s+\(prefers-reduced-motion:\s*reduce\)/gm);
    expect(matches?.length ?? 0).toBe(1);
  });

  it("reduced-motion block zeros --motion-1..3", () => {
    expect(tokens).toMatch(
      /@media\s+\(prefers-reduced-motion:\s*reduce\)[\s\S]*--motion-1:\s*0ms[\s\S]*--motion-2:\s*0ms[\s\S]*--motion-3:\s*0ms/,
    );
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

// ─── FR-004: font tokens applied in app.css ──────────────────────────────────
describe("FR-004: app.css applies --font-ui and --font-mono", () => {
  let appCss: string;

  beforeAll(() => {
    appCss = readCss("app.css");
  });

  it("body uses var(--font-ui)", () => {
    expect(appCss).toMatch(/body\s*\{[^}]*font-family:\s*var\(--font-ui\)/s);
  });

  it("terminal-host uses var(--font-mono)", () => {
    expect(appCss).toContain("var(--font-mono)");
  });
});

// ─── FR-003: view.css uses --status-* for session status colors ──────────────
describe("FR-003: view.css uses var(--status-*) for session status colors", () => {
  let viewCss: string;

  beforeAll(() => {
    viewCss = readCss("view.css");
  });

  it("view.css uses var(--status-running) for session-status-running", () => {
    expect(viewCss).toContain("var(--status-running)");
  });

  it("view.css uses var(--status-waiting) for session-status-waiting", () => {
    expect(viewCss).toContain("var(--status-waiting)");
  });

  it("view.css uses var(--status-stopped) for session-status-stopped", () => {
    expect(viewCss).toContain("var(--status-stopped)");
  });

  it("view.css uses var(--status-pending) for session-status-pending", () => {
    expect(viewCss).toContain("var(--status-pending)");
  });

  it("view.css does NOT reference --session-status-* tokens", () => {
    expect(viewCss).not.toMatch(/--session-status-/);
  });

  it("view.css uses var(--status-pending) for run-state-pending background", () => {
    expect(viewCss).toContain("var(--status-pending)");
  });

  it("view.css run-state pills use soft status fills with vivid status text (web-ui-refresh)", () => {
    expect(viewCss).toContain("var(--status-running-soft)");
    expect(viewCss).toContain("var(--status-stopped-soft)");
    expect(viewCss).toContain("var(--status-waiting-soft)");
    expect(viewCss).toContain("var(--status-pending-soft)");
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
describe("FR-TOKEN-001: --row-* token resolution via getComputedStyle (happy-dom)", () => {
  it("resolves --row-* tokens to their declared literal values", () => {
    const tokensCss = readCss("tokens.css");

    const style = document.createElement("style");
    style.textContent = tokensCss;
    document.head.appendChild(style);

    const root = document.documentElement;
    const computed = getComputedStyle(root);

    expect(computed.getPropertyValue("--row-radius").trim()).toBe("4px");
    expect(computed.getPropertyValue("--row-padding-y").trim()).toBe("6px");
    expect(computed.getPropertyValue("--row-padding-x").trim()).toBe("8px");
    expect(computed.getPropertyValue("--row-font-size").trim()).toBe("0.875rem");
    expect(computed.getPropertyValue("--row-line-height").trim()).toBe("1.4");
    expect(computed.getPropertyValue("--row-min-height").trim()).toBe("2rem");

    document.head.removeChild(style);
  });

  it("resolves --fg and --bg via semantic → primitive chain", () => {
    const tokensCss = readCss("tokens.css");

    const style = document.createElement("style");
    style.textContent = tokensCss;
    document.head.appendChild(style);

    const root = document.documentElement;
    const computed = getComputedStyle(root);

    // web-ui-refresh palette (spec-20260714-web-ui-refresh Design Token Values):
    // canvas #0b0c0e / text #e8eaee — legacy #1e1e1e / #e6e6e6 must NOT survive.
    expect(computed.getPropertyValue("--fg").trim()).toBe("#e8eaee");
    expect(computed.getPropertyValue("--bg").trim()).toBe("#0b0c0e");

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
describe("ADR-0060 structural: app.css .terminal-host declares height:var(--dvh) (regression gate)", () => {
  let appCss: string;

  beforeAll(() => {
    appCss = readCss("app.css");
  });

  it("app.css .terminal-host block contains height: var(--dvh)", () => {
    expect(appCss).toMatch(/\.terminal-host\s*\{[^}]*height\s*:\s*var\(--dvh\)/s);
  });

  it("app.css .terminal-host block contains flex: 1 1 0 (ADR-0029 coexistence)", () => {
    expect(appCss).toMatch(/\.terminal-host\s*\{[^}]*flex\s*:\s*1\s+1\s+0/s);
  });

  it("app.css .terminal-host block does NOT re-apply safe-area-inset-bottom (already in .app-shell via shell.css)", () => {
    const match = appCss.match(/\.terminal-host\s*\{([^}]*)\}/s);
    const block = match ? match[1] : "";
    expect(block).not.toContain("safe-area-inset-bottom");
  });
});
