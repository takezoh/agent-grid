import type { Connection } from "../socket/connection";
import { useDaemonStore } from "../store/daemon";
import type { Card } from "../wire/server";
import { RunStateBadge } from "./RunStateBadge";

export function displayLabel(card: Card, id: string): string {
  const t = card.title?.trim();
  if (t) return t;
  const s = card.subtitle?.trim();
  if (s) return s;
  return id;
}

// conn is retained in the prop signature for API compatibility; SessionList
// does not own subscriptions (ADR 0030) — TerminalPane is the sole owner.
export function SessionList({ conn: _conn }: { conn: Connection }) {
  const sessions = useDaemonStore((s) => s.sessions);
  const activeId = useDaemonStore((s) => s.activeSessionID);
  const selectSession = useDaemonStore((s) => s.selectSession);

  return (
    <ul className="session-list" aria-label="sessions">
      {sessions.map((s) => (
        <li key={s.id}>
          <button
            type="button"
            className={s.id === activeId ? "active" : ""}
            onClick={() => {
              selectSession(s.id);
            }}
          >
            <span className="title">{displayLabel(s.view.card, s.id)}</span>
            <RunStateBadge status={s.view.status} />
          </button>
        </li>
      ))}
    </ul>
  );
}
