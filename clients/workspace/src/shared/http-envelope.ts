/**
 * The shape an HTTP response takes when it crosses the preload IPC boundary,
 * and the one invariant that shape has to carry.
 *
 * A response cannot be handed across IPC as a `Response` object, so main
 * flattens it to `{status, headers, body}`. The flattening used to type `body`
 * as `string` and fill it with `await response.text()` unconditionally, which
 * made "no body" indistinguishable from "empty body". For a null-body status
 * the renderer then rebuilt `new Response("", {status: 204})`, which Chromium
 * rejects, and api/sessions.ts reported the resulting throw as
 * `Error("network")` — a successful DELETE surfaced to the user as
 * "Failed to stop session (network error)".
 *
 * design-host.md's IPC Type Design Invariants already say that a zero value
 * carrying a second meaning belongs in a separate type. This module applies
 * that: `body: string | null`, with `null` reserved for statuses that may not
 * carry a body at all. Owning it here rather than in the renderer means the
 * next route added through this boundary inherits the invariant instead of
 * rediscovering it.
 */

/** An HTTP response flattened for transport across the preload IPC boundary. */
export interface HttpResponseEnvelope {
  status: number;
  headers: Record<string, string>;
  /** `null` iff the status may not carry a body. `""` is an empty body. */
  body: string | null;
}

/**
 * Statuses the WHATWG Fetch Standard forbids from carrying a body.
 *
 * This constant is subordinate to the platform, not a policy of ours: the
 * oracle guard in http-envelope.test.ts asserts that `new Response("", {status})`
 * throws for every member and does not throw for representative non-members. If
 * a runtime ever disagrees — adding 103, say — that test fails and forces this
 * list to be updated rather than letting the two drift apart silently.
 */
export const NULL_BODY_STATUSES: ReadonlySet<number> = new Set([101, 204, 205, 304]);

export function isNullBodyStatus(status: number): boolean {
  return NULL_BODY_STATUSES.has(status);
}

/**
 * Flatten a fetch `Response` into an envelope.
 *
 * The status is the authority for whether a body may exist — not whether the
 * upstream actually sent bytes. An upstream that puts a body on a 204 is
 * misbehaving, and forwarding those bytes would produce an envelope the
 * renderer cannot turn back into a `Response`.
 */
export async function buildResponseEnvelope(response: Response): Promise<HttpResponseEnvelope> {
  return {
    status: response.status,
    headers: Object.fromEntries(response.headers.entries()),
    body: isNullBodyStatus(response.status) ? null : await response.text(),
  };
}
