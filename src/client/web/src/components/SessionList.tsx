// SessionList — sidebar session picker, partitioned by workspace and grouped
// by project (workspace switcher row → alphabetical project headers with fold
// toggles → sessions under each).
//
// Layered listbox model:
//   - Workspace switcher: SegmentedControl (radiogroup) — only rendered when
//     ≥ 2 workspaces exist.
//   - Per-project section: disclosure button (aria-expanded) + nested
//     UnifiedListbox of sessions. Each project has its own cursor; this is
//     deliberate — keeps keyboard nav contained inside a project so the
//     active-session highlight never drifts to a different project.
//
// ADRs retained:
//   - ADR-0079 (Title chain resolved in driver layer; Web adds placeholder)
//   - ADR-0032 (session-status-slot + session-status-spinner kept)
//   - ADR-0030 (conn prop retained for API compat)
//   - FR-A11Y-001 (44×44px touch target on every interactive row)
//   - FR-TOKEN-001/002 (--row-* sizing tokens shared with palette listbox)

import { useEffect, useId, useMemo, useState } from "react";
import type { Connection } from "../socket/connection";
import "../css/view.css";
import { formatElapsed, useNow1Hz } from "../hooks/useNow1Hz";
import {
  DEFAULT_WORKSPACE,
  groupSessionsByProject,
  selectDistinctWorkspaces,
  useDaemonStore,
} from "../store/daemon";
import type { Card, SessionInfo } from "../wire/server";
import { SessionContextMenu } from "./SessionContextMenu";
import { StatusIcon, normalizeStatus as toStatusKind } from "./StatusIcon";
import { SegmentedControl } from "./primitives/SegmentedControl";
import { TagPill } from "./primitives/TagPill";
import { UnifiedListbox } from "./primitives/UnifiedListbox";

// ---------------------------------------------------------------------------
// Title slot policy (ADR-0079)
// ---------------------------------------------------------------------------

export const TITLE_PLACEHOLDER = "New Session";

export function titleText(card: Card): string {
  return card.title?.trim() || TITLE_PLACEHOLDER;
}

/**
 * @deprecated Use {@link titleText} directly. Kept only for tests that still
 * target the legacy 1-slot chain.
 */
export function displayLabel(card: Card, _id: string): string {
  return titleText(card);
}

// ---------------------------------------------------------------------------
// SessionRow — FR-009: dot + title + age / mono metadata line
// ---------------------------------------------------------------------------

interface SessionRowProps {
  session: SessionInfo;
  isActive: boolean;
  daemonDisconnected: boolean;
  onOpen: (sessionId: string) => void;
  onRequestTerminate?: (sessionId: string, label: string, opener: HTMLElement) => void;
}

function metadataLine(driver?: string, model?: string, effort?: string): string {
  return [driver?.trim(), model?.trim(), effort?.trim()].filter(Boolean).join(" · ");
}

function SessionRow({
  session,
  isActive,
  daemonDisconnected,
  onOpen,
  onRequestTerminate,
}: SessionRowProps) {
  const now = useNow1Hz();
  const card = session.view.card;
  const status = session.view.status;
  const normalized = toStatusKind(status);
  const title = titleText(card);
  const driver = session.root_driver?.trim() || undefined;
  const metadata = metadataLine(driver, session.view.model, session.view.effort);
  const tags = card.tags ?? [];
  const elapsed = session.view.status_changed_at
    ? formatElapsed(now - new Date(session.view.status_changed_at).getTime())
    : "";

  return (
    <SessionContextMenu
      sessionId={session.id}
      sessionLabel={title}
      daemonDisconnected={daemonDisconnected}
      onOpen={onOpen}
      onRequestTerminate={onRequestTerminate}
    >
      {/* Trigger child must be a plain DOM element: Radix Slot merges the
          contextmenu handler + ref onto this div. */}
      <div
        className={["session-list__row", isActive ? "session-list__row--active" : ""]
          .filter(Boolean)
          .join(" ")}
        data-session-id={session.id}
      >
        <span
          className={`session-status-slot session-status-${normalized}`}
          aria-label={`status: ${normalized}`}
          title={normalized}
        >
          <StatusIcon
            status={normalized}
            activeClass="session-status-spinner"
            inactiveClass="session-status-icon"
          />
        </span>
        <div className="session-list__content">
          <div className="session-list__title-row">
            <span className="session-list__title title">{title}</span>
            {elapsed && (
              <span className="session-list__age font-mono" aria-label="elapsed">
                {elapsed}
              </span>
            )}
          </div>
          {(metadata || tags.length > 0) && (
            <div className="session-list__meta-row" aria-label="session metadata">
              {metadata && <span className="session-list__meta font-mono">{metadata}</span>}
              {tags.length > 0 && (
                <span className="session-list__tags">
                  {tags.map((tag, index) => (
                    <TagPill key={`${index}-${tag.text}`} tag={tag} />
                  ))}
                </span>
              )}
            </div>
          )}
        </div>
      </div>
    </SessionContextMenu>
  );
}

// ---------------------------------------------------------------------------
// WorkspaceSwitcher
// ---------------------------------------------------------------------------

interface WorkspaceSwitcherProps {
  workspaces: string[];
  selected: string;
  onChange: (next: string) => void;
}

