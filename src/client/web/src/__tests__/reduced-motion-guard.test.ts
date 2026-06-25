/**
 * reduced-motion-guard.test.ts
 *
 * FR-MOTION-001: When prefers-reduced-motion: reduce is active, animation/transition
 *   suppressions are applied to spinners, drawer, palette flash, snackbar, toast, tabs.
 *   Running state remains readable via icon + text in the DOM.
 *
 * FR-MOTION-002: The @media (prefers-reduced-motion: reduce) rule exists exactly once
 *   in view.css and is absent from tokens.css and app.css (single canonical location,
 *   ADR-0064).
 *
 * FR-FRAMEWORK-001: view.css must be <= 500 lines.
 *
 * Strategy: happy-dom does not apply CSS @media rules via getComputedStyle. All
 * motion-suppression assertions use fs.readFile + regex to inspect CSS source
 * structure, which is authoritative for FR-MOTION-001 and FR-MOTION-002.
 * A DOM render test additionally confirms that the RunStateBadge running state
 * keeps its icon + text visible when reduced-motion is active.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { cleanup, render } from "@testing-library/react";
import React from "react";
import { afterEach, describe, expect, it } from "vitest";
import { RunStateBadge } from "../components/RunStateBadge";

afterEach(() => {
  cleanup();
});

// ─── CSS file helpers ─────────────────────────────────────────────────────────

const cssDir = path.resolve(__dirname, "../css");

function readCss(filename: string): string {
  return fs.readFileSync(path.join(cssDir, filename), "utf-8");
}

// Count occurrences of `@media (prefers-reduced-motion: reduce)` rule starts.
// Uses line-start anchor (^) with multiline flag so comment lines are excluded.
function countReducedMotionBlocks(content: string): number {
  const matches = content.match(/^@media\s+\(prefers-reduced-motion:\s*reduce\)/gm);
  return matches ? matches.length : 0;
}

// ─── FR-FRAMEWORK-001: view.css line count ────────────────────────────────────

describe("FR-FRAMEWORK-001: view.css line count <= 500", () => {
  it("view.css has at most 500 lines", () => {
    const content = readCss("view.css");
    const lines = content.split("\n").length;
    expect(lines, `view.css line count (${lines})`).toBeLessThanOrEqual(500);
  });
});

// ─── FR-MOTION-002: single consolidated @media block in view.css ─────────────

describe("FR-MOTION-002: single @media (prefers-reduced-motion: reduce) block in view.css", () => {
  it("view.css contains exactly 1 @media (prefers-reduced-motion: reduce) rule", () => {
    const viewCss = readCss("view.css");
    const count = countReducedMotionBlocks(viewCss);
    expect(count, "view.css must have exactly 1 reduced-motion block").toBe(1);
  });

  it("tokens.css contains no @media (prefers-reduced-motion: reduce) rule", () => {
    const tokensCss = readCss("tokens.css");
    const count = countReducedMotionBlocks(tokensCss);
    expect(count, "tokens.css must have 0 reduced-motion blocks").toBe(0);
  });

  it("app.css contains no @media (prefers-reduced-motion: reduce) rule", () => {
    const appCss = readCss("app.css");
    const count = countReducedMotionBlocks(appCss);
    expect(count, "app.css must have 0 reduced-motion blocks").toBe(0);
  });
});

// ─── FR-MOTION-001: animation/transition suppressions present in view.css block

// Extract the content of the @media (prefers-reduced-motion: reduce) block.
// We locate it by finding the rule start and matching braces.
function extractReducedMotionBlock(css: string): string {
  const startIndex = css.search(/^@media\s+\(prefers-reduced-motion:\s*reduce\)/m);
  if (startIndex === -1) return "";

  let depth = 0;
  let blockStart = -1;
  let blockEnd = -1;

  for (let i = startIndex; i < css.length; i++) {
    if (css[i] === "{") {
      depth++;
      if (blockStart === -1) blockStart = i;
    } else if (css[i] === "}") {
      depth--;
      if (depth === 0) {
        blockEnd = i;
        break;
      }
    }
  }

  if (blockStart === -1 || blockEnd === -1) return "";
  return css.slice(blockStart, blockEnd + 1);
}

describe("FR-MOTION-001: reduced-motion block contains required suppressions", () => {
  const viewCss = readCss("view.css");
  const reducedBlock = extractReducedMotionBlock(viewCss);

  it("reduced-motion block is non-empty", () => {
    expect(reducedBlock.length, "reduced-motion block must be non-empty").toBeGreaterThan(0);
  });

  it("suppresses .run-state-spinner animation", () => {
    expect(reducedBlock).toMatch(/\.run-state-spinner/);
    expect(reducedBlock).toMatch(/animation:\s*none\s*!important/);
  });

  it("suppresses .session-status-spinner animation", () => {
    expect(reducedBlock).toMatch(/\.session-status-spinner/);
  });

  it("suppresses .session-drawer transition and animation", () => {
    expect(reducedBlock).toMatch(/\.session-drawer/);
    expect(reducedBlock).toMatch(/transition:\s*none\s*!important/);
  });

  it("suppresses .session-drawer__slide transition", () => {
    expect(reducedBlock).toMatch(/\.session-drawer__slide/);
  });

  it("suppresses .palette-active-context--flash animation", () => {
    expect(reducedBlock).toMatch(/\.palette-active-context--flash/);
  });

  it("suppresses .palette-listbox__row--flash animation", () => {
    expect(reducedBlock).toMatch(/\.palette-listbox__row--flash/);
  });

  it("suppresses .undo-snackbar transition and animation", () => {
    expect(reducedBlock).toMatch(/\.undo-snackbar/);
  });

  it("suppresses .notification-toast transition and animation", () => {
    expect(reducedBlock).toMatch(/\.notification-toast/);
  });

  it("suppresses .main-tab or .main-tab-panel transition", () => {
    expect(reducedBlock).toMatch(/\.main-tab/);
  });
});

// ─── FR-MOTION-001: RunStateBadge running state is readable when reduced-motion

describe("FR-MOTION-001: RunStateBadge running state readable with reduced-motion", () => {
  it("renders icon span + status text for running state", () => {
    // Activate reduced-motion preference via the setMatchMedia helper.
    globalThis.setMatchMedia("(prefers-reduced-motion: reduce)", true);

    const { getByRole, container } = render(
      React.createElement(RunStateBadge, { status: "running" }),
    );

    // The badge element is readable via aria-label.
    const badge = getByRole("generic", { name: /status:\s*running/i });
    expect(badge).toBeTruthy();

    // The text content includes "running".
    expect(badge.textContent).toContain("running");

    // The spinner span (aria-hidden) is present in the DOM — visual-only;
    // reduced-motion suppresses the animation, not the element itself.
    const spinner = container.querySelector(".run-state-spinner");
    expect(spinner).not.toBeNull();
  });

  it("renders status text 'running' visible in the DOM", () => {
    globalThis.setMatchMedia("(prefers-reduced-motion: reduce)", true);

    const { container } = render(React.createElement(RunStateBadge, { status: "running" }));

    // Text node "running" should be present inside the badge.
    const badge = container.querySelector(".run-state-badge");
    expect(badge).not.toBeNull();
    expect(badge?.textContent).toContain("running");
  });
});
