// CommandPalette — overlay DOM owner for the command palette feature.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// Web-ui-refresh m5 (FR-026..030): 3-part layout — input, sectioned list, footer.
// FR:
//   - FR-003 focus trap + blur on open
//   - FR-007 role="dialog" / aria-modal
//   - FR-009 active context snapshot (setActiveContextSnapshot wiring)
//   - FR-010 session change → toast + announcer (FR-027)
//   - FR-012 frozenSnapshotRef capture on submitting false→true
//   - FR-013 frozenSnapshotRef release on submitting true→false
//   - FR-017 Esc steps back (paramSelect → toolSelect; toolSelect → close)
//   - FR-018 Back / Close / overlay outside-click dismissal
//   - FR-020 surfaces the submitting state (footer status)
//   - FR-023 close + restore opener focus when push becomes invalid
//   - FR-024 Status priority text in footer
//   - FR-025 footer status: ctx===null → 'Unavailable', loading → 'Loading commands...'
//   - FR-029 input.focus() driven by refocusSeq observation
//   - FR-033 announcer suppressed while submitting
// ADRs:
//   - 0030 terminal-keyed-remount
//   - 0036 palette-2phase-store-architecture
//   - 0039 palette-focus-trap-minimal
//   - 0050 unified listbox
//   - 0055 submit-freeze-lift-state
//   - 0057 PaletteAnnouncer single aria-live slot

import type { KeyboardEvent, MouseEvent } from "react";
import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useFocusTrap } from "../../hooks/useFocusTrap";
import type { ToolCtx } from "../../lib/tools";
import { listTools } from "../../lib/tools";
import { useDaemonStore } from "../../store/daemon";
import { usePaletteStore } from "../../store/palette";
import { useDaemonSnapshot } from "../../store/useDaemonSnapshot";
import { PaletteAnnouncer } from "./PaletteAnnouncer";
import { PaletteFooter } from "./PaletteFooter";
import { ParamSelectPhase } from "./ParamSelectPhase";
import { ToolSelectPhase } from "./ToolSelectPhase";
import { useActiveContextBridge } from "./hooks/useActiveContextBridge";
import { useFrozenSnapshot } from "./hooks/useFrozenSnapshot";
import { useSessionChangeFeedback } from "./hooks/useSessionChangeFeedback";
import { useToolCtx } from "./hooks/useToolCtx";

export interface CommandPaletteProps {
  // httpFactory swaps the SessionsApi for hermetic tests. Production callers
  // omit it and get makeSessionsApi() — same shape ToolSelectPhase honors.
  httpFactory?: () => ToolCtx["http"];
}

