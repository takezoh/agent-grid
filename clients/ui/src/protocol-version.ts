// Projection of protocol/protocol-version.json. Static conformance tests keep
// every shipped client projection byte-identical to the manifest.
export const PROTOCOL_VERSION = "protocol-3";

export const PROTOCOL_VERSION_HEADER = "X-Agent-Grid-Protocol-Version";

/**
 * Closed protocol-version token for wire/DOM detail. Untrusted/raw values
 * (credential-like material, free-form strings) never survive as-is.
 */
export function sanitizeProtocolVersionToken(actual: unknown): string {
  if (typeof actual !== "string" || !actual) return "<missing>";
  if (/^protocol-\d+$/.test(actual)) return actual;
  return "<invalid>";
}

/** Closed protocol-mismatch detail contract shared by listConfigs and health. */
export function protocolMismatchReason(actual: unknown): string {
  return `incompatible protocol: client ${PROTOCOL_VERSION}, server ${sanitizeProtocolVersionToken(actual)}`;
}
