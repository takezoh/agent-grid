// Pure repository-first New Session picker (no DOM/HTTP).

export type AccountState =
  | "unauthenticated"
  | "authorizing"
  | "authenticated"
  | "denied"
  | "cancelled"
  | "expired"
  | "refresh_failed"
  | "host_misconfigured";
export type CatalogOutcome =
  | "empty"
  | "complete-current"
  | "refreshing-retained"
  | "source-failed-retained"
  | "authentication-required-retained"
  | "catalog_too_large"
  | "refresh_deadline";

export interface HostSource {
  id: string;
  available: boolean;
  status?: string;
}
export interface HostRepositoryIdentity {
  repository_id: string;
  owner: string;
  name: string;
}
export interface HostProjectTarget {
  server_id: string;
  project: string;
  server_label?: string;
}
export interface HostTarget {
  source: "project" | "repository";
  project?: HostProjectTarget;
  repository?: HostRepositoryIdentity;
}
export interface HostPreparation {
  preparation_id: string;
  intent_id: string;
  server_id: string;
  repository: HostRepositoryIdentity;
  stage: string;
  revision: number;
  failure_code?: string;
  retry_origin?: string;
  environment_disposition?: string;
  session_id?: string;
}
export interface AccountProjection {
  state: AccountState | string;
  login?: string;
  failure_code?: string;
}
export interface CatalogProjection {
  outcome: CatalogOutcome | string;
  generation: number;
  stale?: boolean;
  failure_code?: string;
  repositories: HostRepositoryIdentity[];
}
export interface HostRepositoryProjection {
  sources: HostSource[];
  account: AccountProjection;
  catalog: CatalogProjection;
  targets: HostTarget[];
  preparations: HostPreparation[];
}

export type ProjectRow = {
  source: "project";
  key: string;
  serverId: string;
  serverLabel: string;
  project: string;
  label: string;
};
export type RepositoryRow = {
  source: "repository";
  key: string;
  repositoryId: string;
  owner: string;
  name: string;
  label: string;
};
export type TargetRow = ProjectRow | RepositoryRow;
export type ExecutionServer = {
  serverId: string;
  serverLabel: string;
  available: boolean;
  reason?: string;
};
export type PickerPhase =
  | "target-select"
  | "server-select"
  | "settings-auth"
  | "preparing"
  | "retry";
export type SearchValidation = "validation-required" | "validated" | "invalidated";
export type AuthReturn =
  | "success"
  | "denied"
  | "expired"
  | "cancel"
  | "failure"
  | "unauthenticated";
export type SourceFilter = "all";
export type GitHubAction = "status" | "start" | "poll" | "cancel" | "refresh_catalog";
export type GitHubActionOutcome =
  | "success"
  | "denied"
  | "cancel"
  | "failure"
  | "authorizing"
  | "unauthenticated";
export interface GitHubActionResult {
  outcome: GitHubActionOutcome;
  account: AccountProjection;
  catalog: CatalogProjection;
  reason?: string;
}

export interface RepositoryPickerState {
  query: string;
  sourceFilter: SourceFilter;
  phase: PickerPhase;
  activeKey: string | null;
  selected: TargetRow | null;
  selectedServerId: string | null;
  focusReturnKey: string | null;
  account: AccountProjection;
  catalog: CatalogProjection;
  sources: HostSource[];
  projects: ProjectRow[];
  repositories: RepositoryRow[];
  servers: ExecutionServer[];
  preparation: HostPreparation | null;
  announce: string | null;
  announceRevision: number;
  searchValidation: SearchValidation;
  settingsReturn: AuthReturn | null;
  githubEntryKey: string;
}

