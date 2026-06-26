// useInputMode — the single truth source for 閲覧 (view) vs 入力 (input) mode on
// the mobile terminal (FR-MOB-MODE-001..006, ADR 0068 / 0073).
//
// The boolean `active` drives `data-input-active` on the terminal-host wrapper
// and the helper textarea's `readonly` / focus. Every transition flows through a
// *pure reducer* so the four exit paths (blur / Esc / outside-tap / gate-false)
// and the FAB toggle are reasoned about without timing or DOM in the loop:
//
//   - enter:        view → input. removes readonly, focuses, data-input-active='true'.
//   - exit(reason): input → view. re-adds readonly, blurs, data-input-active='false'.
//                   reason ∈ {blur, esc, outside-tap, fab, gate-false}.
//                   Only blur / esc announce '閲覧モードに戻りました' (FR-MOB-MODE-006);
//                   the others are silent (user-initiated, no SR surprise).
//   - toggle:       flips; entering is silent, exiting via toggle is silent (FAB).
//
// Idempotency is the load-bearing invariant against the counterexample-B race
// (FR-MOB-MODE-003): an `exit` while already in view mode is a no-op that emits
// no message and changes no reference, so a stray blur fired by the FAB stealing
// focus cannot produce a phantom transition or a duplicate announcement.

import { type RefObject, useCallback, useEffect, useReducer, useRef } from "react";

/** Message announced (once, debounced) when the user is dropped back to view mode. */
export const VIEW_MODE_ANNOUNCEMENT = "閲覧モードに戻りました";

/** Every way input mode can end. Only `blur` / `esc` announce. */
export type ExitReason = "blur" | "esc" | "outside-tap" | "fab" | "gate-false";

export type InputModeAction =
  | { type: "toggle" }
  | { type: "enter" }
  | { type: "exit"; reason: ExitReason };

export interface InputModeState {
  /** true = 入力モード (keyboard intended), false = 閲覧モード. */
  active: boolean;
  /** Pending AriaLive text for this transition, or null when nothing to announce. */
  lastMessage: string | null;
}

export const INITIAL_INPUT_MODE_STATE: InputModeState = { active: false, lastMessage: null };

/** Reasons that produce a screen-reader announcement on exit (FR-MOB-MODE-006). */
function announces(reason: ExitReason): boolean {
  return reason === "blur" || reason === "esc";
}

/**
 * inputModeReducer — pure state machine for the mode toggle. No DOM, no timers.
 */
export function inputModeReducer(state: InputModeState, action: InputModeAction): InputModeState {
  switch (action.type) {
    case "enter":
      // Already in input mode → no-op (idempotent, no spurious focus churn).
      if (state.active) return state;
      return { active: true, lastMessage: null };

    case "toggle":
      if (state.active) {
        // Toggle-out is the FAB path: silent, no announcement.
        return { active: false, lastMessage: null };
      }
      return { active: true, lastMessage: null };

    case "exit":
      // Idempotency guard: exiting while already in view mode must NOT re-announce
      // (kills the counterexample-B duplicate '閲覧モードに戻りました' from a FAB
      // pointerdown blur arriving after the user already left input mode).
      if (!state.active) return state;
      return {
        active: false,
        lastMessage: announces(action.reason) ? VIEW_MODE_ANNOUNCEMENT : null,
      };

    default:
      return state;
  }
}

export interface UseInputModeOptions {
  /** terminal-host wrapper that carries `data-input-active`. */
  hostRef: RefObject<HTMLElement | null>;
  /** `.xterm-helper-textarea` (tests inject a mock textarea). */
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  /** AriaLive sink — called once per blur/Esc exit (debounced downstream). */
  announce?: (text: string) => void;
}

export interface UseInputModeApi {
  /** Current mode: true = 入力, false = 閲覧. */
  active: boolean;
  /** FAB toggle (FR-MOB-MODE-003 / 004). */
  toggle: () => void;
  /** Explicit enter (helper textarea direct focus path). */
  enter: () => void;
  /** Exit with a cause; only blur/esc announce. */
  exit: (reason: ExitReason) => void;
}

/**
 * useInputMode wires the pure reducer to the DOM: it owns the helper textarea's
 * `readonly` + focus/blur, stamps `data-input-active` on the host, subscribes to
 * the textarea `blur` and document `Escape` exit sources while active, and routes
 * the announcement to `announce` exactly once per blur/Esc exit.
 */
export function useInputMode(options: UseInputModeOptions): UseInputModeApi {
  const { hostRef, textareaRef, announce } = options;
  const [state, dispatch] = useReducer(inputModeReducer, INITIAL_INPUT_MODE_STATE);

  const announceRef = useRef<typeof announce>(announce);
  announceRef.current = announce;

  const toggle = useCallback(() => dispatch({ type: "toggle" }), []);
  const enter = useCallback(() => dispatch({ type: "enter" }), []);
  const exit = useCallback((reason: ExitReason) => dispatch({ type: "exit", reason }), []);

  // Reflect `active` onto the DOM: data-input-active + readonly + focus/blur.
  useEffect(() => {
    const host = hostRef.current;
    if (host) host.setAttribute("data-input-active", state.active ? "true" : "false");

    const ta = textareaRef.current;
    if (!ta) return;
    if (state.active) {
      ta.removeAttribute("readonly");
      ta.focus();
    } else {
      ta.setAttribute("readonly", "");
      ta.blur();
    }
  }, [state.active, hostRef, textareaRef]);

  // Subscribe to blur / Escape exit sources only while in input mode.
  useEffect(() => {
    if (!state.active) return;
    const ta = textareaRef.current;

    const onBlur = (): void => dispatch({ type: "exit", reason: "blur" });
    const onKeyDown = (e: KeyboardEvent): void => {
      if (e.key === "Escape") dispatch({ type: "exit", reason: "esc" });
    };

    ta?.addEventListener("blur", onBlur);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      ta?.removeEventListener("blur", onBlur);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [state.active, textareaRef]);

  // Emit the AriaLive message exactly when the reducer produced one.
  useEffect(() => {
    if (state.lastMessage) announceRef.current?.(state.lastMessage);
  }, [state.lastMessage]);

  return { active: state.active, toggle, enter, exit };
}
