// PaletteFooter — single-line context + key hints (FR-026).
//
// Replaces the old ActiveContextHeader row and title chrome. Context flashes
// on active session change. Session options (worktree / host) live in the
// param phase actions row next to the confirm button; closing is Esc or an
// overlay click — the footer carries no chrome buttons besides Back.

import type { KeyboardEvent } from "react";
import { useEffect, useRef, useState } from "react";
import { usePaletteStore } from "../../store/palette";
import type { ActiveContextSnapshot } from "../../store/palette_active_context";

export interface PaletteFooterProps {
  phase: "toolSelect" | "paramSelect";
  snapshot?: ActiveContextSnapshot;
  flashSeq?: number;
  statusText?: string | null;
  submitting?: boolean;
  composing: boolean;
  onBack: () => void;
  onClose: () => void;
}

const FLASH_MS = 600;

function contextLabel(snap: ActiveContextSnapshot): string {
  if (snap.kind === "none") return "No active session";
  if (snap.kind === "unknown") return `??? / ${snap.sid8}`;
  return `${snap.projBase} / ${snap.sid8}`;
}

function contextTitle(snap: ActiveContextSnapshot): string | undefined {
  if (snap.kind === "resolved") return `${snap.fullPath}\n${snap.fullSessionId}`;
  if (snap.kind === "unknown") return snap.fullSessionId;
  return undefined;
}

export function PaletteFooter(props: PaletteFooterProps): JSX.Element {
  const storeSnapshot = usePaletteStore((s) => s.activeContextSnapshot);
  const storeFlashSeq = usePaletteStore((s) => s.flashSeq);
  const snap = props.snapshot ?? storeSnapshot;
  const flashSeq = props.flashSeq ?? storeFlashSeq;

  const [flashing, setFlashing] = useState(false);
  const lastSeqRef = useRef(flashSeq);

  useEffect(() => {
    if (lastSeqRef.current === flashSeq) return;
    lastSeqRef.current = flashSeq;
    setFlashing(true);
    const id = setTimeout(() => setFlashing(false), FLASH_MS);
    return () => clearTimeout(id);
  }, [flashSeq]);

  const contextClass = `palette-footer__context${
    flashing ? " palette-footer__context--flash" : ""
  }`;

  function onFooterKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    if (e.key === "Escape") {
      e.preventDefault();
      props.onClose();
    }
  }

  return (
    <footer className="palette-footer" data-testid="palette-footer" onKeyDown={onFooterKeyDown}>
      <div className="palette-footer__main">
        {props.phase === "paramSelect" && (
          <button
            type="button"
            aria-label="Back"
            className="palette-footer__back"
            onClick={props.onBack}
            data-testid="palette-back"
          >
            ←
          </button>
        )}
        <output
          className={contextClass}
          title={contextTitle(snap)}
          data-testid="palette-active-context"
        >
          <span className="palette-footer__context-text">{contextLabel(snap)}</span>
        </output>
        {props.statusText !== null && props.statusText !== undefined && (
          <span className="palette-footer__status" data-testid="palette-progress">
            {props.submitting && (
              <span aria-hidden="true" className="palette-footer__status-spinner">
                ⟳{" "}
              </span>
            )}
            {props.statusText}
          </span>
        )}
      </div>
      <p className="palette-footer__hints" aria-hidden="true">
        ↑↓ navigate · Enter select · Esc close
      </p>
    </footer>
  );
}
