// SessionList — sidebar session picker upgraded to role=listbox (FR-TOKEN-001/002, FR-A11Y-001).
//
// Uses UnifiedListbox primitive (m5-unified-listbox-primitive) to provide:
//   - role=listbox + aria-activedescendant skip-disabled navigation (FR-TOKEN-002)
//   - --row-* token sizing so palette rows and SessionList rows compute identically (FR-TOKEN-001)
//   - 2-line ellipsis on long displayLabel (-webkit-line-clamp:2) (ADR-0033)
//   - minimum 44x44px touch target (FR-A11Y-001)
//
// ADR-0033 (displayLabel chain): title → subtitle → id
// ADR-0032 (RunStateBadge multi-encoding): session-status-slot / session-status-spinner kept
// ADR-0030: conn prop retained for API compat; SessionList does NOT own subscriptions

import { useEffect, useState } from "react";
import type { Connection } from "../socket/connection";
import "../css/view.css";
import { useDaemonStore } from "../store/daemon";
import type { Card } from "../wire/server";
import { UnifiedListbox } from "./primitives/UnifiedListbox";

// ---------------------------------------------------------------------------
// displayLabel (ADR-0033: title → subtitle → id chain)
// ---------------------------------------------------------------------------

export function displayLabel(card: Card, id: string): string {
  const t = card.title?.trim();
  if (t) return t;
  const s = card.subtitle?.trim();
  if (s) return s;
  return id;
}

// ---------------------------------------------------------------------------
// Status helpers (ADR-0032)
// ---------------------------------------------------------------------------

const KNOWN = new Set(["running", "waiting", "idle", "stopped", "pending"]);
const ACTIVE = new Set(["running", "waiting"]);

function normalizeStatus(status?: string): string {
  return status && KNOWN.has(status) ? status : "unknown";
}

// ---------------------------------------------------------------------------
// SessionRow — one row rendered inside UnifiedListbox as label prop
// ---------------------------------------------------------------------------

interface SessionRowProps {
  sessionId: string;
  card: Card;
  status?: string;
  isActive: boolean;
}

function SessionRow({ sessionId, card, status, isActive }: SessionRowProps) {
  const normalized = normalizeStatus(status);
  const active = ACTIVE.has(normalized);
  const label = displayLabel(card, sessionId);

  return (
    <div
      className={["session-list__row", isActive ? "session-list__row--active" : ""]
        .filter(Boolean)
        .join(" ")}
    >
      <span
        className={`session-status-slot session-status-${normalized}`}
        aria-label={`status: ${normalized}`}
        title={normalized}
      >
        {active && <span className="session-status-spinner" aria-hidden="true" />}
      </span>
      <span className="session-list__label--clamped title">{label}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SessionList
// ---------------------------------------------------------------------------

// conn is retained in the prop signature for API compatibility; SessionList
// does not own subscriptions (ADR 0030) — TerminalPane is the sole owner.
export function SessionList({ conn: _conn }: { conn: Connection }) {
  const sessions = useDaemonStore((s) => s.sessions);
  const activeId = useDaemonStore((s) => s.activeSessionID);
  const selectSession = useDaemonStore((s) => s.selectSession);
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);

  // cursorId tracks the preview cursor for aria-activedescendant navigation.
  // It is separate from the committed selection (activeId) so ArrowDown/Up can
  // move the cursor without calling selectSession (FR-TOKEN-002).
  // When the committed selection changes externally (e.g. another tab), sync
  // the cursor so it doesn't drift away from the active row.
  const [cursorId, setCursorId] = useState<string | null>(activeId);
  useEffect(() => {
    setCursorId(activeId);
  }, [activeId]);

  const items = sessions.map((s) => ({
    id: s.id,
    label: (
      <SessionRow
        sessionId={s.id}
        card={s.view.card}
        status={s.view.status}
        isActive={s.id === activeId}
      />
    ),
    disabled: daemonDisconnected,
    disabledReason: daemonDisconnected ? "Daemon disconnected" : undefined,
  }));

  return (
    <div className="session-list">
      <UnifiedListbox
        ariaLabel="sessions"
        items={items}
        activeId={cursorId}
        onActiveChange={(id) => {
          // Preview / hover cursor movement — updates aria-activedescendant only.
          // Actual session selection is committed only via onActivate (click/Enter).
          setCursorId(id);
        }}
        onActivate={(id) => {
          selectSession(id);
        }}
      />
    </div>
  );
}
