// ToolDef / ToolRegistry — declarative command-palette tool layer.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// ADRs:
//   - 0036 palette-2phase-store-architecture (store is pure state; I/O lives
//     in ToolDef.submit, ctx-injected)
//   - 0047 palette-disabledreason-single-source (push availability and
//     submit-time re-validation share a single disabledReason(daemon)
//     function per ToolDef)
//
// This file is the framework + standard scope plus dynamic push expansion.
// session-config-extension supplies the curated `pushCommands` list on the
// daemon snapshot; `listTools()` calls `toolsForPush()` to materialize one
// scope='push' ToolDef per entry, each sharing scopeDisabledReason('push',
// daemon) so the segment UI and the submit-time re-validation can never
// drift (ADR-0047).

import type { SessionsApi } from "../api/sessions";
import type { LaunchProfile, ServerConfigOutcome, SessionConfigProject } from "../api/sessions";
import { type ActiveContextSnapshot, deriveActiveContext } from "../store/palette_active_context";
import type { ActiveOccupant, SessionInfo } from "../wire/server";
import type { TerminalGeometry } from "./terminalGeometry";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export type ToolScope = "standard" | "push";

// ParamOption is a single listbox choice: the wire-side `value` plus a
// pre-formatted `label` for display. Keeping `label` as a plain string (not
// a getText projection) lets the palette store materialize once at param-
// select time and then render without callbacks; the static `label` doubles
// as the substring filter source for the listbox typeahead.
export interface ParamOption {
  value: string;
  label: string;
}

// ParamDef is a discriminated union over `kind`, the only field every variant
// shares besides id/label/required. Splitting by kind keeps the listbox
// (static-options / dynamic-options) and free-form input (text) variants
// statically distinguishable at the call site, so ParamSelectPhase can
// switch(kind) and the type checker enforces exhaustiveness.
//
// - 'text': free-form input field. Optional `placeholder` hints the user.
// - 'static-options': listbox sourced from a baked-in `options` array
//   declared at tool-definition time (no daemon dependency).
// - 'dynamic-options': listbox sourced at param-select time by looking up
//   `materializeKey` against the daemon snapshot. The materialization itself
//   lives outside this file (palette-store / ParamSelectPhase wire it up via
//   the option-source helpers below, e.g. projectOptions). materializeKey is
//   a closed enum on purpose — adding a new dynamic source is a deliberate
//   change to this union, not an accidental string typo.
export type ParamDef =
  | {
      id: string;
      kind: "text";
      label: string;
      placeholder?: string;
      required?: boolean;
    }
  | {
      id: string;
      kind: "static-options";
      label: string;
      options: ParamOption[];
      required?: boolean;
    }
  | {
      id: string;
      kind: "dynamic-options";
      label: string;
      // 'projects' → daemon.projects (path listing for new-session "Project").
      // 'commands' → daemon.commands (curated [session].commands list for the
      // new-session "Command" picker; sourced from /api/session-config — see
      // ADR-0041).
      materializeKey: "projects" | "commands";
      required?: boolean;
    };

// DaemonSnapshot is the read-only slice of daemon state that ToolDef.submit
// and ToolDef.disabledReason can observe. We define it locally instead of
// reusing store/daemon DaemonState to keep the tool layer decoupled from
// zustand internals (snapshot is a plain value, no actions) and to allow
// later sources (e.g. session-config-extension) to extend `projects` /
// `pushCommands` without touching the store shape. ADR-0036 keeps store
// pure; ctx.daemon is the projection palette-store hands to submit().
// DaemonOccupant is the tool-layer alias for the wire-level ActiveOccupant
// ('main' | 'log' | 'frame'): which buffer is active on the daemon.
// Aliasing (Y2) keeps the literal union single-sourced at the wire boundary
// so a future scope ('chat' / 'scratch' / ...) widens in exactly one place.
// Web UI only cares about this to gate push scope (push targets the active
// session's frame buffer); see FR-004 (initial scope selection) and FR-006
// (push-segment disabled state). Optional on DaemonSnapshot because not all
// callers (older tests, stub snapshots) provide it; `undefined` is treated
// as "no frame" for scope purposes.
export type DaemonOccupant = ActiveOccupant;

export interface DaemonSnapshot {
  sessions: SessionInfo[];
  activeSessionID: string | null;
  activeOccupant?: DaemonOccupant;
  projects: SessionConfigProject[];
  serverOutcomes: ServerConfigOutcome[];
  pushCommands: string[];
}

