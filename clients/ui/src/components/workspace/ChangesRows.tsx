import { type KeyboardEvent, type ReactNode, useCallback } from "react";
import { useDaemonStore } from "../../store/daemon";
import {
  type TurnRow,
  selectTurnRows,
  useWorkspaceActivityStore,
} from "../../store/workspaceActivity";
import { KIND_GLYPH } from "./changesShared";

/** Passive reconnect hint shown above the rows while the WS transport is down. */
export function ChangesDegradedNotice(): ReactNode {
  const transportDegraded = useWorkspaceActivityStore((s) => s.transportDegraded);
  if (!transportDegraded) return null;
  return (
    // biome-ignore lint/a11y/useSemanticElements: passive reconnect hint; <output> implies form association
    <span className="changes__degraded" role="status">
      Activity feed reconnecting…
    </span>
  );
}

export function ChangesRowsList(): ReactNode {
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const rows = useWorkspaceActivityStore((s) => selectTurnRows(s, activeSessionID));
  const openDrawerFromRow = useWorkspaceActivityStore((s) => s.openDrawerFromRow);

  const activateRow = useCallback(
    (path: string, kind: TurnRow["kind"]) => {
      if (!activeSessionID) return;
      openDrawerFromRow({ sessionId: activeSessionID, path, kind });
    },
    [activeSessionID, openDrawerFromRow],
  );

  const onRowKeyDown = (
    e: KeyboardEvent<HTMLButtonElement>,
    path: string,
    kind: TurnRow["kind"],
  ) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      activateRow(path, kind);
    }
  };

  if (rows.length === 0) {
    return (
      <p className="changes__empty" data-testid="changes-empty">
        No file activity yet
      </p>
    );
  }

  return (
    <ul className="changes__rows">
      {rows.map((row) => (
        <li key={`${row.turnId}-${row.path}`} className="changes__row-wrap">
          <button
            type="button"
            className="changes__row"
            data-path={row.path}
            onClick={() => activateRow(row.path, row.kind)}
            onKeyDown={(e) => onRowKeyDown(e, row.path, row.kind)}
          >
            {row.actor === "operator" ? (
              <span
                className="changes__glyph changes__glyph--operator"
                aria-label={`Operator edited ${row.path}`}
              >
                M
              </span>
            ) : (
              <span className={`changes__glyph changes__glyph--${row.kind}`} aria-hidden="true">
                {KIND_GLYPH[row.kind]}
              </span>
            )}
            <span className="changes__path" dir="rtl">
              <bdi dir="ltr">{row.path}</bdi>
            </span>
            <span className="changes__count" aria-label={`${row.count} events`}>
              {row.count}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}
