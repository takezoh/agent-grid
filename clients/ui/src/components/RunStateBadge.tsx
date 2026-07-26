import "../css/view.css";
import type { SessionStatus } from "../wire/server";
import { StatusIcon, normalizeStatus, statusLabel } from "./StatusIcon";

export type RunStateBadgeProps = {
  status?: SessionStatus;
};

export function RunStateBadge({ status }: RunStateBadgeProps) {
  const kind = normalizeStatus(status);
  const signal = status?.signal?.kind === "bell";
  const label = `status: ${statusLabel(kind)}${signal ? "; Notification" : ""}`;
  return (
    <span className={`run-state-badge run-state-${kind}`} aria-label={label}>
      <StatusIcon
        status={kind}
        signal={signal}
        activeClass="run-state-spinner"
        inactiveClass="run-state-icon"
      />
      {kind}
    </span>
  );
}
