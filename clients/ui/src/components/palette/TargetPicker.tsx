import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import type { JSX } from "react";
import type { SessionConfigProject } from "../../api/sessions";
import { type FleetProvider, makeFleetProvider } from "../../fleet/provider";
import { projectTargetValue } from "../../lib/tools";
import {
  type ExecutionServer,
  GITHUB_ENTRY_KEY,
  type HostPreparation,
  type HostRepositoryProjection,
  type ProjectRow,
  type RepositoryPickerState,
  type RepositoryRow,
  type TargetRow,
  confirmationLabels,
  emptyPickerState,
  filteredTargets,
  githubStatusLabel,
  needsGitHubAuth,
  projectsSource,
  reduceRepositoryPicker,
  repositoryKey,
  serverKey,
} from "../../store/repository_picker";

export interface TargetPickerProps {
  focused: boolean;
  disabled: boolean;
  projects: readonly SessionConfigProject[];
  value: unknown;
  onSelectProject: (projectValue: string) => void;
  onEnter: () => void;
  onAnnounce: (message: string) => void;
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
    onEnter,
    onAnnounce,
    provider: providerProp,
  } = props;
  const provider = useMemo(() => providerProp ?? makeFleetProvider(), [providerProp]);
  const [state, dispatch] = useReducer(reduceRepositoryPicker, undefined, () => {
    const seed = reduceRepositoryPicker(emptyPickerState(), { type: "open" });
    return reduceRepositoryPicker(seed, {
      type: "applyProjection",
      projection: mergeDaemonProjects(
        {
          sources: [
            { id: "projects", available: true },
            { id: "github", available: false, status: "unauthenticated" },
          ],
          account: { state: "unauthenticated" },
          catalog: { outcome: "empty", generation: 0, repositories: [] },
          targets: [],
          preparations: [],
        },
        projects,
      ),
      servers: serversFromProjects(projects),
    });
  });
  const filterRef = useRef<HTMLInputElement | null>(null);
  const lastAnnounced = useRef(0);
  const announceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

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
    // selection and in-flight preparation are not wiped to unauthenticated empty.
    let lastComplete: HostRepositoryProjection | null = null;
    const pull = async () => {
      try {
        const projection = mergeDaemonProjects(await provider.repositoryProjection(), projects);
        if (cancelled) return;
        lastComplete = projection;
        dispatch({
          type: "applyProjection",
          projection,
          servers: serversFromProjects(projects),
        });
      } catch {
        if (cancelled) return;
        if (lastComplete) {
          // Source-scoped failure: keep retained catalog/account/selection surfaces.
          dispatch({
            type: "applyProjection",
            projection: {
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
            },
            servers: serversFromProjects(projects),
          });
          return;
        }
        // Cold start with no retained projection: typed unauthenticated empty only.
        dispatch({
          type: "applyProjection",
          projection: mergeDaemonProjects(
            {
              sources: [
                { id: "projects", available: true },
                { id: "github", available: false, status: "unauthenticated" },
              ],
              account: { state: "unauthenticated" },
              catalog: { outcome: "empty", generation: 0, repositories: [] },
              targets: [],
              preparations: [],
            },
            projects,
          ),
          servers: serversFromProjects(projects),
        });
      }
    };
    void pull();
    const id = setInterval(() => void pull(), 1500);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [provider, projects]);

  useEffect(() => {
    if (state.announceRevision === lastAnnounced.current || !state.announce) return;
    const prep = state.preparation;
    const terminal =
      !prep || prep.stage === "failed" || prep.stage === "cancelled" || prep.stage === "available";
    if (announceTimer.current) clearTimeout(announceTimer.current);
    const message = state.announce;
    const fire = () => {
      lastAnnounced.current = state.announceRevision;
      if (message) onAnnounce(message);
    };
    if (terminal) fire();
    else announceTimer.current = setTimeout(fire, 500);
    return () => {
      if (announceTimer.current) clearTimeout(announceTimer.current);
    };
  }, [state.announce, state.announceRevision, state.preparation, onAnnounce]);

  useEffect(() => {
    if (!focused || disabled) return;
    filterRef.current?.focus();
  }, [focused, disabled]);

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
    if (state.selected?.source === "project") {
      const match = projects.find((p) => projectKeyMatch(p, state.selected as ProjectRow));
      if (match && projectTargetValue(match) === value) return;
    }
    const match = projects.find((p) => projectTargetValue(p) === value);
    if (!match) return;
    const key = `project:${match.serverId ?? "local"}:${match.path}`;
    if (state.projects.some((p) => p.key === key) && state.selected?.key !== key) {
      dispatch({ type: "select", key });
    }
  }, [value, projects, state.projects, state.selected]);

  const { projects: visibleProjects, repositories: visibleRepos } = useMemo(
    () => filteredTargets(state),
    [state],
  );

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return;
    if (e.nativeEvent.isComposing) return;
    if (state.phase === "settings-auth" || state.phase === "preparing" || state.phase === "retry") {
      if (e.key === "Escape") {
        e.preventDefault();
        dispatch({ type: "back" });
      }
      return;
    }
    const keys =
      state.phase === "server-select"
        ? state.servers.filter((s) => s.available).map((s) => serverKey(s.serverId))
        : [...visibleProjects.map((p) => p.key), ...visibleRepos.map((r) => r.key)];
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
      if (
        state.phase === "server-select" &&
        state.selectedServerId &&
        state.selected?.source === "repository"
      ) {
        void startRepository(state, provider, dispatch);
        return;
      }
      if (state.phase === "server-select" && state.activeKey) {
        dispatch({ type: "select", key: state.activeKey });
        return;
      }
      const key = state.activeKey && keys.includes(state.activeKey) ? state.activeKey : keys[0];
      if (!key) return;
      dispatch({ type: "select", key });
      const row = [...visibleProjects, ...visibleRepos].find((r) => r.key === key);
      if (row?.source === "project") {
        const match = projects.find((p) => projectKeyMatch(p, row));
        if (match) onSelectProject(projectTargetValue(match));
        onEnter();
      }
      return;
    }
    if (e.key === "Escape" && state.phase === "server-select") {
      e.preventDefault();
      dispatch({ type: "back" });
    }
  };

  const refreshGitHub = useCallback(async () => {
    try {
      await provider.githubAction("refresh_catalog");
      const projection = mergeDaemonProjects(await provider.repositoryProjection(), projects);
      dispatch({
        type: "applyProjection",
        projection,
        servers: serversFromProjects(projects),
      });
    } catch {
      /* source-local */
    }
  }, [provider, projects]);

  const labels = confirmationLabels(state);

  return (
    <fieldset
      className={`palette-param target-picker ${focused ? "focused" : ""}`}
      data-param-id="project"
      data-testid="target-picker"
      aria-label="Project"
    >
      <label className="palette-param-label" htmlFor="palette-param-project-input">
        {state.phase === "server-select" ? "Execution server" : "Project"}
      </label>
      <input
        ref={setFilterRef}
        id="palette-param-project-input"
        className="palette-param-filter"
        type="text"
        role="combobox"
        aria-controls="palette-param-project"
        aria-expanded="true"
        aria-autocomplete="list"
        aria-activedescendant={state.activeKey ? optionDomId(state.activeKey) : undefined}
        value={state.query}
        disabled={disabled || state.phase === "preparing"}
        placeholder={
          state.phase === "server-select"
            ? "Filter servers..."
            : "Filter projects and repositories..."
        }
        onChange={(e) => dispatch({ type: "setQuery", query: e.target.value })}
        onKeyDown={onKeyDown}
        data-testid="palette-param-project-filter"
        data-source-filter="all"
      />

      {state.phase === "settings-auth" ? (
        <SettingsAuthPanel
          state={state}
          provider={provider}
          onReturn={(outcome) => dispatch({ type: "returnFromSettings", outcome })}
        />
      ) : null}

      {state.phase === "preparing" || state.phase === "retry" ? (
        <PreparationPanel
          state={state}
          onCancel={() => dispatch({ type: "cancelPreparation" })}
          onRetry={() => dispatch({ type: "retryPreparation" })}
          onExit={() => dispatch({ type: "exitRetry" })}
        />
      ) : null}

      {state.phase === "target-select" || state.phase === "server-select" ? (
        <>
          {/* Adjacent control groups — not inside listbox options. */}
          {state.phase === "target-select" ? (
            <fieldset className="target-picker__controls" aria-label="Source status">
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
              <div className="target-picker__source-status" data-testid="github-source-status">
                <span>GitHub repositories</span>
                {needsGitHubAuth(state) ? (
                  <button
                    type="button"
                    id={optionDomId(GITHUB_ENTRY_KEY)}
                    data-testid="github-auth-entry"
                    data-focus-key={GITHUB_ENTRY_KEY}
                    onClick={() => dispatch({ type: "openSettings" })}
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
            </fieldset>
          ) : null}

          {state.phase === "server-select" && state.selected?.source === "repository" ? (
            <div className="target-picker__confirm" data-testid="repository-server-confirm">
              <span data-testid="confirm-repository">Repository {labels.repository}</span>
              {labels.server ? (
                <span data-testid="confirm-server">Server {labels.server}</span>
              ) : (
                <span>Select an execution server</span>
              )}
              <button
                type="button"
                data-testid="server-select-back"
                onClick={() => dispatch({ type: "back" })}
              >
                Back
              </button>
            </div>
          ) : null}

          <div
            id="palette-param-project"
            className="palette-param-listbox target-picker__listbox"
            // biome-ignore lint/a11y/useSemanticElements: ARIA listbox pattern
            role="listbox"
            tabIndex={-1}
            aria-label={state.phase === "server-select" ? "Execution servers" : "Targets"}
            aria-activedescendant={state.activeKey ? optionDomId(state.activeKey) : undefined}
            data-testid="target-listbox"
          >
            {state.phase === "server-select" ? (
              state.servers.map((s) => {
                const key = serverKey(s.serverId);
                const selected = state.selectedServerId === s.serverId;
                const active = state.activeKey === key;
                return (
                  // biome-ignore lint/a11y/useFocusableInteractive: focus on combobox via aria-activedescendant
                  <div
                    key={key}
                    id={optionDomId(key)}
                    // biome-ignore lint/a11y/useSemanticElements: ARIA listbox uses div+role=option
                    role="option"
                    aria-selected={selected}
                    aria-disabled={!s.available || undefined}
                    data-stable-key={key}
                    data-source="server"
                    className={`palette-param-option ${active ? "selected" : ""} ${!s.available ? "disabled" : ""}`}
                    onMouseDown={(e) => {
                      e.preventDefault();
                      if (!s.available) return;
                      dispatch({ type: "selectServer", serverId: s.serverId });
                    }}
                  >
                    <span className="target-picker__badge">Server</span>
                    <span className="target-picker__identity">{s.serverLabel}</span>
                    <span className="target-picker__meta">{s.serverId}</span>
                    {!s.available ? <span>{s.reason ?? "Unavailable"}</span> : null}
                  </div>
                );
              })
            ) : (
              <>
                <div
                  className="target-picker__category"
                  role="presentation"
                  data-testid="category-projects"
                >
                  Projects
                </div>
                {visibleProjects.length === 0 ? (
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
                )}
                <div
                  className="target-picker__category"
                  role="presentation"
                  data-testid="category-github"
                >
                  GitHub repositories
                </div>
                {visibleRepos.length === 0 ? (
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
                      onPick={() => dispatch({ type: "select", key: row.key })}
                    />
                  ))
                )}
              </>
            )}
          </div>
        </>
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
      {row.source === "project" ? (
        <span className="target-picker__meta">{row.serverId}</span>
      ) : (
        <span className="target-picker__meta">{row.repositoryId}</span>
      )}
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
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const started = await provider.githubAction("start");
        if (cancelled) return;
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

function PreparationPanel(props: {
  state: RepositoryPickerState;
  onCancel: () => void;
  onRetry: () => void;
  onExit: () => void;
}): JSX.Element {
  const p = props.state.preparation;
  if (!p) return <div />;
  const repo = `${p.repository.owner}/${p.repository.name}`;
  const retry = props.state.phase === "retry";
  return (
    <section
      className="target-picker__prep"
      data-testid="preparation-panel"
      aria-label="Preparation progress"
    >
      <div data-testid="prep-repository">Repository {repo}</div>
      <div data-testid="prep-server">Server {p.server_id}</div>
      <div data-testid="prep-stage" data-stage={p.stage}>
        Stage: {p.stage}
      </div>
      {p.environment_disposition ? (
        <div data-testid="prep-environment">{p.environment_disposition}</div>
      ) : null}
      {p.failure_code ? <div data-testid="prep-failure">{p.failure_code}</div> : null}
      {p.retry_origin ? (
        <div data-testid="prep-retry-origin">Retry from {p.retry_origin}</div>
      ) : null}
      <div className="target-picker__settings-actions">
        {!retry ? (
          <button type="button" data-testid="prep-cancel" onClick={props.onCancel}>
            Cancel
          </button>
        ) : (
          <>
            <button type="button" data-testid="prep-retry" onClick={props.onRetry}>
              Retry
            </button>
            <button type="button" data-testid="prep-exit" onClick={props.onExit}>
              Exit
            </button>
          </>
        )}
      </div>
    </section>
  );
}

async function startRepository(
  state: RepositoryPickerState,
  provider: FleetProvider,
  dispatch: (e: Parameters<typeof reduceRepositoryPicker>[1]) => void,
): Promise<void> {
  if (state.selected?.source !== "repository" || !state.selectedServerId) return;
  const row = state.selected as RepositoryRow;
  try {
    const result = (await provider.createRepositorySession(state.selectedServerId, {
      repository_id: row.repositoryId,
      owner: row.owner,
      name: row.name,
    })) as Partial<HostPreparation>;
    const prep: HostPreparation = {
      preparation_id: String(result.preparation_id ?? `prep-${row.repositoryId}`),
      intent_id: String(result.intent_id ?? `intent-${row.repositoryId}`),
      server_id: state.selectedServerId,
      repository: {
        repository_id: row.repositoryId,
        owner: row.owner,
        name: row.name,
      },
      stage: String(result.stage ?? "clone-in-progress"),
      revision: Number(result.revision ?? 1),
      failure_code: result.failure_code,
      retry_origin: result.retry_origin,
      environment_disposition: result.environment_disposition,
      session_id: result.session_id,
    };
    dispatch({ type: "startPreparation", preparation: prep });
  } catch (err) {
    dispatch({
      type: "startPreparation",
      preparation: {
        preparation_id: `prep-${row.repositoryId}`,
        intent_id: `intent-${row.repositoryId}`,
        server_id: state.selectedServerId,
        repository: {
          repository_id: row.repositoryId,
          owner: row.owner,
          name: row.name,
        },
        stage: "failed",
        revision: 1,
        failure_code: err instanceof Error ? err.message : "create_failed",
        retry_origin: "clone",
      },
    });
  }
}

// Re-export for tests
export { repositoryKey, reduceRepositoryPicker, emptyPickerState };
