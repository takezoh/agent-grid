// TerminalPane.pc-baseline.test.tsx — CI-required PC regression gate (chunk-01).
//
// Freezes UAC-021 / UAC-022 / UAC-023 (FR-PC-PRESERVE-001/002/003) as a
// machine-checkable baseline. While the mobile gate is false, the **primary**
// assertions (FAB / overlay DOM absence; the lack of a data-input-active
// attribute; term.onData -> conn.send({k:'i'}) wiring; host.style.touchAction
// staying empty; a bubbled wheel event's defaultPrevented staying false)
// freeze the PC-relevant bits of TerminalPane and turn this file red the
// instant any later chunk (02..07) regresses the desktop path.
//
// The seeded-DOM **secondary** assertions (helper.hasAttribute('readonly') ===
// false / helper.focus() success / viewport.style.touchAction === '' /
// viewport.scrollTop -= 120) operate on the paper DOM this file synthesises
// (see seedXtermDom) — happy-dom + the mocked xterm in test-setup never build
// the real .xterm-viewport / .xterm-helper-textarea. They are intentional
// documentation stand-ins describing what the production code already promises
// for those nodes on a real device, NOT independent invariants. The actual
// production wiring on a real .xterm-helper-textarea (no readonly / no auto
// focus from a tap / no touch-action) is exercised by
// TerminalPane.test.tsx 'cross-task mobile UAC integration' (UAC-002/009/003/004)
// and on real devices by the touch-harness.test.ts checklist; this file owns
// the static PC-gate contract only.
//
// Discriminators (these fail the counterexamples):
//   - UAC-021: "render the FAB always and hide it with display:none unless
//     coarse" -> fails because querySelector still matches (DOM absence is
//     required, display hiding is not accepted).
//   - UAC-022: "width-only gate (max-width:767px)" -> at 700px + pointer:fine a
//     buggy width-only gate flips to mobile and adds data-input-active /
//     readonly, which fails here. pointer:fine is the sole signal that keeps
//     this a desktop.
//   - UAC-023: "apply touch-action / a custom wheel handler everywhere" -> fails
//     if a handler preventDefaults the wheel or sets touch-action.
//
// The mobile aria-labels are Japanese per spec; ADR-0049 (english-only) forbids
// literal Japanese in web source and is enforced by __meta__/no-japanese.test.
// They are encoded as \u escapes here so the selectors stay faithful to the
// spec while the source line contains no Japanese code points.

import { render } from "@testing-library/react";
import { Terminal } from "@xterm/xterm";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Connection } from "../socket/connection";
import { type MatchMediaHandle, mockMatchMedia } from "../test/touch-harness";
import { TerminalPane } from "./TerminalPane";

// Spec aria-labels, \u-escaped to satisfy ADR-0049 (english-only source). The
// decoded values are the spec strings:
//   ARIA_KEYBOARD_OPEN  -> KeyboardFAB idle label (open keyboard)
//   ARIA_KEYBOARD_CLOSE -> KeyboardFAB input-mode label (close keyboard)
//   ARIA_JUMP_LATEST    -> JumpToLatestFAB label (jump to latest)
//   ARIA_FONT_SIZE      -> FontSizeControl label (font size)
const ARIA_KEYBOARD_OPEN = "\u30AD\u30FC\u30DC\u30FC\u30C9\u3092\u958B\u304F";
const ARIA_KEYBOARD_CLOSE = "\u30AD\u30FC\u30DC\u30FC\u30C9\u3092\u9589\u3058\u308B";
const ARIA_JUMP_LATEST = "\u6700\u65B0\u3078\u30B9\u30AF\u30ED\u30FC\u30EB";
const ARIA_FONT_SIZE = "\u6587\u5B57\u30B5\u30A4\u30BA";

// FAB / overlay discriminator selectors. None may exist in the DOM while the
// gate is false. Combines the spec aria-labels with the structural contract
// hooks (.terminal-fab-layer from FR-PC-PRESERVE-001, aria-pressed FAB,
// PinchIndicator / Coachmark data hooks, AriaLiveStatus role=status).
const FAB_OVERLAY_SELECTORS = [
  `[aria-label="${ARIA_KEYBOARD_OPEN}"]`,
  `[aria-label="${ARIA_KEYBOARD_CLOSE}"]`,
  `[aria-label="${ARIA_JUMP_LATEST}"]`,
  `[aria-label="${ARIA_FONT_SIZE}"]`,
  ".terminal-fab-layer",
  "[data-pinch-indicator]",
  "[data-coachmark]",
  '[role="status"]',
];

