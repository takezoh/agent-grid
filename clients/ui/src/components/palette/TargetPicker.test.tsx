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
