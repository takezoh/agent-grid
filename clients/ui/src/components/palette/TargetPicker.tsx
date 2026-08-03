import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import type { JSX } from "react";
import type { ServerConfigOutcome, SessionConfigProject } from "../../api/sessions";
import { type FleetProvider, makeFleetProvider } from "../../fleet/provider";
import { projectTargetValue } from "../../lib/tools";
import {
  type ExecutionServer,
  GITHUB_ENTRY_KEY,
  type HostRepositoryProjection,
  type ProjectRow,
  type RepositoryPickerState,
  type SourceFilter,
  type TargetRow,
  emptyPickerState,
  filteredTargets,
  githubStatusLabel,
  needsGitHubAuth,
  projectsSource,
  reduceRepositoryPicker,
  repositoryKey,
} from "../../store/repository_picker";

export interface TargetPickerProps {
  focused: boolean;
  disabled: boolean;
  projects: readonly SessionConfigProject[];
  value: unknown;
  onSelectProject: (projectValue: string) => void;
  onSelectRepository?: (repository: {
    repository_id: string;
    owner: string;
    name: string;
  }) => void;
  onEnter: () => void;
  onAnnounce: (message: string) => void;
  executionServers?: readonly ServerConfigOutcome[];
  provider?: FleetProvider;
  inputRef?: React.Ref<HTMLInputElement>;
}

function serversFromProjects(projects: readonly SessionConfigProject[]): ExecutionServer[] {
  const map = new Map<string, ExecutionServer>();
  for (const p of projects) {
    if (!p.serverId) continue;
    if (!map.has(p.serverId)) {
      map.set(p.serverId, {
        serverId: p.serverId,
        serverLabel: p.serverLabel || p.serverId,
        available: true,
      });
    }
  }
  return [...map.values()];
}

function resolveServers(
  projection: HostRepositoryProjection,
  projects: readonly SessionConfigProject[],
  configured?: readonly ServerConfigOutcome[],
): ExecutionServer[] {
  if (configured && configured.length > 0) {
    return configured.map((server) => ({
      serverId: server.serverId,
      serverLabel: server.serverLabel,
      available: server.state === "connected",
      isDefaultExecution: server.isDefaultExecution,
      reason: server.reason,
    }));
  }
  if (projection.execution_servers && projection.execution_servers.length > 0) {
    return projection.execution_servers;
  }
  return serversFromProjects(projects);
}

function mergeDaemonProjects(
  projection: HostRepositoryProjection,
  projects: readonly SessionConfigProject[],
): HostRepositoryProjection {
  const seen = new Set(
    projection.targets.flatMap((t) => {
      if (t.source !== "project" || !t.project) return [];
      return [`${t.project.server_id}\0${t.project.project}`];
    }),
  );
  const targets = [...projection.targets];
  for (const p of projects) {
    const serverId = p.serverId ?? "local";
    const key = `${serverId}\0${p.path}`;
    if (seen.has(key)) continue;
    seen.add(key);
    targets.push({
      source: "project",
      project: { server_id: serverId, project: p.path, server_label: p.serverLabel || serverId },
    });
  }
  return { ...projection, targets };
}