export type RepositoryPickerEvent =
  | { type: "open" }
  | { type: "setQuery"; query: string }
  | { type: "applyProjection"; projection: HostRepositoryProjection; servers?: ExecutionServer[] }
  | { type: "activate"; key: string }
  | { type: "select"; key: string }
  | { type: "selectServer"; serverId: string }
  | { type: "back" }
  | { type: "openSettings" }
  | { type: "returnFromSettings"; outcome: AuthReturn }
  | { type: "startPreparation"; preparation: HostPreparation }
  | { type: "preparationUpdate"; preparation: HostPreparation }
  | { type: "cancelPreparation" }
  | { type: "retryPreparation" }
  | { type: "exitRetry" }
  | { type: "validateSearch"; knownFound: boolean; ambiguityFailure: boolean };

export const GITHUB_ENTRY_KEY = "github:entry";
export const PROJECTS_ENTRY_KEY = "projects:status";
export const projectKey = (serverId: string, project: string) => `project:${serverId}:${project}`;
export const repositoryKey = (id: string) => `repository:${id}`;
export const serverKey = (serverId: string) => `server:${serverId}`;

export function emptyPickerState(): RepositoryPickerState {
  return {
    query: "",
    sourceFilter: "all",
    phase: "target-select",
    activeKey: null,
    selected: null,
    selectedServerId: null,
    focusReturnKey: null,
    account: { state: "unauthenticated" },
    catalog: { outcome: "empty", generation: 0, repositories: [] },
    sources: [
      { id: "projects", available: true },
      { id: "github", available: false, status: "unauthenticated" },
    ],
    projects: [],
    repositories: [],
    servers: [],
    preparation: null,
    announce: null,
    announceRevision: 0,
    searchValidation: "validation-required",
    settingsReturn: null,
    githubEntryKey: GITHUB_ENTRY_KEY,
  };
}

function projectRows(targets: HostTarget[] | null | undefined): ProjectRow[] {
  const out: ProjectRow[] = [];
  for (const t of targets ?? []) {
    if (t.source !== "project" || !t.project?.server_id || !t.project.project) continue;
    const { server_id: serverId, project, server_label } = t.project;
    const serverLabel = server_label || serverId;
    out.push({
      source: "project",
      key: projectKey(serverId, project),
      serverId,
      serverLabel,
      project,
      label: `${serverLabel} · ${project}`,
    });
  }
  return out;
}

function repositoryRows(
  targets: HostTarget[] | null | undefined,
  catalog: CatalogProjection | null | undefined,
): RepositoryRow[] {
  const fromTargets = (targets ?? []).filter((t) => t.source === "repository" && t.repository);
  const repos =
    fromTargets.length > 0
      ? fromTargets.map((t) => t.repository as HostRepositoryIdentity)
      : (catalog?.repositories ?? []);
  const out: RepositoryRow[] = [];
  for (const r of repos) {
    if (!r.repository_id || !r.owner || !r.name) continue;
    out.push({
      source: "repository",
      key: repositoryKey(r.repository_id),
      repositoryId: r.repository_id,
      owner: r.owner,
      name: r.name,
      label: `${r.owner}/${r.name}`,
    });
  }
  return out;
}

export function matchesOwnerName(row: RepositoryRow, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return (
    row.owner.toLowerCase().includes(q) ||
    row.name.toLowerCase().includes(q) ||
    row.label.toLowerCase().includes(q)
  );
}

export function matchesProject(row: ProjectRow, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return (
    row.project.toLowerCase().includes(q) ||
    row.serverLabel.toLowerCase().includes(q) ||
    row.label.toLowerCase().includes(q)
  );
}

export function filteredTargets(state: RepositoryPickerState) {
  return {
    projects: state.projects.filter((p) => matchesProject(p, state.query)),
    repositories: state.repositories.filter((r) => matchesOwnerName(r, state.query)),
  };
}

export function optionKeys(state: RepositoryPickerState): string[] {
  if (state.phase === "server-select") {
    return state.servers.filter((s) => s.available).map((s) => serverKey(s.serverId));
  }
  const { projects, repositories } = filteredTargets(state);
  return [...projects.map((p) => p.key), ...repositories.map((r) => r.key)];
}

