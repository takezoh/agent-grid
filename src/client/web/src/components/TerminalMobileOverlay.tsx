// TerminalMobileOverlay — the single mount point that wires every mobile hook
// (chunk-02..06) onto the live xterm terminal and renders the FAB overlay layer
// (ADR 0069 / 0072 / 0074, FR-MOB-FAB-* / VVP-* / COACH-* / PINCH-*).
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
import { type JSX, type RefObject, useCallback, useEffect, useRef, useState } from "react";
import "../css/terminal-fab-layer.css";
import { AnnouncerProvider, useAnnouncer } from "../hooks/useAnnouncer";
import { useCoachmarkOnce } from "../hooks/useCoachmarkOnce";
import { useFontSize } from "../hooks/useFontSize";
import { useHostPointerInterceptor } from "../hooks/useHostPointerInterceptor";
import { useInputMode } from "../hooks/useInputMode";
import { useJumpToLatest } from "../hooks/useJumpToLatest";
import { type TerminalLike, useTerminalTouchGestures } from "../hooks/useTerminalTouchGestures";
import { useVisualViewportLift } from "../hooks/useVisualViewportLift";
import { AriaLiveStatus } from "./AriaLiveStatus";
import { Coachmark } from "./Coachmark";
import { FontSizeControl } from "./FontSizeControl";
import { JumpToLatestFAB } from "./JumpToLatestFAB";
import { KeyboardFAB } from "./KeyboardFAB";
import { PinchIndicator } from "./PinchIndicator";

/** How long after the last pinch frame the PinchIndicator stays "active". */
const PINCH_ACTIVE_LINGER_MS = 150;

export interface TerminalMobileOverlayProps {
  hostRef: RefObject<HTMLElement | null>;
  termRef: RefObject<Terminal | null>;
  viewportRef: RefObject<HTMLElement | null>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  /** ADR-0034 rAF-coalesced refit shared with the terminal lifecycle. */
  scheduleFit: () => void;
  /** ADR-0066 seed-flush completion; gates the jump-to-latest FAB. */
  seedReady: boolean;
}

export function TerminalMobileOverlay(props: TerminalMobileOverlayProps): JSX.Element {
  return (
    <AnnouncerProvider>
      <TerminalMobileLayer {...props} />
    </AnnouncerProvider>
  );
}

/**
 * useMobilePinch tracks whether a pinch is in progress so PinchIndicator can show
 * the live readout, lingering briefly after the last frame so the fade reads as
 * one gesture rather than flickering per touchmove.
 */
function useMobilePinch(applyPinch: (ratio: number) => void): {
  pinchActive: boolean;
  onPinchFontSize: (ratio: number) => void;
} {
  const [pinchActive, setPinchActive] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const onPinchFontSize = useCallback(
    (ratio: number): void => {
      applyPinch(ratio);
      setPinchActive(true);
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => setPinchActive(false), PINCH_ACTIVE_LINGER_MS);
    },
    [applyPinch],
  );

  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  return { pinchActive, onPinchFontSize };
}

function TerminalMobileLayer(props: TerminalMobileOverlayProps): JSX.Element {
  const { hostRef, termRef, viewportRef, textareaRef, scheduleFit, seedReady } = props;
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

  // fontSize (ADR 0070/0034): pinch + stepper share one clamp/persist/refit hook.
  const font = useFontSize({ scheduleFit });
  // Apply the resolved fontSize to the live terminal; scheduleFit (inside
  // useFontSize) then re-flows the grid (FR-MOB-PINCH-002 / FR-MOB-STEPPER-001).
  useEffect(() => {
    const term = termRef.current;
    if (term) term.options.fontSize = font.fontSize;
  }, [font.fontSize, termRef]);

  // pinch → fontSize (FR-MOB-PINCH-001/004): the gesture machine's ratio drives
  // useFontSize.applyPinch, completing the chunk-04 → chunk-05 wiring.
  const { pinchActive, onPinchFontSize } = useMobilePinch(font.applyPinch);
  useTerminalTouchGestures({
    viewportRef,
    termRef: termRef as RefObject<TerminalLike | null>,
    onPinchFontSize,
    scheduleFit,
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
        {coach.showCoachmark && <Coachmark onDismiss={coach.dismiss} />}
        <AriaLiveStatus />
      </div>
      {/* Separate Toast portal (ADR 0063 z-index isolation, FR-MOB-FAB-004). */}
      <PinchIndicator fontSize={font.fontSize} active={pinchActive} onReset={() => font.reset()} />
    </>
  );
}
