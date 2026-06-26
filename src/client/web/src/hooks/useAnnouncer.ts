// useAnnouncer — Context-distributed setText API for the terminal AriaLive slot
// (FR-MOB-MODE-006, ADR 0073).
//
// `AriaLiveStatus` renders the live region; any child (KeyboardFAB exit, host
// interceptor outside-tap, gate teardown) announces through `useAnnouncer()`.
// To stop screen-reader ear-fatigue from kinetic scroll / repeated mode flips,
// identical consecutive text is debounced for 1.5s: the same string within the
// window is a no-op, a different string emits immediately, and the same string
// after the window emits again. A monotonically increasing `seq` lets the live
// region remount its text node so even an identical re-emit reaches the SR.

import {
  type ReactNode,
  createContext,
  createElement,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
} from "react";

/** Debounce window for suppressing identical consecutive announcements. */
export const ANNOUNCER_DEBOUNCE_MS = 1500;

export interface AnnouncerContextValue {
  /** Current live-region text ("" = empty / nothing announced). */
  text: string;
  /** Bumps on every accepted emit so the live region can remount identical text. */
  seq: number;
  /** Announce `text`, suppressing an identical string within ANNOUNCER_DEBOUNCE_MS. */
  announce: (text: string) => void;
}

const AnnouncerContext = createContext<AnnouncerContextValue | null>(null);

export interface AnnouncerProviderProps {
  children: ReactNode;
}

/**
 * AnnouncerProvider owns the live-region text + debounce bookkeeping and shares
 * the `announce` API via Context.
 */
export function AnnouncerProvider(props: AnnouncerProviderProps): ReactNode {
  const [state, setState] = useState<{ text: string; seq: number }>({ text: "", seq: 0 });

  // Last accepted emit, for the identical-text debounce.
  const lastRef = useRef<{ text: string; ts: number }>({
    text: "",
    ts: Number.NEGATIVE_INFINITY,
  });

  const announce = useCallback((next: string): void => {
    const now = Date.now();
    const last = lastRef.current;
    if (next === last.text && now - last.ts < ANNOUNCER_DEBOUNCE_MS) {
      // Identical text within the window — suppress to spare the SR.
      return;
    }
    lastRef.current = { text: next, ts: now };
    setState((prev) => ({ text: next, seq: prev.seq + 1 }));
  }, []);

  const value = useMemo<AnnouncerContextValue>(
    () => ({ text: state.text, seq: state.seq, announce }),
    [state.text, state.seq, announce],
  );

  return createElement(AnnouncerContext.Provider, { value }, props.children);
}

/**
 * useAnnouncer reads the announcer Context. Throws if used outside an
 * AnnouncerProvider so a missing provider fails loudly rather than silently
 * dropping mode announcements.
 */
export function useAnnouncer(): AnnouncerContextValue {
  const ctx = useContext(AnnouncerContext);
  if (ctx === null) {
    throw new Error("useAnnouncer must be used within an <AnnouncerProvider>.");
  }
  return ctx;
}
