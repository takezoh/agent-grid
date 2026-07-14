// PaletteAnnouncer — visually-hidden aria-live slot for the command palette.
//
// FR-027 / ADR-0057: exactly one aria-live region inside the palette. Session
// switch and disabled-command feedback announce here; no persistent visible
// status row (web-ui-refresh m5 / FR-026).
//
// FR-031: key={reannounceKey} drives remount so same-message re-emits still
// reach the screen reader.
// FR-033: inlineStatus.seq change replaces the DOM text child.

import { useEffect, useState } from "react";
import { usePaletteStore } from "../../store/palette";

export interface PaletteAnnouncerProps {
  // CommandPalette flows session-change and disabled-feedback announce through
  // here (ADR-0057 single slot). seq drives value-change detection.
  announce?: { message: string; seq: number };
}

export function PaletteAnnouncer(props: PaletteAnnouncerProps = {}): JSX.Element {
  const inlineStatus = usePaletteStore((s) => s.inlineStatus);

  const [lastAnnounceSeq, setLastAnnounceSeq] = useState(-1);
  const [showingAnnounce, setShowingAnnounce] = useState(props.announce != null);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional — only seq drives re-announce
  useEffect(() => {
    if (!props.announce) return;
    if (props.announce.seq === lastAnnounceSeq) return;
    setLastAnnounceSeq(props.announce.seq);
    setShowingAnnounce(true);
    const id = setTimeout(() => setShowingAnnounce(false), 4000);
    return () => clearTimeout(id);
  }, [props.announce?.seq]);

  const text = showingAnnounce && props.announce ? props.announce.message : inlineStatus.message;

  const reannounceKey =
    showingAnnounce && props.announce ? `a-${props.announce.seq}` : `s-${inlineStatus.seq}`;

  return (
    <output
      className="palette-announcer"
      aria-live="polite"
      aria-atomic="true"
      data-testid="palette-inline-status"
    >
      {text === "" ? null : (
        <span key={reannounceKey} data-seq={reannounceKey}>
          {text}
        </span>
      )}
    </output>
  );
}
