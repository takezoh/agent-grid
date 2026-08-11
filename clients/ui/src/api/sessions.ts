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

import { makeFleetProvider } from "../fleet/provider";
import { fleetSessionIdFor, resolveSessionRoute } from "../fleet/session-identity";
import { PROTOCOL_VERSION, PROTOCOL_VERSION_HEADER } from "../protocol-version";

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

/**
 * A session id reached an api module that could not be turned into a route.
 *
 * One failure for all four session-scoped api modules. They used to disagree:
 * two passed a bare id straight through to a non-fleet URL that no shipping
 * host serves, one rejected the same value, and one substituted a default
 * server. The name says the outcome ("this session could not be resolved")
 * rather than the form ("not a fleet key"), so callers have nothing to
 * re-branch on.
 *
 * Thrown before any request is issued, and outside doFetch's try — otherwise
 * it would be caught and remapped to Error("network"), which is exactly the
 * misdiagnosis this change removes on the Electron side.
 */
export class UnresolvedSessionError extends Error {
  readonly unresolvedSession = true as const;
  constructor(readonly sessionId: string) {
    super(`unresolved session: ${sessionId}`);
    this.name = "UnresolvedSessionError";
  }
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

export interface CreateRepositorySessionPayload {
  repository_id: string;
  owner: string;
  name: string;
  version: 2;
  profile_id: string;
  profile_revision: string;
  parameters?: Record<string, string | number | boolean>;
  cols?: number;
  rows?: number;
}

/** Project entry returned by the exact-version GET /api/session-config wire. */
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
  isDefaultExecution?: boolean;
  state: "connected" | "unavailable" | "incompatible_protocol";
  reason?: string;
  commandProfiles?: LaunchProfile[];
}

/** Exact-version client view of GET /api/session-config. */
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
  createRepositorySession?(
    serverId: string,
    payload: CreateRepositorySessionPayload,
  ): Promise<{ preparation_id: string; intent_id: string; session_id?: string }>;
  deleteSession(id: string): Promise<void>;
  pushCommand(sessionId: string, command: string): Promise<void>;
  getSessionConfig(): Promise<SessionConfig>;
  getSessionMessages(sessionId: string): Promise<SessionMessagesResponse>;
  markSessionMessagesRead(sessionId: string, lastReadMessageId?: string): Promise<void>;
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

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
    const headers = new Headers(init.headers);
    headers.set(PROTOCOL_VERSION_HEADER, PROTOCOL_VERSION);
    res = await fetchImpl(url, { ...init, headers });
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

function normalizeProjects(raw: unknown): SessionConfigProject[] {
  if (!Array.isArray(raw)) {
    throw new Error("invalid exact-version session-config: projects must be an array");
  }
  return raw.map((p, i): SessionConfigProject => {
    if (p && typeof p === "object") {
      const o = p as Record<string, unknown>;
      if (
        typeof o.path !== "string" ||
        typeof o.isGit !== "boolean" ||
        typeof o.isSandboxed !== "boolean"
      ) {
        throw new Error(`invalid exact-version session-config: projects[${i}]`);
      }
      const commandProfiles = normalizeLaunchProfiles(o.command_profiles);
      return {
        path: o.path,
        isGit: o.isGit,
        isSandboxed: o.isSandboxed,
        ...(commandProfiles.length > 0 ? { commandProfiles } : {}),
      };
    }
    throw new Error(`invalid exact-version session-config: projects[${i}]`);
  });
}

function normalizeLaunchProfiles(raw: unknown): LaunchProfile[] {
  if (!Array.isArray(raw)) {
    throw new Error("invalid exact-version session-config: command profiles must be an array");
  }
  return raw.map((value, profileIndex): LaunchProfile => {
    if (!value || typeof value !== "object") {
      throw new Error(`invalid exact-version session-config: command profiles[${profileIndex}]`);
    }
    const profile = value as Record<string, unknown>;
    if (
      typeof profile.id !== "string" ||
      typeof profile.revision !== "string" ||
      !Array.isArray(profile.parameters)
    ) {
      throw new Error(`invalid exact-version session-config: command profiles[${profileIndex}]`);
    }
    const parameters = profile.parameters.map((parameter, parameterIndex) =>
      normalizeLaunchParameter(parameter, profileIndex, parameterIndex),
    );
    return {
      id: profile.id,
      label: typeof profile.label === "string" && profile.label ? profile.label : profile.id,
      revision: profile.revision,
      parameters,
    };
  });
}

