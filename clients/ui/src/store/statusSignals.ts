import { parseSessionKey, sessionKey } from "../fleet/session-protocol";
import type { SessionInfo } from "../wire/server";

const highestConsumed = new Map<string, number>();
const reconnectSuppressed = new Set<string>();

/** Stable signal identity across bare wire ids and fleet-qualified store ids. */
function signalIdentityKey(session: SessionInfo, epoch: string): string {
  const fleet = parseSessionKey(session.id);
  // Prefer the fleet-qualified id's server when present so a mismatched
  // serverId tag (e.g. initialServerHint vs Connection.serverId) cannot
  // split consume/seed maps.
  const bareId = session.upstreamSessionId ?? fleet?.sessionId ?? session.id;
  const serverId = fleet?.serverId ?? session.serverId;
  const identity =
    serverId != null && serverId !== "" ? sessionKey({ serverId, sessionId: bareId }) : bareId;
  return `${identity}\u0000${epoch}`;
}

function projectStatusSignals(
  sessions: readonly SessionInfo[],
  reconnectHello: boolean,
): SessionInfo[] {
  return sessions.map((session) => {
    const signal = session.status.signal;
    if (!signal || signal.kind !== "bell") return session;
    // turn/completed and the following idle transition are distinct server
    // events. Do not spend the one-shot bell on the preceding active frame:
    // that would render an unobserved spinner+bell combination and leave the
    // stable idle frame silent.
    if (session.status.lifecycle === "active") {
      return { ...session, status: { ...session.status, signal: undefined } };
    }
    const key = signalIdentityKey(session, signal.epoch);
    const highest = highestConsumed.get(key) ?? 0;
    if (reconnectHello && signal.sequence <= highest) {
      reconnectSuppressed.add(key);
    }
    if (signal.sequence < highest || reconnectSuppressed.has(key)) {
      return { ...session, status: { ...session.status, signal: undefined } };
    }
    if (signal.sequence > highest) {
      highestConsumed.set(key, signal.sequence);
      reconnectSuppressed.delete(key);
    }
    return session;
  });
}

// Normal view updates retain the newest bell so it is actually observable.
// A repeated snapshot in the same connection is not a replay.
export function consumeStatusSignals(sessions: readonly SessionInfo[]): SessionInfo[] {
  return projectStatusSignals(sessions, false);
}

// A hello frame identifies a new connection. An already-seen sequence in that
// frame is reconnect replay and remains suppressed for later snapshots until a
// newer sequence arrives.
export function seedStatusSignals(sessions: readonly SessionInfo[]): SessionInfo[] {
  return projectStatusSignals(sessions, true);
}

export function resetConsumedStatusSignals(): void {
  highestConsumed.clear();
  reconnectSuppressed.clear();
}
