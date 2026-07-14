import { type KeyboardEvent, type ReactNode, useCallback } from "react";
import { useDaemonStore } from "../../store/daemon";
import { rowKey, selectTurnRows, useWorkspaceActivityStore } from "../../store/workspaceActivity";

export type ActivityRailProps = {
  onOpenTree: () => void;
};

const KIND_LABEL: Record<string, string> = {
  read: "read",
  create: "create",
  edit: "edit",
  delete: "delete",
};

export function ActivityRail({ onOpenTree }: ActivityRailProps): ReactNode {
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const rows = useWorkspaceActivityStore((s) => selectTurnRows(s, activeSessionID));
  const expandedRows = useWorkspaceActivityStore((s) => s.expandedRows);
  const openDrawerFromRow = useWorkspaceActivityStore((s) => s.openDrawerFromRow);
  const toggleRowExpanded = useWorkspaceActivityStore((s) => s.toggleRowExpanded);
  const transportDegraded = useWorkspaceActivityStore((s) => s.transportDegraded);

  const activateRow = useCallback(
    (path: string, kind: (typeof rows)[0]["kind"]) => {
      if (!activeSessionID) return;
      openDrawerFromRow({ sessionId: activeSessionID, path, kind });
    },
    [activeSessionID, openDrawerFromRow],
  );

  const onRowKeyDown = (
    e: KeyboardEvent<HTMLButtonElement>,
    path: string,
    kind: (typeof rows)[0]["kind"],
  ) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      activateRow(path, kind);
    }
  };

  return (
    <aside className="activity-rail" data-testid="activity-rail" aria-label="Workspace activity">
      <div className="activity-rail__header">
        <button
          type="button"
          className="activity-rail__workspace-affordance"
          data-role="workspace-tree-affordance"
          aria-label="Open workspace tree"
          onClick={onOpenTree}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onOpenTree();
            }
          }}
        >
          Workspace
        </button>
        {transportDegraded && (
          // biome-ignore lint/a11y/useSemanticElements: passive reconnect hint; <output> implies form association
          <span className="activity-rail__degraded" role="status">
            Activity feed reconnecting…
          </span>
        )}
      </div>
      <ul className="activity-rail__rows">
        {rows.map((row) => {
          const key = rowKey(row.path);
          const expanded = expandedRows.has(key);
          return (
            <li key={`${row.turnId}-${row.path}`} className="activity-rail__row-wrap">
              <button
                type="button"
                className="activity-rail__row"
                data-path={row.path}
                onClick={() => activateRow(row.path, row.kind)}
                onKeyDown={(e) => onRowKeyDown(e, row.path, row.kind)}
              >
                {row.actor === "operator" ? (
                  <span
                    className="activity-rail__kind activity-rail__kind--operator"
                    aria-label={`Operator edited ${row.path}`}
                  >
                    operator
                  </span>
                ) : (
                  <span className={`activity-rail__kind activity-rail__kind--${row.kind}`}>
                    {KIND_LABEL[row.kind] ?? row.kind}
                  </span>
                )}
                <span className="activity-rail__path">{row.path}</span>
                {row.count > 1 && (
                  <span className="activity-rail__badge" aria-label={`${row.count} events`}>
                    {row.count}
                  </span>
                )}
              </button>
              {row.count > 1 && (
                <button
                  type="button"
                  className="activity-rail__expand"
                  aria-expanded={expanded}
                  aria-label={`${expanded ? "Collapse" : "Expand"} events for ${row.path}`}
                  onClick={() => toggleRowExpanded(key)}
                >
                  {expanded ? "▾" : "▸"}
                </button>
              )}
              {expanded && (
                <ul className="activity-rail__events">
                  {row.events.map((ev, i) => (
                    <li key={`${ev.tool_call_id ?? i}-${ev.kind}`}>
                      {ev.kind}: {ev.path}
                    </li>
                  ))}
                </ul>
              )}
            </li>
          );
        })}
      </ul>
    </aside>
  );
}
