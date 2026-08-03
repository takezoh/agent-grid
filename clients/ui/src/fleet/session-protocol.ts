import type { SessionInfo } from "../wire/server";

export interface SessionRef {
  serverId: string;
  sessionId: string;
}

export type FleetMemberState =
  | "connecting"
  | "connected"
  | "reconnecting"
  | "unavailable"
  | "incompatible_protocol";

export interface FleetMember {
  serverId: string;
  serverLabel: string;
  generation: number;
  state: FleetMemberState;
  stale?: boolean;
  reason?: string;
  sessions: SessionInfo[];
}

export interface FleetSession {
  ref: SessionRef;
  serverLabel: string;
  session: SessionInfo;
  stale: boolean;
}

export interface FleetSnapshot {
  revision: number;
  members: FleetMember[];
  sessions: FleetSession[];
}

/** Closed producer-side health reason. Never carries raw transport text. */
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

export type CurrentHealthKind = "checking" | "connected" | "unavailable" | "incompatible_protocol";

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

export type HealthDeliveryLiveness = "connecting" | "live" | "lost";

/** Consumer view after delivery-liveness overlay; host identity/revision are never minted here. */
export interface FleetHealthView {
  hostInstanceId: string | null;
  revision: number | null;
  delivery: HealthDeliveryLiveness;
  servers: FleetHealthServerV1[];
}

export function sessionKey(ref: SessionRef): string {
  return JSON.stringify([ref.serverId, ref.sessionId]);
}

export function parseSessionKey(value: string): SessionRef | null {
  try {
    const parsed = JSON.parse(value) as unknown;
    if (
      Array.isArray(parsed) &&
      parsed.length === 2 &&
      typeof parsed[0] === "string" &&
      typeof parsed[1] === "string"
    ) {
      return { serverId: parsed[0], sessionId: parsed[1] };
    }
  } catch {
    // Legacy/local IDs are not compound fleet keys.
  }
  return null;
}

export function initialFleetSnapshot(
  servers: Array<{ serverId: string; serverLabel: string }>,
): FleetSnapshot {
  return projectSnapshot(
    0,
    servers.map((server) => ({
      ...server,
      generation: 0,
      state: "connecting",
      sessions: [],
    })),
  );
}

export function reduceFleetMember(snapshot: FleetSnapshot, incoming: FleetMember): FleetSnapshot {
  const index = snapshot.members.findIndex((member) => member.serverId === incoming.serverId);
  if (index < 0) return snapshot;
  const current = snapshot.members[index];
  if (!current) return snapshot;
  if (incoming.generation < current.generation) return snapshot;

  const failed = incoming.state !== "connected";
  const replacement: FleetMember = failed
    ? {
        ...incoming,
        stale: current.sessions.length > 0,
        sessions: current.sessions,
      }
    : { ...incoming, stale: false };
  if (JSON.stringify(replacement) === JSON.stringify(current)) return snapshot;

  const members = snapshot.members.slice();
  members[index] = replacement;
  return projectSnapshot(snapshot.revision + 1, members);
}

