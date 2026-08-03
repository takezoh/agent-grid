/**
 * Producer-side health classifier and legacy projection helpers.
 * Host authority for observation generation lives on FleetSessionRuntime.
 */

export type HealthReason =
  | "connection_timeout"
  | "connection_failed"
  | "http_error"
  | "websocket_failed"
  | "empty_ticket"
  | "protocol_mismatch"
  | "refresh_failed"
  | "delivery_lost";

export const HEALTH_REASONS: readonly HealthReason[] = [
  "connection_timeout",
  "connection_failed",
  "http_error",
  "websocket_failed",
  "empty_ticket",
  "protocol_mismatch",
  "refresh_failed",
  "delivery_lost",
] as const;

export type CurrentHealthKind =
  | "checking"
  | "connected"
  | "unavailable"
  | "incompatible_protocol";

export type PriorHealthKind = "connected" | "unavailable" | "incompatible_protocol";

export interface HealthPriorObservation {
  kind: PriorHealthKind;
  observedAt: string;
  reason?: HealthReason;
  detail?: string;
}

export type QualifiedHealthObservation =
  | { kind: "checking"; observedAt: string }
  | { kind: "connected"; observedAt: string }
  | { kind: "unavailable"; observedAt: string; reason?: HealthReason; detail?: string }
  | {
      kind: "incompatible_protocol";
      observedAt: string;
      reason?: HealthReason;
      detail?: string;
    }
  | { kind: "disabled" }
  | {
      kind: "health_unavailable";
      observedAt: string;
      reason?: HealthReason;
      detail?: string;
      prior?: HealthPriorObservation;
    };

export interface FleetHealthServerV1 {
  serverId: string;
  serverLabel: string;
  enabled: boolean;
  observation: QualifiedHealthObservation;
}

export interface FleetHealthSnapshotV1 {
  schemaVersion: 1;
  hostInstanceId: string;
  revision: number;
  servers: FleetHealthServerV1[];
}

/** @deprecated Prefer FleetHealthSnapshotV1; retained for settings bridge migration. */
export type ServerHealthState =
  | "connected"
  | "unavailable"
  | "incompatible_protocol"
  | "disabled"
  | "checking"
  | "health_unavailable";

/** @deprecated Prefer FleetHealthSnapshotV1. */
export interface ServerHealth {
  serverId: string;
  displayName: string;
  state: ServerHealthState;
  reason?: string;
  stale?: boolean;
}

