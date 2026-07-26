import type { SessionInfo } from "../wire/server";

const highestConsumed = new Map<string, number>();

export function consumeStatusSignals(sessions: readonly SessionInfo[]): SessionInfo[] {
  return sessions.map((session) => {
    const signal = session.status.signal;
    if (!signal || signal.kind !== "bell") return session;
    const key = `${session.id}\u0000${signal.epoch}`;
    const highest = highestConsumed.get(key) ?? 0;
    if (signal.sequence <= highest) {
      return { ...session, status: { ...session.status, signal: undefined } };
    }
    highestConsumed.set(key, signal.sequence);
    return session;
  });
}

export function resetConsumedStatusSignals(): void {
  highestConsumed.clear();
}
