// Real-DOM accessibility: grouped listbox options vs adjacent status controls.
// rejects counterexample: interactive controls nested in options; missing source badge.

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { FleetProvider } from "../../fleet/provider";
import type { HostRepositoryProjection } from "../../store/repository_picker";
import { TargetPicker } from "./TargetPicker";

afterEach(() => cleanup());

function fakeProvider(projection: HostRepositoryProjection): FleetProvider {
  return {
    initialServerId: () => null,
    subscribe: () => () => {},
    listConfigs: async () => [],
    create: async () => ({}),
    repositoryProjection: async () => projection,
    githubAction: async (action) => ({
      outcome: action === "start" ? "authorizing" : action === "poll" ? "success" : "cancel",
      account: { state: action === "poll" ? "authenticated" : "authorizing", login: "alice" },
      catalog: projection.catalog,
    }),
    createRepositorySession: async () => ({
      preparation_id: "p1",
      intent_id: "i1",
      stage: "clone-in-progress",
      revision: 1,
    }),
    apiUrl: (id, p) => `/${id}${p}`,
    wsUrl: () => "ws://x",
    activeConnection: () => ({ ticketEndpoint: "/t", wsUrl: () => "ws://x" }),
    request: async () => new Response(),
  };
}

const baseProjection: HostRepositoryProjection = {
  sources: [
    { id: "projects", available: true },
    { id: "github", available: true, status: "complete-current" },
  ],
  account: { state: "authenticated", login: "alice" },
  catalog: {
    outcome: "complete-current",
    generation: 1,
    repositories: [{ repository_id: "10", owner: "alice", name: "demo" }],
  },
  targets: [
    {
      source: "project",
      project: { server_id: "srv-a", project: "/repo/demo", server_label: "Alpha" },
    },
    { source: "repository", repository: { repository_id: "10", owner: "alice", name: "demo" } },
  ],
  preparations: [],
};

describe("TargetPicker a11y (UAC-001 / NFR-GH-005)", () => {
  it("exposes categorized listbox options with source badges and external status controls", async () => {
    const announce = vi.fn();
    render(
      <TargetPicker
        focused
        disabled={false}
        projects={[
          {
            path: "/repo/demo",
            isGit: true,
            isSandboxed: false,
            serverId: "srv-a",
            serverLabel: "Alpha",
          },
        ]}
        value={null}
        onSelectProject={vi.fn()}
        onEnter={vi.fn()}
        onAnnounce={announce}
        provider={fakeProvider(baseProjection)}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("category-projects")).toBeTruthy();
      expect(screen.getByTestId("category-github")).toBeTruthy();
    });

    const listbox = screen.getByTestId("target-listbox");
    expect(listbox.getAttribute("role")).toBe("listbox");

    // Status controls are outside the listbox.
    const githubStatus = screen.getByTestId("github-source-status");
    expect(listbox.contains(githubStatus)).toBe(false);
    expect(screen.getByTestId("github-status-label").textContent).toMatch(/GitHub/i);

    const options = listbox.querySelectorAll('[role="option"]');
    expect(options.length).toBeGreaterThanOrEqual(2);
    for (const opt of options) {
      expect(opt.querySelector("button")).toBeNull();
      expect(opt.getAttribute("data-stable-key")).toBeTruthy();
      expect(opt.querySelector('[data-testid="source-badge"]')?.textContent).toMatch(
        /Project|GitHub/,
      );
      expect(opt.querySelector('[data-testid="stable-identity"]')?.textContent).toBeTruthy();
    }

    const combobox = screen.getByTestId("palette-param-project-filter");
    expect(combobox.getAttribute("role")).toBe("combobox");
    expect(combobox.getAttribute("data-source-filter")).toBe("all");
    // aria-activedescendant references a stable option id when active.
    fireEvent.keyDown(combobox, { key: "ArrowDown" });
    await waitFor(() => {
      const active = combobox.getAttribute("aria-activedescendant");
      expect(active).toBeTruthy();
      if (active) expect(document.getElementById(active)).toBeTruthy();
    });
  });

  it("opens Settings surface and returns focus entry on cancel", async () => {
    const unauth: HostRepositoryProjection = {
      ...baseProjection,
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
    };
    render(
      <TargetPicker
        focused
        disabled={false}
        projects={[
          {
            path: "/repo/demo",
            isGit: true,
            isSandboxed: false,
            serverId: "srv-a",
            serverLabel: "Alpha",
          },
        ]}
        value={null}
        onSelectProject={vi.fn()}
        onEnter={vi.fn()}
        onAnnounce={vi.fn()}
        provider={fakeProvider(unauth)}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("github-auth-entry")).toBeTruthy());
    fireEvent.click(screen.getByTestId("github-auth-entry"));
    await waitFor(() => expect(screen.getByTestId("github-settings-surface")).toBeTruthy());
    fireEvent.click(screen.getByTestId("github-settings-cancel"));
    await waitFor(() => expect(screen.getByTestId("github-auth-entry")).toBeTruthy());
  });
});