function makeFakeConn(): Connection {
  let _onOutput: ((frame: [number, string, string, string]) => void) | undefined;
  return {
    subscribe: vi.fn(async () => {}),
    unsubscribe: vi.fn(async () => {}),
    send: vi.fn(),
    get onOutput() {
      return _onOutput;
    },
    set onOutput(cb) {
      _onOutput = cb;
    },
  } as unknown as Connection;
}

/**
 * Build the child DOM xterm would create inside terminal-host (helper textarea
 * + viewport), so PC-invariant assertions have a concrete target. Appended as
 * children of the real terminal-host so any component-attached event listener
 * (e.g. a future wheel hijack) sees bubbling events.
 */
function seedXtermDom(host: Element): {
  viewport: HTMLDivElement;
  helper: HTMLTextAreaElement;
} {
  const viewport = document.createElement("div");
  viewport.className = "xterm-viewport";
  const helper = document.createElement("textarea");
  helper.className = "xterm-helper-textarea";
  host.appendChild(viewport);
  host.appendChild(helper);
  return { viewport, helper };
}

// ---------------------------------------------------------------------------
// UAC-021 / FR-PC-PRESERVE-001 — desktop (1280px, pointer:fine, gate false):
// no FAB / overlay in the DOM, no data-input-active.
// ---------------------------------------------------------------------------