export type NewSessionTarget =
  | { source: "project"; project: string }
  | {
      source: "repository";
      repository: { repository_id: string; owner: string; name: string };
    };

export function readNewSessionTarget(payload: Record<string, unknown>): NewSessionTarget {
  const value = payload.target;
  if (value && typeof value === "object") {
    const target = value as Record<string, unknown>;
    if (target.source === "project" && typeof target.project === "string" && target.project) {
      return { source: "project", project: target.project };
    }
    if (
      target.source === "repository" &&
      target.repository &&
      typeof target.repository === "object"
    ) {
      const repository = target.repository as Record<string, unknown>;
      if (
        typeof repository.repository_id === "string" &&
        typeof repository.owner === "string" &&
        typeof repository.name === "string" &&
        repository.repository_id &&
        repository.owner &&
        repository.name
      ) {
        return {
          source: "repository",
          repository: {
            repository_id: repository.repository_id,
            owner: repository.owner,
            name: repository.name,
          },
        };
      }
    }
  }
  // Project-only callers from the pre-repository palette contract continue
  // to construct the typed target at the submit boundary.
  return { source: "project", project: readString(payload, "project") };
}

// NotificationsApi is the minimal surface ToolDef.submit needs for toast
// emission. Defined here (rather than imported from store/notifications)
// because the store today exposes add({level, message}); we want a
// success/error helper signature on ctx so submit code reads naturally
// ("notify.success(...)") without leaking the store's action shape into
// every tool. The palette-store adapter (later task) supplies the concrete
// implementation by wrapping useNotificationsStore.getState().add.
// `add` is the lower-level variant used by push submit (FR-014) to emit an
// info toast with a structured title and message (UAC-011 / UAC-018).
export interface NotificationsApi {
  success(message: string): void;
  error(message: string): void;
  // FR-014: structured toast for push submit (info level, title + message).
  add(input: {
    level: "info" | "warn" | "error";
    message: string;
    title?: string;
    body?: string;
  }): void;
}

// ToolStoreCtx is the narrow palette-store action slice ToolDef.submit
// consumes through ctx.store. Distinct from the broader PaletteActions
// interface in store/palette.ts (the full PaletteState action set used by
// the React layer) so test fakes only implement what tools actually use:
//   - close() — close the palette after a successful submit.
// ToolStoreCtx stays minimal on purpose: every new method here is one more
// thing test fakes must implement. Tools that need richer access should go
// through ctx.http or ctx.daemon instead.
export interface ToolStoreCtx {
  close(): void;
}

// ToolDaemonActions is the narrow daemon-action surface a ToolDef.submit
// needs to invoke (as opposed to ctx.daemon, which is a read-only snapshot).
// FR-021: new-session.submit must call daemon.selectSession(rc.id) so the
// freshly-created session becomes active in the SessionList without waiting
// for the next view-update tick. Kept minimal — adding daemon writes to
// ToolDef is an exception (ADR-0036), not the rule; widening this contract
// should require a fresh plan.
export interface ToolDaemonActions {
  selectSession(id: string | null): void;
}

export interface ToolCtx {
  http: SessionsApi;
  daemon: DaemonSnapshot;
  // daemonActions: write-side daemon API (selectSession). Distinct from
  // ctx.daemon (read-only snapshot) so the read/write asymmetry stays
  // visible at the ToolDef.submit call site (FR-021).
  daemonActions: ToolDaemonActions;
  notify: NotificationsApi;
  store: ToolStoreCtx;
  // Reads the latest fitted xterm grid at submit time. A function keeps the
  // palette context stable while window / drawer resizes update App's ref.
  getTerminalGeometry(): TerminalGeometry | null;
  // FR-014 / UAC-018: active context snapshot captured by CommandPalette at
  // submit start time. When absent, makePushToolDef falls back to deriving
  // from ctx.daemon. Aligned with lift-state (ADR-0055).
  frozenActiveContext?: ActiveContextSnapshot;
}

export interface ToolDef {
  id: string;
  label: string;
  scope: ToolScope;
  params: ParamDef[] | null;
  // disabledReason returns null when the tool is currently usable, and a
  // short human-readable English sentence when not. ADR-0047 makes this
  // the single source of truth for listbox "disabled" rendering
  // and submit-time re-validation. Standard scope is always null in this
  // task; push scope adds real predicates in tools-registry-dynamic-push.
  disabledReason(daemon: DaemonSnapshot): string | null;
  submit(ctx: ToolCtx, payload: Record<string, unknown>): Promise<void>;
}

