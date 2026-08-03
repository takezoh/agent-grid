import { afterEach, describe, expect, it, vi } from "vitest";
import { createGitHubActionDispatcher } from "../src/main/github-action-dispatch.js";
import { githubActionResult, projectionContainsCredential } from "../src/main/repository-lease.js";
import type { AccountProjection, CatalogProjection } from "../src/main/github-host.js";

afterEach(() => {
  vi.doUnmock("electron");
  vi.doUnmock("../src/main/app-entry.js");
  vi.doUnmock("../src/main/github-host.js");
  vi.doUnmock("../src/main/server-health.js");
  vi.doUnmock("../src/main/fleet-session-runtime.js");
  vi.resetModules();
});

describe("Workspace GitHub action IPC", () => {
  it("publishes authentication before a slow catalog refresh completes", async () => {
    let finishRefresh!: () => void;
    const refreshBlocked = new Promise<void>((resolve) => { finishRefresh = resolve; });
    const account = {
      projection: vi.fn(() => ({ state: "authorizing" as const })),
      startDeviceFlow: vi.fn(),
      cancelDeviceFlow: vi.fn(),
      pollDeviceFlow: vi.fn(async () => ({ state: "authenticated" as const, login: "alice" })),
    };
    const catalog = {
      projection: vi.fn(() => ({
        outcome: "empty" as const,
        generation: 0,
        stale: false,
        repositories: [],
      })),
      refresh: vi.fn(async () => {
        await refreshBlocked;
        return catalog.projection();
      }),
    };
    const dispatch = createGitHubActionDispatcher(account, catalog);

    const poll = dispatch("poll");
    const observed = await Promise.race([
      poll.then(() => "settled" as const),
      new Promise<"blocked">((resolve) => setTimeout(() => resolve("blocked"), 20)),
    ]);
    finishRefresh();
    await poll;

    expect(catalog.refresh).toHaveBeenCalledTimes(1);
    expect(observed).toBe("settled");
  });

  it("resolves a rejected start as the current credential-free typed projection", async () => {
    const credentialSentinel = "gho_must_not_cross_ipc";
    const accountProjection: AccountProjection = {
      state: "host_misconfigured",
      failure_code: "host_misconfigured",
    };
    const catalogProjection: CatalogProjection = {
      outcome: "empty",
      generation: 7,
      stale: false,
      repositories: [],
    };
    const handlers = new Map<string, (...args: unknown[]) => unknown>();
    const startDeviceFlow = vi.fn(async () => {
      throw new Error(`status 404 client_id=placeholder ${credentialSentinel}`);
    });
    const account = {
      startDeviceFlow,
      projection: vi.fn(() => accountProjection),
    };
    const catalog = {
      projection: vi.fn(() => catalogProjection),
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
    vi.doMock("../src/main/app-entry.js", () => ({
      startWorkspaceApp: vi.fn(async () => ({
        setDesktopConfig: vi.fn(),
        openSettings: vi.fn(),
      })),
    }));
    vi.doMock("../src/main/github-host.js", () => ({
      Account: vi.fn(function MockAccount() { return account; }),
      Catalog: vi.fn(function MockCatalog() { return catalog; }),
      FileCacheStore: vi.fn(function MockFileCacheStore() {}),
      FileCredentialStore: vi.fn(function MockFileCredentialStore() { return {
        load: () => ({ record: { access_token: "", generation: 0 }, ok: false }),
      }; }),
      projectionContainsCredential: vi.fn(() => false),
      fixedOAuthApp: vi.fn(() => ({
        schema_version: 1,
        client_id: "explicit-placeholder",
        scopes: ["read:user", "repo"],
      })),
    }));
    vi.doMock("../src/main/server-health.js", () => ({
      FleetHealthMonitor: vi.fn(function MockFleetHealthMonitor() { return {
        invalidate: vi.fn(),
        list: vi.fn(),
      }; }),
    }));
    vi.doMock("../src/main/fleet-session-runtime.js", () => ({
      FleetSessionRuntime: vi.fn(function MockFleetSessionRuntime() {}),
    }));

    await import("../src/main/electron-entry.js");
    const dispatch = handlers.get("agent-grid:github-action");
    expect(dispatch).toBeTypeOf("function");
    if (!dispatch) return;

    const settled = await Promise.resolve(dispatch({}, "start")).then(
      (value) => ({ outcome: "resolved" as const, value }),
      (error) => ({ outcome: "rejected" as const, error }),
    );

    if (settled.outcome !== "resolved") throw settled.error;
    const expected = githubActionResult(accountProjection, catalogProjection);
    expect(settled.value).toEqual(expected);
    expect(settled.value).toMatchObject({
      outcome: "failure",
      account: {
        state: "host_misconfigured",
        failure_code: "host_misconfigured",
      },
      catalog: catalogProjection,
    });
    expect(account.projection).toHaveBeenCalled();
    expect(catalog.projection).toHaveBeenCalled();
    expect(projectionContainsCredential(settled.value)).toBe(false);
    expect(JSON.stringify(settled.value)).not.toContain(credentialSentinel);
  });
});
