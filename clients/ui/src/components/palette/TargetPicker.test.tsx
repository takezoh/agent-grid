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
  it("defaults to GitHub repositories and switches the single list with accessible tabs", async () => {
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

    const githubTab = await screen.findByRole("tab", { name: "GitHub repositories" });
    const projectsTab = screen.getByRole("tab", { name: "Projects" });
    expect(githubTab.getAttribute("aria-selected")).toBe("true");
    expect(projectsTab.getAttribute("aria-selected")).toBe("false");
    expect(githubTab.tabIndex).toBe(0);
    expect(projectsTab.tabIndex).toBe(-1);
    expect(githubTab.getAttribute("aria-controls")).toBe("target-source-panel");
    expect(projectsTab.getAttribute("aria-controls")).toBe("target-source-panel");
    expect(screen.getByRole("tabpanel").getAttribute("aria-labelledby")).toBe(
      "target-source-tab-github",
    );

    const listbox = screen.getByTestId("target-listbox");
    expect(listbox.getAttribute("role")).toBe("listbox");
    expect(screen.getByTestId("target-option-repository:10")).toBeTruthy();
    expect(screen.queryByTestId("target-option-project:srv-a:/repo/demo")).toBeNull();

    // Status controls are outside the listbox.
    const githubStatus = screen.getByTestId("github-source-status");
    expect(listbox.contains(githubStatus)).toBe(false);
    expect(screen.getByTestId("github-status-label").textContent).toMatch(/GitHub/i);

    const options = listbox.querySelectorAll('[role="option"]');
    expect(options.length).toBe(1);
    for (const opt of options) {
      expect(opt.querySelector("button")).toBeNull();
      expect(opt.getAttribute("data-stable-key")).toBeTruthy();
      expect(opt.querySelector('[data-testid="source-badge"]')?.textContent).toMatch(
        /Project|GitHub/,
      );
      expect(opt.querySelector('[data-testid="stable-identity"]')?.textContent).toBeTruthy();
      expect(opt.querySelector(".target-picker__meta")).toBeNull();
    }

    const combobox = screen.getByTestId("palette-param-project-filter");
    expect(combobox.getAttribute("role")).toBe("combobox");
    expect(combobox.getAttribute("data-source-filter")).toBe("github");
    // aria-activedescendant references a stable option id when active.
    fireEvent.keyDown(combobox, { key: "ArrowDown" });
    await waitFor(() => {
      const active = combobox.getAttribute("aria-activedescendant");
      expect(active).toBeTruthy();
      if (active) expect(document.getElementById(active)).toBeTruthy();
    });

    fireEvent.change(combobox, { target: { value: "demo" } });
    fireEvent.keyDown(githubTab, { key: "ArrowRight" });
    await waitFor(() => {
      expect(projectsTab.getAttribute("aria-selected")).toBe("true");
      expect(screen.getByTestId("target-option-project:srv-a:/repo/demo")).toBeTruthy();
    });
    expect(
      screen
        .getByTestId("target-option-project:srv-a:/repo/demo")
        .querySelector(".target-picker__meta"),
    ).toBeNull();
    expect(document.activeElement).toBe(projectsTab);
    expect(projectsTab.tabIndex).toBe(0);
    expect(githubTab.tabIndex).toBe(-1);
    expect(combobox.getAttribute("value")).toBe("demo");
    expect(screen.queryByTestId("target-option-repository:10")).toBeNull();
    expect(screen.getByTestId("projects-source-status")).toBeTruthy();
    expect(screen.queryByTestId("github-source-status")).toBeNull();
    expect(combobox.getAttribute("data-source-filter")).toBe("projects");

    fireEvent.keyDown(projectsTab, { key: "Home" });
    await waitFor(() => expect(githubTab.getAttribute("aria-selected")).toBe("true"));
    fireEvent.keyDown(githubTab, { key: "End" });
    await waitFor(() => expect(projectsTab.getAttribute("aria-selected")).toBe("true"));
  });

  it("keeps an external project value hidden and Enter selects only the visible repository", async () => {
    const onEnter = vi.fn();
    const onSelectProject = vi.fn();
    const onSelectRepository = vi.fn();
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
        value={JSON.stringify(["srv-a", "/repo/demo"])}
        onSelectProject={onSelectProject}
        onSelectRepository={onSelectRepository}
        onEnter={onEnter}
        onAnnounce={vi.fn()}
        provider={fakeProvider(baseProjection)}
      />,
    );

    const repository = await screen.findByTestId("target-option-repository:10");
    expect(repository).toBeTruthy();
    expect(screen.queryByTestId("target-option-project:srv-a:/repo/demo")).toBeNull();
    const combobox = screen.getByTestId("palette-param-project-filter");
    const active = combobox.getAttribute("aria-activedescendant");
    expect(active).toBeTruthy();
    if (active) expect(document.getElementById(active)).toBeTruthy();

    fireEvent.keyDown(combobox, { key: "Enter" });
    await waitFor(() => expect(onEnter).toHaveBeenCalledTimes(1));
    expect(onSelectRepository).toHaveBeenCalledWith({
      repository_id: "10",
      owner: "alice",
      name: "demo",
    });
    expect(screen.queryByTestId("repository-server-confirm")).toBeNull();
    expect(onSelectProject).not.toHaveBeenCalled();
  });

  it("advances a GitHub repository into the shared New Session flow without a server-confirm branch", async () => {
    const onEnter = vi.fn();
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
        onEnter={onEnter}
        onAnnounce={vi.fn()}
        provider={fakeProvider(baseProjection)}
      />,
    );

    fireEvent.mouseDown(await screen.findByTestId("target-option-repository:10"));

    await waitFor(() => expect(onEnter).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId("repository-server-confirm")).toBeNull();
    expect(screen.queryByTestId("repository-create")).toBeNull();
  });

  it("delegates GitHub authentication to Workspace Settings without rendering inline auth (UAC-005/013)", async () => {
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
    const openGitHubSettings = vi.fn(async () => undefined);
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
        provider={{ ...fakeProvider(unauth), openGitHubSettings }}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("github-auth-entry")).toBeTruthy());
    const filter = screen.getByTestId("palette-param-project-filter") as HTMLInputElement;
    fireEvent.change(filter, { target: { value: "demo" } });
    fireEvent.click(screen.getByTestId("github-auth-entry"));
    await waitFor(() => {
      expect(openGitHubSettings).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByTestId("github-settings-surface")).toBeNull();
    expect(filter.value).toBe("demo");
  });

  it("shows the typed host configuration error opened from New Session", async () => {
    const unsafeReason = "status 404 client_id=Ov23liAgentGridDeviceFlow";
    const unsafeDetail = "access_token=gho_renderer_leak";
    const unsafeCode = "LEAK-CODE";
    const unsafeUri = "https://attacker.invalid/device";
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
    const provider: FleetProvider = {
      ...fakeProvider(unauth),
      githubAction: async () => ({
        outcome: "failure",
        account: {
          state: "host_misconfigured",
          failure_code: "host_misconfigured",
          user_code: unsafeCode,
          verification_uri: unsafeUri,
        },
        catalog: unauth.catalog,
        reason: unsafeReason,
        detail: unsafeDetail,
      }),
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

    await waitFor(() => {
      expect(screen.getByTestId("github-auth-entry")).toBeTruthy();
    });
    fireEvent.click(screen.getByTestId("github-auth-entry"));

    await waitFor(() => {
      expect(screen.getByTestId("github-auth-error").textContent).toBe(
        "GitHub OAuth App is not configured on this host.",
      );
    });
    expect(screen.getByTestId("github-settings-surface").textContent).not.toMatch(
      /client[_-]?id|status 404|access[_-]?token/i,
    );
    expect(screen.queryByTestId("github-device-uri")).toBeNull();
    expect(screen.queryByTestId("github-device-code")).toBeNull();
    const visible = screen.getByTestId("github-settings-surface").textContent ?? "";
    for (const unsafe of [unsafeReason, unsafeDetail, unsafeCode, unsafeUri]) {
      expect(visible).not.toContain(unsafe);
    }
  });
});