export function TargetPicker(props: TargetPickerProps): JSX.Element {
  const {
    focused,
    disabled,
    projects,
    value,
    onSelectProject,
    onSelectRepository,
    onEnter,
    onAnnounce,
    provider: providerProp,
  } = props;
  const provider = useMemo(() => providerProp ?? makeFleetProvider(), [providerProp]);
  const [state, dispatch] = useReducer(reduceRepositoryPicker, undefined, () => {
    const seed = reduceRepositoryPicker(emptyPickerState(), { type: "open" });
    const initial: HostRepositoryProjection = {
      sources: [
        { id: "projects", available: true },
        { id: "github", available: false, status: "unauthenticated" },
      ],
      account: { state: "unauthenticated" },
      catalog: { outcome: "empty", generation: 0, repositories: [] },
      targets: [],
      preparations: [],
    };
    const projection = mergeDaemonProjects(initial, projects);
    return reduceRepositoryPicker(seed, {
      type: "applyProjection",
      projection,
      servers: resolveServers(projection, projects, props.executionServers),
    });
  });
  const filterRef = useRef<HTMLInputElement | null>(null);
  const githubTabRef = useRef<HTMLButtonElement | null>(null);
  const projectsTabRef = useRef<HTMLButtonElement | null>(null);
  const lastAnnounced = useRef(0);

  const setFilterRef = (el: HTMLInputElement | null) => {
    filterRef.current = el;
    const ext = props.inputRef;
    if (typeof ext === "function") ext(el);
    else if (ext && typeof ext === "object")
      (ext as React.MutableRefObject<HTMLInputElement | null>).current = el;
  };

  useEffect(() => {
    let cancelled = false;
    // Last complete projection retained across transient poll failures so catalog
    // selection is not wiped to an unauthenticated empty state.
    let lastComplete: HostRepositoryProjection | null = null;
    const pull = async () => {
      try {
        const projection = mergeDaemonProjects(await provider.repositoryProjection(), projects);
        if (cancelled) return;
        lastComplete = projection;
        dispatch({
          type: "applyProjection",
          projection,
          servers: resolveServers(projection, projects, props.executionServers),
        });
      } catch {
        if (cancelled) return;
        if (lastComplete) {
          // Source-scoped failure: keep retained catalog/account/selection surfaces.
          const failed: HostRepositoryProjection = {
            ...lastComplete,
            sources: lastComplete.sources.map((s) =>
              s.id === "github"
                ? { ...s, available: s.available, status: "source-failed-retained" }
                : s,
            ),
            catalog: {
              ...lastComplete.catalog,
              outcome: "source-failed-retained",
              stale: true,
              failure_code: lastComplete.catalog.failure_code ?? "projection_poll_failed",
            },
          };
          dispatch({
            type: "applyProjection",
            projection: failed,
            servers: resolveServers(failed, projects, props.executionServers),
          });
          return;
        }
        // Cold start with no retained projection: typed unauthenticated empty only.
        const empty: HostRepositoryProjection = {
          sources: [
            { id: "projects", available: true },
            { id: "github", available: false, status: "unauthenticated" },
          ],
          account: { state: "unauthenticated" },
          catalog: { outcome: "empty", generation: 0, repositories: [] },
          targets: [],
          preparations: [],
        };
        const projection = mergeDaemonProjects(empty, projects);
        dispatch({
          type: "applyProjection",
          projection,
          servers: resolveServers(projection, projects, props.executionServers),
        });
      }
    };
    void pull();
    const id = setInterval(() => void pull(), 1500);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [provider, projects, props.executionServers]);

  useEffect(() => {
    if (state.announceRevision === lastAnnounced.current || !state.announce) return;
    lastAnnounced.current = state.announceRevision;
    onAnnounce(state.announce);
  }, [state.announce, state.announceRevision, onAnnounce]);

  useEffect(() => {
    if (!focused || disabled) return;
    filterRef.current?.focus();
  }, [focused, disabled]);

  // UAC-005 / UAC-013: after GitHub settings cancel/complete, restore DOM focus
  // to the github-auth-entry control (not only aria-activedescendant).
  useEffect(() => {
    if (!focused || disabled) return;
    if (state.phase !== "target-select") return;
    if (state.activeKey !== GITHUB_ENTRY_KEY) return;
    if (state.settingsReturn == null) return;
    const entry = document.querySelector<HTMLElement>('[data-testid="github-auth-entry"]');
    entry?.focus();
  }, [focused, disabled, state.phase, state.activeKey, state.settingsReturn]);

  useEffect(() => {
    if (state.selected?.source === "project") {
      const match = projects.find((p) => projectKeyMatch(p, state.selected as ProjectRow));
      if (match) {
        const v = projectTargetValue(match);
        if (v !== value) onSelectProject(v);
      }
    }
  }, [state.selected, projects, value, onSelectProject]);

  useEffect(() => {
    if (typeof value !== "string" || !value) return;
    if (state.selected?.source === "repository") return;
    if (state.selected?.source === "project") {
      const match = projects.find((p) => projectKeyMatch(p, state.selected as ProjectRow));
      if (match && projectTargetValue(match) === value) return;
    }
    const match = projects.find((p) => projectTargetValue(p) === value);
    if (!match) return;
    const key = `project:${match.serverId ?? "local"}:${match.path}`;
    if (state.projects.some((p) => p.key === key) && state.selected?.key !== key) {
      dispatch({ type: "syncProject", key });
    }
  }, [value, projects, state.projects, state.selected]);

  const { projects: visibleProjects, repositories: visibleRepos } = useMemo(
    () => filteredTargets(state),
    [state],
  );

  const visibleOptionKeys = [
    ...visibleProjects.map((p) => p.key),
    ...visibleRepos.map((r) => r.key),
  ];
  const activeOptionId =
    state.activeKey && visibleOptionKeys.includes(state.activeKey)
      ? optionDomId(state.activeKey)
      : undefined;

  const activateSourceTab = (sourceFilter: SourceFilter) => {
    dispatch({ type: "setSourceFilter", sourceFilter });
    (sourceFilter === "github" ? githubTabRef : projectsTabRef).current?.focus();
  };

  const onSourceTabKeyDown = (
    event: React.KeyboardEvent<HTMLButtonElement>,
    sourceFilter: SourceFilter,
  ) => {
    let next: SourceFilter | null = null;
    if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
      next = sourceFilter === "github" ? "projects" : "github";
    } else if (event.key === "Home") {
      next = "github";
    } else if (event.key === "End") {
      next = "projects";
    }
    if (!next) return;
    event.preventDefault();
    activateSourceTab(next);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return;
    if (e.nativeEvent.isComposing) return;
    if (state.phase === "settings-auth") {
      if (e.key === "Escape") {
        e.preventDefault();
        dispatch({ type: "back" });
      }
      return;
    }
    const keys = visibleOptionKeys;
    const idx = state.activeKey ? keys.indexOf(state.activeKey) : -1;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const next = keys[Math.min(idx + 1, keys.length - 1)] ?? keys[0];
      if (next) dispatch({ type: "activate", key: next });
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      const next = keys[Math.max(idx - 1, 0)] ?? keys[0];
      if (next) dispatch({ type: "activate", key: next });
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      const key = state.activeKey && keys.includes(state.activeKey) ? state.activeKey : keys[0];
      if (!key) return;
      dispatch({ type: "select", key });
      const row = [...visibleProjects, ...visibleRepos].find((r) => r.key === key);
      if (row?.source === "project") {
        const match = projects.find((p) => projectKeyMatch(p, row));
        if (match) onSelectProject(projectTargetValue(match));
        onEnter();
      } else if (row?.source === "repository") {
        onSelectRepository?.({
          repository_id: row.repositoryId,
          owner: row.owner,
          name: row.name,
        });
        onEnter();
      }
      return;
    }
  };

  const refreshGitHub = useCallback(async () => {
    try {
      await provider.githubAction("refresh_catalog");
      const projection = mergeDaemonProjects(await provider.repositoryProjection(), projects);
      dispatch({
        type: "applyProjection",
        projection,
        servers: resolveServers(projection, projects, props.executionServers),
      });
    } catch {
      /* source-local */
    }
  }, [provider, projects, props.executionServers]);

  return (
    <fieldset
      className={`palette-param target-picker ${focused ? "focused" : ""}`}
      data-param-id="project"
      data-testid="target-picker"
      aria-label="Project"
    >
      <label className="palette-param-label" htmlFor="palette-param-project-input">
        {state.sourceFilter === "github" ? "GitHub repository" : "Project"}
      </label>
      {state.phase === "target-select" ? (
        <div className="target-picker__tabs" role="tablist" aria-label="Target source">
          <button
            ref={githubTabRef}
            id="target-source-tab-github"
            className="target-picker__tab"
            type="button"
            role="tab"
            aria-selected={state.sourceFilter === "github"}
            aria-controls="target-source-panel"
            tabIndex={state.sourceFilter === "github" ? 0 : -1}
            onClick={() => activateSourceTab("github")}
            onKeyDown={(event) => onSourceTabKeyDown(event, "github")}
          >
            GitHub repositories
          </button>
          <button
            ref={projectsTabRef}
            id="target-source-tab-projects"
            className="target-picker__tab"
            type="button"
            role="tab"
            aria-selected={state.sourceFilter === "projects"}
            aria-controls="target-source-panel"
            tabIndex={state.sourceFilter === "projects" ? 0 : -1}
            onClick={() => activateSourceTab("projects")}
            onKeyDown={(event) => onSourceTabKeyDown(event, "projects")}
          >
            Projects
          </button>
        </div>
      ) : null}
      <input
        ref={setFilterRef}
        id="palette-param-project-input"
        className="palette-param-filter"
        type="text"
        role="combobox"
        aria-controls="palette-param-project"
        aria-expanded="true"
        aria-autocomplete="list"
        aria-activedescendant={activeOptionId}
        value={state.query}
        disabled={disabled}
        placeholder={
          state.sourceFilter === "github" ? "Filter GitHub repositories..." : "Filter projects..."
        }
        onChange={(e) => dispatch({ type: "setQuery", query: e.target.value })}
        onKeyDown={onKeyDown}
        data-testid="palette-param-project-filter"
        data-source-filter={state.sourceFilter}
      />

      {state.phase === "settings-auth" ? (
        <SettingsAuthPanel
          state={state}
          provider={provider}
          onReturn={(outcome) => dispatch({ type: "returnFromSettings", outcome })}
        />
      ) : null}

      {state.phase === "target-select" ? (
        <div
          id="target-source-panel"
          className="target-picker__panel"
          role="tabpanel"
          aria-labelledby={`target-source-tab-${state.sourceFilter}`}
        >
          {/* Source controls remain adjacent to, never nested in, listbox options. */}
          <fieldset className="target-picker__controls" aria-label="Source status">
            {state.sourceFilter === "projects" ? (
              <div className="target-picker__source-status" data-testid="projects-source-status">
                <span>Projects</span>
                {!projectsSource(state)?.available ? (
                  <button
                    type="button"
                    data-testid="projects-retry"
                    onClick={() => void refreshGitHub()}
                  >
                    Retry projects
                  </button>
                ) : (
                  <span data-testid="projects-ok">Available</span>
                )}
              </div>
            ) : (
              <div className="target-picker__source-status" data-testid="github-source-status">
                <span>GitHub repositories</span>
                {needsGitHubAuth(state) ? (
                  <button
                    type="button"
                    id={optionDomId(GITHUB_ENTRY_KEY)}
                    data-testid="github-auth-entry"
                    data-focus-key={GITHUB_ENTRY_KEY}
                    onClick={() => {
                      if (provider.openGitHubSettings) {
                        void provider.openGitHubSettings();
                        return;
                      }
                      dispatch({ type: "openSettings" });
                    }}
                  >
                    {githubStatusLabel(state)}
                  </button>
                ) : (
                  <>
                    <span data-testid="github-status-label">{githubStatusLabel(state)}</span>
                    {state.catalog.stale || state.catalog.outcome === "source-failed-retained" ? (
                      <button
                        type="button"
                        data-testid="github-retry"
                        onClick={() => void refreshGitHub()}
                      >
                        Retry
                      </button>
                    ) : state.catalog.outcome === "refreshing-retained" ? (
                      <span data-testid="github-refreshing">Refreshing…</span>
                    ) : (
                      <button
                        type="button"
                        data-testid="github-refresh"
                        onClick={() => void refreshGitHub()}
                      >
                        Refresh
                      </button>
                    )}
                  </>
                )}
              </div>
            )}
          </fieldset>
          <div
            id="palette-param-project"
            className="palette-param-listbox target-picker__listbox"
            // biome-ignore lint/a11y/useSemanticElements: ARIA listbox pattern
            role="listbox"
            tabIndex={-1}
            aria-label={state.sourceFilter === "github" ? "GitHub repositories" : "Projects"}
            aria-activedescendant={activeOptionId}
            data-testid="target-listbox"
          >
            {state.sourceFilter === "projects" ? (
              visibleProjects.length === 0 ? (
                <div className="palette-param-listbox__empty" role="presentation">
                  No project matches
                </div>
              ) : (
                visibleProjects.map((row) => (
                  <TargetOption
                    key={row.key}
                    row={row}
                    active={state.activeKey === row.key}
                    selected={state.selected?.key === row.key}
                    onPick={() => {
                      dispatch({ type: "select", key: row.key });
                      const match = projects.find((p) => projectKeyMatch(p, row));
                      if (match) onSelectProject(projectTargetValue(match));
                      onEnter();
                    }}
                  />
                ))
              )
            ) : visibleRepos.length === 0 ? (
              <div className="palette-param-listbox__empty" role="presentation">
                No repository matches
              </div>
            ) : (
              visibleRepos.map((row) => (
                <TargetOption
                  key={row.key}
                  row={row}
                  active={state.activeKey === row.key}
                  selected={state.selected?.key === row.key}
                  onPick={() => {
                    dispatch({ type: "select", key: row.key });
                    onSelectRepository?.({
                      repository_id: row.repositoryId,
                      owner: row.owner,
                      name: row.name,
                    });
                    onEnter();
                  }}
                />
              ))
            )}
          </div>
        </div>
      ) : null}
    </fieldset>
  );
}

