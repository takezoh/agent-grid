// useMobileGate — mobile UX gate hook (FR-MOB-GATE-001 / FR-MOB-GATE-002, ADR 0067).
//
// The single boolean truth source for "are we in the mobile experience?" is
//   matchMedia('(max-width: 767px) and (pointer: coarse)').matches
// an AND contract (ADR 0067): a narrow *desktop* window (≤767px but
// pointer:fine) must NOT be treated as mobile, and a touch screen attached to a
// wide PC must NOT either. CSS `display:none` hiding is forbidden — the gate is
// the truth source for *conditional render* so mobile overlays never appear in
// the PC a11y tree.
//
// SSR / environments without `window.matchMedia` (old browsers) fall back to a
// hard `false` so the legacy PC path is never perturbed (FR-PC-PRESERVE-*).
//
// This hook intentionally stays a pure boolean + a `true→false` transition
// notification (`onLeaveMobile`). The full ordered teardown mandated by
// FR-MOB-GATE-002 (unsubscribe → discard input-mode state → unmount overlays →
// release helper textarea readonly) is assembled by the TerminalPane wiring in
// a later chunk; this hook only supplies the edge signal without dictating that
// order.

import { useEffect, useRef, useState } from "react";

/** The AND-contract media query that defines the mobile gate (ADR 0067). */
export const MOBILE_GATE_QUERY = "(max-width: 767px) and (pointer: coarse)";

export interface UseMobileGateOptions {
  /**
   * Fired exactly on a `true → false` gate transition (e.g. device rotation
   * crossing the 767px boundary, or a pointer becoming fine). Never fired on
   * mount, on `false → true`, or on `false → false`.
   */
  onLeaveMobile?: () => void;
}

/** True only when `window.matchMedia` is usable in this environment. */
function hasMatchMedia(): boolean {
  return typeof window !== "undefined" && typeof window.matchMedia === "function";
}

/** Evaluate the gate now; `false` when matchMedia is unavailable (SSR/legacy). */
function evaluateGate(): boolean {
  if (!hasMatchMedia()) return false;
  return window.matchMedia(MOBILE_GATE_QUERY).matches;
}

/**
 * useMobileGate returns the current gate boolean and subscribes to
 * `MediaQueryList` change events. `options.onLeaveMobile` is invoked on the
 * `true → false` edge only.
 */
export function useMobileGate(options?: UseMobileGateOptions): boolean {
  const [matches, setMatches] = useState<boolean>(evaluateGate);

  // Keep the latest callback without forcing a re-subscribe.
  const onLeaveRef = useRef<(() => void) | undefined>(options?.onLeaveMobile);
  onLeaveRef.current = options?.onLeaveMobile;

  // Track the previously-observed value so we can detect the true→false edge.
  const prevRef = useRef<boolean>(matches);

  useEffect(() => {
    if (!hasMatchMedia()) return;
    const mql = window.matchMedia(MOBILE_GATE_QUERY);

    const apply = (next: boolean): void => {
      const wasMobile = prevRef.current;
      prevRef.current = next;
      setMatches(next);
      if (wasMobile && !next) {
        onLeaveRef.current?.();
      }
    };

    const listener = (event: MediaQueryListEvent): void => apply(event.matches);

    // Reconcile any change that happened between render and effect commit.
    if (mql.matches !== prevRef.current) apply(mql.matches);

    if (typeof mql.addEventListener === "function") {
      mql.addEventListener("change", listener);
    } else {
      // Old Safari (<14) only exposes the deprecated addListener API.
      mql.addListener(listener);
    }

    return () => {
      if (typeof mql.removeEventListener === "function") {
        mql.removeEventListener("change", listener);
      } else {
        mql.removeListener(listener);
      }
    };
    // Empty deps: matchMedia query is constant; callback updates flow via ref.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return matches;
}
