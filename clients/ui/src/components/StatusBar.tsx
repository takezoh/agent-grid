import type { JSX } from "react";
import { formatElapsed, useNow1Hz } from "../hooks/useNow1Hz";
import { type ConnectionStatus, useDaemonStore } from "../store/daemon";
import "../css/view.css";

export type StatusBarProps = {
  statusLine?: string;
  statusChangedAt?: string;
};

function isConnectionNominal(status: ConnectionStatus, daemonDisconnected: boolean): boolean {
  return status === "open" && !daemonDisconnected;
}

function connectionLabel(status: ConnectionStatus, daemonDisconnected: boolean): string {
  if (status === "closed") return "connection closed";
  if (daemonDisconnected) return "daemon disconnected, reconnecting…";
  if (status === "reconnecting") return "reconnecting to server…";
  if (status === "connecting") return "connecting…";
  return "connected";
}

/**
 * FR-023: 26px bottom status bar — status_line left, connection state right.
 * Always visible while a session is active: the bar is the shell's ground
 * line ("connected" when nominal), never a sometimes-there surface.
 */
export function StatusBar({ statusLine, statusChangedAt }: StatusBarProps): JSX.Element | null {
  const now = useNow1Hz();
  const status = useDaemonStore((s) => s.status);
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);
  const nominal = isConnectionNominal(status, daemonDisconnected);
  const conn = connectionLabel(status, daemonDisconnected);

  const elapsed = statusChangedAt ? formatElapsed(now - new Date(statusChangedAt).getTime()) : "";

  return (
    <footer className="status-bar" data-role="status-bar" aria-label="session status">
      <div className="status-bar__left">
        {statusLine?.trim() && <span className="status-bar__line">{statusLine}</span>}
        {elapsed && (
          <span className="status-bar__elapsed font-mono" aria-label="elapsed">
            {elapsed}
          </span>
        )}
      </div>
      <span
        className="status-bar__connection font-mono"
        aria-label="connection state"
        data-nominal={nominal ? "true" : "false"}
      >
        {conn}
      </span>
    </footer>
  );
}
