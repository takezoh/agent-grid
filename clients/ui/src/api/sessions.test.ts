import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiHttpError, ApiWireFormatError } from "./sessions";
import { makeSessionsApi } from "./sessions";

function json(value: unknown, status = 200): Response {
  return Response.json(value, { status });
}

beforeEach(() => {
  window.history.replaceState({}, "", "/?server=srv-current");
});

afterEach(() => {
  window.agentGridWorkspace = undefined;
  vi.restoreAllMocks();
});

describe("server-qualified SessionsApi", () => {
  it("aggregates healthy configs while preserving typed partial failure", async () => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue([
          {
            serverId: "srv-a",
            serverLabel: "A",
            state: "connected",
            config: {
              base_command_profiles: [],
              projects: [
                {
                  path: "/a",
                  isGit: false,
                  isSandboxed: false,
                  command_profiles: [],
                },
              ],
              push_commands: ["/clear"],
            },
          },
          {
            serverId: "srv-b",
            serverLabel: "B",
            state: "unavailable",
            reason: "offline",
          },
        ]),
        create: vi.fn(),
      },
    };

    const config = await makeSessionsApi(vi.fn()).getSessionConfig();

    expect(config.projects).toEqual([
      {
        path: "/a",
        isGit: false,
        isSandboxed: false,
        serverId: "srv-a",
        serverLabel: "A",
      },
    ]);
    expect(config.serverOutcomes).toEqual([
      { serverId: "srv-a", serverLabel: "A", state: "connected" },
      { serverId: "srv-b", serverLabel: "B", state: "unavailable", reason: "offline" },
    ]);
    expect(config.pushCommands).toEqual(["/clear"]);
  });

  it("creates through the provider with an explicit server ID", async () => {
    const create = vi.fn().mockResolvedValue({ id: "session-1" });
    window.agentGridWorkspace = {
      sessionFleet: { listConfigs: vi.fn(), create },
    };

    const result = await makeSessionsApi(vi.fn()).createSession({
      server_id: "srv-b",
      project: "/repo",
      version: 2,
      profile_id: "shell",
      profile_revision: "r1",
    });

    expect(result).toEqual({ id: '["srv-b","session-1"]', serverId: "srv-b" });
    expect(create).toHaveBeenCalledWith("srv-b", {
      project: "/repo",
      version: 2,
      profile_id: "shell",
      profile_revision: "r1",
    });
  });

  it("does not use the initial view server when create payload omits server_id", async () => {
    await expect(
      makeSessionsApi(vi.fn()).createSession({
        project: "/repo",
        version: 2,
        profile_id: "shell",
        profile_revision: "r1",
      }),
    ).rejects.toThrow("requires explicit server_id");
  });

  it("fails closed when no server is selected", async () => {
    window.history.replaceState({}, "", "/");
    await expect(
      makeSessionsApi(vi.fn()).createSession({
        project: "/repo",
        version: 2,
        profile_id: "shell",
        profile_revision: "r1",
      }),
    ).rejects.toThrow("requires explicit server_id");
  });

  it("qualifies session-scoped REST routes", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    const api = makeSessionsApi(fetchImpl);

    await api.deleteSession("a/b");

    expect(fetchImpl).toHaveBeenCalledWith(
      "/host/servers/srv-current/api/sessions/a%2Fb",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("preserves typed HTTP errors on qualified routes", async () => {
    const api = makeSessionsApi(
      vi.fn().mockResolvedValue(new Response("conflict", { status: 409 })),
    );

    try {
      await api.pushCommand("session-1", "/clear");
      throw new Error("expected rejection");
    } catch (error) {
      expect((error as ApiHttpError).status).toBe(409);
      expect((error as ApiHttpError).body).toBe("conflict");
    }
  });

  it("preserves typed wire errors from provider create", async () => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn(),
        create: vi.fn().mockResolvedValue({ unexpected: true }),
      },
    };

    try {
      await makeSessionsApi(vi.fn()).createSession({
        server_id: "srv-a",
        project: "/repo",
        version: 2,
        profile_id: "shell",
        profile_revision: "r1",
      });
      throw new Error("expected rejection");
    } catch (error) {
      expect((error as ApiWireFormatError).wireFormat).toBe(true);
      expect((error as ApiWireFormatError).url).toBe("FleetProvider.create");
    }
  });

  it("normalizes typed projects, profiles, and every parameter variant", async () => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue([
          {
            serverId: "srv-a",
            serverLabel: "A",
            state: "connected",
            config: {
              base_command_profiles: [],
              projects: [
                {
                  path: "/typed",
                  isGit: true,
                  isSandboxed: true,
                  command_profiles: [
                    {
                      id: "profile",
                      revision: "r1",
                      parameters: [
                        {
                          name: "choice",
                          label: "Choice",
                          type: "enum",
                          required: true,
                          default: "a",
                          values: ["a", "b"],
                        },
                        {
                          name: "flag",
                          label: "Flag",
                          type: "boolean",
                          required: false,
                          default: true,
                        },
                        {
                          name: "text",
                          label: "Text",
                          type: "string",
                          required: false,
                          default: "x",
                          pattern: "^x",
                        },
                        {
                          name: "count",
                          label: "Count",
                          type: "number",
                          required: false,
                          default: 2,
                          min: 1,
                          max: 3,
                        },
                      ],
                    },
                  ],
                },
              ],
              push_commands: [],
            },
          },
        ]),
        create: vi.fn(),
      },
    };

    const config = await makeSessionsApi(vi.fn()).getSessionConfig();

    expect(config.projects[0]).toMatchObject({
      path: "/typed",
      isGit: true,
      isSandboxed: true,
      commandProfiles: [
        {
          id: "profile",
          label: "profile",
          revision: "r1",
          parameters: [
            expect.objectContaining({ name: "choice", values: ["a", "b"] }),
            expect.objectContaining({ name: "flag", default: true }),
            expect.objectContaining({ name: "text", pattern: "^x" }),
            expect.objectContaining({ name: "count", min: 1, max: 3 }),
          ],
        },
      ],
    });
  });

  it("rejects malformed fields instead of applying a compatibility fallback", async () => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue([
          {
            serverId: "srv-a",
            serverLabel: "A",
            state: "connected",
            config: {
              projects: [7, { nope: true }, null],
              push_commands: "wrong",
            },
          },
        ]),
        create: vi.fn(),
      },
    };

    await expect(makeSessionsApi(vi.fn()).getSessionConfig()).rejects.toThrow(
      "invalid exact-version session-config",
    );
  });

  it("rejects an exact-version server that omits the repository command catalog", async () => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue([
          {
            serverId: "srv-a",
            serverLabel: "A",
            state: "connected",
            config: {
              projects: [],
              push_commands: [],
            },
          },
        ]),
        create: vi.fn(),
      },
    };

    await expect(makeSessionsApi(vi.fn()).getSessionConfig()).rejects.toThrow(
      "invalid exact-version session-config: base_command_profiles must be an array",
    );
  });

  it("keeps a valid parameterless repository command profile", async () => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue([
          {
            serverId: "srv-a",
            serverLabel: "A",
            state: "connected",
            config: {
              base_command_profiles: [
                { id: "shell", label: "Shell", revision: "revision-shell", parameters: [] },
              ],
              projects: [],
              push_commands: [],
            },
          },
        ]),
        create: vi.fn(),
      },
    };

    await expect(makeSessionsApi(vi.fn()).getSessionConfig()).resolves.toMatchObject({
      serverOutcomes: [
        {
          serverId: "srv-a",
          commandProfiles: [
            { id: "shell", label: "Shell", revision: "revision-shell", parameters: [] },
          ],
        },
      ],
    });
  });

  it.each([
    { name: "model", label: "Model", type: "enum", required: true },
    { name: "flag", label: "Flag", type: "boolean", required: false, default: "yes" },
    { name: "text", label: "Text", type: "string", required: false, pattern: 7 },
    { name: "count", label: "Count", type: "number", required: false, min: 10, max: 1 },
  ])("rejects a malformed exact-version $type parameter", async (parameter) => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue([
          {
            serverId: "srv-a",
            serverLabel: "A",
            state: "connected",
            config: {
              base_command_profiles: [
                {
                  id: "bad",
                  label: "Bad",
                  revision: "revision-bad",
                  parameters: [parameter],
                },
              ],
              projects: [],
              push_commands: [],
            },
          },
        ]),
        create: vi.fn(),
      },
    };

    await expect(makeSessionsApi(vi.fn()).getSessionConfig()).rejects.toThrow(
      "invalid exact-version session-config: command profiles[0].parameters[0]",
    );
  });

  it("deduplicates push commands across healthy servers", async () => {
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs: vi.fn().mockResolvedValue(
          ["a", "b"].map((serverId) => ({
            serverId,
            serverLabel: serverId,
            state: "connected",
            config: {
              base_command_profiles: [],
              projects: [],
              push_commands: ["/clear", `/${serverId}`],
            },
          })),
        ),
        create: vi.fn(),
      },
    };

    expect((await makeSessionsApi(vi.fn()).getSessionConfig()).pushCommands).toEqual([
      "/clear",
      "/a",
      "/b",
    ]);
  });

  it("posts push commands to the qualified current server", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(null, { status: 200 }));

    await makeSessionsApi(fetchImpl).pushCommand("s/1", "/clear");

    expect(fetchImpl).toHaveBeenCalledWith(
      "/host/servers/srv-current/api/sessions/s%2F1/push",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ command: "/clear" }),
      }),
    );
  });

  it("reads messages and supplies safe defaults for an incomplete response", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(json({ messages: null }));

    const response = await makeSessionsApi(fetchImpl).getSessionMessages("s/1");

    expect(response).toEqual({ session_id: "s/1", summary: undefined, messages: [] });
    expect(fetchImpl).toHaveBeenCalledWith(
      "/host/servers/srv-current/api/sessions/s%2F1/messages",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("reads message summaries and marks a specific message read", async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce(
        json({
          session_id: "server-session",
          summary: { unread_count: 1, pending_delivery_count: 0 },
          messages: [{ id: "m1" }],
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const api = makeSessionsApi(fetchImpl);

    const response = await api.getSessionMessages("s1");
    await api.markSessionMessagesRead("s1", "m1");

    expect(response.session_id).toBe("server-session");
    expect(response.messages).toEqual([{ id: "m1" }]);
    expect(fetchImpl).toHaveBeenLastCalledWith(
      "/host/servers/srv-current/api/sessions/s1/messages/read",
      expect.objectContaining({ body: JSON.stringify({ last_read_message_id: "m1" }) }),
    );
  });

  it("marks all messages read when no message ID is supplied", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    await makeSessionsApi(fetchImpl).markSessionMessagesRead("s1");
    expect(fetchImpl).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ body: "{}" }),
    );
  });

  it("wraps network failures while preserving the cause", async () => {
    const cause = new TypeError("offline");
    const api = makeSessionsApi(vi.fn().mockRejectedValue(cause));
    try {
      await api.deleteSession("s1");
      throw new Error("expected rejection");
    } catch (error) {
      expect((error as Error).message).toBe("network");
      expect((error as Error).cause).toBe(cause);
    }
  });

  it("does not wrap an AbortError", async () => {
    const aborted = new DOMException("cancelled", "AbortError");
    const api = makeSessionsApi(vi.fn().mockRejectedValue(aborted));
    await expect(api.deleteSession("s1")).rejects.toBe(aborted);
  });

  it("records an unreadable HTTP error body", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const response = {
      ok: false,
      status: 500,
      text: vi.fn().mockRejectedValue(new Error("body failed")),
    } as unknown as Response;
    try {
      await makeSessionsApi(vi.fn().mockResolvedValue(response)).deleteSession("s1");
      throw new Error("expected rejection");
    } catch (error) {
      expect((error as ApiHttpError).body).toContain("unreadable");
      expect(warn).toHaveBeenCalled();
    }
  });
});
