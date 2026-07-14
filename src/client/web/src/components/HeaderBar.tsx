import type { JSX } from "react";
import "../css/icon-button.css";
import type { Card } from "../wire/server";
import { OverflowMenu } from "./OverflowMenu";
import { RunStateBadge } from "./RunStateBadge";
import { titleText } from "./SessionList";
import { SessionTerminateButton } from "./SessionTerminateButton";
import { SegmentedControl } from "./primitives/SegmentedControl";

export type MainMode = "terminal" | "workspace";

export type HeaderBarProps = {
  /** Full project path — breadcrumb uses basename only (FR-011 / UAC-004). */
  project?: string;
  card?: Card;
  status?: string;
  model?: string;
  effort?: string;
  driver?: string;
  sessionId?: string;
  /** FR-024: stop button wires ConfirmDialog via this callback. */
  onRequestTerminate?: (sessionId: string, label: string, opener: HTMLElement) => void;
  /** Mobile: show compact title-only layout (FR-013 / UAC-006). */
  mobile?: boolean;
  /** Main-area mode (Terminal / Workspace). Switch renders when both set. */
  mode?: MainMode;
  onModeChange?: (next: MainMode) => void;
};

function projectBasename(projectPath: string): string {
  const normalized = projectPath.replace(/\\/g, "/");
  const parts = normalized.split("/").filter(Boolean);
  return parts[parts.length - 1] ?? projectPath;
}

function metadataLine(driver?: string, model?: string, effort?: string): string {
  return [driver?.trim(), model?.trim(), effort?.trim()].filter(Boolean).join(" · ");
}

/**
 * FR-011 / FR-022: 44px header — breadcrumb, status pill, mono metadata, icon actions.
 */
export function HeaderBar({
  project,
  card,
  status,
  model,
  effort,
  driver,
  sessionId,
  onRequestTerminate,
  mobile = false,
  mode,
  onModeChange,
}: HeaderBarProps): JSX.Element {
  const sessionTitle = card ? titleText(card) : undefined;
  const meta = metadataLine(driver, model, effort);

  return (
    <header className="header-bar" data-role="header-bar" aria-label="session header">
      {!mobile && project && (
        <nav className="header-bar__breadcrumb" aria-label="session context">
          <span className="header-bar__project">{projectBasename(project)}</span>
          {sessionTitle && (
            <>
              <span className="header-bar__sep" aria-hidden="true">
                /
              </span>
              <span className="header-bar__title">{sessionTitle}</span>
            </>
          )}
        </nav>
      )}
      {mobile && sessionTitle && <span className="header-bar__mobile-title">{sessionTitle}</span>}
      {!mobile && meta && (
        <span className="header-bar__meta font-mono" aria-label="session metadata">
          {meta}
        </span>
      )}
      <div className="header-bar__actions">
        {mode !== undefined && onModeChange !== undefined && sessionId && (
          <div className="header-bar__mode" data-testid="mode-switch">
            <SegmentedControl<MainMode>
              ariaLabel="main mode"
              segments={[
                { value: "terminal", label: "Terminal" },
                { value: "workspace", label: "Workspace" },
              ]}
              value={mode}
              onChange={onModeChange}
            />
          </div>
        )}
        <RunStateBadge status={status} />
        {sessionId && onRequestTerminate && (
          <SessionTerminateButton
            sessionId={sessionId}
            sessionLabel={sessionTitle ?? "New Session"}
            onRequestTerminate={(id, opener) =>
              onRequestTerminate(id, sessionTitle ?? "New Session", opener)
            }
          />
        )}
        <OverflowMenu />
      </div>
    </header>
  );
}
