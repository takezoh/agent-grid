import { type JSX, useEffect, useRef } from "react";
import { reduceStatusAxes } from "../fleet/presentation";
import type { FleetHealthView } from "../fleet/session-protocol";
import { formatElapsed, useNow1Hz } from "../hooks/useNow1Hz";
import { type ConnectionStatus, useDaemonStore } from "../store/daemon";
import "../css/connections.css";
import "../css/view.css";

export type StatusBarProps = {
  statusLine?: string;
  statusChangedAt?: string;
  onRetry?: () => void;
  health?: FleetHealthView | null;
  onOpenConnections?: () => void;
};

/**
 * FR-023 / FR-REC-004: persistent StatusBar owns active recovery Retry and
 * enabled-fleet aggregate/Connections. No listConfigs polling.
 */
export function StatusBar({
  statusLine,
  statusChangedAt,
  onRetry,
  health = null,
  onOpenConnections,
}: StatusBarProps): JSX.Element {
  const now = useNow1Hz();
  const status = useDaemonStore((s) => s.status) as ConnectionStatus;
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);
  const rootRef = useRef<HTMLElement | null>(null);
  const retryWasFocused = useRef(false);

  const axes = reduceStatusAxes({
    connectionStatus: status,
    daemonDisconnected,
    health,
  });

  useEffect(() => {
    if (!axes.showRetry) {
      // Transfer only when Retry was focused at the moment it disappears.
      if (retryWasFocused.current) {
        rootRef.current?.focus();
      }
      retryWasFocused.current = false;
    }
  }, [axes.showRetry]);

  const elapsed = statusChangedAt ? formatElapsed(now - new Date(statusChangedAt).getTime()) : "";

  return (
    <footer
      ref={rootRef}
      className="status-bar"
      data-role="status-bar"
      aria-label="session status"
      tabIndex={-1}
    >
      <div className="status-bar__left">
        {statusLine?.trim() && <span className="status-bar__line">{statusLine}</span>}
        {elapsed && (
          <span className="status-bar__elapsed font-mono" aria-label="elapsed">
            {elapsed}
          </span>
        )}
      </div>
      <div className="status-bar__connection-group">
        <output
          className="status-bar__connection font-mono"
          aria-live="polite"
          aria-label="connection state"
          data-nominal={axes.nominal ? "true" : "false"}
        >
          {axes.activeText}
        </output>
        {axes.showRetry && onRetry && (
          <button
            type="button"
            className="status-bar__retry"
            onClick={() => {
              onRetry();
            }}
            onFocus={() => {
              retryWasFocused.current = true;
            }}
            onBlur={(event) => {
              // Clear only when focus actually left Retry while it remains mounted.
              // Unmount blur keeps the flag so the disappearance effect can transfer.
              if (!event.currentTarget.isConnected) return;
              retryWasFocused.current = false;
            }}
          >
            Retry
          </button>
        )}
        {axes.showConnections && onOpenConnections && (
          <button type="button" className="status-bar__connections" onClick={onOpenConnections}>
            Connections
          </button>
        )}
      </div>
    </footer>
  );
}
