import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * Finding 4: replaceFleetRuntime must rebind mounted renderer health
 * subscriptions so config save does not drop delivery.
 */

describe("electron-entry fleet runtime replacement rebind", () => {
  const handlers = new Map<string, (...args: unknown[]) => unknown>();
  let runtimeInstances: Array<{
    start: ReturnType<typeof vi.fn>;
    stop: ReturnType<typeof vi.fn>;
    subscribe: ReturnType<typeof vi.fn>;
    subscribeHealth: ReturnType<typeof vi.fn>;
    healthSnapshot: ReturnType<typeof vi.fn>;
    _publishHealth: (snapshot: unknown) => void;
    _healthListeners: Set<(snapshot: unknown) => void>;
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

  it("resubscribes existing renderer health listeners across runtime replacement", async () => {
    // rejects counterexample: clear health unsubscribes and never reattach
    const sent: Array<{ channel: string; payload: unknown }> = [];
    const destroyedHandlers: Array<() => void> = [];
    const sender = {
      id: 42,
      isDestroyed: () => false,
      send: (channel: string, payload: unknown) => {
        sent.push({ channel, payload });
      },
      once: (event: string, handler: () => void) => {
        if (event === "destroyed") destroyedHandlers.push(handler);
      },
    };

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
      screen: {
        getAllDisplays: vi.fn(() => []),
        getPrimaryDisplay: vi.fn(() => ({ id: 1 })),
        getDisplayMatching: vi.fn(() => ({ id: 1 })),
        dipToScreenPoint: vi.fn((point) => point),
      },
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
        const healthListeners = new Set<(snapshot: unknown) => void>();
        const instance = {
          start: vi.fn(),
          stop: vi.fn(),
          subscribe: vi.fn(() => () => undefined),
          subscribeHealth: vi.fn((listener: (snapshot: unknown) => void) => {
            healthListeners.add(listener);
            listener({
              schemaVersion: 1,
              hostInstanceId: `host-${runtimeInstances.length}`,
              revision: 0,
              servers: [],
            });
            return () => healthListeners.delete(listener);
          }),
          healthSnapshot: vi.fn(() => ({
            schemaVersion: 1,
            hostInstanceId: "test",
            revision: 0,
            servers: [],
          })),
          _publishHealth(snapshot: unknown) {
            for (const listener of healthListeners) listener(snapshot);
          },
          _healthListeners: healthListeners,
        };
        runtimeInstances.push(instance);
        return instance;
      }),
    }));

    await import("../src/main/electron-entry.js");
    await vi.waitFor(() => expect(onDesktopConfigLoaded).toBeTypeOf("function"));
    onDesktopConfigLoaded?.(baseConfig);
    expect(runtimeInstances).toHaveLength(1);

    const subscribeHealth = handlers.get("agent-grid:session-fleet-health-subscribe");
    expect(subscribeHealth).toBeTypeOf("function");
    subscribeHealth?.({ sender });
    expect(runtimeInstances[0]!.subscribeHealth).toHaveBeenCalledTimes(1);
    expect(runtimeInstances[0]!._healthListeners.size).toBe(1);

    const save = handlers.get("agent-grid:desktop-config-save");
    expect(save).toBeTypeOf("function");
    sent.length = 0;
    await save?.(
      {},
      {
        servers: baseConfig.servers,
        appearance: baseConfig.appearance,
        workspace: baseConfig.workspace,
        github: baseConfig.github,
      },
    );

    expect(runtimeInstances).toHaveLength(2);
    expect(runtimeInstances[0]!.stop).toHaveBeenCalledTimes(1);
    // Exactly one subscription on the replacement runtime for this renderer.
    expect(runtimeInstances[1]!.subscribeHealth).toHaveBeenCalledTimes(1);
    expect(runtimeInstances[1]!._healthListeners.size).toBe(1);
    // Old runtime no longer has listeners (unsubscribed before replace).
    expect(runtimeInstances[0]!._healthListeners.size).toBe(0);

    runtimeInstances[1]!._publishHealth({
      schemaVersion: 1,
      hostInstanceId: "host-replaced",
      revision: 3,
      servers: [
        {
          serverId: "srv-a",
          serverLabel: "A",
          enabled: true,
          observation: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
        },
      ],
    });
    expect(
      sent.some(
        (entry) =>
          entry.channel === "agent-grid:session-fleet-health" &&
          (entry.payload as { hostInstanceId?: string }).hostInstanceId === "host-replaced",
      ),
    ).toBe(true);

    // Capture and invoke the real renderer destroyed callback — prove cleanup.
    expect(destroyedHandlers.length).toBeGreaterThan(0);
    const unsubCountBefore = runtimeInstances[1]!._healthListeners.size;
    expect(unsubCountBefore).toBe(1);
    for (const handler of [...destroyedHandlers]) {
      handler();
    }
    expect(runtimeInstances[1]!._healthListeners.size).toBe(0);

    // Re-subscribe after destroy, then replace again — still one subscription.
    destroyedHandlers.length = 0;
    subscribeHealth?.({ sender });
    expect(runtimeInstances[1]!._healthListeners.size).toBe(1);
    await save?.(
      {},
      {
        servers: baseConfig.servers,
        appearance: baseConfig.appearance,
        workspace: baseConfig.workspace,
        github: baseConfig.github,
      },
    );
    expect(runtimeInstances).toHaveLength(3);
    expect(runtimeInstances[2]!.subscribeHealth).toHaveBeenCalledTimes(1);
    expect(runtimeInstances[2]!._healthListeners.size).toBe(1);
  });
});