function WorkspaceSwitcher({ workspaces, selected, onChange }: WorkspaceSwitcherProps) {
  if (workspaces.length < 2) return null;
  return (
    <div className="session-list__workspace-bar" data-role="workspace-switcher">
      <SegmentedControl
        ariaLabel="workspaces"
        segments={workspaces.map((w) => ({ value: w, label: w }))}
        value={selected}
        onChange={onChange}
        idPrefix="ws"
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// ProjectGroup
// ---------------------------------------------------------------------------

interface ProjectGroupProps {
  project: string;
  projectPath: string;
  sessions: SessionInfo[];
  folded: boolean;
  onToggleFold: (projectPath: string) => void;
  activeId: string | null;
  daemonDisconnected: boolean;
  selectSession: (id: string) => void;
  onRequestTerminate?: (sessionId: string, label: string, opener: HTMLElement) => void;
}

function ProjectGroup({
  project,
  projectPath,
  sessions,
  folded,
  onToggleFold,
  activeId,
  daemonDisconnected,
  selectSession,
  onRequestTerminate,
}: ProjectGroupProps) {
  const activeInGroup = sessions.some((s) => s.id === activeId);
  const [cursorId, setCursorId] = useState<string | null>(
    activeInGroup ? activeId : (sessions[0]?.id ?? null),
  );

  useEffect(() => {
    setCursorId((prev) => {
      if (activeInGroup) return activeId;
      if (prev !== null && !sessions.some((s) => s.id === prev)) {
        return sessions[0]?.id ?? null;
      }
      return prev;
    });
  }, [activeId, activeInGroup, sessions]);

  const uid = useId();
  const headerId = `${uid}-header`;
  const panelId = `${uid}-panel`;

  return (
    <section className="session-list__project" data-role="project-group">
      <button
        type="button"
        id={headerId}
        className="session-list__project-header"
        aria-expanded={!folded}
        aria-controls={panelId}
        onClick={() => onToggleFold(projectPath)}
        title={projectPath}
      >
        <span className="session-list__project-chevron" aria-hidden="true">
          {folded ? "▶" : "▼"}
        </span>
        <span className="session-list__project-name">{project}</span>
        <span className="session-list__project-count" aria-hidden="true">
          {sessions.length}
        </span>
      </button>
      {!folded && (
        <section id={panelId} aria-labelledby={headerId} className="session-list__project-panel">
          <UnifiedListbox
            ariaLabel={`sessions in ${project}`}
            items={sessions.map((s) => ({
              id: s.id,
              label: (
                <SessionRow
                  session={s}
                  isActive={s.id === activeId}
                  daemonDisconnected={daemonDisconnected}
                  onOpen={selectSession}
                  onRequestTerminate={onRequestTerminate}
                />
              ),
              disabled: daemonDisconnected,
              disabledReason: daemonDisconnected ? "Daemon disconnected" : undefined,
            }))}
            activeId={cursorId}
            onActiveChange={(id) => setCursorId(id)}
            onActivate={(id) => selectSession(id)}
          />
        </section>
      )}
    </section>
  );
}

// ---------------------------------------------------------------------------
// SessionList
// ---------------------------------------------------------------------------

export interface SessionListProps {
  conn: Connection;
  /** Opens the session-terminate ConfirmDialog owned by App (same contract
      as HeaderBar's stop button). Optional so standalone mounts still work. */
  onRequestTerminate?: (sessionId: string, label: string, opener: HTMLElement) => void;
}

export function SessionList({ conn: _conn, onRequestTerminate }: SessionListProps) {
  const sessions = useDaemonStore((s) => s.sessions);
  const activeId = useDaemonStore((s) => s.activeSessionID);
  const selectSession = useDaemonStore((s) => s.selectSession);
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);
  const selectedWorkspace = useDaemonStore((s) => s.selectedWorkspace);
  const setSelectedWorkspace = useDaemonStore((s) => s.setSelectedWorkspace);
  const foldedProjects = useDaemonStore((s) => s.foldedProjects);
  const toggleProjectFold = useDaemonStore((s) => s.toggleProjectFold);

  const workspaces = useMemo(() => selectDistinctWorkspaces(sessions), [sessions]);
  const groups = useMemo(
    () => groupSessionsByProject(sessions, selectedWorkspace),
    [sessions, selectedWorkspace],
  );

  const empty = groups.length === 0;

  return (
    <div className="session-list" data-workspace={selectedWorkspace}>
      <WorkspaceSwitcher
        workspaces={workspaces}
        selected={selectedWorkspace}
        onChange={setSelectedWorkspace}
      />
      {empty ? (
        <output className="session-list__empty">
          {selectedWorkspace === DEFAULT_WORKSPACE
            ? "No sessions yet."
            : `No sessions in workspace "${selectedWorkspace}".`}
        </output>
      ) : (
        <div className="session-list__projects">
          {groups.map((g) => (
            <ProjectGroup
              key={g.projectPath}
              project={g.project}
              projectPath={g.projectPath}
              sessions={g.sessions}
              folded={foldedProjects.has(g.projectPath)}
              onToggleFold={toggleProjectFold}
              activeId={activeId}
              daemonDisconnected={daemonDisconnected}
              selectSession={selectSession}
              onRequestTerminate={onRequestTerminate}
            />
          ))}
        </div>
      )}
    </div>
  );
}
