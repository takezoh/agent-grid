import type { SessionInfo } from "../wire/server";

// Session identity owner (ADR adr-fleet-session-identity-boundary-ownership).
//
// Two id spaces exist in this client and they are not interchangeable:
//
//   fleet key          `["srv-a","s1"]`  — how the SPA addresses a session across servers
//   upstream session id `s1`             — how one gateway addresses its own session
//
// Both used to be plain `string`, so every crossing between them grew its own
// local convention: 19 call sites across 8 modules with 8 divergent failure
// policies, two of which decided the opposite thing about the same input. This
// module is the single owner for that translation on the store / HTTP side;
// `socket/connection.ts` owns the WS wire side.
//
// It lives beside `session-protocol.ts` rather than inside it: that file is
// already 432 lines and owns health schema and decoding, and the file-length
// rule in AGENTS.md is a responsibility-splitting heuristic worth honoring
// here. `session-protocol.ts` re-exports these names so existing importers are
// untouched.

export interface SessionRef {
  serverId: string;
  sessionId: string;
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

/**
 * A session as the store and the UI hold it.
 *
 * `SessionInfo` is the wire mirror: it decodes a gateway payload, whose `id` is
 * a bare upstream id and which never carries fleet fields. A store row is a
 * different thing — it is fleet-addressed and it knows which server it came
 * from — so it is a different type. Requiring `serverId` and
 * `upstreamSessionId` here is what makes "the authority is present" a compile
 * error to violate, and it is why the optional-vs-required question on
 * `SessionInfo` disappears instead of being answered.
 */
export interface FleetSessionRow extends SessionInfo {
  /** Compound fleet key — `sessionKey({serverId, sessionId: upstreamSessionId})`. */
  id: string;
  serverId: string;
  upstreamSessionId: string;
  /** Display name for the owning server; the id is the authority, this is chrome. */
  serverLabel?: string;
}

/**
 * Where a session-scoped HTTP request should go.
 *
 * `source` records which authority answered, so a test can pin precedence
 * rather than only the resolved value.
 */
export type SessionRoute =
  | { ok: true; serverId: string; upstreamSessionId: string; source: "row" | "decode" }
  | { ok: false; reason: "unresolved" };

/**
 * Resolve the session component of a request path.
 *
 * Precedence is row authority > id-string decode > unresolved. The row wins
 * even when the id string decodes to something else: re-deriving a route from
 * an id string while holding the authoritative row is the pattern that produced
 * this defect class twice already.
 *
 * This never throws. `api/sessions.ts`'s `doFetch` maps every non-Abort throw
 * to `Error("network")`, so a throwing resolver would report an internal
 * contract violation as a transport failure — the same misdiagnosis this
 * change is fixing on the Electron side. Callers branch on `ok` before issuing
 * a request, which keeps the failure outside `doFetch`'s catch entirely.
 */
export function resolveSessionRoute(input: FleetSessionRow | string): SessionRoute {
  if (typeof input !== "string") {
    return {
      ok: true,
      serverId: input.serverId,
      upstreamSessionId: input.upstreamSessionId,
      source: "row",
    };
  }
  const ref = parseSessionKey(input);
  if (ref) {
    return { ok: true, serverId: ref.serverId, upstreamSessionId: ref.sessionId, source: "decode" };
  }
  return { ok: false, reason: "unresolved" };
}

/**
 * The fleet key for a session on a given server.
 *
 * Callers that mint an id — the membership producer, a create response, a
 * capsule listing — say what they mean here instead of encoding the key
 * themselves. Keeping `sessionKey` for the module's own use is what lets the
 * ownership gate treat a direct import of it as a new crossing.
 */
export function fleetSessionIdFor(ref: SessionRef): string {
  return sessionKey(ref);
}

/**
 * Build a store row from a fleet reference and the gateway's session payload.
 *
 * The row's id, server and upstream id all come from the same reference, so a
 * row cannot end up keyed by one server and tagged with another.
 */
export function toFleetSessionRow(
  ref: SessionRef,
  session: SessionInfo,
  serverLabel?: string,
): FleetSessionRow {
  return {
    ...session,
    id: sessionKey(ref),
    serverId: ref.serverId,
    upstreamSessionId: ref.sessionId,
    ...(serverLabel === undefined ? {} : { serverLabel }),
  };
}
