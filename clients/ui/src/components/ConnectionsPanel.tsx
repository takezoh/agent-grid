import { type JSX, useMemo, useState } from "react";
import { projectConnectionsDiagnosis } from "../fleet/connections-presentation";
import type { FleetHealthView } from "../fleet/session-protocol";
import "../css/connections.css";

export type ConnectionsPanelProps = {
  open: boolean;
  health: FleetHealthView | null;
  configPath?: string | null;
  onClose: () => void;
  /** Host-neutral per-server reconnect; App wires FleetProvider.reconnect. */
  onReconnect?: (serverId: string) => Promise<void>;
};

async function copyTextWithFallback(text: string): Promise<void> {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  // Insecure contexts / missing Clipboard API: use a transient selection.
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  const ok = document.execCommand("copy");
  document.body.removeChild(textarea);
  if (!ok) throw new Error("copy failed");
}

/**
 * Closed credential-free projection for reconnect failures. Providers must
 * throw closed messages; the panel never surfaces raw host response bodies.
 */
function reconnectFailureDetail(error: unknown): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return "reconnect failed";
}

/**
 * Browser Connections diagnosis surface. Shares row semantics with Workspace
 * via connections-presentation. Owns per-server Reconnect (not StatusBar Retry).
 */
export function ConnectionsPanel({
  open,
  health,
  configPath,
  onClose,
  onReconnect,
}: ConnectionsPanelProps): JSX.Element | null {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  /** Independent single-flight pending flags — concurrent different-server reconnects allowed. */
  const [pendingByServer, setPendingByServer] = useState<Record<string, true>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const diagnosis = useMemo(
    () =>
      projectConnectionsDiagnosis(health, {
        configPath: configPath ?? undefined,
        hostKind: "browser",
      }),
    [health, configPath],
  );

  if (!open) return null;

  const clearPending = (serverId: string) => {
    setPendingByServer((prev) => {
      if (!(serverId in prev)) return prev;
      const next = { ...prev };
      delete next[serverId];
      return next;
    });
  };

  const handleReconnect = (serverId: string, label: string) => {
    if (!onReconnect) return;
    // Single-flight per server while that server's promise is pending.
    if (pendingByServer[serverId]) return;
    setPendingByServer((prev) => ({ ...prev, [serverId]: true }));
    setErrors((prev) => {
      if (!(serverId in prev)) return prev;
      const next = { ...prev };
      delete next[serverId];
      return next;
    });
    void onReconnect(serverId)
      .then(() => {
        clearPending(serverId);
        setErrors((prev) => {
          if (!(serverId in prev)) return prev;
          const next = { ...prev };
          delete next[serverId];
          return next;
        });
      })
      .catch((error: unknown) => {
        clearPending(serverId);
        setErrors((prev) => ({
          ...prev,
          [serverId]: `Reconnect failed for ${label}: ${reconnectFailureDetail(error)}`,
        }));
      });
  };

  return (
    <dialog className="connections-panel" open aria-label="Connections" aria-modal="true">
      <div className="connections-panel__header">
        <h2>Connections</h2>
        <button type="button" onClick={onClose} aria-label="Close Connections">
          Close
        </button>
      </div>
      {diagnosis.deliveryLossExplanation ? (
        <div
          className="connections-panel__delivery-loss"
          data-role="delivery-loss-diagnosis"
          data-state="health_unavailable"
        >
          <strong>Health delivery lost</strong>
          <p data-role="delivery-loss-explanation">{diagnosis.deliveryLossExplanation}</p>
          {diagnosis.recoveryGuidance ? (
            <p data-role="delivery-loss-recovery">{diagnosis.recoveryGuidance}</p>
          ) : null}
        </div>
      ) : null}
      <ul className="connections-panel__list">
        {diagnosis.rows.map((row) => {
          const pending = pendingByServer[row.serverId] === true;
          const error = errors[row.serverId];
          return (
            <li key={row.serverId} data-server-id={row.serverId} data-state={row.state}>
              <strong>{row.label}</strong>
              <span>{row.stateLabel}</span>
              {row.reason ? <span>{row.reason}</span> : null}
              {row.stalePrior ? <span>stale prior</span> : null}
              {row.enabled && onReconnect ? (
                <div className="connections-panel__row-actions">
                  <button
                    type="button"
                    className="connections-panel__reconnect"
                    aria-label={`Reconnect ${row.label}`}
                    disabled={pending}
                    aria-busy={pending}
                    data-pending={pending ? "true" : "false"}
                    onClick={() => handleReconnect(row.serverId, row.label)}
                  >
                    {pending ? "Reconnecting…" : "Reconnect"}
                  </button>
                  {pending ? (
                    <output data-role="reconnect-pending" aria-live="polite">
                      Reconnecting {row.label}…
                    </output>
                  ) : null}
                  {error ? (
                    <output
                      className="connections-panel__reconnect-error"
                      data-role="reconnect-error"
                      data-server-id={row.serverId}
                    >
                      {error}
                    </output>
                  ) : null}
                </div>
              ) : null}
            </li>
          );
        })}
      </ul>
      {diagnosis.configPath || diagnosis.restartInstruction ? (
        <div className="connections-panel__config">
          {diagnosis.restartInstruction ? <p>{diagnosis.restartInstruction}</p> : null}
          {diagnosis.configPath ? (
            <>
              <code data-role="config-path">{diagnosis.configPath}</code>
              <button
                type="button"
                onClick={() => {
                  void copyTextWithFallback(diagnosis.configPath ?? "")
                    .then(() => {
                      setCopyState("copied");
                    })
                    .catch(() => {
                      setCopyState("failed");
                    });
                }}
              >
                Copy configuration path
              </button>
              {copyState === "copied" ? <output data-role="copy-status">Copied</output> : null}
              {copyState === "failed" ? (
                <output data-role="copy-status">Could not copy path</output>
              ) : null}
            </>
          ) : null}
        </div>
      ) : null}
    </dialog>
  );
}