// ---------------------------------------------------------------------------
// Standard tools
// ---------------------------------------------------------------------------

// readString narrows a payload value to a non-empty string and throws
// otherwise. Tools are called by the palette store, which guards against
// missing required params at param-select time; but a defensive check here
// keeps the wire payload clean if a future caller forgets a field, and
// gives the developer a clear error message instead of "TypeError: cannot
// read property of undefined" deep in fetch.
function readString(payload: Record<string, unknown>, key: string): string {
  const v = payload[key];
  if (typeof v !== "string" || v === "") {
    throw new Error(`tool payload: missing or empty string field "${key}"`);
  }
  return v;
}

// readOptionalBool returns the boolean value for `key` if present, or
// undefined when the field is absent. Anything other than boolean / absent
// throws — silent coercion of "true" / 1 / null would mask wire bugs.
function readOptionalBool(payload: Record<string, unknown>, key: string): boolean | undefined {
  if (!(key in payload)) return undefined;
  const v = payload[key];
  if (v === undefined) return undefined;
  if (typeof v !== "boolean") {
    throw new Error(`tool payload: field "${key}" must be boolean, got ${typeof v}`);
  }
  return v;
}

const newSessionTool: ToolDef = {
  id: "new-session",
  label: "New Session",
  scope: "standard",
  // project = dynamic listbox (materialized from ctx.daemon.projects at
  // param-select time via materializeKey = 'projects'); command = free-form
  // text input. worktree is NOT declared as a param here because the
  // ParamSelectPhase gates it via a Tab toggle whose visibility depends on
  // the selected project's isGit flag (FR-013). It flows through `payload`
  // as an optional field ("worktree": bool) — see submit below.
  params: [
    {
      id: "project",
      kind: "dynamic-options",
      materializeKey: "projects",
      label: "Project",
      required: true,
    },
    {
      id: "command",
      kind: "dynamic-options",
      materializeKey: "commands",
      label: "Command",
      required: true,
    },
  ],
  disabledReason: () => null,
  async submit(ctx, payload) {
    const target = readNewSessionTarget(payload);
    const projectTarget = target.source === "project" ? target.project : null;
    const selectedProject = findProject(ctx.daemon.projects, projectTarget);
    const project = selectedProject?.path ?? projectTarget;
    const executionServerId =
      typeof payload.executionServerId === "string" && payload.executionServerId
        ? payload.executionServerId
        : selectedProject?.serverId;
    const command = readString(payload, "command");
    const worktree = readOptionalBool(payload, "worktree");
    const geometry = ctx.getTerminalGeometry();
    // Build the payload with only present fields so JSON.stringify doesn't
    // serialize "worktree": undefined (which would break the server's
    // omitempty contract on round-trip).
    const profile =
      target.source === "project"
        ? selectedProject?.commandProfiles?.find((candidate) => candidate.id === command)
        : ctx.daemon.serverOutcomes
            .find((server) => server.serverId === executionServerId)
            ?.commandProfiles?.find((candidate) => candidate.id === command);
    if (!profile) {
      throw new Error(`command profile "${command}" is not available for the selected target`);
    }
    if (target.source === "repository") {
      if (!executionServerId) throw new Error("repository session requires an execution server");
      if (!ctx.http.createRepositorySession) {
        throw new Error("repository session creation is unavailable");
      }
      const result = await ctx.http.createRepositorySession(executionServerId, {
        ...target.repository,
        version: 2,
        profile_id: profile.id,
        profile_revision: profile.revision,
        parameters: profileParameterValues(profile, payload),
        ...(geometry !== null ? { cols: geometry.cols, rows: geometry.rows } : {}),
      });
      if (result.session_id) ctx.daemonActions.selectSession(result.session_id);
      ctx.notify.success("Repository session preparation started");
      ctx.store.close();
      return;
    }
    const req: {
      server_id?: string;
      project: string;
      version: 2;
      profile_id: string;
      profile_revision: string;
      parameters?: Record<string, string | number | boolean>;
      cols?: number;
      rows?: number;
      worktree?: boolean;
    } = {
      project: project as string,
      version: 2,
      profile_id: profile.id,
      profile_revision: profile.revision,
      parameters: profileParameterValues(profile, payload),
    };
    if (executionServerId) req.server_id = executionServerId;
    if (geometry !== null) {
      req.cols = geometry.cols;
      req.rows = geometry.rows;
    }
    if (worktree !== undefined) req.worktree = worktree;
    const rc = await ctx.http.createSession(req);
    ctx.daemonActions.selectSession(rc.id);
    // Notification first, then close: if notify throws the palette stays
    // open with the active error toast visible. close() is idempotent so a
    // future re-entry that calls submit() twice is fine.
    ctx.notify.success("Session created");
    ctx.store.close();
  },
};

