/**
 * Typed deep-link helpers for agent-grid:// URIs (FR-P1-09).
 * Hand-written cover file; generator may replace sibling transport code.
 */

export type DeepLinkKind = "session" | "approval";

export interface DeepLink {
  kind: DeepLinkKind;
  id: string;
  uri: string;
}

const RE = /^agent-grid:\/\/(session|approval)\/([^/?#]+)$/;

export function parseDeepLink(uri: string): DeepLink {
  const m = RE.exec(uri);
  if (!m) {
    throw new Error(`malformed agent-grid URI: ${uri}`);
  }
  return { kind: m[1] as DeepLinkKind, id: m[2], uri };
}

export function constructDeepLink(kind: DeepLinkKind, id: string): DeepLink {
  if (!id) {
    throw new Error("deep link id required");
  }
  const uri = `agent-grid://${kind}/${id}`;
  return { kind, id, uri };
}
