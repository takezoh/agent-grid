// UAC coverage for pure repository picker reducer.
// rejects counterexample: source-first topology, hover-only provenance, global blocking,
// metadata ranking, and lost query/filter/focus across scoped recovery.

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
  serverKey,
  stageAnnouncement,
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
      {
        source: "project",
        project: { server_id: "srv-b", project: "/repo/other", server_label: "Beta" },
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
  let s = reduceRepositoryPicker(emptyPickerState(), { type: "open" });
  s = reduceRepositoryPicker(s, {
    type: "applyProjection",
    projection: projection(),
    servers: [
      { serverId: "srv-a", serverLabel: "Alpha", available: true },
      { serverId: "srv-b", serverLabel: "Beta", available: true },
    ],
  });
  return s;
}

describe("UAC-001 / UAC-019 categorized all-source open", () => {
  it("opens with all-source filter and both categories from projection", () => {
    // rejects counterexample: restoring previous source filter or source-first step
    const s = openWithCatalog();
    expect(s.sourceFilter).toBe("all");
    expect(s.projects).toHaveLength(2);
    expect(s.repositories).toHaveLength(2);
    expect(s.projects[0]?.label).toContain("Alpha");
    expect(s.repositories[0]?.label).toBe("alice/demo");
  });
});

describe("UAC-002 row provenance and post-select identity", () => {
  it("keeps repository owner/name through server selection", () => {
    // rejects counterexample: hover-only provenance or basename-only selection
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    expect(s.selected?.source).toBe("repository");
    expect(s.selected && s.selected.source === "repository" && s.selected.owner).toBe("alice");
    expect(s.selected && s.selected.source === "repository" && s.selected.name).toBe("demo");
    expect(s.phase).toBe("server-select");
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-a" });
    expect(s.selectedServerId).toBe("srv-a");
    expect(s.selected && s.selected.source === "repository" && s.selected.label).toBe("alice/demo");
  });
});

describe("UAC-003 owner/name search validation", () => {
  it("finds known owner/name and invalidates on ambiguity failure", () => {
    const corpus: RepositoryRow[] = [
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
    const known = corpus[0];
    expect(known).toBeTruthy();
    if (known) expect(matchesOwnerName(known, "alice/demo")).toBe(true);
    expect(
      evaluateSearchValidation(corpus, [{ query: "alice/demo", intendedKey: repositoryKey("10") }]),
    ).toBe("validated");
    // Ambiguous cue "demo" hits both — invalidates owner/name-only assumption.
    expect(
      evaluateSearchValidation(corpus, [
        { query: "demo", intendedKey: repositoryKey("10"), ambiguous: true },
      ]),
    ).toBe("invalidated");
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, {
      type: "validateSearch",
      knownFound: true,
      ambiguityFailure: true,
    });
    expect(s.searchValidation).toBe("invalidated");
  });
});

describe("UAC-004 GitHub failure does not block projects", () => {
  it("keeps projects when GitHub is unauthenticated", () => {
    // rejects counterexample: global picker block on GitHub auth failure
    let s = reduceRepositoryPicker(emptyPickerState(), { type: "open" });
    s = reduceRepositoryPicker(s, {
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
          {
            source: "project",
            project: { server_id: "srv-b", project: "/repo/other", server_label: "Beta" },
          },
        ],
      }),
    });
    expect(s.projects).toHaveLength(2);
    s = reduceRepositoryPicker(s, { type: "select", key: projectKey("srv-a", "/repo/demo") });
    expect(s.selected?.source).toBe("project");
    expect(s.selectedServerId).toBe("srv-a");
  });
});

describe("UAC-005 / UAC-013 Settings auth return preserves context", () => {
  it("returns focus to GitHub entry and keeps query/filter after success or cancel", () => {
    // rejects counterexample: inline OAuth, query reset, focus elsewhere
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "setQuery", query: "demo" });
    s = reduceRepositoryPicker(s, { type: "openSettings" });
    expect(s.phase).toBe("settings-auth");
    expect(s.query).toBe("demo");
    s = reduceRepositoryPicker(s, { type: "returnFromSettings", outcome: "success" });
    expect(s.phase).toBe("target-select");
    expect(s.query).toBe("demo");
    expect(s.sourceFilter).toBe("all");
    expect(s.activeKey).toBe(GITHUB_ENTRY_KEY);

    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, { type: "openSettings" });
    s = reduceRepositoryPicker(s, { type: "returnFromSettings", outcome: "cancel" });
    expect(s.selected).toBeNull();
    expect(s.activeKey).toBe(GITHUB_ENTRY_KEY);
    expect(s.query).toBe("demo");
  });
});

