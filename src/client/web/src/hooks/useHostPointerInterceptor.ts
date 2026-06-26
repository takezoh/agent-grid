// useHostPointerInterceptor — the single capture-phase pointerdown listener on
// the terminal-host that both (a) blocks focus-steal in 閲覧 mode and (b) detects
// outside-tap in 入力 mode (FR-MOB-MODE-002 / 005, ADR 0068).
//
// Why one listener: focus-block and outside-tap both hinge on the very first
// pointerdown reaching the host. Splitting them into two listeners (or two
// phases) re-introduces the race the ADR set out to kill — one could call
// preventDefault while the other had already let focus move. Collapsing both
// into a single capture-phase handler makes the ordering total and provable:
//
//   - 閲覧 mode  → e.preventDefault(): the synthesized mousedown/focus that would
//                 move the caret into the helper textarea never happens, so the
//                 soft keyboard stays closed (focus dispatch count = 0).
//   - 入力 mode  → if the target is the helper textarea or inside a [data-overlay]
//                 (FAB / popover), keep input mode; otherwise it's an outside-tap
//                 and we exit via the supplied callback.
//
// The listener is attached exactly once (asserted by spying on addEventListener),
// in the capture phase, so nothing downstream can pre-empt it.

import { type RefObject, useEffect, useRef } from "react";

export interface UseHostPointerInterceptorOptions {
  /** terminal-host element the listener attaches to. */
  hostRef: RefObject<HTMLElement | null>;
  /** `.xterm-helper-textarea` — tapping it in input mode keeps input mode. */
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  /** Current input-mode flag, read lazily so the listener never re-subscribes. */
  isActive: () => boolean;
  /** Invoked for an outside-tap while in input mode (→ exit('outside-tap')). */
  onOutsideTap: () => void;
}

/** True when `target` is the helper textarea or a descendant of it. */
function isHelperTarget(target: Element | null, ta: HTMLTextAreaElement | null): boolean {
  if (!target || !ta) return false;
  return target === ta || ta.contains(target);
}

/** True when `target` sits inside an opted-out overlay (FAB / popover). */
function isOverlayTarget(target: Element | null): boolean {
  return !!target && typeof target.closest === "function" && !!target.closest("[data-overlay]");
}

/**
 * useHostPointerInterceptor attaches one capture-phase `pointerdown` handler to
 * `hostRef.current` for the lifetime of the component.
 */
export function useHostPointerInterceptor(options: UseHostPointerInterceptorOptions): void {
  const { hostRef, textareaRef, isActive, onOutsideTap } = options;

  // Route the volatile callbacks through refs so the listener attaches once and
  // never re-subscribes (the "1 系統だけ" contract).
  const isActiveRef = useRef(isActive);
  isActiveRef.current = isActive;
  const onOutsideTapRef = useRef(onOutsideTap);
  onOutsideTapRef.current = onOutsideTap;

  // Attach-once is the "1 系統だけ" contract: hostRef / textareaRef identities are
  // stable and the volatile callbacks are read through refs, so an empty dep
  // array is correct here.
  // biome-ignore lint/correctness/useExhaustiveDependencies: attach-once listener; refs are stable.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const handler = (event: Event): void => {
      const target = (event.target as Element | null) ?? null;

      if (!isActiveRef.current()) {
        // 閲覧 mode: stop the pointer from moving focus into the helper textarea.
        event.preventDefault();
        return;
      }

      // 入力 mode: a tap on the helper textarea or an overlay keeps input mode.
      if (isHelperTarget(target, textareaRef.current)) return;
      if (isOverlayTarget(target)) return;

      // Everything else is an outside-tap → leave input mode.
      onOutsideTapRef.current();
    };

    host.addEventListener("pointerdown", handler, { capture: true });
    return () => host.removeEventListener("pointerdown", handler, { capture: true });
  }, []);
}
