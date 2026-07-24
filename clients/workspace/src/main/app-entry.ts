/**
 * Production Electron main entry wiring.
 * Kept free of a hard `electron` import so unit tests stay clean;
 * Windows production injects Electron constructors.
 */

import { ControlEndpoint, defaultControlPath } from "./control-endpoint.js";
import { DaemonConfigResolver } from "./daemon-config.js";
import {
  loadOrCreateDesktopConfig,
  resolveConfigDirectory,
  type DesktopConfig,
} from "./desktop-config.js";
import {
  createElectronWindowFactory,
  type ElectronBrowserWindowConstructor,
} from "./electron-window-factory.js";
import { FileStateStore, defaultWorkspaceStatePath } from "./file-state-store.js";
import { IdleQuitController } from "./idle-quit.js";
import { WindowRegistry } from "./window-registry.js";

export interface ElectronAppLike {
  requestSingleInstanceLock(): boolean;
  quit(): void;
  whenReady(): Promise<void>;
  on(event: "window-all-closed", listener: () => void): void;
  on(event: "second-instance", listener: () => void): void;
}

export interface AppEntryOptions {
  app: ElectronAppLike;
  BrowserWindow: ElectronBrowserWindowConstructor;
  preloadPath: string;
  /** Test/packaging override; defaults to dist/ui/index.html. */
  uiEntryUrl?: string;
  args?: readonly string[];
  config?: DesktopConfig;
  controlPath?: string;
  statePath?: string;
  idleMs?: number;
}

export async function startWorkspaceApp(opts: AppEntryOptions): Promise<{
  registry: WindowRegistry;
  endpoint: ControlEndpoint;
  stop: () => Promise<void>;
}> {
  if (!opts.app.requestSingleInstanceLock()) {
    opts.app.quit();
    throw new Error("second instance — handed off to primary");
  }

  await opts.app.whenReady();

  const desktopConfig = opts.config ?? loadOrCreateDesktopConfig(
    resolveConfigDirectory(opts.args ?? process.argv.slice(1)),
  );
  const config = DaemonConfigResolver.fromServers(desktopConfig.servers);
  const store = new FileStateStore(opts.statePath ?? defaultWorkspaceStatePath());
  const factory = createElectronWindowFactory({
    config,
    BrowserWindow: opts.BrowserWindow,
    preloadPath: opts.preloadPath,
    uiEntryUrl: opts.uiEntryUrl ?? new URL("../ui/index.html", import.meta.url).toString(),
    appearance: desktopConfig.appearance,
    workspace: desktopConfig.workspace,
  });
  const registry = new WindowRegistry(factory, store);
  const idle = new IdleQuitController({
    idleMs: opts.idleMs ?? desktopConfig.workspace.idle_quit_seconds * 1000,
    openCount: () => registry.openCount,
    onQuit: () => opts.app.quit(),
  });
  // Track closes via periodic poll is crude; wire openSession callers.
  const endpoint = new ControlEndpoint({
    path: opts.controlPath ?? defaultControlPath(),
    registry,
    onQuit: () => opts.app.quit(),
  });
  // Wrap open path to refresh idle timer: monkey-patch via proxy registry is heavy;
  // instead, endpoint openSession already mutates registry — poll after ops.
  const originalOpen = registry.openSession.bind(registry);
  registry.openSession = async (session) => {
    const w = await originalOpen(session);
    idle.onWindowsChanged();
    return w;
  };
  const originalClose = registry.closeSessionView.bind(registry);
  registry.closeSessionView = (session) => {
    originalClose(session);
    idle.onWindowsChanged();
  };

  opts.app.on("window-all-closed", () => {
    idle.onWindowsChanged();
  });
  opts.app.on("second-instance", () => {
    // Focus any open window.
    const sessions = registry.listSessions();
    if (sessions[0]) void registry.openSession(sessions[0]);
  });

  await endpoint.start();
  idle.onWindowsChanged();

  return {
    registry,
    endpoint,
    stop: async () => {
      idle.dispose();
      await endpoint.stop();
    },
  };
}