function optionDomId(key: string): string {
  return `palette-param-project-opt-${key.replace(/[^a-zA-Z0-9_-]/g, "_")}`;
}

function projectKeyMatch(p: SessionConfigProject, row: ProjectRow): boolean {
  return (p.serverId ?? "local") === row.serverId && p.path === row.project;
}

function TargetOption(props: {
  row: TargetRow;
  active: boolean;
  selected: boolean;
  onPick: () => void;
}): JSX.Element {
  const { row, active, selected, onPick } = props;
  return (
    // biome-ignore lint/a11y/useFocusableInteractive: focus on combobox via aria-activedescendant
    <div
      id={optionDomId(row.key)}
      // biome-ignore lint/a11y/useSemanticElements: ARIA listbox uses div+role=option
      role="option"
      aria-selected={selected}
      data-stable-key={row.key}
      data-source={row.source}
      data-testid={`target-option-${row.key}`}
      className={`palette-param-option ${active ? "selected" : ""}`}
      onMouseDown={(e) => {
        e.preventDefault();
        onPick();
      }}
    >
      <span className="target-picker__badge" data-testid="source-badge">
        {row.source === "project" ? "Project" : "GitHub"}
      </span>
      <span className="target-picker__identity" data-testid="stable-identity">
        {row.source === "project" ? row.label : `${row.owner}/${row.name}`}
      </span>
    </div>
  );
}

