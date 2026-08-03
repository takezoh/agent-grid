import { type JSX, useMemo, useState } from "react";
import { projectConnectionsDiagnosis } from "../fleet/connections-presentation";
import type { FleetHealthView } from "../fleet/session-protocol";
import "../css/connections.css";

export type ConnectionsPanelProps = {
  open: boolean;
  health: FleetHealthView | null;
  configPath?: string | null;
  onClose: () => void;
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
 * Browser Connections diagnosis surface. Shares row semantics with Workspace
 * via connections-presentation; does not own Retry.
 */
export function ConnectionsPanel({
  open,
  health,
  configPath,
  onClose,
}: ConnectionsPanelProps): JSX.Element | null {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const diagnosis = useMemo(
    () =>
      projectConnectionsDiagnosis(health, {
        configPath: configPath ?? undefined,
        hostKind: "browser",
      }),
    [health, configPath],
  );

  if (!open) return null;

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
        {diagnosis.rows.map((row) => (
          <li key={row.serverId} data-server-id={row.serverId} data-state={row.state}>
            <strong>{row.label}</strong>
            <span>{row.stateLabel}</span>
            {row.reason ? <span>{row.reason}</span> : null}
            {row.stalePrior ? <span>stale prior</span> : null}
          </li>
        ))}
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
