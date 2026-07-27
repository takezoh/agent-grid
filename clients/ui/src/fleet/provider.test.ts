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

  it("Workspace transport exposes the same contract without bearer material", async () => {
    const projected = [
      {
        serverId: "srv-a",
        serverLabel: "A",
        state: "connected" as const,
        config: { projects: ["/a"] },
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
    };

    const outcomes = await makeFleetProvider().listConfigs();

    expect(outcomes).toEqual(projected);
    expect(JSON.stringify(outcomes)).not.toContain("renderer-token");
  });

  it("does not synthesize a server from the browser origin", () => {
    window.history.replaceState({}, "", "/");
    expect(makeFleetProvider(vi.fn() as typeof fetch).currentServerId()).toBeNull();
  });
});