function normalizeLaunchParameter(
  raw: unknown,
  profileIndex: number,
  parameterIndex: number,
): LaunchParameter {
  const invalid = () =>
    new Error(
      `invalid exact-version session-config: command profiles[${profileIndex}].parameters[${parameterIndex}]`,
    );
  if (!raw || typeof raw !== "object") throw invalid();
  const parameter = raw as Record<string, unknown>;
  if (
    typeof parameter.name !== "string" ||
    typeof parameter.label !== "string" ||
    typeof parameter.required !== "boolean"
  ) {
    throw invalid();
  }
  const base = {
    name: parameter.name,
    label: parameter.label,
    required: parameter.required,
  };
  switch (parameter.type) {
    case "enum": {
      if (
        !Array.isArray(parameter.values) ||
        !parameter.values.every((value) => typeof value === "string") ||
        (parameter.default !== undefined && typeof parameter.default !== "string")
      ) {
        throw invalid();
      }
      return {
        ...base,
        type: "enum",
        values: parameter.values,
        ...(parameter.default === undefined ? {} : { default: parameter.default }),
      };
    }
    case "boolean":
      if (parameter.default !== undefined && typeof parameter.default !== "boolean") {
        throw invalid();
      }
      return {
        ...base,
        type: "boolean",
        ...(parameter.default === undefined ? {} : { default: parameter.default }),
      };
    case "string":
      if (
        (parameter.default !== undefined && typeof parameter.default !== "string") ||
        (parameter.pattern !== undefined && typeof parameter.pattern !== "string")
      ) {
        throw invalid();
      }
      return {
        ...base,
        type: "string",
        ...(parameter.default === undefined ? {} : { default: parameter.default }),
        ...(parameter.pattern === undefined ? {} : { pattern: parameter.pattern }),
      };
    case "number": {
      const finiteOptionalNumber = (value: unknown) =>
        value === undefined || (typeof value === "number" && Number.isFinite(value));
      if (
        !finiteOptionalNumber(parameter.default) ||
        !finiteOptionalNumber(parameter.min) ||
        !finiteOptionalNumber(parameter.max) ||
        (typeof parameter.min === "number" &&
          typeof parameter.max === "number" &&
          parameter.min > parameter.max)
      ) {
        throw invalid();
      }
      return {
        ...base,
        type: "number",
        ...(parameter.default === undefined ? {} : { default: parameter.default as number }),
        ...(parameter.min === undefined ? {} : { min: parameter.min as number }),
        ...(parameter.max === undefined ? {} : { max: parameter.max as number }),
      };
    }
    default:
      throw invalid();
  }
}

function normalizeStringList(raw: unknown, fieldName: string): string[] {
  if (!Array.isArray(raw)) {
    throw new Error(`invalid exact-version session-config: ${fieldName} must be an array`);
  }
  if (!raw.every((v) => typeof v === "string")) {
    throw new Error(`invalid exact-version session-config: ${fieldName} entries`);
  }
  return raw as string[];
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function makeSessionsApi(fetchImpl?: typeof fetch): SessionsApi {
  const f = fetchImpl ?? fetch.bind(globalThis);
  const fleet = makeFleetProvider(f);
  const routedSession = (value: string) => {
    const route = resolveSessionRoute(value);
    if (route.ok) return { serverId: route.serverId, sessionId: route.upstreamSessionId };
    // Not a fleet key. Which server to use when nothing says is a fleet
    // configuration question, not an id-resolution one, so it stays here
    // instead of inside the resolver — folding it in would make precedence a
    // four-step rule nobody can check.
    const serverId = fleet.initialServerId();
    if (!serverId) throw new UnresolvedSessionError(value);
    return { serverId, sessionId: value };
  };
  const routedRequest = (value: string, suffix: string, init: RequestInit): Promise<Response> => {
    const target = routedSession(value);
    const pathname = `/api/sessions/${encodeURIComponent(target.sessionId)}${suffix}`;
    return request(
      (_input, requestInit) => fleet.request(target.serverId, pathname, requestInit),
      pathname,
      init,
    );
  };

  return {
    async createSession(payload: CreateSessionPayload): Promise<{ id: string; serverId?: string }> {
      const serverId = payload.server_id;
      if (!serverId) throw new Error("createSession requires explicit server_id");
      const { server_id: _, ...serverPayload } = payload;
      const body = (await fleet.create(serverId, serverPayload)) as Record<string, unknown>;
      if (typeof body.id !== "string" || body.id === "") {
        throw makeWireFormatError("FleetProvider.create", "missing string `id`", body);
      }
      return { id: fleetSessionIdFor({ serverId, sessionId: body.id }), serverId };
    },

    async deleteSession(id: string): Promise<void> {
      await routedRequest(id, "", {
        method: "DELETE",
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
      await routedRequest(sessionId, "/push", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
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
          const state = outcome.state;
          const rawConfig =
            outcome.config && typeof outcome.config === "object"
              ? (outcome.config as Record<string, unknown>)
              : null;
          if (
            state === "connected" &&
            (!rawConfig || !Array.isArray(rawConfig.base_command_profiles))
          ) {
            throw new Error(
              "invalid exact-version session-config: base_command_profiles must be an array",
            );
          }
          const commandProfiles = rawConfig
            ? normalizeLaunchProfiles(rawConfig.base_command_profiles)
            : [];
          serverOutcomes.push({
            serverId: String(outcome.serverId ?? ""),
            serverLabel: String(outcome.serverLabel ?? outcome.serverId ?? ""),
            ...(outcome.isDefaultExecution === true ? { isDefaultExecution: true } : {}),
            state,
            ...(typeof outcome.reason === "string" ? { reason: outcome.reason } : {}),
            ...(commandProfiles.length > 0 ? { commandProfiles } : {}),
          });
          if (state !== "connected" || !rawConfig) {
            continue;
          }
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

    async createRepositorySession(serverId, payload) {
      const result = (await fleet.createRepositorySession(serverId, payload)) as Record<
        string,
        unknown
      >;
      if (typeof result.preparation_id !== "string" || typeof result.intent_id !== "string") {
        throw makeWireFormatError(
          "/api/repository-sessions",
          "repository preparation response is invalid",
          result,
        );
      }
      return {
        preparation_id: result.preparation_id,
        intent_id: result.intent_id,
        ...(typeof result.session_id === "string" ? { session_id: result.session_id } : {}),
      };
    },

    async getSessionMessages(sessionId: string): Promise<SessionMessagesResponse> {
      const url = `/api/sessions/${encodeURIComponent(sessionId)}/messages`;
      const res = await routedRequest(sessionId, "/messages", {
        method: "GET",
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
      await routedRequest(sessionId, "/messages/read", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(lastReadMessageId ? { last_read_message_id: lastReadMessageId } : {}),
      });
    },
  };
}