function announce(state: RepositoryPickerState, message: string): RepositoryPickerState {
  if (state.announce === message) return state;
  return { ...state, announce: message, announceRevision: state.announceRevision + 1 };
}

function recoverFocus(
  state: RepositoryPickerState,
  preferred?: string | null,
): RepositoryPickerState {
  const keys = optionKeys(state);
  if (preferred && keys.includes(preferred)) return { ...state, activeKey: preferred };
  if (state.activeKey && keys.includes(state.activeKey)) return state;
  if (state.selected && keys.includes(state.selected.key)) {
    return { ...state, activeKey: state.selected.key };
  }
  return { ...state, activeKey: keys[0] ?? null };
}

function clearInvalidSelection(state: RepositoryPickerState): RepositoryPickerState {
  const selected = state.selected;
  if (!selected) return state;
  // In-flight preparation/retry must retain selection even when the catalog
  // transiently omits the repository (projection refresh races).
  if (state.phase === "preparing" || state.phase === "retry") {
    return state;
  }
  const ok =
    selected.source === "project"
      ? state.projects.some((p) => p.key === selected.key)
      : state.repositories.some((r) => r.key === selected.key);
  if (ok) return state;
  return announce(
    { ...state, selected: null, selectedServerId: null, phase: "target-select" },
    selected.source === "project"
      ? "Selected project is no longer available"
      : "Selected repository is no longer available",
  );
}

function applyServers(
  state: RepositoryPickerState,
  servers: ExecutionServer[],
): RepositoryPickerState {
  let next: RepositoryPickerState = { ...state, servers };
  if (!next.selectedServerId) return next;
  const cur = servers.find((s) => s.serverId === next.selectedServerId);
  if (cur?.available) return next;
  next = announce(
    {
      ...next,
      selectedServerId: null,
      phase: next.phase === "preparing" ? next.phase : "server-select",
    },
    cur?.reason ?? "Selected server is unavailable",
  );
  const first = servers.find((s) => s.available);
  return recoverFocus(next, first ? serverKey(first.serverId) : null);
}

export function stageAnnouncement(p: HostPreparation): string {
  const repo = `${p.repository.owner}/${p.repository.name}`;
  const server = p.server_id;
  switch (p.stage) {
    case "confirmed":
      return `Preparing ${repo} on ${server}`;
    case "clone-in-progress":
      return `Cloning ${repo} on ${server}`;
    case "clone-ready":
      return `Clone ready for ${repo} on ${server}`;
    case "devcontainer-in-progress":
      return p.environment_disposition === "reuse"
        ? `Reusing repository environment for ${repo} on ${server}`
        : `Preparing devcontainer for ${repo} on ${server}`;
    case "available":
      return `Repository session available for ${repo} on ${server}`;
    case "failed":
      return `${p.retry_origin === "devcontainer" ? "Devcontainer" : "Clone"} failed for ${repo} on ${server}: ${p.failure_code ?? "error"}`;
    case "cancelled":
      return `Cancelled at ${p.retry_origin ?? "clone"} for ${repo} on ${server}`;
    default:
      return `Preparation ${p.stage} for ${repo} on ${server}`;
  }
}

function reducePreparation(
  state: RepositoryPickerState,
  preparation: HostPreparation,
): RepositoryPickerState {
  const prev = state.preparation;
  const stageChanged =
    !prev || prev.stage !== preparation.stage || prev.revision !== preparation.revision;
  const next: RepositoryPickerState = {
    ...state,
    preparation,
    phase:
      preparation.stage === "failed" || preparation.stage === "cancelled" ? "retry" : "preparing",
  };
  return stageChanged ? announce(next, stageAnnouncement(preparation)) : next;
}

