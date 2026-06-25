/**
 * swipe.ts — touch swipe detection utilities (FR-DRAWER-005 / ADR-0060).
 *
 * Pure functions only; no DOM side-effects.
 * Used by SessionDrawer to detect left→right swipe for cancel-close.
 */

/** A 2-D coordinate pair extracted from a touch event. */
export type SwipePoint = { x: number; y: number };

/** Minimal interface used by pointFromTouch — compatible with both native Touch and React.Touch. */
export interface TouchLike {
  clientX: number;
  clientY: number;
}

/**
 * Extract a SwipePoint from a Touch-like object (clientX / clientY).
 * Accepts both native Touch and React's synthetic Touch type.
 */
export function pointFromTouch(t: TouchLike): SwipePoint {
  return { x: t.clientX, y: t.clientY };
}

/**
 * Return the delta between start and end coordinates.
 * dx > 0 = right, dx < 0 = left; dy > 0 = down, dy < 0 = up.
 */
export function swipeDelta(start: SwipePoint, end: SwipePoint): { dx: number; dy: number } {
  return { dx: end.x - start.x, dy: end.y - start.y };
}

/**
 * Return true when the gesture from start → end qualifies as a left-to-right
 * swipe per FR-DRAWER-005 thresholds:
 *   horizontal distance >= minDx (default 50)
 *   |vertical drift|   <  maxDy (default 30)
 */
export function isLeftToRightSwipe(
  start: SwipePoint,
  end: SwipePoint,
  opts?: { minDx?: number; maxDy?: number },
): boolean {
  const minDx = opts?.minDx ?? 50;
  const maxDy = opts?.maxDy ?? 30;
  const { dx, dy } = swipeDelta(start, end);
  return dx >= minDx && Math.abs(dy) < maxDy;
}