// ---------------------------------------------------------------------------
// Public ParamDef option-source helpers
// ---------------------------------------------------------------------------
//
// projectOptions is the sole materialize implementation for
// `materializeKey === 'projects'`. Exposed so palette store /
// ParamSelectPhase can fill the listbox options at param-select time
// without duplicating the projection logic. Other materializeKey values
// are intentionally NOT added in this PR; introducing a new key is a
// deliberate widening of the ParamDef union.

export function projectOptions(daemon: DaemonSnapshot): ParamOption[] {
  return (daemon.projects ?? []).map((p) => ({
    value: projectTargetValue(p),
    label: p.serverLabel ? `${p.serverLabel} · ${p.path}` : p.path,
  }));
}

export function projectTargetValue(project: SessionConfigProject): string {
  return project.serverId ? JSON.stringify([project.serverId, project.path]) : project.path;
}

export function findProject(
  projects: readonly SessionConfigProject[],
  target: string | null | undefined,
): SessionConfigProject | undefined {
  if (!target) return undefined;
  return projects.find((project) => projectTargetValue(project) === target);
}

// commandOptions is the materialize implementation for
// `materializeKey === 'commands'`. The curated [session].commands list lives
// on the daemon snapshot (sourced from /api/session-config). value === label
// since the command string IS what we send on the wire AND what we show to
// the user (FR/UAC for new-session "Command" picker, web-ui-fixes 2026-06-24).
export function commandOptions(
  daemon: DaemonSnapshot,
  projectPath?: string | null,
  executionServerId?: string | null,
  targetSource: NewSessionTarget["source"] = "project",
): ParamOption[] {
  const profiles =
    targetSource === "repository"
      ? daemon.serverOutcomes.find((server) => server.serverId === executionServerId)
          ?.commandProfiles
      : findProject(daemon.projects, projectPath)?.commandProfiles;
  return (profiles ?? []).map((profile) => ({
    value: profile.id,
    label: profile.label,
  }));
}

export function profileParameterID(name: string): string {
  return `profile:${name}`;
}

function profileParameterValues(
  profile: LaunchProfile,
  payload: Record<string, unknown>,
): Record<string, string | number | boolean> {
  const values: Record<string, string | number | boolean> = {};
  for (const parameter of profile.parameters) {
    const raw = payload[profileParameterID(parameter.name)];
    if (raw === undefined || raw === "") continue;
    if (parameter.type === "number") {
      const number = typeof raw === "number" ? raw : Number(raw);
      if (Number.isFinite(number)) values[parameter.name] = number;
      continue;
    }
    if (parameter.type === "boolean") {
      values[parameter.name] = raw === true || raw === "true";
      continue;
    }
    if (typeof raw === "string") values[parameter.name] = raw;
  }
  return values;
}

// ---------------------------------------------------------------------------
// Scope availability
// ---------------------------------------------------------------------------