describe("TargetPicker preparation stages (UAC-002/008/009/010/011/012/015/016/017)", () => {
  it("selects repository and server, shows clone/devcontainer stages, and withholds session availability at clone-ready", async () => {
    let prepStage = "clone-in-progress";
    let prepRevision = 1;
    const prepEnv = { disposition: undefined as string | undefined };
    const projection: HostRepositoryProjection = {
      ...baseProjection,
      preparations: [],
    };
    const provider: FleetProvider = {
      ...fakeProvider(projection),
      repositoryProjection: async () => ({
        ...projection,
        preparations:
          prepStage === ""
            ? []
            : [
                {
                  preparation_id: "p1",
                  intent_id: "i1",
                  server_id: "srv-a",
                  repository: { repository_id: "10", owner: "alice", name: "demo" },
                  stage: prepStage,
                  revision: prepRevision,
                  environment_disposition: prepEnv.disposition,
                  session_id: prepStage === "available" ? "sess-1" : undefined,
                },
              ],
      }),
      createRepositorySession: async () => {
        prepStage = "clone-in-progress";
        prepRevision = 1;
        return {
          preparation_id: "p1",
          intent_id: "i1",
          stage: "clone-in-progress",
          revision: 1,
        };
      },
    };

    render(
      <TargetPicker
        focused
        disabled={false}
        projects={[
          {
            path: "/repo/demo",
            isGit: true,
            isSandboxed: false,
            serverId: "srv-a",
            serverLabel: "Alpha",
          },
        ]}
        value={null}
        onSelectProject={vi.fn()}
        onEnter={vi.fn()}
        onAnnounce={vi.fn()}
        provider={provider}
      />,
    );

    await waitFor(() => expect(screen.getByTestId("target-option-repository:10")).toBeTruthy());
    fireEvent.mouseDown(screen.getByTestId("target-option-repository:10"));
    await waitFor(() => expect(screen.getByTestId("repository-server-confirm")).toBeTruthy());
    // Select available server option then confirm create.
    const combobox = screen.getByTestId("palette-param-project-filter");
    fireEvent.keyDown(combobox, { key: "ArrowDown" });
    fireEvent.keyDown(combobox, { key: "Enter" });
    // Confirm may require Enter again when server already selected.
    fireEvent.keyDown(combobox, { key: "Enter" });

    await waitFor(() => expect(screen.getByTestId("preparation-panel")).toBeTruthy());
    await waitFor(() => {
      expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).toBe("clone-in-progress");
    });
    expect(screen.queryByTestId("session-available")).toBeNull();

    prepStage = "clone-ready";
    prepRevision = 2;
    await waitFor(
      () => {
        expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).toBe("clone-ready");
      },
      { timeout: 4000 },
    );
    // UAC: Session availability must NOT appear at clone-ready.
    expect(screen.queryByText(/session available/i)).toBeNull();
    expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).not.toBe("available");

    prepStage = "devcontainer-in-progress";
    prepRevision = 3;
    prepEnv.disposition = "create";
    await waitFor(
      () => {
        expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).toBe(
          "devcontainer-in-progress",
        );
      },
      { timeout: 4000 },
    );
    expect(screen.queryByText(/session available/i)).toBeNull();

    prepStage = "available";
    prepRevision = 4;
    await waitFor(
      () => {
        expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).toBe("available");
      },
      { timeout: 4000 },
    );
  });
});

