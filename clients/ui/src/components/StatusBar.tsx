import { type JSX, useEffect, useState } from "react";
import type { FleetHealth } from "../auth";
import { makeFleetProvider } from "../fleet/provider";
import { formatElapsed, useNow1Hz } from "../hooks/useNow1Hz";
import { type ConnectionStatus, useDaemonStore } from "../store/daemon";
import "../css/view.css";

export type StatusBarProps = {
  statusLine?: string;
  statusChangedAt?: string;
  onRetry?: () => void;
};

function isConnectionNominal(status: ConnectionStatus, daemonDisconnected: boolean): boolean {
  return status === "open" && !daemonDisconnected;
}

function fleetLabel(fleet: FleetHealth | null): string | null {
  if (!fleet) return null;
  const enabled = fleet.servers.filter((server) => server.state !== "disabled");
  const unavailable = enabled.filter((server) => server.state === "unavailable");
  if (unavailable.length === 0) return null;
  if (unavailable.length === enabled.length) return "all servers unavailable";
  return `${unavailable.length}/${enabled.length} servers unavailable`;
}

function connectionLabel(
  status: ConnectionStatus,
  daemonDisconnected: boolean,
  fleet: FleetHealth | null,
): string {
  if (status === "closed") return "connection closed";
  if (daemonDisconnected) return "daemon disconnected, reconnecting…";
  if (status === "reconnecting") return "reconnecting to server…";
  if (status === "connecting") return "connecting…";
  return fleetLabel(fleet) ?? "connected";
}

/**
 * FR-023: 26px bottom status bar — status_line left, connection state right.
 * Always visible while a session is active: the bar is the shell's ground
 * line ("connected" when nominal), never a sometimes-there surface.
 */
export function StatusBar({
  statusLine,
  statusChangedAt,
  onRetry,
}: StatusBarProps): JSX.Element | null {
  const now = useNow1Hz();
  const status = useDaemonStore((s) => s.status);
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);
  const [fleet, setFleet] = useState<FleetHealth | null>(null);
  useEffect(() => {
    const provider = makeFleetProvider();
    let active = true;
    const refresh = () => {
      void provider
        .listConfigs()
        .then((outcomes) => {
          if (active) {
            setFleet({
              checkedAt: new Date().toISOString(),
              servers: outcomes.map((outcome) => ({
                serverId: outcome.serverId,
                displayName: outcome.serverLabel,
                state: outcome.state,
                ...(outcome.reason ? { reason: outcome.reason } : {}),
              })),
            });
          }
        })
        .catch(() => {});
    };
    refresh();
    const timer = window.setInterval(refresh, 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);
  const fleetDegraded = fleetLabel(fleet) !== null;
  const nominal = isConnectionNominal(status, daemonDisconnected) && !fleetDegraded;
  const conn = connectionLabel(status, daemonDisconnected, fleet);

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
      <div className="status-bar__connection-group">
        <span
          className="status-bar__connection font-mono"
          aria-label="connection state"
          data-nominal={nominal ? "true" : "false"}
        >
          {conn}
        </span>
        {status === "closed" && onRetry && (
          <button type="button" className="status-bar__retry" onClick={onRetry}>
            Retry
          </button>
        )}
      </div>
    </footer>
  );
}
