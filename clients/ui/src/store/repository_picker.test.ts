import { describe, expect, it } from "vitest";
import {
  GITHUB_ENTRY_KEY,
  type HostRepositoryProjection,
  type RepositoryPickerState,
  type RepositoryRow,
  emptyPickerState,
  evaluateSearchValidation,
  filteredTargets,
  matchesOwnerName,
  projectKey,
  reduceRepositoryPicker,
  repositoryKey,
} from "./repository_picker";

function projection(partial?: Partial<HostRepositoryProjection>): HostRepositoryProjection {
  return {
    sources: [
      { id: "projects", available: true },
      { id: "github", available: true, status: "complete-current" },
    ],
    account: { state: "authenticated", login: "alice" },
    catalog: {
      outcome: "complete-current",
      generation: 1,
      repositories: [
        { repository_id: "10", owner: "alice", name: "demo" },
        { repository_id: "11", owner: "alice", name: "demo-extra" },
      ],
    },
    targets: [
      {
        source: "project",
        project: { server_id: "srv-a", project: "/repo/demo", server_label: "Alpha" },
      },
      { source: "repository", repository: { repository_id: "10", owner: "alice", name: "demo" } },
      {
        source: "repository",
        repository: { repository_id: "11", owner: "alice", name: "demo-extra" },
      },
    ],
    preparations: [],
    ...partial,
  };
}

function openWithCatalog(): RepositoryPickerState {
  return reduceRepositoryPicker(reduceRepositoryPicker(emptyPickerState(), { type: "open" }), {
    type: "applyProjection",
    projection: projection(),
  });
}

describe("repository picker source tabs", () => {
  it("opens on GitHub and switches to projects without mixing sources", () => {
    const state = openWithCatalog();
    expect(state.sourceFilter).toBe("github");
    expect(filteredTargets(state).repositories).toHaveLength(2);
    expect(filteredTargets(state).projects).toHaveLength(0);

    const projects = reduceRepositoryPicker(state, {
      type: "setSourceFilter",
      sourceFilter: "projects",
    });
    expect(filteredTargets(projects).projects).toHaveLength(1);
    expect(filteredTargets(projects).repositories).toHaveLength(0);
  });

  it("keeps externally synchronized project hidden on the GitHub tab", () => {
    const state = openWithCatalog();
    const synced = reduceRepositoryPicker(state, {
      type: "syncProject",
      key: projectKey("srv-a", "/repo/demo"),
    });
    expect(synced.selected?.source).toBe("project");
    expect(synced.selectedServerId).toBe("srv-a");
    expect(filteredTargets(synced).projects).toHaveLength(0);
  });
});

describe("repository selection delegates to the shared New Session flow", () => {
  it("retains stable repository identity without entering server or preparation phases", () => {
    const selected = reduceRepositoryPicker(openWithCatalog(), {
      type: "select",
      key: repositoryKey("10"),
    });
    expect(selected.phase).toBe("target-select");
    expect(selected.selectedServerId).toBeNull();
    expect(selected.selected).toMatchObject({
      source: "repository",
      repositoryId: "10",
      owner: "alice",
      name: "demo",
      label: "alice/demo",
    });
  });

  it("clears a repository removed by a refreshed catalog while preserving the query", () => {
    let state = reduceRepositoryPicker(openWithCatalog(), { type: "setQuery", query: "demo" });
    state = reduceRepositoryPicker(state, { type: "select", key: repositoryKey("10") });
    state = reduceRepositoryPicker(state, {
      type: "applyProjection",
      projection: projection({
        catalog: { outcome: "complete-current", generation: 2, repositories: [] },
        targets: [],
      }),
    });
    expect(state.selected).toBeNull();
    expect(state.query).toBe("demo");
    expect(state.announce).toMatch(/no longer available/i);
  });
});

describe("GitHub scoped state", () => {
  it("does not block projects when GitHub is unauthenticated", () => {
    const state = reduceRepositoryPicker(emptyPickerState(), {
      type: "applyProjection",
      projection: projection({
        account: { state: "unauthenticated" },
        catalog: { outcome: "authentication-required-retained", generation: 0, repositories: [] },
        sources: [
          { id: "projects", available: true },
          { id: "github", available: false, status: "unauthenticated" },
        ],
        targets: [
          {
            source: "project",
            project: { server_id: "srv-a", project: "/repo/demo", server_label: "Alpha" },
          },
        ],
      }),
    });
    const projects = reduceRepositoryPicker(state, {
      type: "setSourceFilter",
      sourceFilter: "projects",
    });
    expect(filteredTargets(projects).projects).toHaveLength(1);
  });

  it("restores GitHub focus and keeps the query after returning from settings", () => {
    let state = reduceRepositoryPicker(openWithCatalog(), { type: "setQuery", query: "demo" });
    state = reduceRepositoryPicker(state, { type: "openSettings" });
    state = reduceRepositoryPicker(state, { type: "returnFromSettings", outcome: "success" });
    expect(state.phase).toBe("target-select");
    expect(state.query).toBe("demo");
    expect(state.activeKey).toBe(GITHUB_ENTRY_KEY);
  });
});

describe("UAC-003 owner/name search validation", () => {
  const rows: RepositoryRow[] = [
    {
      source: "repository",
      key: repositoryKey("10"),
      repositoryId: "10",
      owner: "alice",
      name: "demo",
      label: "alice/demo",
    },
    {
      source: "repository",
      key: repositoryKey("11"),
      repositoryId: "11",
      owner: "alice",
      name: "demo-extra",
      label: "alice/demo-extra",
    },
  ];

  it("matches only stable owner/name identity and detects ambiguity", () => {
    const first = rows[0];
    expect(first).toBeDefined();
    if (!first) return;
    expect(matchesOwnerName(first, "alice/demo")).toBe(true);
    expect(matchesOwnerName(first, "description metadata")).toBe(false);
    expect(
      evaluateSearchValidation(rows, [
        { query: "demo", intendedKey: repositoryKey("10"), ambiguous: true },
      ]),
    ).toBe("invalidated");
  });
});
