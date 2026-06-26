// PinchIndicator — centre-screen live fontSize readout during a pinch
// (FR-MOB-PINCH-004, ADR 0063 / 0070).
//
// It does NOT introduce a new toast primitive (ADR 0063 non-breaking): it reuses
// NotificationToast in its `ariaHidden` mode so the readout is purely visual and
// never announced (a pinch is a continuous gesture; announcing every frame would
// flood the screen reader). While the pinch is active the readout is fully shown;
// ~800ms after touchend it fades out and unmounts. Tapping the readout resets the
// fontSize to the default (the caller wires `reset(14)` + `scheduleFit()`).

import { type JSX, useEffect, useState } from "react";
import "../css/font-size-control.css";
import { NotificationToast } from "./NotificationToast";

/** Fade delay after the pinch ends before the indicator disappears (ms). */
export const PINCH_FADE_MS = 800;

export interface PinchIndicatorProps {
  /** Current fontSize in px to display. */
  fontSize: number;
  /** True while the pinch gesture is in progress (kept visible). */
  active: boolean;
  /** Tap handler — caller composes useFontSize.reset(14) + scheduleFit(). */
  onReset: () => void;
}

export function PinchIndicator({
  fontSize,
  active,
  onReset,
}: PinchIndicatorProps): JSX.Element | null {
  const [shown, setShown] = useState(active);
  const [fading, setFading] = useState(false);

  useEffect(() => {
    if (active) {
      // Pinch (re)started: show immediately and cancel any pending fade.
      setShown(true);
      setFading(false);
      return;
    }
    // Pinch ended: begin the ~800ms fade, then unmount.
    setFading(true);
    const t = setTimeout(() => {
      setShown(false);
      setFading(false);
    }, PINCH_FADE_MS);
    return () => clearTimeout(t);
  }, [active]);

  if (!shown) return null;

  const cls = `pinch-indicator${fading ? " pinch-indicator--fading" : ""}`;

  return (
    <NotificationToast ariaHidden>
      {/* A real <button> keeps tap activation accessible to pointer users; it sits
          inside the aria-hidden surface and is removed from the tab order because
          keyboard users reach fontSize via FontSizeControl (the non-pinch path). */}
      <button type="button" className={cls} tabIndex={-1} onClick={onReset}>
        {fontSize}px
      </button>
    </NotificationToast>
  );
}
