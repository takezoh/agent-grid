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
            { serverId: "srv-a", displayName: "A", enabled: true },
            { serverId: "srv-b", displayName: "B", enabled: true },
          ],
        });
      }
      if (url.includes("/api/capabilities")) {
        return Response.json({ protocolVersion: "protocol-2" });
      }
      if (url.includes("srv-a")) return Response.json({ projects: ["/a"] });
      return new Response("not ready", { status: 503, headers: { "Retry-After": "1" } });
    }) as typeof fetch;

    const outcomes = await makeFleetProvider(fetchImpl).listConfigs();

    expect(outcomes).toEqual([
      {
        serverId: "srv-a",
        serverLabel: "A",
        state: "connected",
        config: { projects: ["/a"] },
      },
      {
        serverId: "srv-b",
        serverLabel: "B",
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
          servers: [{ serverId: "srv-a", displayName: "A", enabled: true }],
        });
      }
      return Response.json({ protocolVersion: "old" });
    }) as typeof fetch;

    await expect(makeFleetProvider(fetchImpl).listConfigs()).resolves.toEqual([
      {
        serverId: "srv-a",
        serverLabel: "A",
        state: "incompatible_protocol",
        reason: "incompatible protocol: client protocol-2, server old",
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
    window.agentGridWorkspace = {
      hostedModeInfo: {
        hosted: true,
        serverId: "srv-a",
        baseUrl: "https://gateway.invalid",
        token: "renderer-token",
      },
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue(projected),
        create: vi.fn(),
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
      },
    };

    const outcomes = await makeFleetProvider().listConfigs();

    expect(outcomes).toEqual(projected);
    expect(JSON.stringify(outcomes)).not.toContain("renderer-token");

    const repo = await makeFleetProvider().repositoryProjection();
    expect(repo.targets.some((t) => t.source === "project")).toBe(true);
    expect(repo.targets.some((t) => t.source === "repository")).toBe(true);
    expect(JSON.stringify(repo)).not.toContain("renderer-token");
    expect(JSON.stringify(repo)).not.toMatch(/gho_|ghu_|github_pat/i);
  });

  it("Web repositoryProjection treats 404 as transient failure (not empty wipe)", async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/host/repository") {
        return new Response("missing", { status: 404 });
      }
      return new Response("nope", { status: 500 });
    }) as typeof fetch;
    await expect(makeFleetProvider(fetchImpl).repositoryProjection()).rejects.toThrow(
      /HTTP 404/,
    );
  });

  it("does not synthesize a server from the browser origin", () => {
    window.history.replaceState({}, "", "/");
    expect(makeFleetProvider(vi.fn() as typeof fetch).initialServerId()).toBeNull();
  });
});
