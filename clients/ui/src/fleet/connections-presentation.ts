import { fleetOutcomeLabel, observationToPresentation } from "./presentation";
import type {
  FleetHealthServerV1,
  FleetHealthView,
  QualifiedHealthObservation,
} from "./session-protocol";

export interface ConnectionsRow {
  serverId: string;
  label: string;
  state: string;
  stateLabel: string;
  reason?: string;
  stalePrior?: boolean;
  enabled: boolean;
}

export interface ConnectionsDiagnosis {
  rows: ConnectionsRow[];
  configPath?: string;
  restartInstruction?: string;
  /**
   * Credential-free notice when delivery is lost before any configured-server
   * row exists (config seed failed or never arrived). Absent for normal empty
   * fleets and non-empty observations.
   */
  deliveryLossExplanation?: string;
  /** Actionable reconnect / configuration guidance for the dual-failure state. */
  recoveryGuidance?: string;
}

/** Zero-server delivery-loss copy shared by pure projection consumers. */
export const DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION =
  "Fleet health delivery was lost before server configuration could be loaded.";

export const DELIVERY_LOST_BEFORE_CONFIG_RECOVERY =
  "Wait for the host health channel to reconnect automatically. Confirm the web host is running and can load servers.json; after configuration changes, restart the web host and reopen Connections.";

function stateLabel(observation: QualifiedHealthObservation): string {
  switch (observation.kind) {
    case "connected":
      return "Connected";
    case "checking":
      return "Checking";
    case "disabled":
      return "Disabled";
    case "incompatible_protocol":
      return "Incompatible protocol";
    case "health_unavailable":
      return "Health unavailable";
    case "unavailable":
      return "Unavailable";
  }
}

function disambiguatedLabel(
  servers: readonly FleetHealthServerV1[],
  server: FleetHealthServerV1,
): string {
  const collisions = servers.filter((candidate) => candidate.serverLabel === server.serverLabel);
  if (collisions.length <= 1) return server.serverLabel;
  return `${server.serverLabel} · ${server.serverId}`;
}

/** Sole pure owner of Connections row/label/reason semantics for both hosts. */
export function projectConnectionsDiagnosis(
  health: FleetHealthView | null,
  options: { configPath?: string; hostKind?: "browser" | "workspace" } = {},
): ConnectionsDiagnosis {
  const servers = health?.servers ?? [];
  const rows: ConnectionsRow[] = servers.map((server) => {
    const presentation = observationToPresentation(server.observation);
    const label = disambiguatedLabel(servers, server);
    const reason =
      fleetOutcomeLabel(presentation)?.replace(
        /^(?:Unavailable|Incompatible protocol|Health unavailable|Checking…)(?: — )?/,
        "",
      ) || presentation.reason;
    return {
      serverId: server.serverId,
      label,
      state: presentation.state,
      stateLabel: stateLabel(server.observation),
      ...(reason ? { reason } : {}),
      stalePrior: presentation.stale === true,
      enabled: server.enabled,
    };
  });
  const diagnosis: ConnectionsDiagnosis = { rows };
  // Dual failure: health delivery closed before any configured-server seed.
  // StatusBar already surfaces Connections; the panel must not render blank.
  if (health?.delivery === "lost" && servers.length === 0) {
    diagnosis.deliveryLossExplanation = DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION;
    diagnosis.recoveryGuidance = DELIVERY_LOST_BEFORE_CONFIG_RECOVERY;
  }
  if (options.configPath) {
    diagnosis.configPath = options.configPath;
    if (options.hostKind === "browser") {
      diagnosis.restartInstruction =
        "Edit servers.json at the path below, then restart the web host to apply configuration changes.";
    }
  } else if (diagnosis.deliveryLossExplanation && options.hostKind === "browser") {
    // Config path unavailable (e.g. /host/config failed) — still guide settings recovery.
    diagnosis.restartInstruction =
      "Server configuration path is unavailable from the host. Fix servers.json access for the web host, then restart the host to apply configuration.";
  }
  return diagnosis;
}
