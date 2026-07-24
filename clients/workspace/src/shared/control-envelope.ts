/**
 * Closed {op,id,schema_version} JSON Lines envelope for Boundary-1.
 * Mirrors AgentGrid.Shell.Core.WorkspaceLauncher.ControlEnvelope.
 * additionalProperties: false — any extra field is rejected (FR-B1-02).
 */

export const CURRENT_SCHEMA_VERSION = 2;

export const ALLOWED_OPS = new Set(["openSession", "activate", "quit"] as const);
export type ControlOp = "openSession" | "activate" | "quit";

export interface ControlEnvelope {
  op: ControlOp;
  server_id?: string;
  session_id?: string;
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
    if (key !== "op" && key !== "server_id" && key !== "session_id" && key !== "schema_version") {
      return { ok: false, error: `unknown field: ${key}` };
    }
  }
  if (typeof obj.op !== "string") {
    return { ok: false, error: "missing op" };
  }
  if (!ALLOWED_OPS.has(obj.op as ControlOp)) {
    return { ok: false, error: `unknown op: ${obj.op}` };
  }
  const server_id = typeof obj.server_id === "string" ? obj.server_id : undefined;
  const session_id = typeof obj.session_id === "string" ? obj.session_id : undefined;
  if ("server_id" in obj && server_id === undefined) {
    return { ok: false, error: "server_id must be a string" };
  }
  if ("session_id" in obj && session_id === undefined) {
    return { ok: false, error: "session_id must be a string" };
  }
  if (obj.op === "openSession" && (!server_id || !session_id)) {
    return { ok: false, error: "openSession requires server_id and session_id" };
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
      server_id,
      session_id,
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