describe("UAC-006 / UAC-007 refresh failure retains catalog", () => {
  it("keeps stale repositories and query during source-failed refresh", () => {
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "setQuery", query: "demo" });
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, {
      type: "applyProjection",
      projection: projection({
        catalog: {
          outcome: "source-failed-retained",
          generation: 1,
          stale: true,
          failure_code: "rate_limited",
          repositories: [{ repository_id: "10", owner: "alice", name: "demo" }],
        },
        sources: [
          { id: "projects", available: true },
          { id: "github", available: true, status: "source-failed-retained" },
        ],
      }),
    });
    expect(s.query).toBe("demo");
    expect(s.repositories.some((r) => r.repositoryId === "10")).toBe(true);
    expect(s.catalog.stale).toBe(true);
    expect(s.selected?.key).toBe(repositoryKey("10"));
  });
});

describe("UAC-008 repository-first server association and back", () => {
  it("selects repository before server and restores focus on back", () => {
    // rejects counterexample: server-first or lost selection on back
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "setQuery", query: "demo" });
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    expect(s.phase).toBe("server-select");
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-b" });
    expect(s.selectedServerId).toBe("srv-b");
    s = reduceRepositoryPicker(s, { type: "back" });
    expect(s.phase).toBe("target-select");
    expect(s.query).toBe("demo");
    expect(s.selected?.key).toBe(repositoryKey("10"));
    expect(s.activeKey).toBe(repositoryKey("10"));
    expect(s.selectedServerId).toBeNull();
  });
});

describe("UAC-009..011 / UAC-015..016 preparation stages", () => {
  it("announces clone then devcontainer and retries from retained origin", () => {
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-a" });
    s = reduceRepositoryPicker(s, {
      type: "startPreparation",
      preparation: {
        preparation_id: "p1",
        intent_id: "i1",
        server_id: "srv-a",
        repository: { repository_id: "10", owner: "alice", name: "demo" },
        stage: "clone-in-progress",
        revision: 1,
      },
    });
    expect(s.phase).toBe("preparing");
    expect(s.announce).toContain("Cloning");
    s = reduceRepositoryPicker(s, {
      type: "preparationUpdate",
      preparation: {
        preparation_id: "p1",
        intent_id: "i1",
        server_id: "srv-a",
        repository: { repository_id: "10", owner: "alice", name: "demo" },
        stage: "devcontainer-in-progress",
        revision: 2,
        environment_disposition: "create",
      },
    });
    expect(s.announce).toContain("devcontainer");
    s = reduceRepositoryPicker(s, {
      type: "preparationUpdate",
      preparation: {
        preparation_id: "p1",
        intent_id: "i1",
        server_id: "srv-a",
        repository: { repository_id: "10", owner: "alice", name: "demo" },
        stage: "failed",
        revision: 3,
        failure_code: "devcontainer_failed",
        retry_origin: "devcontainer",
      },
    });
    expect(s.phase).toBe("retry");
    expect(s.preparation?.retry_origin).toBe("devcontainer");
    s = reduceRepositoryPicker(s, { type: "retryPreparation" });
    expect(s.preparation?.stage).toBe("devcontainer-in-progress");
    expect(s.announce).toContain("devcontainer");

    // Cancel during clone → retry from clone
    s = reduceRepositoryPicker(s, {
      type: "startPreparation",
      preparation: {
        preparation_id: "p2",
        intent_id: "i2",
        server_id: "srv-a",
        repository: { repository_id: "10", owner: "alice", name: "demo" },
        stage: "clone-in-progress",
        revision: 1,
      },
    });
    s = reduceRepositoryPicker(s, { type: "cancelPreparation" });
    expect(s.phase).toBe("retry");
    expect(s.preparation?.retry_origin).toBe("clone");
    s = reduceRepositoryPicker(s, { type: "retryPreparation" });
    expect(s.preparation?.stage).toBe("clone-in-progress");
  });

  it("reuse disposition is distinct from create", () => {
    const msg = stageAnnouncement({
      preparation_id: "p",
      intent_id: "i",
      server_id: "srv-a",
      repository: { repository_id: "10", owner: "alice", name: "demo" },
      stage: "devcontainer-in-progress",
      revision: 1,
      environment_disposition: "reuse",
    });
    expect(msg).toContain("Reusing");
  });
});

