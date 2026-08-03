// Projection of protocol/protocol-version.json.
export const PROTOCOL_VERSION = "protocol-3";
export const PROTOCOL_VERSION_HEADER = "X-Agent-Grid-Protocol-Version";

export function protocolMismatchReason(actual: unknown): string {
  const observed = typeof actual === "string" && actual ? actual : "<missing>";
  return `incompatible protocol: client ${PROTOCOL_VERSION}, server ${observed}`;
}
