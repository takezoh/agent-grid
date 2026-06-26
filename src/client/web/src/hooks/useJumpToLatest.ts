// useJumpToLatest — drives the ↓最新 (jump-to-latest) FAB from the terminal
// viewport's scroll position (FR-MOB-JUMP-001..006, ADR 0073 / 0066 / 0064).
//
// The hook subscribes to `.xterm-viewport` scroll and decides whether the FAB
// should exist in the DOM at all (the component conditionally renders on
// `shouldShowFab`; CSS opacity:0 hiding is forbidden — UAC-012). It is the single
// truth source for three load-bearing contracts:
//
//   1. Tail detection (±2px): the viewport is "at tail" when
//      |scrollTop - (scrollHeight - clientHeight)| <= 2. At tail → no FAB; away
//      from tail → FAB. The 2px margin absorbs sub-pixel scrollTop on Retina
//      (DPR 2/3) without chattering (ADR 0073 §4).
//
//   2. Mode independence: the hook knows nothing about 閲覧/入力 mode, so the FAB
//      appears whenever the user is in scrollback regardless of input mode
//      (UAC-015).
//
//   3. Seed gating (FR-MOB-JUMP-005): while ADR 0066's two-phase scrollback seed
//      flush is incomplete (`seedReady=false`) the FAB is forced absent, and even
//      after the seed completes it stays absent until the first real scroll event
//      arrives — killing the late-join "FAB flicker" where the pre-seed
//      scrollTop=0 is mis-read as not-at-tail.
//
// On the false→true transition the hook announces '最新へ移動できます' once via the
// injected announcer; the announcer's 1.5s identical-text debounce (useAnnouncer)
// collapses the repeated transitions kinetic scroll produces into a single emit.

import { type RefObject, useCallback, useEffect, useRef, useState } from "react";

/** Polite live-region text announced when the FAB first appears (FR-MOB-JUMP-004). */
export const JUMP_FAB_ANNOUNCEMENT = "最新へ移動できます";

/** Tail-detection margin in CSS px (ADR 0073 §4). */
export const TAIL_THRESHOLD_PX = 2;

/** The reduced-motion media query (ADR 0064). */
export const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

/** How `jumpToBottom` asks the terminal to scroll. */
export type JumpBehavior = "instant" | "smooth";

/**
 * isAtTail — true when `scrollTop` is within TAIL_THRESHOLD_PX of the maximum
 * scroll offset `(scrollHeight - clientHeight)`. Pure, so the ±2px boundary can
 * be unit-tested without a DOM.
 */
export function isAtTail(scrollTop: number, scrollHeight: number, clientHeight: number): boolean {
  return Math.abs(scrollTop - (scrollHeight - clientHeight)) <= TAIL_THRESHOLD_PX;
}

export interface UseJumpToLatestOptions {
  /** The `.xterm-viewport` element whose scroll position is observed. */
  viewportRef: RefObject<HTMLElement | null>;
  /** Scroll the terminal to the bottom; `behavior` is reduced-motion aware. */
  scrollToBottom: (behavior: JumpBehavior) => void;
  /** ADR 0066 seed-flush completion signal; false forces the FAB absent. */
  seedReady: boolean;
  /** AriaLive sink (useAnnouncer.announce) — debounced downstream. */
  announce?: (text: string) => void;
}

export interface UseJumpToLatestApi {
  /** True only when the viewport is in scrollback past the ±2px tail margin. */
  shouldShowFab: boolean;
  /** Jump to the latest output (instant under reduced-motion, else smooth). */
  jumpToBottom: () => void;
}

/**
 * useJumpToLatest wires the viewport scroll listener to `shouldShowFab` and the
 * `jumpToBottom` action, applying the seed gate and the edge-triggered polite
 * announcement.
 */
export function useJumpToLatest(options: UseJumpToLatestOptions): UseJumpToLatestApi {
  const { viewportRef, scrollToBottom, seedReady, announce } = options;
  const [shouldShowFab, setShouldShowFab] = useState(false);

  const announceRef = useRef<typeof announce>(announce);
  announceRef.current = announce;

  // Mirror of shouldShowFab usable synchronously inside the scroll listener, so
  // the announcement fires exactly on the false→true edge (not on every scroll
  // that keeps us away from tail).
  const shownRef = useRef(false);

  useEffect(() => {
    const el = viewportRef.current;

    // Seed gate: until the ADR 0066 flush completes (or with no viewport) keep
    // the FAB forced-absent and do not subscribe yet. After the seed completes
    // we subscribe but the FAB stays false until the first scroll event lands
    // (追従中) — there is no initial evaluation here on purpose.
    if (!seedReady || !el) {
      shownRef.current = false;
      setShouldShowFab(false);
      return;
    }

    const onScroll = (): void => {
      const node = viewportRef.current;
      if (!node) return;
      const next = !isAtTail(node.scrollTop, node.scrollHeight, node.clientHeight);
      if (next && !shownRef.current) {
        // false→true edge: announce once. Identical re-emits within 1.5s are
        // suppressed by the announcer, so kinetic-scroll oscillation is quiet.
        announceRef.current?.(JUMP_FAB_ANNOUNCEMENT);
      }
      shownRef.current = next;
      setShouldShowFab(next);
    };

    el.addEventListener("scroll", onScroll);
    return () => el.removeEventListener("scroll", onScroll);
  }, [seedReady, viewportRef]);

  const jumpToBottom = useCallback((): void => {
    const reduce = window.matchMedia?.(REDUCED_MOTION_QUERY).matches ?? false;
    scrollToBottom(reduce ? "instant" : "smooth");
  }, [scrollToBottom]);

  return { shouldShowFab, jumpToBottom };
}
