// Thin HTTP wrapper around the daemon-backed /api/sessions endpoints used by
// the command palette and tool registry. This is the single seam through
// which web UI talks to the gateway for session CRUD + push.
//
// Wire formats mirror services/server/web/mux.go:
//   - POST   /api/sessions             → 201 Created, body: apiSessionInfo
//   - DELETE /api/sessions/{id}        → 204 No Content
//   - POST   /api/sessions/{id}/push   → 200 OK (empty body); 4xx/5xx error
//                                       envelope via gatewayError
//   - GET    /api/session-config       → 200 OK, body: apiSessionConfig
//
// The push route (POST /api/sessions/{id}/push) is implemented by
// services/server/web/mux_push.go (ADR-0045 SendCommand pattern; ADR-0046 409
// active-mismatch). pushCommand below is the client side of that contract.

import { readBearerTokenFromHash } from "../auth";
import { makeFleetProvider } from "../fleet/provider";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

/**
 * Error thrown for any non-2xx HTTP response. `status` is the HTTP status
 * code; `body` is the response body text (if any). The palette store branches
 * on `status === 401` to trigger an auth-error toast (FR-024).
 *
 * Network failures (fetch itself rejecting) surface as plain Error("network"),
 * NOT as ApiHttpError — those have no HTTP status to attach.
 */
export interface ApiHttpError extends Error {
  status: number;
  body?: string;
}

/**
 * Error thrown when an HTTP 2xx response carries an unparseable / unexpected
 * payload (JSON.parse failure, or successful parse but missing required
 * fields). Distinct from ApiHttpError because status is 200 — the server
 * "succeeded" from HTTP's point of view, but the wire-format contract is
 * broken. Distinct from the "network" sentinel because the network round-
 * trip itself worked.
 *
 * The palette store can branch on this to surface a "server returned an
 * unexpected response" toast rather than a generic crash; in the meantime
 * having a typed Error with a `cause` preserves the original SyntaxError /
 * shape diagnostic for Sentry / dev console.
 *
 * (Spec: review finding "minor — createSession / getSessionConfig res.json
 * leaking SyntaxError".)
 */
export interface ApiWireFormatError extends Error {
  wireFormat: true;
  url: string;
}

export interface CreateSessionPayload {
  server_id?: string;
  project: string;
  version: 2;
  profile_id: string;
  profile_revision: string;
  parameters?: Record<string, string | number | boolean>;
  // Initial PTY size hint measured from the fitted browser terminal. Both
  // fields are sent together; omission selects the daemon's 80x24 default.
  cols?: number;
  rows?: number;
  worktree?: boolean;
  // ADR-0042: palette new-session payload wire mirror. "" / undefined / "auto"
  // all mean "follow project config"; "host" forces direct/host launch.
  // We expose only the "host" override on the type because palette UI never
  // sends explicit "auto" — omitting the field is the same thing on the wire.
  sandbox?: "host";
}

/**
 * Project entry returned by GET /api/session-config. session-config-extension
 * (a later task) will fill in real `isGit` / `isSandboxed` values; until then
 * the wire still returns plain `string[]` and we normalize to this shape with
 * both flags false (see makeSessionsApi.getSessionConfig).
 */
export interface SessionConfigProject {
  path: string;
  isGit: boolean;
  isSandboxed: boolean;
  serverId?: string;
  serverLabel?: string;
  commandProfiles?: LaunchProfile[];
}

export type LaunchParameter =
  | {
      name: string;
      label: string;
      type: "enum";
      required: boolean;
      default?: string;
      values: string[];
    }
  | {
      name: string;
      label: string;
      type: "boolean";
      required: boolean;
      default?: boolean;
    }
  | {
      name: string;
      label: string;
      type: "string";
      required: boolean;
      default?: string;
      pattern?: string;
    }
  | {
      name: string;
      label: string;
      type: "number";
      required: boolean;
      default?: number;
      min?: number;
      max?: number;
    };

export interface LaunchProfile {
  id: string;
  label: string;
  revision: string;
  parameters: LaunchParameter[];
}