// scopeDisabledReason is the single source of truth (ADR-0047) for whether a
// given palette scope is currently usable, returning either a short English
// reason or null when the scope is enabled. The same helper is consumed by tools-registry-dynamic-push
// to gate per-push ToolDef.disabledReason — keeping the literal strings in one
// place so the segment UI and the submit-time re-validation can never drift.
//
// - 'standard' is always enabled (FR-004 baseline): there is no precondition
//   the standard tools (new-session) can fail to meet from the daemon's
//   perspective; any HTTP-level rejection surfaces inside submit().
// - 'push' requires an active session AND an active-occupant of 'frame'
//   (FR-005, FR-006). occupant is optional on the snapshot because the wire
//   may not yet populate it; treat undefined as "no frame" — failing closed
//   keeps push disabled in the worst case rather than letting a stale push
//   fire against a session whose active buffer just shifted to main/log.
export function scopeDisabledReason(scope: ToolScope, daemon: DaemonSnapshot): string | null {
  if (scope === "standard") return null;
  if (!daemon.activeSessionID) return "No active session";
  if (daemon.activeOccupant !== "frame") return "No push-capable driver";
  return null;
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// makePushToolDef builds a paramless ToolDef for one curated push command.
// Each push ToolDef:
//   - id is "push:" + command, keyed so palette-store can find it via the
//     same lookup it uses for standard tools
//   - label is the bare command string (e.g. "save"), so the listbox row
//     shows exactly what will be sent
//   - params=null marks the tool as paramless (FR-010): hitting Enter on the
//     listbox entry fires submit() immediately without entering ParamSelectPhase
//   - disabledReason delegates to scopeDisabledReason('push', daemon) so the
//     listbox "disabled" row and the per-ToolDef gate use the same
//     function call (ADR-0047). Standard tools have their own permanent-null
//     disabledReason; push tools mirror the scope-level predicate exactly.
//   - submit calls ctx.http.pushCommand(activeSessionID, command). The store
//     (palette-store.submit) re-evaluates disabledReason before calling submit
//     (FR-023), so this implementation does NOT re-check — duplicating the
//     guard here would defeat the single-source rule and risk the two checks
//     drifting. We still defensively read activeSessionID and throw rather
//     than send a "" sessionId on the wire, in case a test calls submit
//     directly without going through palette-store.
function makePushToolDef(command: string): ToolDef {
  return {
    id: `push:${command}`,
    label: command,
    scope: "push",
    params: null,
    disabledReason(daemon: DaemonSnapshot): string | null {
      return scopeDisabledReason("push", daemon);
    },
    async submit(ctx: ToolCtx): Promise<void> {
      const sessionId = ctx.daemon.activeSessionID;
      if (!sessionId) {
        // palette-store should have blocked this via the disabledReason
        // re-check (FR-023); reaching here means the store skipped the gate
        // or a test exercised submit() directly. Either way we cannot send a
        // push to "no session", so throw a clear message instead of POSTing
        // to /api/sessions//push (which would 404 silently on some routers).
        throw new Error("no active session");
      }
      await ctx.http.pushCommand(sessionId, command);
      // FR-014 / UAC-018: snapshot is captured at submit start time.
      // frozenActiveContext comes from CommandPalette (command-palette-lift-state
      // task). When absent, fall back to deriving from ctx.daemon — this keeps
      // ToolSelectPhase (paramless push fast path) and direct test calls working
      // without requiring the caller to pre-compute the snapshot.
      const snap: ActiveContextSnapshot =
        ctx.frozenActiveContext ??
        deriveActiveContext(sessionId, ctx.daemon.sessions, ctx.daemon.projects);
      // Same ordering as standard tools: notify first, then close. If notify
      // throws the palette stays open with the error visible; ctx.store.close
      // is a no-op when the palette is already closed.
      if (snap.kind === "resolved") {
        ctx.notify.add({
          level: "info",
          title: `${snap.fullPath}\n${snap.fullSessionId}`,
          message: `Sent '${command}' → ${snap.projBase} · ${snap.sid8}`,
        });
      } else if (snap.kind === "unknown") {
        ctx.notify.add({
          level: "info",
          title: snap.fullSessionId,
          message: `Sent '${command}' → ??? · ${snap.sid8}`,
        });
      } else {
        // kind === 'none': activeSessionID was null — normally blocked by
        // the guard above (defensive branch for structural completeness).
        ctx.notify.add({ level: "info", message: `Sent '${command}'` });
      }
      ctx.store.close();
    },
  };
}

// toolsForPush materializes one push ToolDef per entry in pushCommands. The
// list is taken straight from the daemon snapshot's pushCommands (populated
// by session-config-extension); we do NOT filter on disabledReason here so
// that the listbox can render disabled rows with reason text rather than
// disappearing the entries entirely when push is unavailable. Order
// preserves the configured pushCommands order — the curated server list is
// the source of truth for surfacing priority.
export function toolsForPush(
  _daemon: DaemonSnapshot,
  pushCommands: ReadonlyArray<string>,
): ToolDef[] {
  return pushCommands.map((cmd) => makePushToolDef(cmd));
}

// listTools is the single entry point palette-store calls to enumerate
// available tools. Ordering is deterministic: standard tools first
// (new-session — the user-visible "create" CTA), then one push-scope
// ToolDef per entry in pushCommands. shutdown is intentionally NOT
// registered here (FR-028) — palette-store handles shutdown via a different
// code path (a dedicated confirm modal) so it does not appear in the tool
// list, even if "shutdown" or "/quit" happens to be in the curated push list.
export function listTools(daemon: DaemonSnapshot, pushCommands: ReadonlyArray<string>): ToolDef[] {
  return [newSessionTool, ...toolsForPush(daemon, pushCommands)];
}
