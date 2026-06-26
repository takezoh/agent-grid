// TerminalMobileOverlay — the single mount point that wires every mobile hook
// (chunk-02..06) onto the live xterm terminal and renders the FAB overlay layer
// (ADR 0069 / 0072 / 0074 / 0077, FR-MOB-FAB-* / VVP-* / COACH-* /
// FR-MOB-SWIPE-ARROW-*).
//
// ADR 0077 retired the pinch path: PinchIndicator is no longer rendered,
// useMobilePinch is removed, and the touch-gesture hook now emits arrow keys
// for horizontal swipes (input mode only) instead of fontSize ratios.
//
// TerminalPane renders this *only* under `useMobileGate() === true`, so on the PC
// path none of these hooks ever run (no data-input-active, no listeners, no FABs).
// The component is split in two so the live-region provider sits above every hook
// that announces through it:
//   TerminalMobileOverlay → <AnnouncerProvider> → TerminalMobileLayer (hooks+JSX).
//
// The `.terminal-fab-layer` is an absolute sibling of terminal-host anchored to
// terminal-slot (ADR 0029/0065 box invariant, UAC-025): FAB appearance never
// reflows terminal-host. visualViewport-lift fans out to all FABs by writing a
// single CSS custom property on this layer (ADR 0069, zero React re-renders).

import type { Terminal } from "@xterm/xterm";
import { type JSX, type RefObject, useEffect, useRef } from "react";
import "../css/terminal-fab-layer.css";
import { AnnouncerProvider, useAnnouncer } from "../hooks/useAnnouncer";
import { useCoachmarkOnce } from "../hooks/useCoachmarkOnce";
import { useFontSize } from "../hooks/useFontSize";
import { useHostPointerInterceptor } from "../hooks/useHostPointerInterceptor";
import { useInputMode } from "../hooks/useInputMode";
import { useJumpToLatest } from "../hooks/useJumpToLatest";
import { type TerminalLike, useTerminalTouchGestures } from "../hooks/useTerminalTouchGestures";
import { useVisualViewportLift } from "../hooks/useVisualViewportLift";
import { useDaemonStore } from "../store/daemon";
import { AriaLiveStatus } from "./AriaLiveStatus";
import { Coachmark } from "./Coachmark";
import { DriverShortcutBar } from "./DriverShortcutBar";
import { FontSizeControl } from "./FontSizeControl";
import { JumpToLatestFAB } from "./JumpToLatestFAB";
import { KeyboardFAB } from "./KeyboardFAB";

export interface TerminalMobileOverlayProps {
  hostRef: RefObject<HTMLElement | null>;
  termRef: RefObject<Terminal | null>;
  viewportRef: RefObject<HTMLElement | null>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  /** ADR-0034 rAF-coalesced refit shared with the terminal lifecycle. */
  scheduleFit: () => void;
  /** ADR-0066 seed-flush completion; gates the jump-to-latest FAB. */
  seedReady: boolean;
  /**
   * ADR 0077: thin closure provided by `TerminalPane` that forwards a raw input
   * string into the active session's wire frame (`{k:"i", d, sessionId}`).
   * Kept narrow so the overlay never depends on `Connection` directly.
   */
  sendInput: (data: string) => void;
}

export function TerminalMobileOverlay(props: TerminalMobileOverlayProps): JSX.Element {
  return (
    <AnnouncerProvider>
      <TerminalMobileLayer {...props} />
    </AnnouncerProvider>
  );
}

function TerminalMobileLayer(props: TerminalMobileOverlayProps): JSX.Element {
  const { hostRef, termRef, viewportRef, textareaRef, scheduleFit, seedReady, sendInput } = props;
  const { announce } = useAnnouncer();
  const layerRef = useRef<HTMLDivElement | null>(null);

  // view/input mode (ADR 0068): stamps data-input-active, owns helper readonly/focus.
  const input = useInputMode({ hostRef, textareaRef, announce });

  // One capture-phase pointerdown listener: focus-block in view mode, outside-tap exit
  // in input mode (ADR 0068).
  useHostPointerInterceptor({
    hostRef,
    textareaRef,
    isActive: () => input.active,
    onOutsideTap: () => input.exit("outside-tap"),
  });

  // visualViewport-lift (ADR 0069): write --terminal-fab-offset while in input mode.
  useVisualViewportLift({ layerRef, active: input.active });

  // fontSize (ADR 0070/0034/0077): stepper-only path now; pinch is retired.
  const font = useFontSize({ scheduleFit });
  // Apply the resolved fontSize to the live terminal; scheduleFit (inside
  // useFontSize) then re-flows the grid (FR-MOB-STEPPER-001).
  useEffect(() => {
    const term = termRef.current;
    if (term) term.options.fontSize = font.fontSize;
  }, [font.fontSize, termRef]);

  // ADR 0077: horizontal swipe → arrow key spam. The reducer emits arrow effects
  // in cell-width increments; this callback forms the VT100 byte sequence and
  // hands one wire frame per touchmove to `sendInput`. Effects are gated by
  // `isInputActive`, so view mode preserves the legacy scroll-only behaviour.
  useTerminalTouchGestures({
    viewportRef,
    termRef: termRef as RefObject<TerminalLike | null>,
    onArrowKey: (direction, count) => {
      const seq = direction === "right" ? "\x1b[C" : "\x1b[D";
      sendInput(seq.repeat(count));
    },
    isInputActive: () => input.active,
  });

  // jump-to-latest FAB (ADR 0073/0066): scroll-position driven, seed-gated.
  const jump = useJumpToLatest({
    viewportRef,
    scrollToBottom: () => termRef.current?.scrollToBottom(),
    seedReady,
    announce,
  });

  // first-run coachmark (ADR 0072): shown once on the initial view-mode entry.
  const coach = useCoachmarkOnce({ active: !input.active });

  // Active session の driver を引いて DriverShortcutBar に渡す.
  // sessions / activeSessionID は安定参照 (applyViewUpdate が identity 保全)
  // なので無駄な再 render は起きない.
  const activeDriver = useDaemonStore((s) => {
    if (!s.activeSessionID) return null;
    const sess = s.sessions.find((x) => x.id === s.activeSessionID);
    return sess?.root_driver ?? null;
  });

  return (
    <>
      {/* FR-MOB-FAB-004 fixed stack; absolute sibling of terminal-host (UAC-025). */}
      <div ref={layerRef} className="terminal-fab-layer" data-overlay="true">
        <JumpToLatestFAB show={jump.shouldShowFab} onJump={jump.jumpToBottom} />
        <KeyboardFAB active={input.active} onToggle={input.toggle} />
        <FontSizeControl
          fontSize={font.fontSize}
          onIncrease={font.increase}
          onDecrease={font.decrease}
          onReset={() => font.reset()}
        />
        <DriverShortcutBar driver={activeDriver} inputActive={input.active} sendInput={sendInput} />
        {coach.showCoachmark && <Coachmark onDismiss={coach.dismiss} />}
        <AriaLiveStatus />
      </div>
    </>
  );
}