export function reduceRepositoryPicker(
  state: RepositoryPickerState,
  event: RepositoryPickerEvent,
): RepositoryPickerState {
  switch (event.type) {
    case "open":
      return { ...emptyPickerState(), sourceFilter: "all" };
    case "setQuery":
      return recoverFocus({ ...state, query: event.query });
    case "applyProjection": {
      let next: RepositoryPickerState = {
        ...state,
        account: event.projection.account,
        catalog: event.projection.catalog,
        sources: event.projection.sources,
        projects: projectRows(event.projection.targets),
        repositories: repositoryRows(event.projection.targets, event.projection.catalog),
      };
      if (event.servers) next = applyServers(next, event.servers);
      // Preserve preparing/retry before clearInvalidSelection so transient catalog
      // omission cannot drop an in-flight preparation view (UAC-prep retain).
      if (next.phase === "settings-auth") return next;
      if (next.phase === "preparing" || next.phase === "retry") {
        next = clearInvalidSelection(next);
        const prep = event.projection.preparations.find(
          (p) => p.preparation_id === next.preparation?.preparation_id,
        );
        return prep ? reducePreparation(next, prep) : next;
      }
      next = clearInvalidSelection(next);
      return recoverFocus(next, next.activeKey ?? next.focusReturnKey);
    }
    case "activate":
      return { ...state, activeKey: event.key };
    case "select": {
      if (state.phase === "server-select") {
        const id = event.key.startsWith("server:") ? event.key.slice(7) : event.key;
        return reduceRepositoryPicker(state, { type: "selectServer", serverId: id });
      }
      const project = state.projects.find((p) => p.key === event.key);
      if (project) {
        return {
          ...state,
          selected: project,
          selectedServerId: project.serverId,
          activeKey: project.key,
          focusReturnKey: project.key,
          phase: "target-select",
        };
      }
      const repo = state.repositories.find((r) => r.key === event.key);
      if (!repo) return state;
      const first = state.servers.find((s) => s.available);
      return recoverFocus(
        {
          ...state,
          selected: repo,
          selectedServerId: null,
          activeKey: repo.key,
          focusReturnKey: repo.key,
          phase: "server-select",
        },
        first ? serverKey(first.serverId) : null,
      );
    }
    case "selectServer": {
      if (state.selected?.source !== "repository") return state;
      const server = state.servers.find((s) => s.serverId === event.serverId);
      if (!server?.available) {
        return announce(
          recoverFocus({ ...state, selectedServerId: null }, null),
          server?.reason ?? "Server unavailable",
        );
      }
      return {
        ...state,
        selectedServerId: server.serverId,
        activeKey: serverKey(server.serverId),
        phase: "server-select",
      };
    }
    case "back": {
      if (state.phase === "server-select" && state.selected?.source === "repository") {
        return recoverFocus(
          { ...state, phase: "target-select", selectedServerId: null },
          state.focusReturnKey ?? state.selected.key,
        );
      }
      if (state.phase === "settings-auth") {
        return {
          ...state,
          phase: "target-select",
          activeKey: GITHUB_ENTRY_KEY,
          settingsReturn: null,
        };
      }
      if (state.phase === "retry") {
        return {
          ...state,
          phase: "target-select",
          preparation: null,
          activeKey: state.focusReturnKey,
        };
      }
      return state;
    }
    case "openSettings":
      return {
        ...state,
        phase: "settings-auth",
        focusReturnKey: GITHUB_ENTRY_KEY,
        settingsReturn: null,
      };
    case "returnFromSettings": {
      const selected = state.selected?.source === "repository" ? null : state.selected;
      return {
        ...state,
        phase: "target-select",
        selected,
        selectedServerId: selected?.source === "project" ? selected.serverId : null,
        settingsReturn: event.outcome,
        activeKey: GITHUB_ENTRY_KEY,
        focusReturnKey: GITHUB_ENTRY_KEY,
      };
    }
    case "startPreparation":
      return announce(
        {
          ...state,
          phase: "preparing",
          preparation: event.preparation,
          selectedServerId: event.preparation.server_id,
        },
        stageAnnouncement(event.preparation),
      );
    case "preparationUpdate":
      return reducePreparation(state, event.preparation);
    case "cancelPreparation": {
      if (!state.preparation) return state;
      const cancelled: HostPreparation = {
        ...state.preparation,
        stage: "cancelled",
        retry_origin:
          state.preparation.stage === "devcontainer-in-progress" ||
          state.preparation.stage === "clone-ready"
            ? "devcontainer"
            : "clone",
        revision: state.preparation.revision + 1,
      };
      return announce(
        { ...state, phase: "retry", preparation: cancelled },
        `Cancelled at ${cancelled.retry_origin} for ${cancelled.repository.owner}/${cancelled.repository.name} on ${cancelled.server_id}`,
      );
    }
    case "retryPreparation": {
      if (!state.preparation) return state;
      const origin = state.preparation.retry_origin ?? "clone";
      const prep: HostPreparation = {
        ...state.preparation,
        stage: origin === "devcontainer" ? "devcontainer-in-progress" : "clone-in-progress",
        failure_code: undefined,
        revision: state.preparation.revision + 1,
      };
      return announce({ ...state, phase: "preparing", preparation: prep }, stageAnnouncement(prep));
    }
    case "exitRetry":
      return {
        ...state,
        phase: "target-select",
        preparation: null,
        activeKey: state.focusReturnKey,
      };
    case "validateSearch":
      return {
        ...state,
        searchValidation: event.ambiguityFailure || !event.knownFound ? "invalidated" : "validated",
      };
    default:
      return state;
  }
}

