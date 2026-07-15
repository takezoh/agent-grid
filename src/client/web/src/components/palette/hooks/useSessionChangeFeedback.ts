// useSessionChangeFeedback — toast + announcer on active session change (FR-027).
//
// When announceSeq increments and submitting is false, emits a notifications
// toast ("Switched to <label>") alongside the palette announcer message.

import { useEffect, useReducer, useRef } from "react";
import { useNotificationsStore } from "../../../store/notifications";
import type { ActiveContextSnapshot } from "../../../store/palette_active_context";

function sessionLabel(snap: ActiveContextSnapshot): string {
  if (snap.kind === "none") return "No active session";
  if (snap.kind === "unknown") return `unknown / ${snap.sid8}`;
  return `${snap.projBase} / ${snap.sid8}`;
}

function announceMessage(snap: ActiveContextSnapshot): string {
  if (snap.kind === "none") return "Active session cleared";
  if (snap.kind === "unknown") return `Active session changed to unknown / ${snap.sid8}`;
  return `Active session changed to ${snap.projBase} / ${snap.sid8}`;
}

/**
 * Returns a ref holding the latest announce message (or undefined).
 * Also routes a notifications toast when announceSeq increments (FR-027).
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
    useNotificationsStore.getState().add({
      level: "info",
      message: `Switched to ${sessionLabel(activeContextSnapshot)}`,
    });
  }, [announceSeq, submitting, activeContextSnapshot]);

  return announceRef;
}
