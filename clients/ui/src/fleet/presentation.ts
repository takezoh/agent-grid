import type {
  FleetHealthServerV1,
  FleetHealthView,
  HealthReason,
  QualifiedHealthObservation,
} from "./session-protocol";

export type FleetPresentationState =
  | "checking"
  | "connected"
  | "unavailable"
  | "incompatible_protocol"
  | "disabled"
  | "health_unavailable";

export interface FleetPresentationOutcome {
  state: FleetPresentationState;
  reason?: string;
  stale?: boolean;
}

const HEALTH_REASON_LABEL: Record<HealthReason, string> = {
  connection_timeout: "Connection timed out",
  connection_failed: "Could not connect to server",
  http_error: "Server HTTP error",
  websocket_failed: "WebSocket connection failed",
  empty_ticket: "Empty websocket ticket",
  protocol_mismatch: "Incompatible protocol",
  refresh_failed: "Health refresh failed",
  delivery_lost: "Health delivery lost",
};

export function healthReasonText(reason?: HealthReason, detail?: string): string | undefined {
  if (detail) return detail;
  if (!reason) return undefined;
  return HEALTH_REASON_LABEL[reason];
}

export function observationToPresentation(
  observation: QualifiedHealthObservation,
): FleetPresentationOutcome {
  switch (observation.kind) {
    case "disabled":
      return { state: "disabled" };
    case "checking":
      return { state: "checking" };
    case "connected":
      return { state: "connected" };
    case "unavailable":
      return {
        state: "unavailable",
        reason: healthReasonText(observation.reason, observation.detail),
      };
    case "incompatible_protocol":
      return {
        state: "incompatible_protocol",
        reason: healthReasonText(observation.reason, observation.detail),
      };
    case "health_unavailable":
      return {
        state: "health_unavailable",
        reason: healthReasonText(observation.reason, observation.detail),
        stale: Boolean(observation.prior),
      };
  }
}

export function fleetOutcomeLabel(outcome: FleetPresentationOutcome): string | null {
  if (outcome.state === "connected" || outcome.state === "disabled") return null;
  if (outcome.state === "checking") return "Checking…";
  const label =
    outcome.state === "incompatible_protocol"
      ? "Incompatible protocol"
      : outcome.state === "health_unavailable"
        ? "Health unavailable"
        : "Unavailable";
  const stale = outcome.stale ? " (stale prior)" : "";
  return outcome.reason ? `${label} — ${outcome.reason}${stale}` : `${label}${stale}`;
}

export function summarizeFleet(outcomes: readonly FleetPresentationOutcome[]): string | null {
  const enabled = outcomes.filter((outcome) => outcome.state !== "disabled");
  if (enabled.length === 0) return null;
  const incompatible = enabled.filter((outcome) => outcome.state === "incompatible_protocol");
  const unavailable = enabled.filter(
    (outcome) => outcome.state === "unavailable" || outcome.state === "health_unavailable",
  );
  const checking = enabled.filter((outcome) => outcome.state === "checking");
  if (incompatible.length === 0 && unavailable.length === 0) {
    if (checking.length > 0) return "checking fleet health…";
    return null;
  }

  const summary = (count: number, state: string): string => {
    if (count === enabled.length) {
      if (state === "unavailable") return "all enabled servers unavailable";
      if (state === "incompatible") return "all enabled servers incompatible";
      return `all servers ${state}`;
    }
    return `${count}/${enabled.length} servers ${state}`;
  };
  const parts: string[] = [];
  if (incompatible.length > 0) parts.push(summary(incompatible.length, "incompatible"));
  if (unavailable.length > 0) parts.push(summary(unavailable.length, "unavailable"));
  return parts.join("; ");
}

export function presentationOutcomesFromHealth(
  servers: readonly FleetHealthServerV1[],
): FleetPresentationOutcome[] {
  return servers.map((server) => observationToPresentation(server.observation));
}

export function fleetIsDegraded(view: FleetHealthView | null): boolean {
  if (!view) return false;
  // Delivery loss is degraded even before any configured-server snapshot exists
  // (empty servers must not look nominal/connected).
  if (view.delivery === "lost") return true;
  return summarizeFleet(presentationOutcomesFromHealth(view.servers)) !== null;
}

/** Aggregate text when host health delivery is lost independent of server rows. */
export const HEALTH_DELIVERY_LOST_SUMMARY = "health delivery lost";

export interface StatusAxesPresentation {
  activeText: string;
  fleetSummary: string | null;
  showRetry: boolean;
  showConnections: boolean;
  nominal: boolean;
}

export function reduceStatusAxes(input: {
  connectionStatus: "connecting" | "open" | "reconnecting" | "closed";
  daemonDisconnected: boolean;
  health: FleetHealthView | null;
}): StatusAxesPresentation {
  const serverSummary = input.health
    ? summarizeFleet(presentationOutcomesFromHealth(input.health.servers))
    : null;
  // Delivery-lost without seeded rows is still a fleet diagnosis, not nominal.
  const deliveryLost = input.health?.delivery === "lost";
  const fleetSummary = serverSummary ?? (deliveryLost ? HEALTH_DELIVERY_LOST_SUMMARY : null);
  const showConnections = fleetSummary !== null;
  let activeText = "connected";
  let showRetry = false;
  if (input.connectionStatus === "closed") {
    activeText = "connection closed";
    showRetry = true;
  } else if (input.daemonDisconnected) {
    activeText = "daemon disconnected, reconnecting…";
  } else if (input.connectionStatus === "reconnecting") {
    activeText = "reconnecting to server…";
  } else if (input.connectionStatus === "connecting") {
    activeText = "connecting…";
  } else if (fleetSummary) {
    activeText = fleetSummary;
  }
  const activeNominal =
    input.connectionStatus === "open" && !input.daemonDisconnected && !showRetry;
  return {
    activeText,
    fleetSummary,
    showRetry,
    showConnections,
    // An open active session must not hide delivery-lost fleet degradation.
    nominal: activeNominal && !showConnections && !deliveryLost,
  };
}