export interface ServerConfigOutcome {
  serverId: string;
  serverLabel: string;
  state: "connected" | "unavailable";
  reason?: string;
}

/**
 * SessionConfig is the normalized client view of the GET /api/session-config
 * response. We expose both legacy and forward-compat fields so palette code
 * doesn't have to change again when the server is upgraded:
 *
 *   - projectRoots / projectPaths: forward-compat (session-config-extension
 *     will add `project_roots` / `project_paths` to the wire). Until then,
 *     they're [] and the UI falls back to `projectPaths` only.
 *   - projects: normalized list of project entries (path + flags). Always
 *     populated, even when the server still returns the legacy `string[]`
 *     shape.
 *   - pushCommands: forward-compat (session-config-extension will populate).
 *     Until then, []. (Normalized from the wire's `push_commands` field —
 *     keeping the client interface camelCase matches projectRoots etc.)
 */
export interface SessionConfig {
  projectRoots: string[];
  projectPaths: string[];
  projects: SessionConfigProject[];
  serverOutcomes: ServerConfigOutcome[];
  pushCommands: string[];
}

export interface SessionMessageReply {
  id: string;
  source_frame_id: string;
  body?: string;
  body_preview?: string;
  created_at: string;
  resolution?: string;
  final_answer_preview?: string;
}

export interface SessionMessage {
  id: string;
  source_frame_id: string;
  target_frame_id: string;
  topic?: string;
  body?: string;
  body_preview?: string;
  created_at: string;
  read?: boolean;
  reply_status?: string;
  delivery_status?: string;
  final_answer_preview?: string;
  reply?: SessionMessageReply;
}

export interface SessionMessagesResponse {
  session_id: string;
  summary?: {
    unread_count: number;
    latest_message_preview?: string;
    latest_reply_preview?: string;
    pending_delivery_count: number;
    last_delivery_status?: string;
  };
  messages: SessionMessage[];
}