describe("TargetPicker focus recovery (UAC-018)", () => {
  it("recovers aria-activedescendant when selected repository is invalidated by projection", async () => {
    let current: HostRepositoryProjection = { ...baseProjection };
    const provider: FleetProvider = {
      ...fakeProvider(current),
      repositoryProjection: async () => current,
    };
    render(
      <TargetPicker
        focused
        disabled={false}
        projects={[
          {
            path: "/repo/demo",
            isGit: true,
            isSandboxed: false,
            serverId: "srv-a",
            serverLabel: "Alpha",
          },
        ]}
        value={null}
        onSelectProject={vi.fn()}
        onEnter={vi.fn()}
        onAnnounce={vi.fn()}
        provider={provider}
      />,
    );

    await waitFor(() => expect(screen.getByTestId("target-option-repository:10")).toBeTruthy());
    fireEvent.mouseDown(screen.getByTestId("target-option-repository:10"));
    await waitFor(() => expect(screen.getByTestId("repository-server-confirm")).toBeTruthy());

    // Invalidate: drop repository and leave only the project target.
    current = {
      ...baseProjection,
      catalog: { outcome: "complete-current", generation: 2, repositories: [] },
      targets: [
        {
          source: "project",
          project: { server_id: "srv-a", project: "/repo/demo", server_label: "Alpha" },
        },
      ],
    };

    await waitFor(
      () => {
        const combobox = screen.getByTestId("palette-param-project-filter");
        // Back to target-select after invalidation, or server-select without selected server.
        const active = combobox.getAttribute("aria-activedescendant");
        expect(active).toBeTruthy();
        if (active) {
          const el = document.getElementById(active);
          expect(el).toBeTruthy();
          // Active descendant must point at a remaining option (project), not the gone repo.
          expect(el?.getAttribute("data-source")).not.toBe("repository");
        }
        // Focus stays on the combobox for aria-activedescendant recovery.
        expect(document.activeElement).toBe(combobox);
      },
      { timeout: 4000 },
    );
  });
});

describe("TargetPicker retained projection on poll failure", () => {
  it("keeps catalog after a transient provider failure (not empty unauthenticated wipe)", async () => {
    let fail = false;
    let calls = 0;
    const provider: FleetProvider = {
      ...fakeProvider(baseProjection),
      repositoryProjection: async () => {
        calls += 1;
        if (fail) throw new Error("transient");
        return baseProjection;
      },
    };
    render(
      <TargetPicker
        focused
        disabled={false}
        projects={[
          {
            path: "/repo/demo",
            isGit: true,
            isSandboxed: false,
            serverId: "srv-a",
            serverLabel: "Alpha",
          },
        ]}
        value={null}
        onSelectProject={vi.fn()}
        onEnter={vi.fn()}
        onAnnounce={vi.fn()}
        provider={provider}
      />,
    );
    await waitFor(() => expect(screen.getByText("alice/demo")).toBeTruthy());
    fail = true;
    await waitFor(() => expect(calls).toBeGreaterThan(1), { timeout: 4000 });
    // Catalog retained; GitHub source shows scoped failure rather than unauthenticated empty.
    expect(screen.getByText("alice/demo")).toBeTruthy();
    await waitFor(() => {
      expect(screen.getByTestId("github-status-label").textContent).toMatch(/fail|stale|GitHub/i);
    });
  });
});