export function CommandPalette(props: CommandPaletteProps = {}): JSX.Element | null {
  const open = usePaletteStore((s) => s.open);
  const phase = usePaletteStore((s) => s.phase);
  const opener = usePaletteStore((s) => s.opener);
  const submitting = usePaletteStore((s) => s.submitting);
  const composing = usePaletteStore((s) => s.composing);
  const error = usePaletteStore((s) => s.error);
  const refocusSeq = usePaletteStore((s) => s.refocusSeq);
  const announceSeq = usePaletteStore((s) => s.announceSeq);
  const activeContextSnapshot = usePaletteStore((s) => s.activeContextSnapshot);
  const flashSeq = usePaletteStore((s) => s.flashSeq);
  const setActiveContextSnapshot = usePaletteStore((s) => s.setActiveContextSnapshot);

  const dialogRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const openerRef = useRef<HTMLElement | null>(null);
  if (opener !== null) openerRef.current = opener;

  useFocusTrap(dialogRef, open);

  useEffect(() => {
    if (!open) return;
    const active = document.activeElement as HTMLElement | null;
    if (active !== null && typeof active.blur === "function") {
      active.blur();
    } else {
      console.warn("[palette] open: activeElement not blurrable; FR-003 defocus is best-effort", {
        activeTag: active === null ? null : active.tagName,
      });
    }
    return () => {
      const o = openerRef.current;
      if (o === null) return;
      if (typeof o.focus !== "function") return;
      if (!document.contains(o)) {
        console.warn(
          "[palette] close: opener detached from DOM; focus restore skipped (FR-017/FR-023 unobservable)",
          { openerTag: o.tagName },
        );
        return;
      }
      o.focus();
      openerRef.current = null;
    };
  }, [open]);

  useEffect(() => {
    void refocusSeq;
    if (!open) return;
    const el = inputRef.current;
    if (el !== null) el.focus();
  }, [refocusSeq, open]);

  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const sessions = useDaemonStore((s) => s.sessions);
  const sessionConfig = useDaemonStore((s) => s.sessionConfig);
  const daemon = useDaemonSnapshot();

  const sessionConfigMissing = sessionConfig === null;
  useEffect(() => {
    if (!open) return;
    if (sessionConfigMissing) {
      console.warn(
        "[palette] sessionConfig not yet fetched; ParamSelectPhase will see empty projects/pushCommands until REST hydrate lands",
      );
    }
  }, [open, sessionConfigMissing]);

  useActiveContextBridge(
    submitting,
    activeSessionID,
    sessions,
    sessionConfig,
    setActiveContextSnapshot,
  );

  const { frozenSnapshotRef } = useFrozenSnapshot(
    submitting,
    daemon,
    activeContextSnapshot,
    flashSeq,
  );

  const announceRef = useSessionChangeFeedback(announceSeq, submitting, activeContextSnapshot);

  const frozenActiveContext = frozenSnapshotRef.current?.activeContext ?? undefined;
  const ctx = useToolCtx(daemon, props.httpFactory, frozenActiveContext);

  const liveStatusText: string | null = (() => {
    if (ctx === null) return "Unavailable";
    if (phase === "toolSelect") {
      const all = listTools(daemon, daemon.pushCommands);
      const enabledCount = all.filter((t) => t.disabledReason(daemon) === null).length;
      if (enabledCount === 0) {
        if (sessionConfig === null) return "Loading commands...";
        return "No commands available";
      }
    }
    if (submitting) return "Sending...";
    return null;
  })();

  if (!open) return null;

  function onOverlayMouseDown(e: MouseEvent<HTMLDivElement>): void {
    if (e.target === e.currentTarget) {
      usePaletteStore.getState().close();
    }
  }

  function onDialogKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    if (e.key === "Escape") {
      const state = usePaletteStore.getState();
      if (state.composing) return;
      e.preventDefault();
      state.back();
    }
  }

  const frozen = frozenSnapshotRef.current;
  const frozenListProps = frozen
    ? { frozenList: frozen.sortedList, frozenCursor: frozen.sortedListCursor }
    : {};

  const frozenHeaderSnapshot = frozen?.activeContext;
  const headerFlashSeq = frozen !== null ? frozen.flashSeq : flashSeq;


  return createPortal(
    <div className="palette-overlay" data-testid="palette-overlay" onMouseDown={onOverlayMouseDown}>
      <div className="palette-sheet" data-role="palette-sheet">
        <div
          ref={dialogRef}
          // biome-ignore lint/a11y/useSemanticElements: native <dialog> requires
          // showModal()/HTMLDialogElement APIs that don't compose with our
          // store-driven open state; the WAI-ARIA dialog role on a generic
          // container is the documented alternative.
          role="dialog"
          aria-modal="true"
          aria-labelledby="palette-title"
          className="palette-dialog"
          onKeyDown={onDialogKeyDown}
        >
          <h2 id="palette-title" className="palette-dialog__title">
            Command Palette
          </h2>
          <PaletteAnnouncer announce={announceRef.current} />
          <div className="palette-body">
            {phase === "toolSelect" ? (
              <ToolSelectPhase
                inputRef={inputRef}
                httpFactory={props.httpFactory}
                activeContextSnapshot={frozenHeaderSnapshot ?? activeContextSnapshot}
                {...frozenListProps}
              />
            ) : ctx !== null ? (
              <ParamSelectPhase ctx={ctx} />
            ) : (
              <div role="alert" className="palette-error" data-testid="palette-ctx-error">
                Command palette unavailable (http client invalid)
              </div>
            )}
          </div>
          <PaletteFooter
            phase={phase}
            snapshot={ctx !== null ? frozenHeaderSnapshot : undefined}
            flashSeq={headerFlashSeq}
            statusText={liveStatusText}
            submitting={submitting}
            composing={composing}
            onBack={() => usePaletteStore.getState().back()}
            onClose={() => usePaletteStore.getState().close()}
          />
          {error !== null && (
            <div role="alert" className="palette-error" data-testid="palette-error">
              {error}
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body,
  );
}
