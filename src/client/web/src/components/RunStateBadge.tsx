import "../css/view.css";

export type RunStateBadgeProps = {
  status?: string;
};

const KNOWN = new Set(["running", "waiting", "idle", "stopped", "pending"]);
const ACTIVE = new Set(["running", "waiting"]);

export function RunStateBadge({ status }: RunStateBadgeProps) {
  const normalized = status && KNOWN.has(status) ? status : "unknown";
  const active = ACTIVE.has(normalized);
  return (
    <span
      className={`run-state-badge run-state-${normalized}`}
      aria-label={`status: ${normalized}`}
    >
      {active && <span className="run-state-spinner" aria-hidden="true" />}
      {normalized}
    </span>
  );
}
