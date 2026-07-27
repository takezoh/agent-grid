// useSessionChangeFeedback — announcer on active session change (FR-027).
//
// When announceSeq increments and submitting is false, updates the palette
// announcer message.

import { useEffect, useReducer, useRef } from "react";
import type { ActiveContextSnapshot } from "../../../store/palette_active_context";

function announceMessage(snap: ActiveContextSnapshot): string {
  if (snap.kind === "none") return "Active session cleared";
  if (snap.kind === "unknown") return `Active session changed to unknown / ${snap.sid8}`;
  return `Active session changed to ${snap.projBase} / ${snap.sid8}`;
}

/**
 * Returns a ref holding the latest announce message (or undefined).
 */
export function useSessionChangeFeedback(
  announceSeq: number,
  submitting: boolean,
  activeContextSnapshot: ActiveContextSnapshot,
): React.MutableRefObject<{ message: string; seq: number } | undefined> {
  const announceRef = useRef<{ message: string; seq: number } | undefined>(undefined);
  const prevAnnounceSeqRef = useRef(announceSeq);
  const [, forceRender] = useReducer((revision: number) => revision + 1, 0);

  useEffect(() => {
    if (prevAnnounceSeqRef.current === announceSeq || submitting) return;
    prevAnnounceSeqRef.current = announceSeq;
    const message = announceMessage(activeContextSnapshot);
    announceRef.current = { message, seq: announceSeq };
    forceRender();
  }, [announceSeq, submitting, activeContextSnapshot]);

  return announceRef;
}