export const githubSource = (s: RepositoryPickerState) => s.sources.find((x) => x.id === "github");
export const projectsSource = (s: RepositoryPickerState) =>
  s.sources.find((x) => x.id === "projects");

export function needsGitHubAuth(state: RepositoryPickerState): boolean {
  const st = state.account.state;
  return (
    st === "unauthenticated" ||
    st === "denied" ||
    st === "cancelled" ||
    st === "expired" ||
    st === "refresh_failed" ||
    githubSource(state)?.status === "authentication-required-retained"
  );
}

export function githubStatusLabel(state: RepositoryPickerState): string {
  const gh = githubSource(state);
  if (state.catalog.stale && state.catalog.outcome === "source-failed-retained") {
    return `Stale: ${state.catalog.failure_code ?? "source failed"}`;
  }
  if (state.catalog.outcome === "refreshing-retained") return "Refreshing…";
  if (needsGitHubAuth(state)) return "Sign in to GitHub";
  if (gh && !gh.available) return gh.status ?? state.account.state;
  if (state.account.state === "authenticated" && state.account.login) {
    return `GitHub · ${state.account.login}`;
  }
  return gh?.status ?? "GitHub";
}

export function confirmationLabels(state: RepositoryPickerState) {
  if (state.selected?.source === "repository") {
    return {
      repository: `${state.selected.owner}/${state.selected.name}`,
      server: state.selectedServerId
        ? (state.servers.find((s) => s.serverId === state.selectedServerId)?.serverLabel ??
          state.selectedServerId)
        : undefined,
    };
  }
  if (state.selected?.source === "project") {
    return { project: state.selected.label, server: state.selected.serverLabel };
  }
  return {};
}

export function evaluateSearchValidation(
  corpus: RepositoryRow[],
  cases: Array<{ query: string; intendedKey?: string; ambiguous?: boolean }>,
): SearchValidation {
  for (const c of cases) {
    const hits = corpus.filter((r) => matchesOwnerName(r, c.query));
    if (c.ambiguous && hits.length !== 1) return "invalidated";
    if (c.intendedKey && !hits.some((h) => h.key === c.intendedKey)) return "invalidated";
  }
  return "validated";
}
