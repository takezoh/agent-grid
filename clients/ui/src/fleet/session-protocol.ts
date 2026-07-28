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
      member.sessions.map((session) => ({
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