/** @deprecated Prefer FleetHealthSnapshotV1. */
export interface FleetHealth {
  checkedAt: string;
  hostInstanceId?: string;
  revision?: number;
  servers: ServerHealth[];
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

export function healthReasonLabel(reason: HealthReason | undefined, detail?: string): string | undefined {
  if (detail) return detail;
  if (!reason) return undefined;
  return HEALTH_REASON_LABEL[reason];
}

/**
 * Converts raw transport failures into a closed HealthReason before any
 * snapshot construction or health logging. Never returns raw error text.
 */
export function classifyHealthFailure(error: unknown): {
  reason: HealthReason;
  detail?: string;
} {
  const message = error instanceof Error ? error.message : String(error);
  if (/timeout|timed out|aborted due to timeout/i.test(message)) {
    return { reason: "connection_timeout" };
  }
  if (/empty websocket ticket/i.test(message)) {
    return { reason: "empty_ticket" };
  }
  if (/websocket/i.test(message)) {
    return { reason: "websocket_failed" };
  }
  if (/^(?:ws-ticket )?HTTP \d{3}$/i.test(message) || /\bHTTP \d{3}\b/i.test(message)) {
    return { reason: "http_error" };
  }
  if (
    /fetch failed|failed to fetch|econnrefused|enotfound|unable to connect|network error|connection refused/i.test(
      message,
    )
  ) {
    return { reason: "connection_failed" };
  }
  if (/incompatible protocol/i.test(message)) {
    // Prefer a closed detail string. If the message already matches the closed
    // contract (client protocol-N, server protocol-M|<missing>|<invalid>), keep
    // it; otherwise omit free-form detail so reason label is used.
    const closed =
      /^incompatible protocol: client protocol-\d+, server (?:protocol-\d+|<(?:missing|invalid)>)$/.test(
        message,
      );
    return {
      reason: "protocol_mismatch",
      ...(closed ? { detail: message } : {}),
    };
  }
  return { reason: "connection_failed" };
}

/** Stable user-facing reason text used by legacy session-member failure fields. */
export function connectionFailureReason(error: unknown): string {
  const classified = classifyHealthFailure(error);
  return healthReasonLabel(classified.reason, classified.detail) ?? "Server connection failed";
}

export function observationToLegacyState(
  observation: QualifiedHealthObservation,
): { state: ServerHealthState; reason?: string; stale?: boolean } {
  if (observation.kind === "disabled") return { state: "disabled" };
  if (observation.kind === "checking") return { state: "checking" };
  if (observation.kind === "connected") return { state: "connected" };
  if (observation.kind === "unavailable") {
    return {
      state: "unavailable",
      reason: healthReasonLabel(observation.reason, observation.detail),
    };
  }
  if (observation.kind === "incompatible_protocol") {
    return {
      state: "incompatible_protocol",
      reason: healthReasonLabel(observation.reason, observation.detail),
    };
  }
  return {
    state: "health_unavailable",
    reason: healthReasonLabel(observation.reason, observation.detail),
    stale: Boolean(observation.prior),
  };
}

export function legacyFleetHealthFromSnapshot(snapshot: FleetHealthSnapshotV1): FleetHealth {
  return {
    checkedAt: new Date().toISOString(),
    hostInstanceId: snapshot.hostInstanceId,
    revision: snapshot.revision,
    servers: snapshot.servers.map((server) => {
      const legacy = observationToLegacyState(server.observation);
      return {
        serverId: server.serverId,
        displayName: server.serverLabel,
        state: legacy.state,
        ...(legacy.reason ? { reason: legacy.reason } : {}),
        ...(legacy.stale ? { stale: true } : {}),
      };
    }),
  };
}

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function isHealthReason(value: unknown): value is HealthReason {
  return typeof value === "string" && (HEALTH_REASONS as readonly string[]).includes(value);
}

function parsePrior(value: unknown): HealthPriorObservation | undefined {
  if (value === undefined) return undefined;
  if (!isObject(value)) throw new Error("invalid health prior");
  if ("stale" in value) throw new Error("stale flag is not allowed on prior");
  const kind = value.kind;
  if (kind !== "connected" && kind !== "unavailable" && kind !== "incompatible_protocol") {
    throw new Error("invalid health prior kind");
  }
  if (typeof value.observedAt !== "string" || !value.observedAt) {
    throw new Error("prior observedAt required");
  }
  const prior: HealthPriorObservation = { kind, observedAt: value.observedAt };
  if (value.reason !== undefined) {
    if (!isHealthReason(value.reason)) throw new Error("invalid health reason");
    prior.reason = value.reason;
  }
  if (value.detail !== undefined) {
    if (typeof value.detail !== "string") throw new Error("invalid health detail");
    prior.detail = value.detail;
  }
  return prior;
}

function parseObservation(value: unknown): QualifiedHealthObservation {
  if (!isObject(value)) throw new Error("invalid health observation");
  if ("stale" in value) throw new Error("independent stale flag is forbidden");
  const kind = value.kind;
  if (kind === "disabled") {
    if (Object.keys(value).some((key) => key !== "kind")) {
      throw new Error("disabled observation must not carry freshness");
    }
    return { kind: "disabled" };
  }
  if (typeof value.observedAt !== "string" || !value.observedAt) {
    throw new Error("observedAt required");
  }
  if (kind === "checking") {
    if (value.reason !== undefined || value.prior !== undefined || value.detail !== undefined) {
      throw new Error("checking observation is exclusive");
    }
    return { kind: "checking", observedAt: value.observedAt };
  }
  if (kind === "connected") {
    if (value.reason !== undefined || value.prior !== undefined || value.detail !== undefined) {
      throw new Error("connected observation is exclusive");
    }
    return { kind: "connected", observedAt: value.observedAt };
  }
  if (kind === "unavailable" || kind === "incompatible_protocol") {
    if (value.prior !== undefined) throw new Error("current observation cannot nest prior");
    const observation: Extract<
      QualifiedHealthObservation,
      { kind: "unavailable" | "incompatible_protocol" }
    > = { kind, observedAt: value.observedAt };
    if (value.reason !== undefined) {
      if (!isHealthReason(value.reason)) throw new Error("invalid health reason");
      observation.reason = value.reason;
    }
    if (value.detail !== undefined) {
      if (typeof value.detail !== "string") throw new Error("invalid health detail");
      observation.detail = value.detail;
    }
    return observation;
  }
  if (kind === "health_unavailable") {
    const observation: Extract<QualifiedHealthObservation, { kind: "health_unavailable" }> = {
      kind: "health_unavailable",
      observedAt: value.observedAt,
    };
    if (value.reason !== undefined) {
      if (!isHealthReason(value.reason)) throw new Error("invalid health reason");
      observation.reason = value.reason;
    }
    if (value.detail !== undefined) {
      if (typeof value.detail !== "string") throw new Error("invalid health detail");
      observation.detail = value.detail;
    }
    const prior = parsePrior(value.prior);
    if (prior) observation.prior = prior;
    return observation;
  }
  throw new Error("unknown health observation kind");
}

export function decodeFleetHealthSnapshotV1(input: unknown): FleetHealthSnapshotV1 {
  if (!isObject(input)) throw new Error("health snapshot must be an object");
  if (input.schemaVersion !== 1) throw new Error("unknown health schema version");
  if (typeof input.hostInstanceId !== "string" || !input.hostInstanceId) {
    throw new Error("hostInstanceId required");
  }
  if (typeof input.revision !== "number" || !Number.isInteger(input.revision) || input.revision < 0) {
    throw new Error("revision required");
  }
  if (!Array.isArray(input.servers)) throw new Error("servers required");
  return {
    schemaVersion: 1,
    hostInstanceId: input.hostInstanceId,
    revision: input.revision,
    servers: input.servers.map((entry) => {
      if (!isObject(entry)) throw new Error("invalid health server entry");
      if (typeof entry.serverId !== "string" || !entry.serverId) throw new Error("serverId required");
      if (typeof entry.serverLabel !== "string") throw new Error("serverLabel required");
      if (typeof entry.enabled !== "boolean") throw new Error("enabled required");
      return {
        serverId: entry.serverId,
        serverLabel: entry.serverLabel,
        enabled: entry.enabled,
        observation: parseObservation(entry.observation),
      };
    }),
  };
}

/** Canonical re-encode through the decoder (parity with UI encodeFleetHealthSnapshotV1). */
export function encodeFleetHealthSnapshotV1(snapshot: FleetHealthSnapshotV1): FleetHealthSnapshotV1 {
  return decodeFleetHealthSnapshotV1(JSON.parse(JSON.stringify(snapshot)));
}
