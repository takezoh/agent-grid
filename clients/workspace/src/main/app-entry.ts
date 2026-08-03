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
import type { SettingsSection } from "./settings-page.js";
import {
  createElectronWindowFactory,
  type ElectronBaseWindowConstructor,
  type ElectronFactoryOptions,
  type ElectronWebContentsViewConstructor,
} from "./electron-window-factory.js";
import { FileStateStore, defaultWorkspaceStatePath } from "./file-state-store.js";
import { IdleQuitController } from "./idle-quit.js";
import { WindowRegistry } from "./window-registry.js";

export const BACKGROUND_ARGUMENT = "--background";

export function shouldOpenHome(args: readonly string[]): boolean {
  return !args.includes(BACKGROUND_ARGUMENT);
}

export interface ElectronAppLike {
  requestSingleInstanceLock(): boolean;
  quit(): void;
  whenReady(): Promise<void>;
  on(event: "window-all-closed", listener: () => void): void;
  on(
    event: "second-instance",
    listener: (event?: unknown, argv?: string[]) => void,
  ): void;
}

export interface AppEntryOptions {
  app: ElectronAppLike;
  BaseWindow: ElectronBaseWindowConstructor;
  WebContentsView: ElectronWebContentsViewConstructor;
  preloadPath: string;
  titleBarPreloadPath: string;
  /** Test/packaging override; defaults to dist/ui/index.html. */
  uiEntryUrl?: string;
  args?: readonly string[];
  config?: DesktopConfig;
  controlPath?: string;
  statePath?: string;
  idleMs?: number;
  systemPrefersDark?: ElectronFactoryOptions["systemPrefersDark"];
  registerRendererInfo?: ElectronFactoryOptions["registerRendererInfo"];
  configureServerTransport?: ElectronFactoryOptions["configureServerTransport"];
  registerWindowChrome?: ElectronFactoryOptions["registerWindowChrome"];
  onDesktopConfigLoaded?: (config: DesktopConfig) => void;
}

export async function startWorkspaceApp(opts: AppEntryOptions): Promise<{
  registry: WindowRegistry;
  endpoint: ControlEndpoint;
  desktopConfig: DesktopConfig;
  setDesktopConfig: (config: DesktopConfig) => void;
  openSettings: (section?: SettingsSection) => void;
  stop: () => Promise<void>;
}> {
  if (!opts.app.requestSingleInstanceLock()) {
    opts.app.quit();
    throw new Error("second instance — handed off to primary");
  }

  await opts.app.whenReady();

  let desktopConfig = opts.config ?? loadOrCreateDesktopConfig(
    resolveConfigDirectory(opts.args ?? process.argv.slice(1)),
  );
  opts.onDesktopConfigLoaded?.(desktopConfig);
  let config = DaemonConfigResolver.fromServers(desktopConfig.servers);
  const store = new FileStateStore(opts.statePath ?? defaultWorkspaceStatePath());
  const factory = createElectronWindowFactory({
    config: {
      resolve: (serverId) => config.resolve(serverId),
    } as DaemonConfigResolver,
    BaseWindow: opts.BaseWindow,
    WebContentsView: opts.WebContentsView,
    preloadPath: opts.preloadPath,
    titleBarPreloadPath: opts.titleBarPreloadPath,
    uiEntryUrl: opts.uiEntryUrl ?? new URL("../ui/index.html", import.meta.url).toString(),
    appearance: desktopConfig.appearance,
    workspace: desktopConfig.workspace,
    getAppearance: () => desktopConfig.appearance,
    getWorkspace: () => desktopConfig.workspace,
    systemPrefersDark: opts.systemPrefersDark,
    registerRendererInfo: opts.registerRendererInfo,
    configureServerTransport: opts.configureServerTransport,
    registerWindowChrome: opts.registerWindowChrome,
  });
  const registry = new WindowRegistry(factory, store);
  let defaultServer = desktopConfig.servers.find((server) => server.enabled);
  let home: ReturnType<typeof factory.createHome> | null = null;
  let settings: ReturnType<typeof factory.createSettings> | null = null;
  const openHome = () => {
    if (!defaultServer) return;
    if (home && !home.isDestroyed()) {
      home.focus();
      return;
    }
    home = factory.createHome(defaultServer.id);
  };
  const openSettings = (section: SettingsSection = "connections") => {
    if (settings && !settings.isDestroyed()) {
      settings.showSection(section);
      settings.focus();
      return;
    }
    settings = factory.createSettings(section);
    idle.onWindowsChanged();
  };
  if (shouldOpenHome(opts.args ?? process.argv.slice(1))) {
    openHome();
  }
  const idle = new IdleQuitController({
    idleMs: opts.idleMs ?? desktopConfig.workspace.idle_quit_seconds * 1000,
    openCount: () =>
      registry.openCount +
      (home && !home.isDestroyed() ? 1 : 0) +
      (settings && !settings.isDestroyed() ? 1 : 0),
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
  opts.app.on("second-instance", (_event, argv) => {
    if (shouldOpenHome(argv ?? [])) {
      openHome();
      return;
    }
    // Focus any open window.
    const sessions = registry.listSessions();
    if (sessions[0]) void registry.openSession(sessions[0]);
  });

  await endpoint.start();
  idle.onWindowsChanged();

  return {
    registry,
    endpoint,
    desktopConfig,
    setDesktopConfig: (next) => {
      desktopConfig = next;
      config = DaemonConfigResolver.fromServers(next.servers);
      defaultServer = next.servers.find((server) => server.enabled);
      idle.setIdleMs(next.workspace.idle_quit_seconds * 1000);
    },
    openSettings,
    stop: async () => {
      idle.dispose();
      await endpoint.stop();
    },
  };
}
