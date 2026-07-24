export interface SessionRef {
  serverId: string;
  sessionId: string;
}

export function sessionKey(ref: SessionRef): string {
  return JSON.stringify([ref.serverId, ref.sessionId]);
}

export function assertSessionRef(ref: SessionRef): void {
  if (!ref.serverId) throw new Error("serverId required");
  if (!ref.sessionId) throw new Error("sessionId required");
}