/* Repository preparation is submitted by the shared New Session form. TargetPicker
 * owns target discovery/auth only; preparation lifecycle coverage lives at the
 * provider/server/runtime boundaries. */
/*
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

    // Non-null project value reproduces palette param sync (E2E regression):
    // applyProjection refresh must not re-select the project and drop prep.
    const projectValue = JSON.stringify(["srv-a", "/repo/demo"]);
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
        value={projectValue}
        onSelectProject={vi.fn()}
        onEnter={vi.fn()}
        onAnnounce={vi.fn()}
        provider={provider}
      />,
    );

    await waitFor(() => expect(screen.getByTestId("target-option-repository:10")).toBeTruthy());
    fireEvent.mouseDown(screen.getByTestId("target-option-repository:10"));
    expect(screen.queryByTestId("repository-server-confirm")).toBeNull();
    expect(screen.queryByRole("listbox", { name: "Execution Servers" })).toBeNull();
    fireEvent.click(screen.getByTestId("repository-create"));

    await waitFor(() => expect(screen.getByTestId("preparation-panel")).toBeTruthy());
    await waitFor(() => {
      expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).toBe("clone-in-progress");
    });
    expect(screen.queryByTestId("session-available")).toBeNull();
    expect(screen.queryByTestId("target-listbox")).toBeNull();

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
    // Prep view must stay; listbox must not return while clone-ready is active.
    expect(screen.getByTestId("preparation-panel")).toBeTruthy();
    expect(screen.queryByTestId("target-listbox")).toBeNull();

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

  it("leaves confirmed and exposes a server-reported failure instead of trapping the flow", async () => {
    let created = false;
    const provider: FleetProvider = {
      ...fakeProvider(baseProjection),
      repositoryProjection: async () => ({
        ...baseProjection,
        preparations: created
          ? [
              {
                preparation_id: "p1",
                intent_id: "i1",
                server_id: "srv-a",
                repository: { repository_id: "10", owner: "alice", name: "demo" },
                stage: "failed",
                failure_code: "clone_failed",
                retry_origin: "clone",
                revision: 3,
              },
            ]
          : [],
      }),
      createRepositorySession: async () => {
        created = true;
        return {
          preparation_id: "p1",
          intent_id: "i1",
          stage: "confirmed",
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
    fireEvent.click(await screen.findByTestId("repository-create"));
    expect((await screen.findByTestId("prep-stage")).getAttribute("data-stage")).toBe("confirmed");
    await waitFor(
      () => {
        expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).toBe("failed");
        expect(screen.getByTestId("prep-failure").textContent).toBe("Repository clone failed.");
        expect(screen.getByTestId("prep-failure").getAttribute("data-failure-code")).toBe(
          "clone_failed",
        );
        expect(screen.getByTestId("prep-retry")).toBeTruthy();
        expect(screen.getByTestId("prep-exit")).toBeTruthy();
        expect(screen.getByTestId("prep-server").textContent).toBe("Server Alpha");
      },
      { timeout: 4000 },
    );
  });
*/

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
    expect(screen.queryByTestId("repository-server-confirm")).toBeNull();

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
        expect(screen.queryByTestId("repository-server-confirm")).toBeNull();
        expect(screen.queryByTestId("target-option-repository:10")).toBeNull();
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
      const label = screen.getByTestId("github-status-label").textContent ?? "";
      // Do not allow bare "GitHub" alone to satisfy stale/failure coverage.
      expect(label).toMatch(/stale|fail|source failed|error/i);
      expect(label.toLowerCase()).not.toBe("github");
    });
  });

  it("keeps catalog and selection when web projection 404s (HTTP 404 is transient)", async () => {
    // Mirrors webProvider: HTTP 404 throws rather than returning empty projection.
    let fail = false;
    let calls = 0;
    const provider: FleetProvider = {
      ...fakeProvider(baseProjection),
      repositoryProjection: async () => {
        calls += 1;
        if (fail) throw new Error("HTTP 404");
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
    // Select the repository so retention must also preserve selection surface.
    fireEvent.click(screen.getByText("alice/demo"));
    fail = true;
    await waitFor(() => expect(calls).toBeGreaterThan(1), { timeout: 4000 });
    expect(screen.getByText("alice/demo")).toBeTruthy();
    await waitFor(() => {
      const label = screen.getByTestId("github-status-label").textContent ?? "";
      expect(label).toMatch(/stale|fail|source failed|error/i);
    });
    // Must not collapse to unauthenticated empty (would hide catalog entirely).
    expect(screen.queryByText(/sign in|unauthenticated/i)).toBeNull();
  });
});

