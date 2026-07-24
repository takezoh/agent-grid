import { selectCodec } from "./adapter";
import type { ClientFrame } from "./client";
import type {
  ActivityEvent,
  ActivityEventEntry,
  FileEventKind,
  ServerFrame,
  SessionInfo,
} from "./server";

// parseSessionInfoLoose validates that an object has at minimum the fields
// required for a valid SessionInfo wire value: id string, view object with card object.
function parseSessionInfoLoose(obj: unknown): obj is SessionInfo {
  if (typeof obj !== "object" || obj === null) return false;
  const sess = obj as Record<string, unknown>;
  if (typeof sess.id !== "string") return false;
  if (typeof sess.view !== "object" || sess.view === null) return false;
  const view = sess.view as Record<string, unknown>;
  if (typeof view.card !== "object" || view.card === null) return false;
  return true;
}

function parseFileEventKind(v: unknown): v is FileEventKind {
  return v === "read" || v === "create" || v === "edit" || v === "delete";
}

function parseActivityEventEntry(obj: unknown): ActivityEventEntry | null {
  if (typeof obj !== "object" || obj === null) return null;
  const e = obj as Record<string, unknown>;
  const path =
    typeof e.path === "string"
      ? e.path
      : typeof e.workspace_relative_path === "string"
        ? e.workspace_relative_path
        : null;
  const kindRaw = e.kind ?? e.file_event_kind;
  if (!path || !parseFileEventKind(kindRaw)) return null;
  const out: ActivityEventEntry = { path, kind: kindRaw };
  const toolCallID = e.tool_call_id ?? e.tool_use_id;
  if (typeof toolCallID === "string" && toolCallID) {
    out.tool_call_id = toolCallID;
  }
  return out;
}

function parseActivityEvent(obj: unknown): ActivityEvent | null {
  if (typeof obj !== "object" || obj === null) return null;
  const e = obj as Record<string, unknown>;
  if (e.type !== "turn_row" && e.type !== "mid_turn_touch") return null;
  if (typeof e.session_id !== "string" || typeof e.sequence !== "number") return null;
  const path =
    typeof e.path === "string"
      ? e.path
      : typeof e.workspace_relative_path === "string"
        ? e.workspace_relative_path
        : null;
  if (!path) return null;

  if (e.type === "mid_turn_touch") {
    const toolCallID = e.tool_call_id ?? e.tool_use_id;
    if (typeof toolCallID !== "string" || !toolCallID) return null;
    return {
      type: "mid_turn_touch",
      session_id: e.session_id,
      sequence: e.sequence,
      path,
      tool_call_id: toolCallID,
    };
  }

  if (typeof e.turn_id !== "string" || typeof e.count !== "number") return null;
  if (!Array.isArray(e.events) || !e.events.every((x) => parseActivityEventEntry(x) !== null)) {
    return null;
  }
  const kindRaw = e.kind ?? e.file_event_kind;
  if (!parseFileEventKind(kindRaw)) return null;
  const drillEntries = e.events.map(parseActivityEventEntry);
  if (drillEntries.some((x) => x === null)) return null;
  return {
    type: "turn_row",
    session_id: e.session_id,
    sequence: e.sequence,
    turn_id: e.turn_id,
    path,
    kind: kindRaw,
    count: e.count,
    events: drillEntries as ActivityEventEntry[],
  };
}