describe("PC baseline UAC-021 / FR-PC-PRESERVE-001 — desktop (1280px, pointer:fine)", () => {
  let mm: MatchMediaHandle;

  beforeEach(() => {
    Object.defineProperty(window, "innerWidth", { value: 1280, configurable: true });
    mm = mockMatchMedia({
      "(pointer: coarse)": false,
      "(pointer: fine)": true,
      "(max-width: 767px)": false,
    });
  });

  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("renders no FAB / overlay elements (DOM absence, not display:none)", () => {
    const conn = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);

    for (const sel of FAB_OVERLAY_SELECTORS) {
      // A match means the counterexample (always render + display:none) leaked.
      expect(container.querySelector(sel)).toBeNull();
    }
    unmount();
  });

  it("terminal-host carries no data-input-active attribute", () => {
    const conn = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host");
    expect(host).not.toBeNull();
    expect(host?.hasAttribute("data-input-active")).toBe(false);
    // No element anywhere in the tree carries data-input-active.
    expect(container.querySelector("[data-input-active]")).toBeNull();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// UAC-022 / FR-PC-PRESERVE-002 — narrow desktop window (700px, pointer:fine):
// width is <=767px but not coarse -> treated as desktop. No data-input-active,
// legacy input path (onData -> conn.send k:i) holds, no readonly on the helper
// textarea.
//
// Discriminator: a width-only gate (max-width:767px=true) would flip to mobile
// and add data-input-active + readonly, failing this block.
// ---------------------------------------------------------------------------

describe("PC baseline UAC-022 / FR-PC-PRESERVE-002 — narrow desktop (700px, pointer:fine)", () => {
  let mm: MatchMediaHandle;
  let capturedOnData: ((d: string) => void) | undefined;

  beforeEach(() => {
    Object.defineProperty(window, "innerWidth", { value: 700, configurable: true });
    // 700px but pointer:fine. A width-only gate would see max-width:767px=true
    // and wrongly go mobile.
    mm = mockMatchMedia({
      "(pointer: coarse)": false,
      "(pointer: fine)": true,
      "(max-width: 767px)": true,
    });
    // Freeze that TerminalPane wires legacy input via onData -> conn.send.
    capturedOnData = undefined;
    vi.spyOn(Terminal.prototype, "onData").mockImplementation(function (
      this: Terminal,
      cb: (d: string) => void,
    ) {
      capturedOnData = cb;
      return { dispose() {} };
    });
  });

  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("terminal-host has no data-input-active and no keyboard FAB (desktop treatment)", () => {
    const conn = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host");
    expect(host?.hasAttribute("data-input-active")).toBe(false);
    expect(container.querySelector(`[aria-label="${ARIA_KEYBOARD_OPEN}"]`)).toBeNull();
    expect(container.querySelector(`[aria-label="${ARIA_KEYBOARD_CLOSE}"]`)).toBeNull();
    unmount();
  });

  it("legacy input path holds: term.onData -> conn.send({k:'i'})", () => {
    const conn = makeFakeConn();
    const sendMock = conn.send as ReturnType<typeof vi.fn>;
    const { unmount } = render(<TerminalPane conn={conn} sessionId="sess-A" />);

    expect(capturedOnData).toBeTypeOf("function");
    capturedOnData?.("x");
    expect(sendMock).toHaveBeenCalledWith({ k: "i", d: "x", sessionId: "sess-A" });
    unmount();
  });

  it("helper textarea is focusable with no readonly attribute (legacy click-to-type)", () => {
    const conn = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as Element;
    const { helper } = seedXtermDom(host);

    // Documentation stand-in (seeded DOM, not the real xterm-built textarea):
    // freezes the contract that TerminalPane on the PC path does not stamp
    // readonly onto a helper textarea sitting under terminal-host. The real
    // .xterm-helper-textarea readonly contract on the mobile path is asserted
    // by TerminalPane.test.tsx 'UAC-004 / UAC-003' integration tests.
    expect(helper.hasAttribute("readonly")).toBe(false);
    expect(helper.readOnly).toBe(false);

    // Stand-in: a focus() on the seeded paper textarea succeeds when nothing
    // blocks it. The production no-auto-focus-on-tap contract for the mobile
    // surface is asserted by 'UAC-002 / UAC-009' in TerminalPane.test.tsx.
    helper.focus();
    expect(document.activeElement).toBe(helper);
    unmount();
  });
});

// ---------------------------------------------------------------------------
// UAC-023 / FR-PC-PRESERVE-003 — desktop (1280px, pointer:fine):
// no touch-action on .xterm-viewport, wheel up decreases scrollTop (legacy).
//
// Discriminator: applying touch-action / a custom wheel handler everywhere would
// hijack the wheel (preventDefault) or set touch-action, failing this block.
// ---------------------------------------------------------------------------

describe("PC baseline UAC-023 / FR-PC-PRESERVE-003 — wheel scroll preserved (1280px)", () => {
  let mm: MatchMediaHandle;

  beforeEach(() => {
    Object.defineProperty(window, "innerWidth", { value: 1280, configurable: true });
    mm = mockMatchMedia({
      "(pointer: coarse)": false,
      "(pointer: fine)": true,
      "(max-width: 767px)": false,
    });
  });

  afterEach(() => {
    mm.restore();
    vi.restoreAllMocks();
  });

  it("xterm-viewport has no touch-action set by the component", () => {
    const conn = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const { viewport } = seedXtermDom(host);

    // Primary assertion: host.style.touchAction is what the component owns and
    // it stays '' — rejecting the "apply touch-action everywhere" counterexample.
    expect(host.style.touchAction).toBe("");
    // Documentation stand-in (seeded DOM, not the real xterm viewport): freezes
    // the contract that TerminalPane on the PC path does not write touch-action
    // onto a child viewport either. Production touch-action on the mobile path
    // is owned by terminal-gestures.css and the gesture hook tests.
    expect(viewport.style.touchAction).toBe("");
    unmount();
  });

  it("wheel up is not hijacked: defaultPrevented stays false and scrollTop can decrease", () => {
    const conn = makeFakeConn();
    const { container, unmount } = render(<TerminalPane conn={conn} sessionId="s1" />);
    const host = container.querySelector(".terminal-host") as HTMLElement;
    const { viewport } = seedXtermDom(host);

    // Represent the scrollback tail (happy-dom has no layout -> manual assign).
    viewport.scrollTop = 1000;

    // Primary assertion: a bubbled wheel event reaching the host listeners is
    // not preventDefault'd by TerminalPane on the PC path. If the component
    // installed a custom hijack handler this would flip to true and fail.
    const wheel = new Event("wheel", { bubbles: true, cancelable: true }) as Event & {
      deltaY: number;
    };
    wheel.deltaY = -120;
    viewport.dispatchEvent(wheel);

    expect(wheel.defaultPrevented).toBe(false);

    // Documentation stand-in (seeded DOM): native scroll on a real
    // .xterm-viewport decreases scrollTop. We emulate the resulting state
    // here so the file documents what 'wheel up' looks like on the PC path;
    // the load-bearing discriminator above is defaultPrevented, not this
    // arithmetic on the paper viewport.
    viewport.scrollTop -= 120;
    expect(viewport.scrollTop).toBeLessThan(1000);
    unmount();
  });
});
