// Projection of protocol/protocol-version.json.
export const PROTOCOL_VERSION = "protocol-3";
export const PROTOCOL_VERSION_HEADER = "X-Agent-Grid-Protocol-Version";

/** Closed protocol-version token; credential-like or free-form values become <invalid>. */
export function sanitizeProtocolVersionToken(actual: unknown): string {
  if (typeof actual !== "string" || !actual) return "<missing>";
  if (/^protocol-\d+$/.test(actual)) return actual;
  return "<invalid>";
}

/** Closed protocol-mismatch detail contract. Never interpolates untrusted raw. */
export function protocolMismatchReason(actual: unknown): string {
  return `incompatible protocol: client ${PROTOCOL_VERSION}, server ${sanitizeProtocolVersionToken(actual)}`;
}