describe("UAC-014 / UAC-017 / UAC-018 scoped invalidation", () => {
  it("preserves in-flight preparation selection when catalog transiently omits the repository", () => {
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-a" });
    s = reduceRepositoryPicker(s, {
      type: "startPreparation",
      preparation: {
        preparation_id: "p1",
        intent_id: "i1",
        server_id: "srv-a",
        repository: { repository_id: "10", owner: "alice", name: "demo" },
        stage: "clone-in-progress",
        revision: 1,
      },
    });
    expect(s.phase).toBe("preparing");
    s = reduceRepositoryPicker(s, {
      type: "applyProjection",
      projection: projection({
        catalog: { outcome: "complete-current", generation: 2, repositories: [] },
        targets: [
          {
            source: "project",
            project: { server_id: "srv-a", project: "/repo/demo", server_label: "Alpha" },
          },
        ],
        preparations: [
          {
            preparation_id: "p1",
            intent_id: "i1",
            server_id: "srv-a",
            repository: { repository_id: "10", owner: "alice", name: "demo" },
            stage: "clone-ready",
            revision: 2,
          },
        ],
      }),
    });
    expect(s.phase).toBe("preparing");
    expect(s.selected?.key).toBe(repositoryKey("10"));
    expect(s.preparation?.stage).toBe("clone-ready");
    expect(s.preparation?.stage).not.toBe("available");
  });

  it("ignores select/selectServer during preparing so palette value-sync cannot drop prep", () => {
    // rejects counterexample: project param value re-select during poll → target-select
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-a" });
    s = reduceRepositoryPicker(s, {
      type: "startPreparation",
      preparation: {
        preparation_id: "p1",
        intent_id: "i1",
        server_id: "srv-a",
        repository: { repository_id: "10", owner: "alice", name: "demo" },
        stage: "clone-in-progress",
        revision: 1,
      },
    });
    const project = projectKey("srv-a", "/repo/demo");
    s = reduceRepositoryPicker(s, { type: "select", key: project });
    expect(s.phase).toBe("preparing");
    expect(s.selected?.key).toBe(repositoryKey("10"));
    expect(s.preparation?.stage).toBe("clone-in-progress");
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-b" });
    expect(s.phase).toBe("preparing");
    expect(s.selectedServerId).toBe("srv-a");
    s = reduceRepositoryPicker(s, {
      type: "applyProjection",
      projection: projection({
        preparations: [
          {
            preparation_id: "p1",
            intent_id: "i1",
            server_id: "srv-a",
            repository: { repository_id: "10", owner: "alice", name: "demo" },
            stage: "clone-ready",
            revision: 2,
          },
        ],
      }),
    });
    // Spurious project select interleaved with prep poll must still leave clone-ready.
    s = reduceRepositoryPicker(s, { type: "select", key: project });
    expect(s.phase).toBe("preparing");
    expect(s.preparation?.stage).toBe("clone-ready");
    expect(s.selected?.key).toBe(repositoryKey("10"));
  });

  it("clears invalid repository selection and unavailable server without losing query", () => {
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "setQuery", query: "demo" });
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, {
      type: "applyProjection",
      projection: projection({
        targets: [
          {
            source: "project",
            project: { server_id: "srv-a", project: "/repo/demo", server_label: "Alpha" },
          },
        ],
        catalog: { outcome: "complete-current", generation: 2, repositories: [] },
      }),
    });
    expect(s.selected).toBeNull();
    expect(s.query).toBe("demo");
    expect(s.announce).toMatch(/no longer available/i);

    s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-a" });
    s = reduceRepositoryPicker(s, {
      type: "applyProjection",
      projection: projection(),
      servers: [
        { serverId: "srv-a", serverLabel: "Alpha", available: false, reason: "offline" },
        { serverId: "srv-b", serverLabel: "Beta", available: true },
      ],
    });
    expect(s.selectedServerId).toBeNull();
    expect(s.selected?.key).toBe(repositoryKey("10"));
    expect(s.activeKey).toBe(serverKey("srv-b"));
  });
});

describe("UAC-012 independent second session identity", () => {
  it("does not block a second repository selection for the same owner/name", () => {
    // Session identity is server-owned; picker must not duplicate-block.
    let s = openWithCatalog();
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-a" });
    s = reduceRepositoryPicker(s, {
      type: "startPreparation",
      preparation: {
        preparation_id: "p1",
        intent_id: "i1",
        server_id: "srv-a",
        repository: { repository_id: "10", owner: "alice", name: "demo" },
        stage: "available",
        revision: 4,
        session_id: "sess-1",
      },
    });
    // Re-open target selection for a second independent create of the same repo.
    s = reduceRepositoryPicker(s, { type: "open" });
    s = reduceRepositoryPicker(s, {
      type: "applyProjection",
      projection: projection(),
      servers: [
        { serverId: "srv-a", serverLabel: "Alpha", available: true },
        { serverId: "srv-b", serverLabel: "Beta", available: true },
      ],
    });
    s = reduceRepositoryPicker(s, { type: "select", key: repositoryKey("10") });
    s = reduceRepositoryPicker(s, { type: "selectServer", serverId: "srv-b" });
    expect(s.selectedServerId).toBe("srv-b");
    expect(s.selected && s.selected.source === "repository" && s.selected.repositoryId).toBe("10");
  });
});

describe("filter owner/name corpus only", () => {
  it("does not match extended metadata fields", () => {
    const row: RepositoryRow = {
      source: "repository",
      key: repositoryKey("1"),
      repositoryId: "1",
      owner: "org",
      name: "svc",
      label: "org/svc",
    };
    expect(matchesOwnerName(row, "microservice description")).toBe(false);
    expect(matchesOwnerName(row, "org")).toBe(true);
    const s = openWithCatalog();
    const next = reduceRepositoryPicker(s, { type: "setQuery", query: "extra" });
    const f = filteredTargets(next);
    expect(f.repositories.map((r) => r.name)).toEqual(["demo-extra"]);
  });
});
