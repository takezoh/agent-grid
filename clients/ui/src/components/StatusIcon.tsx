// StatusIcon — per-status indicator (ADR-0078, supersedes ADR-0032).
//
// running / pending rotation lives on the outer <span> because SVG-element
// rotation is unreliable on Safari / older WebKit. The <span> is a plain
// inline-block whose CSS transform is universally supported.
//
// Motion is INTENTIONALLY exempt from prefers-reduced-motion (ADR-0080): the
// status indicator's motion is functional — it conveys "in progress" state,
// not decorative parallax. Freezing it collapses running / pending / waiting
// / idle into visually identical static glyphs.

import type { ReactNode } from "react";
import type { SessionStatus } from "../wire/server";
import "../css/status-icon.css";

export type StatusKind = "running" | "action" | "idle" | "stopped" | "failed" | "unknown";

const ACTIVE = new Set<StatusKind>(["running"]);

export function normalizeStatus(status?: SessionStatus): StatusKind {
  if (!status) return "unknown";
  if (status.failure === "failed") return "failed";
  if (status.human_input) return "action";
  if (status.lifecycle === "active") return "running";
  if (status.lifecycle === "stopped") return "stopped";
  if (status.lifecycle === "idle") return "idle";
  return "unknown";
}

export function isActiveStatus(kind: StatusKind): boolean {
  return ACTIVE.has(kind);
}

export function statusLabel(kind: StatusKind): string {
  return {
    running: "Working",
    action: "Action required",
    idle: "Idle",
    stopped: "Stopped",
    failed: "Failed",
    unknown: "Unknown",
  }[kind];
}

// `satisfies` enforces exhaustive coverage at compile time: adding a new
// StatusKind without a GEOMETRY entry is a type error, not silent breakage.
const GEOMETRY = {
  running: (
    <>
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--1"
        cx="12"
        cy="3"
        r="1.8"
      />
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--2"
        cx="18.36"
        cy="5.64"
        r="1.8"
      />
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--3"
        cx="21"
        cy="12"
        r="1.8"
      />
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--4"
        cx="18.36"
        cy="18.36"
        r="1.8"
      />
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--5"
        cx="12"
        cy="21"
        r="1.8"
      />
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--6"
        cx="5.64"
        cy="18.36"
        r="1.8"
      />
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--7"
        cx="3"
        cy="12"
        r="1.8"
      />
      <circle
        className="status-icon__spinner-dot status-icon__spinner-dot--8"
        cx="5.64"
        cy="5.64"
        r="1.8"
      />
    </>
  ),
  action: (
    <text className="status-icon__action" x="12" y="17" textAnchor="middle">
      !
    </text>
  ),
  idle: <circle className="status-icon__filled" cx="12" cy="12" r="5" />,
  stopped: <rect className="status-icon__square" x="6" y="6" width="12" height="12" rx="2" />,
  failed: <path className="status-icon__failed" d="M6 6l12 12M18 6L6 18" />,
  unknown: <line className="status-icon__dash" x1="6" y1="12" x2="18" y2="12" />,
} satisfies Record<StatusKind, ReactNode>;

export interface StatusIconProps {
  status: StatusKind;
  signal?: boolean;
  /** Class layered on active states only (e.g. legacy spinner contract). */
  activeClass?: string;
  /** Class layered on inactive states only. */
  inactiveClass?: string;
}

export function StatusIcon({
  status,
  signal,
  activeClass,
  inactiveClass,
}: StatusIconProps): JSX.Element {
  const active = isActiveStatus(status);
  const className = ["status-icon", `status-icon--${status}`, active ? activeClass : inactiveClass]
    .filter(Boolean)
    .join(" ");
  return (
    <span className={className} aria-hidden="true">
      <svg viewBox="0 0 24 24" focusable="false" aria-hidden="true">
        {GEOMETRY[status]}
      </svg>
      {signal ? (
        <span className="status-icon__signal" aria-hidden="true">
          🔔
        </span>
      ) : null}
    </span>
  );
}