/*
  it("shows repository and server identity together; back restores repository focus", async () => {
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
        provider={fakeProvider(baseProjection)}
      />,
    );
    await waitFor(() => expect(screen.getByTestId("target-option-repository:10")).toBeTruthy());
    fireEvent.mouseDown(screen.getByTestId("target-option-repository:10"));
    await waitFor(() => expect(screen.getByTestId("repository-server-confirm")).toBeTruthy());
    await waitFor(() => {
      expect(screen.getByTestId("confirm-repository").textContent).toMatch(/alice\/demo/i);
      expect(screen.getByTestId("confirm-server").textContent).toMatch(/Alpha|srv-a/i);
    });
    fireEvent.click(screen.getByTestId("server-select-back"));
    await waitFor(() => {
      // Back returns to target-select with repository still selected / focus restored.
      const combobox = screen.getByTestId("palette-param-project-filter");
      const active = combobox.getAttribute("aria-activedescendant");
      expect(active).toBeTruthy();
      if (active) {
        const el = document.getElementById(active);
        expect(el?.getAttribute("data-source")).toBe("repository");
      }
      expect(document.activeElement).toBe(combobox);
    });
  });
*/

/*
  it("covers prep cancel → retry → exit and unavailable server affordance", async () => {
    const withServers: HostRepositoryProjection = {
      ...baseProjection,
      execution_servers: [
        { serverId: "srv-a", serverLabel: "Alpha", available: true },
        {
          serverId: "srv-b",
          serverLabel: "Bravo",
          available: false,
          reason: "Server offline",
        },
      ],
    };
    const provider: FleetProvider = {
      ...fakeProvider(withServers),
      createRepositorySession: async () => ({
        preparation_id: "p1",
        intent_id: "i1",
        stage: "clone-in-progress",
        revision: 1,
      }),
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

    expect(screen.queryByText("Bravo")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    const bravo = screen.getByRole("button", { name: "Bravo" }) as HTMLButtonElement;
    expect(bravo.disabled).toBe(true);
    expect(screen.getByTestId("repository-server-confirm").textContent).not.toContain(
      "Server offline",
    );

    fireEvent.click(screen.getByTestId("repository-create"));

    await waitFor(() => expect(screen.getByTestId("preparation-panel")).toBeTruthy());
    fireEvent.click(screen.getByTestId("prep-cancel"));
    await waitFor(() => {
      expect(screen.getByTestId("prep-retry")).toBeTruthy();
      expect(screen.getByTestId("prep-exit")).toBeTruthy();
      expect(screen.getByTestId("prep-retry-origin").textContent).toMatch(/clone/i);
    });
    fireEvent.click(screen.getByTestId("prep-retry"));
    await waitFor(() => {
      expect(screen.getByTestId("prep-stage").getAttribute("data-stage")).toBe("clone-in-progress");
      expect(screen.getByTestId("prep-cancel")).toBeTruthy();
    });
    fireEvent.click(screen.getByTestId("prep-cancel"));
    await waitFor(() => expect(screen.getByTestId("prep-exit")).toBeTruthy());
    fireEvent.click(screen.getByTestId("prep-exit"));
    await waitFor(() => expect(screen.queryByTestId("preparation-panel")).toBeNull());
  });
*/
