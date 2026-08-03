import { afterEach, describe, expect, it, vi } from "vitest";
import { makeFleetProvider } from "./provider";

afterEach(() => {
  window.agentGridWorkspace = undefined;
});

describe("FleetProvider host parity", () => {
  it("Web transport preserves all configured servers and qualifies every request", async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/host/config") {
        return Response.json({
          schema_version: 2,
          servers: [
            { serverId: "srv-a", displayName: "A", enabled: true, defaultExecution: false },
            { serverId: "srv-b", displayName: "B", enabled: true, defaultExecution: true },
          ],
        });
      }
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-3" });
      }
      if (url.includes("srv-a")) return Response.json({ projects: ["/a"] });
      return new Response("not ready", { status: 503, headers: { "Retry-After": "1" } });
    }) as typeof fetch;

    const outcomes = await makeFleetProvider(fetchImpl).listConfigs();

    expect(outcomes).toEqual([
      {
        serverId: "srv-a",
        serverLabel: "A",
        isDefaultExecution: false,
        state: "connected",
        config: { projects: ["/a"] },
      },
      {
        serverId: "srv-b",
        serverLabel: "B",
        isDefaultExecution: true,
        state: "unavailable",
        reason: "HTTP 503",
      },
    ]);
    expect(fetchImpl).toHaveBeenCalledWith("/host/servers/srv-a/api/session-config");
    expect(fetchImpl).toHaveBeenCalledWith("/host/servers/srv-b/api/session-config");
  });

  it("preserves an exact protocol mismatch as the server-list cause", async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/config") {
        return Response.json({
          servers: [{ serverId: "srv-a", displayName: "A", enabled: true, defaultExecution: true }],
        });
      }
      return Response.json({ protocolVersion: "old" });
    }) as typeof fetch;

    await expect(makeFleetProvider(fetchImpl).listConfigs()).resolves.toEqual([
      {
        serverId: "srv-a",
        serverLabel: "A",
        isDefaultExecution: true,
        state: "incompatible_protocol",
        reason: "incompatible protocol: client protocol-3, server old",
      },
    ]);
    expect(fetchImpl).not.toHaveBeenCalledWith("/host/servers/srv-a/api/session-config");
  });

  it("Workspace transport exposes the same contract without bearer material", async () => {
    const projected = [
      {
        serverId: "srv-a",
        serverLabel: "A",
        state: "connected" as const,
        config: { projects: [{ path: "/a" }] },
      },
    ];
    const openSettings = vi.fn().mockResolvedValue(undefined);
    const createRepositorySession = vi.fn().mockResolvedValue({
      preparation_id: "prep-1",
      intent_id: "intent-1",
      stage: "confirmed",
      revision: 1,
    });
    const request = vi.fn().mockResolvedValue({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        preparation_id: "prep-1",
        intent_id: "intent-1",
        server_id: "srv-a",
        repository: { repository_id: "1", owner: "alice", name: "demo" },
        stage: "failed",
        failure_code: "clone_failed",
        retry_origin: "clone",
        revision: 3,
      }),
    });
    window.agentGridWorkspace = {
      hostedModeInfo: {
        hosted: true,
        serverId: "srv-a",
        baseUrl: "https://gateway.invalid",
        token: "renderer-token",
      },
      openSettings,
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue(projected),
        create: vi.fn(),
        createRepositorySession,
        request,
      },
      github: {
        action: vi.fn().mockResolvedValue({
          outcome: "success",
          account: { state: "authenticated", login: "alice" },
          catalog: {
            outcome: "complete-current",
            generation: 1,
            repositories: [{ repository_id: "1", owner: "alice", name: "demo" }],
          },
        }),
        openAuthorization: vi.fn(),
      },
    };

    const provider = makeFleetProvider();
    const outcomes = await provider.listConfigs();

    expect(outcomes).toEqual(projected);
    expect(JSON.stringify(outcomes)).not.toContain("renderer-token");

    const repo = await provider.repositoryProjection();
    expect(repo.targets.some((t) => t.source === "project")).toBe(true);
    expect(repo.targets.some((t) => t.source === "repository")).toBe(true);
    expect(JSON.stringify(repo)).not.toContain("renderer-token");
    expect(JSON.stringify(repo)).not.toMatch(/gho_|ghu_|github_pat/i);
    await provider.openGitHubSettings?.();
    expect(openSettings).toHaveBeenCalledWith("github");
    await provider.createRepositorySession("srv-a", {
      repository_id: "1",
      owner: "alice",
      name: "demo",
      version: 2,
      profile_id: "shell",
      profile_revision: "revision-shell",
    });
    expect(createRepositorySession).toHaveBeenCalledWith("srv-a", {
      repository_id: "1",
      owner: "alice",
      name: "demo",
      version: 2,
      profile_id: "shell",
      profile_revision: "revision-shell",
    });
    const updated = await provider.repositoryProjection();
    expect(request).toHaveBeenCalledWith("srv-a", "/api/repository-preparations/prep-1", {
      method: "GET",
    });
    expect(updated.preparations).toMatchObject([
      { preparation_id: "prep-1", stage: "failed", failure_code: "clone_failed", revision: 3 },
    ]);
  });

  it("Web repositoryProjection treats 404 as transient failure (not empty wipe)", async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/repository") {
        return new Response("missing", { status: 404 });
      }
      return new Response("nope", { status: 500 });
    }) as typeof fetch;
    await expect(makeFleetProvider(fetchImpl).repositoryProjection()).rejects.toThrow(/HTTP 404/);
  });

  it("does not synthesize a server from the browser origin", () => {
    window.history.replaceState({}, "", "/");
    expect(makeFleetProvider(vi.fn() as typeof fetch).initialServerId()).toBeNull();
  });
});