function projectSnapshot(revision: number, members: FleetMember[]): FleetSnapshot {
  const labelCounts = new Map<string, number>();
  for (const member of members) {
    labelCounts.set(member.serverLabel, (labelCounts.get(member.serverLabel) ?? 0) + 1);
  }
  return {
    revision,
    members,
    sessions: members.flatMap((member) =>
      (member.sessions ?? []).map((session) => ({
        ref: { serverId: member.serverId, sessionId: session.id },
        serverLabel:
          labelCounts.get(member.serverLabel) === 1
            ? member.serverLabel
            : `${member.serverLabel} · ${member.serverId}`,
        session: {
          ...session,
          serverId: member.serverId,
          serverLabel: member.serverLabel,
        },
        stale: member.stale === true,
      })),
    ),
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
    > = {
      kind,
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

/** Decode and validate FleetHealthSnapshotV1. Rejects unknown/invalid unions. */
export function decodeFleetHealthSnapshotV1(input: unknown): FleetHealthSnapshotV1 {
  if (!isObject(input)) throw new Error("health snapshot must be an object");
  if (input.schemaVersion !== 1) throw new Error("unknown health schema version");
  if (typeof input.hostInstanceId !== "string" || !input.hostInstanceId) {
    throw new Error("hostInstanceId required");
  }
  if (
    typeof input.revision !== "number" ||
    !Number.isInteger(input.revision) ||
    input.revision < 0
  ) {
    throw new Error("revision required");
  }
  if (!Array.isArray(input.servers)) throw new Error("servers required");
  const servers: FleetHealthServerV1[] = input.servers.map((entry) => {
    if (!isObject(entry)) throw new Error("invalid health server entry");
    if (typeof entry.serverId !== "string" || !entry.serverId) {
      throw new Error("serverId required");
    }
    if (typeof entry.serverLabel !== "string") throw new Error("serverLabel required");
    if (typeof entry.enabled !== "boolean") throw new Error("enabled required");
    return {
      serverId: entry.serverId,
      serverLabel: entry.serverLabel,
      enabled: entry.enabled,
      observation: parseObservation(entry.observation),
    };
  });
  return {
    schemaVersion: 1,
    hostInstanceId: input.hostInstanceId,
    revision: input.revision,
    servers,
  };
}

export function encodeFleetHealthSnapshotV1(
  snapshot: FleetHealthSnapshotV1,
): FleetHealthSnapshotV1 {
  return decodeFleetHealthSnapshotV1(JSON.parse(JSON.stringify(snapshot)));
}

/**
 * Delivery-loss overlay: nest last host observation as stale prior evidence
 * without advancing hostInstanceId or revision.
 */
export function applyHealthDeliveryLoss(
  snapshot: FleetHealthSnapshotV1,
  nowIso: string,
): FleetHealthView {
  return {
    hostInstanceId: snapshot.hostInstanceId,
    revision: snapshot.revision,
    delivery: "lost",
    servers: snapshot.servers.map((server) => {
      if (!server.enabled || server.observation.kind === "disabled") {
        return server;
      }
      let prior: HealthPriorObservation | undefined;
      if (server.observation.kind === "connected") {
        prior = { kind: "connected", observedAt: server.observation.observedAt };
      } else if (
        server.observation.kind === "unavailable" ||
        server.observation.kind === "incompatible_protocol"
      ) {
        prior = {
          kind: server.observation.kind,
          observedAt: server.observation.observedAt,
          ...(server.observation.reason ? { reason: server.observation.reason } : {}),
          ...(server.observation.detail ? { detail: server.observation.detail } : {}),
        };
      } else if (server.observation.kind === "health_unavailable") {
        prior = server.observation.prior;
      }
      return {
        ...server,
        observation: {
          kind: "health_unavailable",
          observedAt: nowIso,
          reason: "delivery_lost",
          ...(prior ? { prior } : {}),
        },
      };
    }),
  };
}

export function healthViewFromSnapshot(snapshot: FleetHealthSnapshotV1): FleetHealthView {
  return {
    hostInstanceId: snapshot.hostInstanceId,
    revision: snapshot.revision,
    delivery: "live",
    servers: snapshot.servers,
  };
}

export function initialHealthView(
  servers: Array<{ serverId: string; serverLabel: string; enabled: boolean }>,
  nowIso: string,
): FleetHealthView {
  return {
    hostInstanceId: null,
    revision: null,
    delivery: "connecting",
    servers: servers.map((server) => ({
      serverId: server.serverId,
      serverLabel: server.serverLabel,
      enabled: server.enabled,
      observation: server.enabled ? { kind: "checking", observedAt: nowIso } : { kind: "disabled" },
    })),
  };
}

/**
 * Delivery-lost seed when no host snapshot has been accepted yet: one row per
 * configured server from fleet authority (no secrets; not the active axis).
 */
export function deliveryLostWithoutSnapshot(
  servers: Array<{ serverId: string; serverLabel: string; enabled: boolean }>,
  nowIso: string,
): FleetHealthView {
  return {
    hostInstanceId: null,
    revision: null,
    delivery: "lost",
    servers: servers.map((server) => ({
      serverId: server.serverId,
      serverLabel: server.serverLabel,
      enabled: server.enabled,
      observation: server.enabled
        ? {
            kind: "health_unavailable" as const,
            observedAt: nowIso,
            reason: "delivery_lost" as const,
          }
        : { kind: "disabled" as const },
    })),
  };
}