export interface SessionsApi {
  createSession(payload: CreateSessionPayload): Promise<{ id: string; serverId?: string }>;
  deleteSession(id: string): Promise<void>;
  pushCommand(sessionId: string, command: string): Promise<void>;
  getSessionConfig(): Promise<SessionConfig>;
  getSessionMessages(sessionId: string): Promise<SessionMessagesResponse>;
  markSessionMessagesRead(sessionId: string, lastReadMessageId?: string): Promise<void>;
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// makeAuthHeaderFn returns an authHeader function with its own "already
// notified" flag, so a stale token-missing warning from one SessionsApi
// instance doesn't suppress the warning on a freshly constructed one (this
// is also what lets each test see the warning independently).
function makeAuthHeaderFn(): (url: string) => Record<string, string> {
  let bearerMissingNotified = false;
  return (url: string): Record<string, string> => {
    const token = readBearerTokenFromHash();
    if (!token) {
      // Dev-time hint only. The server still returns 401 (which palette-store
      // surfaces as a toast), but without this one-shot warn a developer who
      // forgot to put `#token=...` in the URL sees only "401 unauthorized"
      // with no indication that the client never even attached an
      // Authorization header. Surface it once per SessionsApi instance so the
      // dev console has a breadcrumb without spamming on every poll.
      // (Spec: review finding "minor — token missing silent path".)
      if (!bearerMissingNotified) {
        bearerMissingNotified = true;
        // eslint-disable-next-line no-console -- diagnostic, see comment above
        console.warn(
          `[api/sessions] no bearer token in window.location.hash; sending ${url} without Authorization header`,
        );
      }
      return {};
    }
    return { Authorization: `Bearer ${token}` };
  };
}

function makeHttpError(status: number, body: string): ApiHttpError {
  const err = new Error(`HTTP ${status}`) as ApiHttpError;
  err.status = status;
  if (body) err.body = body;
  return err;
}

// makeWireFormatError attaches the URL and a `cause` so the original parse
// failure (SyntaxError from JSON.parse, or a shape-check Error) is preserved
// for logError(Sentry) / dev console, while giving callers a single typed
// shape to branch on.
function makeWireFormatError(url: string, message: string, cause: unknown): ApiWireFormatError {
  // ES2022 Error `cause` (same constraint as makeNetworkError).
  const err = new Error(`${url}: ${message}`, { cause }) as ApiWireFormatError;
  err.wireFormat = true;
  err.url = url;
  return err;
}

// parseJsonBody wraps res.json() so a malformed-JSON failure surfaces as
// ApiWireFormatError instead of leaking the raw SyntaxError. The successful
// branch returns Record<string, unknown> so callers can do typed field
// extraction without `as any`.
async function parseJsonBody(url: string, res: Response): Promise<Record<string, unknown>> {
  let body: unknown;
  try {
    body = await res.json();
  } catch (e) {
    throw makeWireFormatError(url, "response is not valid JSON", e);
  }
  if (body === null || typeof body !== "object" || Array.isArray(body)) {
    throw makeWireFormatError(
      url,
      `response top-level is not a JSON object (got ${body === null ? "null" : Array.isArray(body) ? "array" : typeof body})`,
      body,
    );
  }
  return body as Record<string, unknown>;
}

// makeNetworkError wraps the underlying fetch failure (TypeError / DOMException
// / TLS / CORS / etc.) into an Error whose message is the sentinel "network"
// (palette-store branches on `=== 'network'`) but whose `cause` preserves the
// original exception. Without `cause` the original stack and DOMException name
// disappear, so AbortError vs offline vs CORS vs TLS all collapse into "fetch
// failed". (Spec: review finding "major — fetch reject silent network".)
function makeNetworkError(cause: unknown): Error {
  // `Error` constructor with `{ cause }` is ES2022 standard, supported by all
  // browsers we ship to (Chromium / Firefox / Safari ≥ 15.4) and by jsdom in
  // the vitest environment.
  return new Error("network", { cause });
}

async function throwHttpError(url: string, res: Response): Promise<never> {
  let body = "";
  try {
    body = await res.text();
  } catch (e) {
    const reason = e instanceof Error ? e.name : String(e);
    body = `<unreadable: ${reason}>`;
    // eslint-disable-next-line no-console -- diagnostic, see comment above
    console.warn(
      `[api/sessions] failed to read error body for ${url} status=${res.status}: ${reason}`,
      e,
    );
  }
  throw makeHttpError(res.status, body);
}

async function request(fetchImpl: typeof fetch, url: string, init: RequestInit): Promise<Response> {
  const res = await doFetch(fetchImpl, url, init);
  if (!res.ok) {
    await throwHttpError(url, res);
  }
  return res;
}

async function doFetch(fetchImpl: typeof fetch, url: string, init: RequestInit): Promise<Response> {
  let res: Response;
  try {
    res = await fetchImpl(url, init);
  } catch (e) {
    // AbortError (user-initiated cancel) is functionally different from a real
    // network failure: callers usually want to ignore it (no toast, no retry).
    // Re-throw it untouched so caller code that listens for AbortError sees
    // the original DOMException with its stack intact.
    if (e instanceof DOMException && e.name === "AbortError") {
      throw e;
    }
    if (e instanceof Error && e.name === "AbortError") {
      throw e;
    }
    throw makeNetworkError(e);
  }
  return res;
}

// Distinguishes "field absent" (legitimate forward/backward compat) from
// "field present but wrong type" (wire bug we want to surface).
function isAbsent(raw: unknown): boolean {
  return raw === undefined || raw === null;
}

// Legacy server returns Projects as string[]. session-config-extension will
// change it to SessionConfigProject[]. Normalize both so palette code never
// has to branch on shape.
//
// We silently accept three shapes:
//   1. absent / null            → []  (legacy server with no projects field)
//   2. string[]                 → [{path, isGit:false, isSandboxed:false}]
//   3. SessionConfigProject[]   → pass-through (post session-config-extension)
//
// Any OTHER shape (object instead of array, mixed garbage entries, missing
// `path` on a per-entry basis) is a wire-format bug. We still return a safe
// value so the palette UI doesn't crash, but we emit a console.warn so the
// silent-failure-hunter review concern doesn't recur: a gateway bug that
// drops `projects` mid-flight will at minimum leave a breadcrumb in the dev
// console instead of looking like "no projects configured".
// (Spec: review finding "major — getSessionConfig silent fallback".)
function normalizeProjects(raw: unknown): SessionConfigProject[] {
  if (isAbsent(raw)) return [];
  if (!Array.isArray(raw)) {
    // eslint-disable-next-line no-console -- diagnostic, see header comment
    console.warn(
      "[api/sessions] /api/session-config: `projects` field present but not an array; treating as empty. raw=",
      raw,
    );
    return [];
  }
  return raw.map((p, i): SessionConfigProject => {
    if (typeof p === "string") {
      return { path: p, isGit: false, isSandboxed: false };
    }
    if (p && typeof p === "object") {
      const o = p as Record<string, unknown>;
      const path = typeof o.path === "string" ? o.path : "";
      if (path === "") {
        // eslint-disable-next-line no-console -- diagnostic
        console.warn(
          `[api/sessions] /api/session-config: projects[${i}] object missing string \`path\` field. entry=`,
          p,
        );
      }
      const isGit = typeof o.isGit === "boolean" ? o.isGit : false;
      const isSandboxed = typeof o.isSandboxed === "boolean" ? o.isSandboxed : false;
      const commandProfiles = normalizeLaunchProfiles(o.command_profiles);
      return {
        path,
        isGit,
        isSandboxed,
        ...(commandProfiles.length > 0 ? { commandProfiles } : {}),
      };
    }
    // eslint-disable-next-line no-console -- diagnostic
    console.warn(
      `[api/sessions] /api/session-config: projects[${i}] is neither string nor object; substituting empty entry. raw=`,
      p,
    );
    return { path: "", isGit: false, isSandboxed: false };
  });
}

function normalizeLaunchProfiles(raw: unknown): LaunchProfile[] {
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((value): LaunchProfile[] => {
    if (!value || typeof value !== "object") return [];
    const profile = value as Record<string, unknown>;
    if (
      typeof profile.id !== "string" ||
      typeof profile.revision !== "string" ||
      !Array.isArray(profile.parameters)
    ) {
      return [];
    }
    const parameters = profile.parameters.filter((parameter): parameter is LaunchParameter =>
      Boolean(
        parameter &&
          typeof parameter === "object" &&
          typeof (parameter as Record<string, unknown>).name === "string" &&
          typeof (parameter as Record<string, unknown>).type === "string",
      ),
    );
    return [
      {
        id: profile.id,
        label: typeof profile.label === "string" && profile.label ? profile.label : profile.id,
        revision: profile.revision,
        parameters,
      },
    ];
  });
}

// Same silent-fallback distinction as normalizeProjects: absent → [] is
// legitimate (forward-compat for fields the current server doesn't emit),
// but a present-but-not-an-array value is a wire bug worth surfacing.
function normalizeStringList(raw: unknown, fieldName: string): string[] {
  if (isAbsent(raw)) return [];
  if (!Array.isArray(raw)) {
    // eslint-disable-next-line no-console -- diagnostic, see normalizeProjects
    console.warn(
      `[api/sessions] /api/session-config: \`${fieldName}\` present but not an array; treating as empty. raw=`,
      raw,
    );
    return [];
  }
  return raw.filter((v): v is string => typeof v === "string");
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function makeSessionsApi(fetchImpl?: typeof fetch): SessionsApi {
  const f = fetchImpl ?? fetch.bind(globalThis);
  const authHeader = makeAuthHeaderFn();
  const fleet = makeFleetProvider(f);

  const currentURL = (pathname: string): string => {
    const serverId = fleet.currentServerId();
    if (!serverId) throw new Error("No server selected");
    return fleet.apiUrl(serverId, pathname);
  };

  return {
    async createSession(payload: CreateSessionPayload): Promise<{ id: string; serverId?: string }> {
      const serverId = payload.server_id ?? fleet.currentServerId();
      if (!serverId) throw new Error("createSession requires server_id");
      const { server_id: _, ...serverPayload } = payload;
      const body = (await fleet.create(serverId, serverPayload)) as Record<string, unknown>;
      if (typeof body.id !== "string" || body.id === "") {
        throw makeWireFormatError("FleetProvider.create", "missing string `id`", body);
      }
      return { id: body.id, serverId };
    },

    async deleteSession(id: string): Promise<void> {
      const url = currentURL(`/api/sessions/${encodeURIComponent(id)}`);
      await request(f, url, {
        method: "DELETE",
        headers: { ...authHeader(url) },
      });
    },

    async pushCommand(sessionId: string, command: string): Promise<void> {
      // POST /api/sessions/{id}/push — body {command}, server returns 200 OK
      // with an empty body on success. The request helper raises ApiHttpError
      // for any non-2xx so palette-store can branch on status (401 → auth
      // toast / close per FR-024; 409 → keep open + error per ADR-0046; other
      // 4xx/5xx → generic error toast).
      //
      // encodeURIComponent guards the URL path: the server's session-id
      // allowlist rejects anything outside [A-Za-z0-9_-], but if a UI bug
      // ever leaks a stray '/' we don't want it to silently retarget a
      // different path segment (same defense as deleteSession above).
      const url = currentURL(`/api/sessions/${encodeURIComponent(sessionId)}/push`);
      await request(f, url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...authHeader(url),
        },
        body: JSON.stringify({ command }),
      });
      // No body to parse — server writes only WriteHeader(200) on success
      // (see sendPushDriver in services/server/web/mux_push.go). Returning void
      // is the contract palette-store / tools-registry-dynamic-push expect.
    },

    async getSessionConfig(): Promise<SessionConfig> {
      {
        const outcomes = await fleet.listConfigs();
        const projects: SessionConfigProject[] = [];
        const serverOutcomes: ServerConfigOutcome[] = [];
        const pushCommands = new Set<string>();
        for (const outcome of outcomes) {
          const state = outcome.state === "connected" ? "connected" : "unavailable";
          serverOutcomes.push({
            serverId: String(outcome.serverId ?? ""),
            serverLabel: String(outcome.serverLabel ?? outcome.serverId ?? ""),
            state,
            ...(typeof outcome.reason === "string" ? { reason: outcome.reason } : {}),
          });
          if (state !== "connected" || !outcome.config || typeof outcome.config !== "object") {
            continue;
          }
          const rawConfig = outcome.config as Record<string, unknown>;
          for (const project of normalizeProjects(rawConfig.projects)) {
            projects.push({
              ...project,
              serverId: String(outcome.serverId ?? ""),
              serverLabel: String(outcome.serverLabel ?? outcome.serverId ?? ""),
            });
          }
          for (const command of normalizeStringList(rawConfig.push_commands, "push_commands")) {
            pushCommands.add(command);
          }
        }
        return {
          projectRoots: [],
          projectPaths: [],
          projects,
          serverOutcomes,
          pushCommands: [...pushCommands],
        };
      }
    },

    async getSessionMessages(sessionId: string): Promise<SessionMessagesResponse> {
      const url = currentURL(`/api/sessions/${encodeURIComponent(sessionId)}/messages`);
      const res = await request(f, url, {
        method: "GET",
        headers: { ...authHeader(url) },
      });
      const body = await parseJsonBody(url, res);
      const messages = Array.isArray(body.messages) ? (body.messages as SessionMessage[]) : [];
      return {
        session_id: typeof body.session_id === "string" ? body.session_id : sessionId,
        summary:
          body.summary && typeof body.summary === "object"
            ? (body.summary as SessionMessagesResponse["summary"])
            : undefined,
        messages,
      };
    },

    async markSessionMessagesRead(sessionId: string, lastReadMessageId?: string): Promise<void> {
      const url = currentURL(`/api/sessions/${encodeURIComponent(sessionId)}/messages/read`);
      await request(f, url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...authHeader(url),
        },
        body: JSON.stringify(lastReadMessageId ? { last_read_message_id: lastReadMessageId } : {}),
      });
    },
  };
}
