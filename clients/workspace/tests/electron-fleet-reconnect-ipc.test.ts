import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * Finding 4: agent-grid:session-fleet-reconnect IPC wiring — invalid ID rejection
 * and delegation to exactly the selected server on the live FleetSessionRuntime.
 */

describe("electron-entry session-fleet-reconnect IPC", () => {
  const handlers = new Map<string, (...args: unknown[]) => unknown>();
  let runtimeInstances: Array<{
    start: ReturnType<typeof vi.fn>;
    stop: ReturnType<typeof vi.fn>;
    reconnect: ReturnType<typeof vi.fn>;
    subscribe: ReturnType<typeof vi.fn>;
    subscribeHealth: ReturnType<typeof vi.fn>;
    healthSnapshot: ReturnType<typeof vi.fn>;
  }> = [];

  beforeEach(() => {
    handlers.clear();
    runtimeInstances = [];
    vi.resetModules();
  });

  afterEach(() => {
    vi.doUnmock("electron");
    vi.doUnmock("../src/main/app-entry.js");
    vi.doUnmock("../src/main/github-host.js");
    vi.doUnmock("../src/main/server-health.js");
    vi.doUnmock("../src/main/fleet-session-runtime.js");
    vi.doUnmock("../src/main/desktop-config.js");
    vi.resetModules();
  });

  it("rejects invalid server IDs and delegates valid reconnect to the selected runtime", async () => {
    const baseConfig = {
      configDirectory: "/tmp/agent-grid-test-config",
      servers: [
        {
          id: "srv-a",
          display_name: "A",
          enabled: true,
          default_execution: true,
          url: "http://127.0.0.1:8443",
          auth: { mode: "none" as const },
        },
        {
          id: "srv-b",
          display_name: "B",
          enabled: true,
          default_execution: false,
          url: "http://127.0.0.1:8444",
          auth: { mode: "none" as const },
        },
      ],
      appearance: { theme: "default" as const, density: "comfortable" as const, font_scale: 1 },
      shell: { workspace_executable: "workspace", health_poll_interval_seconds: 5 },
      workspace: { idle_quit_seconds: 0, default_window: { width: 1200, height: 800 } },
      github: { app_id: "1", client_id: "c", scopes: ["read:user"] },
    };

    vi.doMock("electron", () => ({
      BrowserWindow: { fromWebContents: vi.fn(() => null) },
      BaseWindow: class {},
      Menu: { buildFromTemplate: vi.fn(() => ({ popup: vi.fn() })) },
      WebContentsView: class {},
      app: { quit: vi.fn() },
      clipboard: { writeText: vi.fn() },
      ipcMain: {
        handle: vi.fn((channel: string, handler: (...args: unknown[]) => unknown) => {
          handlers.set(channel, handler);
        }),
        on: vi.fn(),
      },
      nativeTheme: { shouldUseDarkColors: true, themeSource: "system" },
      shell: { openExternal: vi.fn(async () => undefined) },
      webContents: { fromId: vi.fn(() => undefined) },
    }));

    let onDesktopConfigLoaded: ((config: unknown) => void) | null = null;
    vi.doMock("../src/main/app-entry.js", () => ({
      startWorkspaceApp: vi.fn(
        async (options: { onDesktopConfigLoaded?: (config: unknown) => void }) => {
          onDesktopConfigLoaded = options.onDesktopConfigLoaded ?? null;
          return {
            setDesktopConfig: vi.fn(),
            openSettings: vi.fn(),
          };
        },
      ),
    }));
    vi.doMock("../src/main/github-host.js", () => ({
      Account: vi.fn(function MockAccount() {
        return { projection: vi.fn(() => ({ state: "unauthenticated" })) };
      }),
      Catalog: vi.fn(function MockCatalog() {
        return {
          projection: vi.fn(() => ({ outcome: "empty", generation: 0, repositories: [] })),
        };
      }),
      FileCacheStore: vi.fn(function MockFileCacheStore() {}),
      FileCredentialStore: vi.fn(function MockFileCredentialStore() {
        return { load: () => ({ record: { access_token: "", generation: 0 }, ok: false }) };
      }),
      projectionContainsCredential: vi.fn(() => false),
      fixedOAuthApp: vi.fn(() => ({
        schema_version: 1,
        client_id: "test",
        scopes: ["read:user", "repo"],
      })),
    }));
    vi.doMock("../src/main/server-health.js", () => ({
      connectionFailureReason: vi.fn(() => "Could not connect to server"),
      legacyFleetHealthFromSnapshot: vi.fn(() => ({ checkedAt: "", servers: [] })),
    }));
    vi.doMock("../src/main/desktop-config.js", async () => {
      const actual = await vi.importActual<typeof import("../src/main/desktop-config.js")>(
        "../src/main/desktop-config.js",
      );
      return {
        ...actual,
        loadEditableDesktopSettings: vi.fn(() => baseConfig),
        saveEditableDesktopSettings: vi.fn((_dir: string, settings: typeof baseConfig) => ({
          ...baseConfig,
          ...settings,
          servers: settings.servers,
        })),
      };
    });
    vi.doMock("../src/main/fleet-session-runtime.js", () => ({
      FleetSessionRuntime: vi.fn(function MockFleetSessionRuntime() {
        const instance = {
          start: vi.fn(),
          stop: vi.fn(),
          reconnect: vi.fn(async (serverId: string) => {
            if (serverId !== "srv-a" && serverId !== "srv-b") {
              throw new Error(`unknown or disabled server '${serverId}'`);
            }
          }),
          subscribe: vi.fn(() => () => undefined),
          subscribeHealth: vi.fn(() => () => undefined),
          healthSnapshot: vi.fn(() => ({
            schemaVersion: 1,
            hostInstanceId: "test",
            revision: 0,
            servers: [],
          })),
        };
        runtimeInstances.push(instance);
        return instance;
      }),
    }));

    await import("../src/main/electron-entry.js");
    await vi.waitFor(() => expect(onDesktopConfigLoaded).toBeTypeOf("function"));
    onDesktopConfigLoaded?.(baseConfig);
    expect(runtimeInstances).toHaveLength(1);

    const reconnectIpc = handlers.get("agent-grid:session-fleet-reconnect");
    expect(reconnectIpc).toBeTypeOf("function");
    if (!reconnectIpc) return;

    // Invalid IDs rejected at the IPC boundary before runtime.reconnect.
    await expect(Promise.resolve(reconnectIpc({}, null))).rejects.toThrow(
      /unknown or disabled server/,
    );
    await expect(Promise.resolve(reconnectIpc({}, ""))).rejects.toThrow(
      /unknown or disabled server/,
    );
    await expect(Promise.resolve(reconnectIpc({}, 42))).rejects.toThrow(
      /unknown or disabled server/,
    );
    expect(runtimeInstances[0]!.reconnect).not.toHaveBeenCalled();

    // Valid selected-server reconnect delegates exactly once to the live runtime.
    await expect(Promise.resolve(reconnectIpc({}, "srv-a"))).resolves.toBeUndefined();
    expect(runtimeInstances[0]!.reconnect).toHaveBeenCalledTimes(1);
    expect(runtimeInstances[0]!.reconnect).toHaveBeenCalledWith("srv-a");
    expect(runtimeInstances[0]!.reconnect).not.toHaveBeenCalledWith("srv-b");

    // Peer selection is independent — still exactly the chosen server.
    await expect(Promise.resolve(reconnectIpc({}, "srv-b"))).resolves.toBeUndefined();
    expect(runtimeInstances[0]!.reconnect).toHaveBeenCalledTimes(2);
    expect(runtimeInstances[0]!.reconnect).toHaveBeenLastCalledWith("srv-b");
  });
});
