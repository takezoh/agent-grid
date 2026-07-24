/**
 * Closed {op,id,schema_version} JSON Lines envelope for Boundary-1.
 * Mirrors AgentGrid.Shell.Core.WorkspaceLauncher.ControlEnvelope.
 * additionalProperties: false — any extra field is rejected (FR-B1-02).
 */

export const CURRENT_SCHEMA_VERSION = 1;

export const ALLOWED_OPS = new Set(["openSession", "activate", "quit"] as const);
export type ControlOp = "openSession" | "activate" | "quit";

export interface ControlEnvelope {
  op: ControlOp;
  id?: string;
  schema_version?: number;
}

export type ParseResult =
  | { ok: true; envelope: ControlEnvelope }
  | { ok: false; error: string };

export function parseControlLine(line: string): ParseResult {
  if (!line || !line.trim()) {
    return { ok: false, error: "empty line" };
  }
  let raw: unknown;
  try {
    raw = JSON.parse(line);
  } catch (e) {
    return { ok: false, error: `malformed json: ${(e as Error).message}` };
  }
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
    return { ok: false, error: "envelope must be an object" };
  }
  const obj = raw as Record<string, unknown>;
  for (const key of Object.keys(obj)) {
    if (key !== "op" && key !== "id" && key !== "schema_version") {
      return { ok: false, error: `unknown field: ${key}` };
    }
  }
  if (typeof obj.op !== "string") {
    return { ok: false, error: "missing op" };
  }
  if (!ALLOWED_OPS.has(obj.op as ControlOp)) {
    return { ok: false, error: `unknown op: ${obj.op}` };
  }
  let id: string | undefined;
  if ("id" in obj) {
    if (typeof obj.id !== "string") {
      return { ok: false, error: "id must be a string" };
    }
    id = obj.id;
  }
  if (obj.op === "openSession" && !id) {
    return { ok: false, error: "openSession requires id" };
  }
  let schema_version = CURRENT_SCHEMA_VERSION;
  if ("schema_version" in obj) {
    if (typeof obj.schema_version !== "number" || !Number.isInteger(obj.schema_version)) {
      return { ok: false, error: "schema_version must be int" };
    }
    schema_version = obj.schema_version;
  }
  return {
    ok: true,
    envelope: {
      op: obj.op as ControlOp,
      id,
      schema_version,
    },
  };
}

export function replyOk(): string {
  return JSON.stringify({ ok: true });
}

export function replyError(error: string): string {
  return JSON.stringify({ ok: false, error });
}