function parseServerFrameHandwritten(raw: string): ServerFrame | null {
  let v: unknown;
  try {
    v = JSON.parse(raw);
  } catch {
    return null;
  }
  // asciicast v2 配列 + sessionId: [number, "o", string, string] — Go wire.go と同順
  if (Array.isArray(v)) {
    if (
      v.length === 4 &&
      typeof v[0] === "number" &&
      v[1] === "o" &&
      typeof v[2] === "string" &&
      typeof v[3] === "string"
    ) {
      return [v[0], "o", v[2], v[3]];
    }
    return null;
  }
  if (typeof v !== "object" || v === null) {
    return null;
  }
  const obj = v as Record<string, unknown>;
  const k = obj.k;
  switch (k) {
    case "c": {
      // Go: code is int omitempty (absent when 0), data is string omitempty.
      if (obj.code !== undefined && typeof obj.code !== "number") return null;
      // Shallow validate data: must be string or absent (Go only emits string).
      if (obj.data !== undefined && typeof obj.data !== "string") return null;
      return {
        k: "c" as const,
        ...(typeof obj.code === "number" ? { code: obj.code } : {}),
        ...(typeof obj.data === "string" ? { data: obj.data } : {}),
        ...(typeof obj.sessionId === "string" ? { sessionId: obj.sessionId } : {}),
      };
    }
    case "h": {
      if (
        !Array.isArray(obj.sessions) ||
        !Array.isArray(obj.features) ||
        typeof obj.serverTime !== "number"
      ) {
        return null;
      }
      if (!obj.sessions.every(parseSessionInfoLoose)) return null;
      const hFrame: import("./server").HelloFrame = {
        k: "h",
        sessions: obj.sessions as SessionInfo[],
        activeSessionID: (obj.activeSessionID as string | null | undefined) ?? null,
        features: obj.features as string[],
        serverTime: obj.serverTime,
      };
      return hFrame;
    }
    case "v": {
      const hasSessions = Array.isArray(obj.sessions);
      const hasActivity = Array.isArray(obj.activity_events);
      if (!hasSessions && !hasActivity) return null;
      if (hasSessions && !(obj.sessions as unknown[]).every(parseSessionInfoLoose)) return null;

      const vFrame: import("./server").ViewUpdateFrame = { k: "v" };
      if (hasSessions) {
        vFrame.sessions = obj.sessions as SessionInfo[];
      }
      if (typeof obj.activity_session_id === "string") {
        vFrame.activity_session_id = obj.activity_session_id;
      }
      if (hasActivity) {
        const parsed = (obj.activity_events as unknown[]).map(parseActivityEvent);
        if (parsed.some((x) => x === null)) return null;
        vFrame.activity_events = parsed as ActivityEvent[];
      }
      // Preserve undefined when the wire omits activeSessionID (Go omitempty
      // strips empty strings). The store's applyViewUpdate distinguishes
      // undefined ("no change, keep current selection") from null/string
      // ("override"). Coercing undefined→null here would clobber the user's
      // selection on every daemon broadcast because the daemon does not
      // track per-web-client selection — its EvtSessionsChanged carries
      // ActiveSessionID="" for web-only deployments.
      if (obj.activeSessionID !== undefined) {
        vFrame.activeSessionID = obj.activeSessionID as string | null;
      }
      return vFrame;
    }
    case "r":
      if (typeof obj.reqId !== "string") return null;
      return { k: "r", reqId: obj.reqId, body: obj.body };
    case "e":
      if (
        typeof obj.reqId !== "string" ||
        typeof obj.code !== "string" ||
        typeof obj.message !== "string"
      ) {
        return null;
      }
      return { k: "e", reqId: obj.reqId, code: obj.code, message: obj.message };
    case "tt": {
      if (typeof obj.sessionId !== "string" || typeof obj.line !== "string") return null;
      return { k: "tt" as const, sessionId: obj.sessionId, line: obj.line };
    }
    case "et": {
      if (typeof obj.sessionId !== "string" || typeof obj.line !== "string") return null;
      return { k: "et" as const, sessionId: obj.sessionId, line: obj.line };
    }
    case "n": {
      if (
        typeof obj.sessionId !== "string" ||
        typeof obj.cmd !== "number" ||
        typeof obj.nowMs !== "number"
      ) {
        return null;
      }
      if (obj.title !== undefined && typeof obj.title !== "string") return null;
      if (obj.body !== undefined && typeof obj.body !== "string") return null;
      return {
        k: "n" as const,
        sessionId: obj.sessionId,
        cmd: obj.cmd,
        ...(typeof obj.title === "string" ? { title: obj.title } : {}),
        ...(typeof obj.body === "string" ? { body: obj.body } : {}),
        nowMs: obj.nowMs,
      };
    }
    // Phase 0/1 approval/question frames (k=ar|ax|qr|qx) share this surface.
    // Pre-Phase-0 clients ignore unknown k values by returning null (no disconnect).
    case "ar":
    case "ax":
    case "qr":
    case "qx":
      return null;
    default:
      return null;
  }
}

function serializeClientFrameHandwritten(f: ClientFrame): string {
  return JSON.stringify(f);
}

const handwrittenCodec = {
  parseServerFrame: parseServerFrameHandwritten,
  serializeClientFrame: serializeClientFrameHandwritten,
};

/** Public entry: routes through the adapter seam (handwritten | generated). */
export function parseServerFrame(raw: string): ServerFrame | null {
  return selectCodec(handwrittenCodec).parseServerFrame(raw);
}

export function serializeClientFrame(f: ClientFrame): string {
  return selectCodec(handwrittenCodec).serializeClientFrame(f);
}
