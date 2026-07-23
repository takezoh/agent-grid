// FontSizeControl — the non-pinch fontSize affordance (FR-MOB-STEPPER-001,
// ADR 0070 / 0075, WCAG 2.5.5 / 4.1.2).
//
// VoiceOver / TalkBack capture two-finger gestures for their own commands, so a
// pinch-only fontSize control is unreachable for AT users (UAC-020 counterexample).
// This disclosure popover is the always-reachable alternative: an "Aa" trigger
// (one IconButton) opens a popover exposing -, +, and Reset, each a thin IconButton
// wrapper so the 44×44 target, real <button role=button>, and non-empty aria-label
// are inherited from the one primitive (chunk-02). The +/-/Reset semantics (±2px,
// reset-to-14, scheduleFit on every activate) live in useFontSize and are passed in
// as callbacks so a single hook instance can be shared with the pinch path (chunk-07).

import { type JSX, useState } from "react";
import "../css/font-size-control.css";
import { IconButton } from "./primitives/IconButton";

export interface FontSizeControlProps {
  /** Current fontSize in px (shown in the popover readout). */
  fontSize: number;
  /** +2px (useFontSize.increase). */
  onIncrease: () => void;
  /** -2px (useFontSize.decrease). */
  onDecrease: () => void;
  /** Reset to 14px (useFontSize.reset). */
  onReset: () => void;
}

export function FontSizeControl({
  fontSize,
  onIncrease,
  onDecrease,
  onReset,
}: FontSizeControlProps): JSX.Element {
  const [open, setOpen] = useState(false);

  return (
    // data-overlay marks a tap here as an overlay tap (not an outside-tap) for the
    // host pointer interceptor, matching KeyboardFAB.
    <div className="font-size-control" data-overlay="">
      <IconButton
        className="font-size-control__trigger"
        aria-label="Font size"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        icon={<span aria-hidden="true">Aa</span>}
      />
      {open && (
        <div className="font-size-control__popover">
          <IconButton
            className="font-size-control__btn"
            aria-label="Decrease font size"
            onClick={onDecrease}
            icon={<span aria-hidden="true">−</span>}
          />
          <span className="font-size-control__value" aria-live="polite">
            {fontSize}px
          </span>
          <IconButton
            className="font-size-control__btn"
            aria-label="Increase font size"
            onClick={onIncrease}
            icon={<span aria-hidden="true">＋</span>}
          />
          <IconButton
            className="font-size-control__btn font-size-control__btn--reset"
            aria-label="Reset font size"
            onClick={onReset}
            icon={<span aria-hidden="true">Reset</span>}
          />
        </div>
      )}
    </div>
  );
}
