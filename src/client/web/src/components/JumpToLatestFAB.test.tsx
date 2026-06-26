// JumpToLatestFAB.test.tsx — conditional-render + mode-independence + tap→tail
// integration (FR-MOB-JUMP-001/002/003/005, UAC-012/013/014/015, ADR 0073/0075).
//
// Discriminating against:
//   - UAC-012: at tail the button is ABSENT from the DOM (conditional render, not
//     CSS opacity:0) — a phantom queryable button would fail the null assertion.
//   - UAC-013: leaving tail makes the button visible with a non-empty aria-label
//     and the 44×44 icon-button contract.
//   - UAC-014: tapping calls scrollToBottom, lands scrollTop at tail (±2px) AND
//     unmounts the FAB (the counterexample updates scrollTop but leaves the FAB).
//   - UAC-015: mode-independence — visible in input mode while in scrollback.
//   - FR-MOB-JUMP-005: with the seed gate open the FAB is absent even in scrollback.

import { act, fireEvent, render, screen } from "@testing-library/react";
import { promises as fs } from "node:fs";
import path from "node:path";
import { type JSX, useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useJumpToLatest } from "../hooks/useJumpToLatest";
import { JumpToLatestFAB } from "./JumpToLatestFAB";

const LABEL = "最新へスクロール";
const SELECTOR = `[aria-label="${LABEL}"]`;

// ---------------------------------------------------------------------------
// Viewport stub (happy-dom has no layout): scrollTop/Height/clientHeight backed
// by manual properties; scrollToBottom snaps scrollTop to tail + fires scroll.
// ---------------------------------------------------------------------------

function makeViewport(scrollHeight: number, clientHeight: number): HTMLDivElement {
  const el = document.createElement("div");
  el.className = "xterm-viewport";
  let top = scrollHeight - clientHeight;
  Object.defineProperty(el, "scrollHeight", { value: scrollHeight, configurable: true });
  Object.defineProperty(el, "clientHeight", { value: clientHeight, configurable: true });
  Object.defineProperty(el, "scrollTop", {
    configurable: true,
    get: () => top,
    set: (v: number) => {
      top = v;
    },
  });
  document.body.appendChild(el);
  return el;
}

function scrollTo(el: HTMLElement, value: number): void {
  act(() => {
    el.scrollTop = value;
    el.dispatchEvent(new Event("scroll"));
  });
}

/**
 * Integration harness: useJumpToLatest drives a real JumpToLatestFAB. The
 * injected scrollToBottom simulates term.scrollToBottom() by snapping scrollTop
 * to tail and emitting the scroll event the real viewport would.
 */
function Harness(props: {
  el: HTMLDivElement;
  seedReady?: boolean;
  inputActive?: boolean;
}): JSX.Element {
  const { el, seedReady = true, inputActive = false } = props;
  const viewportRef = useRef<HTMLElement | null>(el);
  const { shouldShowFab, jumpToBottom } = useJumpToLatest({
    viewportRef,
    seedReady,
    scrollToBottom: () => {
      el.scrollTop = el.scrollHeight - el.clientHeight; // tail
      el.dispatchEvent(new Event("scroll"));
    },
  });
  return (
    <div data-input-active={inputActive ? "true" : "false"}>
      <JumpToLatestFAB show={shouldShowFab} onJump={jumpToBottom} />
    </div>
  );
}

describe("JumpToLatestFAB — conditional render (UAC-012)", () => {
  it("is absent from the DOM when show=false (not opacity:0 hidden)", () => {
    const { container } = render(<JumpToLatestFAB show={false} onJump={() => {}} />);
    expect(container.querySelector(SELECTOR)).toBeNull();
    expect(screen.queryByRole("button", { name: LABEL })).toBeNull();
  });

  it("renders a real labeled <button> on the icon-button (44×44) contract when show=true", () => {
    render(<JumpToLatestFAB show={true} onJump={() => {}} />);
    const btn = screen.getByRole("button", { name: LABEL });
    expect(btn.tagName).toBe("BUTTON");
    expect((btn.getAttribute("aria-label") ?? "").trim().length).toBeGreaterThan(0);
    // 44×44 lives in icon-button.css (no layout engine in happy-dom): assert the
    // button carries the primitive class that owns the contract.
    expect(btn.classList.contains("icon-button")).toBe(true);
    // data-overlay so the host interceptor excludes it from outside-tap.
    expect(btn.hasAttribute("data-overlay")).toBe(true);
  });
});

describe("JumpToLatestFAB — scroll-driven visibility", () => {
  let el: HTMLDivElement;
  beforeEach(() => {
    el = makeViewport(1000, 200); // tail = 800
  });
  afterEach(() => {
    el.remove();
  });

  it("UAC-013: appears when scrollTop leaves the ±2px tail", () => {
    const { container } = render(<Harness el={el} />);
    expect(container.querySelector(SELECTOR)).toBeNull(); // at tail → absent

    scrollTo(el, 400); // scrollback
    expect(container.querySelector(SELECTOR)).not.toBeNull();
  });

  it("UAC-015: visible in INPUT mode while in scrollback (mode independent)", () => {
    const { container } = render(<Harness el={el} inputActive={true} />);
    scrollTo(el, 400);
    const host = container.querySelector('[data-input-active="true"]');
    expect(host).not.toBeNull();
    expect(container.querySelector(SELECTOR)).not.toBeNull();
  });

  it("FR-MOB-JUMP-005: absent in scrollback while the seed gate is closed", () => {
    const { container } = render(<Harness el={el} seedReady={false} />);
    scrollTo(el, 100); // deep scrollback, seed not ready
    expect(container.querySelector(SELECTOR)).toBeNull();
  });

  it("UAC-014: tap → scrollToBottom lands tail (±2px) AND unmounts the FAB", () => {
    const { container } = render(<Harness el={el} />);
    scrollTo(el, 400); // make it visible
    const btn = container.querySelector(SELECTOR) as HTMLButtonElement;
    expect(btn).not.toBeNull();

    fireEvent.click(btn);

    const tail = el.scrollHeight - el.clientHeight;
    expect(Math.abs(el.scrollTop - tail)).toBeLessThanOrEqual(2);
    expect(container.querySelector(SELECTOR)).toBeNull(); // back to FR-MOB-JUMP-001
  });
});

describe("jump-fab.css — internal-only, does not break the 44×44 contract", () => {
  it("styles only .jump-to-latest-fab and never the chunk-07 layout layer", async () => {
    const cssPath = path.join(
      import.meta.dirname ?? __dirname,
      "..",
      "css",
      "jump-fab.css",
    );
    const source = await fs.readFile(cssPath, "utf-8");
    // Internal look only: it must not pin a sub-44px size (that lives in
    // icon-button.css and must not be overridden smaller here).
    expect(source).not.toMatch(/min-(?:width|height):\s*(?:8|16|24|32|40)px/);
    // Layout / stacking / offset belong to view.css (chunk-07), not here.
    expect(source).not.toMatch(/\.terminal-fab-layer/);
    expect(source).not.toMatch(/--terminal-fab-offset/);
    expect(source).not.toMatch(/position:\s*(?:fixed|absolute)/);
  });
});
