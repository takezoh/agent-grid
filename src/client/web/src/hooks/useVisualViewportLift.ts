// useVisualViewportLift — keeps the mobile FAB stack above the iOS soft keyboard
// by writing one CSS custom property, never a React re-render (ADR 0069,
// FR-MOB-VVP-001/002/003).
//
// iOS Safari overlays the soft keyboard without shrinking the layout viewport
// (dvh/vh stay put), so the only way to know how much screen the keyboard ate is
// `window.visualViewport`. While input mode is active we subscribe to its
// `resize` / `scroll` events and stamp `--terminal-fab-offset` on the single
// `.terminal-fab-layer` element. Every FAB reads `bottom: var(--terminal-fab-offset)`
// in CSS, so one property write fans out to all of them with zero React renders
// (the ADR 0069 one-write-to-many fan-out contract).
//
// Three load-bearing invariants:
//   1. subscribe only while `active` (input mode) — a view-mode terminal never
//      touches visualViewport listeners.
//   2. on teardown (exit input mode OR gate true→false rotation) the listeners
//      are removed BEFORE the inline property is dropped, so a late event can
//      never re-stamp a stale offset after teardown began (FR-MOB-VVP-003 order).
//   3. when `window.visualViewport` is absent (legacy browsers / SSR) the hook is
//      a no-op: the CSS default `16px` is the fallback and JS writes nothing
//      (FR-MOB-VVP-002).

import { type RefObject, useEffect } from "react";

/** The inline custom property every FAB anchors its `bottom` to. */
export const FAB_OFFSET_PROP = "--terminal-fab-offset";
/** Base inset (px) used both as the CSS default and the lift floor. */
export const FAB_OFFSET_BASE_PX = 16;

export interface UseVisualViewportLiftOptions {
  /** The `.terminal-fab-layer` element whose inline custom property is updated. */
  layerRef: RefObject<HTMLElement | null>;
  /** True only while in input mode; toggles the subscription on/off. */
  active: boolean;
}

/**
 * computeFabOffset — `innerHeight - vv.height - vv.offsetTop + 16`, floored at the
 * 16px base so a closed keyboard yields exactly the CSS default and an open one
 * lifts the stack by the eaten height. Pure but reads `window.visualViewport`.
 */
export function computeFabOffset(): number {
  const vv = window.visualViewport;
  if (!vv) return FAB_OFFSET_BASE_PX;
  const raw = window.innerHeight - vv.height - vv.offsetTop + FAB_OFFSET_BASE_PX;
  return Math.max(FAB_OFFSET_BASE_PX, raw);
}

/**
 * useVisualViewportLift subscribes to visualViewport metrics while `active` and
 * mirrors the keyboard inset into `--terminal-fab-offset` on `layerRef.current`.
 */
export function useVisualViewportLift({ layerRef, active }: UseVisualViewportLiftOptions): void {
  useEffect(() => {
    if (!active) return;
    const vv = window.visualViewport;
    // FR-MOB-VVP-002: no visualViewport → leave the CSS default 16px untouched.
    if (!vv) return;

    const update = (): void => {
      const el = layerRef.current;
      if (!el) return;
      el.style.setProperty(FAB_OFFSET_PROP, `${computeFabOffset()}px`);
    };

    // Stamp immediately so the very first input-mode frame is already lifted.
    update();
    vv.addEventListener("resize", update);
    vv.addEventListener("scroll", update);

    return () => {
      // FR-MOB-VVP-003: unsubscribe BEFORE discarding the inline state so a late
      // resize/scroll cannot re-stamp an offset after teardown has begun.
      vv.removeEventListener("resize", update);
      vv.removeEventListener("scroll", update);
      layerRef.current?.style.removeProperty(FAB_OFFSET_PROP);
    };
  }, [active, layerRef]);
}
