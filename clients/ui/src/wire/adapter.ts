/**
 * Wire adapter seam for incremental migration from the hand-written codec
 * (ADR-0021) toward the generated TypeScript SDK under clients/sdk/ts
 * (change-20260723-native-clients-phase01 chunk p1-05).
 *
 * Models: clients/sdk/ts/generated (quicktype ← protocol/*.schema.json).
 * Transport: clients/sdk/ts/src (hand-written; openapi.yaml is REST annex only).
 * Mode flag selects the decode path; generated path currently stubs to
 * the hand-written codec until full cutover.
 */

import type { ClientFrame } from "./client";
import type { ServerFrame } from "./server";

export type WireMode = "handwritten" | "generated";

let mode: WireMode = "handwritten";

export function getWireMode(): WireMode {
  return mode;
}

export function setWireMode(next: WireMode): void {
  mode = next;
}

export type ParseServerFrame = (raw: string) => ServerFrame | null;
export type SerializeClientFrame = (f: ClientFrame) => string;

export interface WireCodec {
  parseServerFrame: ParseServerFrame;
  serializeClientFrame: SerializeClientFrame;
}

/**
 * generatedCodecStub compiles against the same surface as the hand-written
 * codec. When clients/sdk/ts exports a full codec, wire it here without
 * changing call sites.
 */
export function generatedCodecStub(handwritten: WireCodec): WireCodec {
  return {
    parseServerFrame: (raw) => handwritten.parseServerFrame(raw),
    serializeClientFrame: (f) => handwritten.serializeClientFrame(f),
  };
}

export function selectCodec(handwritten: WireCodec): WireCodec {
  if (mode === "generated") {
    return generatedCodecStub(handwritten);
  }
  return handwritten;
}