function SettingsAuthPanel(props: {
  state: RepositoryPickerState;
  provider: FleetProvider;
  onReturn: (
    outcome: "success" | "denied" | "expired" | "cancel" | "failure" | "unauthenticated",
  ) => void;
}): JSX.Element {
  const { provider, onReturn } = props;
  const [code, setCode] = useState("");
  const [uri, setUri] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const started = await provider.githubAction("start");
        if (cancelled) return;
        if (
          started.account.state === "host_misconfigured" &&
          started.account.failure_code === "host_misconfigured"
        ) {
          setCode("");
          setUri("");
          setError("GitHub OAuth App is not configured on this host.");
          return;
        }
        if (started.account.state !== "authorizing") {
          setCode("");
          setUri("");
          return;
        }
        const userCode =
          "user_code" in started.account
            ? String((started.account as { user_code?: string }).user_code ?? "")
            : "";
        const verification =
          "verification_uri" in started.account
            ? String((started.account as { verification_uri?: string }).verification_uri ?? "")
            : "https://github.com/login/device";
        setCode(userCode);
        setUri(verification);
      } catch {
        if (!cancelled) onReturn("failure");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [provider, onReturn]);

  return (
    <section
      className="target-picker__settings"
      data-testid="github-settings-surface"
      aria-label="GitHub account"
    >
      <p>Authorize GitHub in Settings. Credentials stay on the host.</p>
      {error ? <p data-testid="github-auth-error">{error}</p> : null}
      {uri ? (
        <p data-testid="github-device-uri">
          Open <a href={uri}>{uri}</a>
          {code ? (
            <>
              {" "}
              and enter <code data-testid="github-device-code">{code}</code>
            </>
          ) : null}
        </p>
      ) : null}
      <div className="target-picker__settings-actions">
        <button
          type="button"
          data-testid="github-settings-poll"
          disabled={busy}
          onClick={() => {
            setBusy(true);
            void provider
              .githubAction("poll")
              .then((r) => {
                const map: Record<
                  string,
                  "success" | "denied" | "cancel" | "failure" | "unauthenticated"
                > = {
                  success: "success",
                  denied: "denied",
                  cancel: "cancel",
                  failure: "failure",
                  unauthenticated: "unauthenticated",
                  authorizing: "unauthenticated",
                };
                onReturn(map[r.outcome] ?? "failure");
              })
              .catch(() => onReturn("failure"))
              .finally(() => setBusy(false));
          }}
        >
          Complete
        </button>
        <button
          type="button"
          data-testid="github-settings-cancel"
          onClick={() => {
            void provider.githubAction("cancel").finally(() => onReturn("cancel"));
          }}
        >
          Cancel
        </button>
        <button type="button" data-testid="github-settings-back" onClick={() => onReturn("cancel")}>
          Back
        </button>
      </div>
    </section>
  );
}

// Re-export for tests
export { repositoryKey, reduceRepositoryPicker, emptyPickerState };
