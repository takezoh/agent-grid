import {
  BrowserWindow,
  BaseWindow,
  Menu,
  WebContentsView,
  app,
  clipboard,
  ipcMain,
  nativeTheme,
  shell,
  webContents,
  type MenuItemConstructorOptions,
  type WebContents,
} from "electron";
import { fileURLToPath } from "node:url";
import { randomUUID } from "node:crypto";
import { startWorkspaceApp } from "./app-entry.js";
import {
  loadEditableDesktopSettings,
  saveEditableDesktopSettings,
  type DesktopConfig,
  type EditableDesktopSettings,
  type GitHubAppConfig,
} from "./desktop-config.js";
import type {
  ElectronBaseWindowConstructor,
  ElectronWebContentsViewConstructor,
} from "./electron-window-factory.js";
import {
  connectionFailureReason,
  legacyFleetHealthFromSnapshot,
} from "./server-health.js";
import { FleetSessionRuntime } from "./fleet-session-runtime.js";
import {
  Account, Catalog, FileCacheStore, FileCredentialStore, fixedOAuthApp, type HttpDoer,
} from "./github-host.js";
import { createGitHubActionDispatcher } from "./github-action-dispatch.js";
import { handoffGitHubAuthorization } from "./github-browser.js";
import { GitHubDeviceFlowController } from "./github-device-flow.js";
import {
  PROTOCOL_VERSION, PROTOCOL_VERSION_HEADER, protocolMismatchReason,
} from "./protocol-version.js";
import {
  authorityFromStore,
  requestRepositoryLease,
  type GitHubAction,
} from "./repository-lease.js";
import { parseRepositoryCreatePayload } from "./repository-create.js";
import type { SettingsSection } from "./settings-page.js";
import {
  isTitleBarMenuId, themeSourceFor, titleBarOverlayFor, type TitleBarMenuId,
} from "./window-chrome.js";

const preloadPath = fileURLToPath(new URL("../preload/electron-preload.cjs", import.meta.url));
const titleBarPreloadPath = fileURLToPath(
  new URL("../preload/titlebar-preload.cjs", import.meta.url),
);
const rendererInfo = new Map<number, import("./daemon-config.js").HostedModeInfo>();
let desktopConfig: DesktopConfig | null = null;
let updateDesktopConfig: ((config: DesktopConfig) => void) | null = null;
let openSettingsWindow: ((section?: SettingsSection) => void) | null = null;
let fleetRuntime: FleetSessionRuntime | null = null;
/** Session fleet subscription: renderer id → { sender, unsubscribe }. */
const fleetSessionByRenderer = new Map<
  number,
  { sender: WebContents; unsubscribe: () => void }
>();
/** Health subscription: renderer id → { sender, unsubscribe }. */
const fleetHealthByRenderer = new Map<
  number,
  { sender: WebContents; unsubscribe: () => void }
>();
const activeChannels = new Map<string, { owner: number; socket: WebSocket }>();

const nodeFetchDoer: HttpDoer = {
  async do(req) {
    const res = await fetch(req.url, { method: req.method, headers: req.headers, body: req.body });
    const headers: Record<string, string> = {};
    res.headers.forEach((v, k) => { headers[k] = v; });
    return { status: res.status, headers, body: await res.text() };
  },
};
const githubCredentialStore = new FileCredentialStore();
let githubAccount = new Account(
  nodeFetchDoer,
  { now: () => new Date() },
  githubCredentialStore,
  fixedOAuthApp(),
);
let githubCatalog = new Catalog(
  nodeFetchDoer,
  { now: () => new Date() },
  new FileCacheStore(),
  githubAccount,
);

function githubStatusAccountProjection() {
  const { record, ok } = githubCredentialStore.load();
  let account = githubAccount.projection();
  if (account.state === "unauthenticated" && ok && record.access_token) {
    account = { state: "authenticated", login: record.login || undefined };
  }
  return account;
}

let dispatchGitHubAction = createGitHubActionDispatcher(
  githubAccount,
  githubCatalog,
  githubStatusAccountProjection,
);
let githubDeviceFlow = new GitHubDeviceFlowController(dispatchGitHubAction);

function replaceGitHubRuntime(config: GitHubAppConfig): void {
  githubDeviceFlow.stop();
  githubAccount = new Account(
    nodeFetchDoer,
    { now: () => new Date() },
    githubCredentialStore,
    { schema_version: 1, client_id: config.client_id, scopes: [...config.scopes] },
  );
  githubCatalog = new Catalog(
    nodeFetchDoer,
    { now: () => new Date() },
    new FileCacheStore(),
    githubAccount,
  );
  dispatchGitHubAction = createGitHubActionDispatcher(
    githubAccount,
    githubCatalog,
    githubStatusAccountProjection,
  );
  githubDeviceFlow = new GitHubDeviceFlowController(dispatchGitHubAction);
}

function githubConfigChanged(current: GitHubAppConfig, next: GitHubAppConfig): boolean {
  return current.app_id !== next.app_id ||
    current.client_id !== next.client_id ||
    JSON.stringify(current.scopes) !== JSON.stringify(next.scopes);
}

function attachFleetSession(sender: WebContents): void {
  if (!fleetRuntime || sender.isDestroyed()) return;
  const previous = fleetSessionByRenderer.get(sender.id);
  previous?.unsubscribe();
  const unsubscribe = fleetRuntime.subscribe((snapshot) => {
    if (!sender.isDestroyed()) sender.send("agent-grid:session-fleet-snapshot", snapshot);
  });
  fleetSessionByRenderer.set(sender.id, { sender, unsubscribe });
  sender.once("destroyed", () => {
    const current = fleetSessionByRenderer.get(sender.id);
    if (current?.unsubscribe === unsubscribe) {
      unsubscribe();
      fleetSessionByRenderer.delete(sender.id);
    }
  });
}

function attachFleetHealth(sender: WebContents): void {
  if (!fleetRuntime || sender.isDestroyed()) return;
  const previous = fleetHealthByRenderer.get(sender.id);
  previous?.unsubscribe();
  const unsubscribe = fleetRuntime.subscribeHealth((snapshot) => {
    if (!sender.isDestroyed()) sender.send("agent-grid:session-fleet-health", snapshot);
  });
  fleetHealthByRenderer.set(sender.id, { sender, unsubscribe });
  sender.once("destroyed", () => {
    const current = fleetHealthByRenderer.get(sender.id);
    if (current?.unsubscribe === unsubscribe) {
      unsubscribe();
      fleetHealthByRenderer.delete(sender.id);
    }
  });
}

/**
 * Replace the host fleet runtime and rebind every mounted renderer subscription
 * so config save/replacement does not drop health delivery.
 */
function replaceFleetRuntime(config: DesktopConfig): void {
  const sessionSenders = [...fleetSessionByRenderer.values()].map((entry) => entry.sender);
  const healthSenders = [...fleetHealthByRenderer.values()].map((entry) => entry.sender);
  for (const entry of fleetSessionByRenderer.values()) entry.unsubscribe();
  fleetSessionByRenderer.clear();
  for (const entry of fleetHealthByRenderer.values()) entry.unsubscribe();
  fleetHealthByRenderer.clear();
  fleetRuntime?.stop();
  fleetRuntime = new FleetSessionRuntime(config.servers);
  fleetRuntime.start();
  for (const sender of sessionSenders) attachFleetSession(sender);
  for (const sender of healthSenders) attachFleetHealth(sender);
}

ipcMain.on("agent-grid:renderer-bootstrap", (event) => {
  event.returnValue = rendererInfo.get(event.sender.id) ?? null;
});

ipcMain.on("agent-grid:window-close", (event) => {
  BrowserWindow.fromWebContents(event.sender)?.close();
});

ipcMain.on("agent-grid:window-minimize", (event) => {
  BrowserWindow.fromWebContents(event.sender)?.minimize();
});

ipcMain.on("agent-grid:window-maximize", (event) => {
  const win = BrowserWindow.fromWebContents(event.sender);
  if (!win) return;
  if (win.isMaximized()) {
    win.unmaximize();
    return;
  }
  win.maximize();
});

ipcMain.handle("agent-grid:desktop-config-load", () => {
  if (!desktopConfig) throw new Error("desktop config is not ready");
  return loadEditableDesktopSettings(desktopConfig.configDirectory);
});

ipcMain.handle("agent-grid:desktop-config-save", (_event, settings: EditableDesktopSettings) => {
  if (!desktopConfig) throw new Error("desktop config is not ready");
  const previousGitHub = desktopConfig.github;
  const next = saveEditableDesktopSettings(desktopConfig.configDirectory, settings);
  desktopConfig = next;
  if (githubConfigChanged(previousGitHub, next.github)) replaceGitHubRuntime(next.github);
  replaceFleetRuntime(next);
  updateDesktopConfig?.(next);
  return next;
});

ipcMain.handle("agent-grid:server-health-list", () => {
  if (!fleetRuntime) throw new Error("fleet runtime is not ready");
  return legacyFleetHealthFromSnapshot(fleetRuntime.healthSnapshot());
});

ipcMain.handle("agent-grid:open-settings", (_event, section: unknown) => {
  if (section !== "github" && section !== "connections") {
    throw new Error("unsupported settings section");
  }
  openSettingsWindow?.(section);
});

// Credential-free GitHub Settings actions only.
ipcMain.handle("agent-grid:github-action", async (_event, action: GitHubAction) => {
  const result = await githubDeviceFlow.action(action);
  if (action === "start" && result.account.state === "authorizing") {
    const opened = await handoffGitHubAuthorization(
      result.account,
      (code) => clipboard.writeText(code),
      (url) => shell.openExternal(url),
    );
    if (!opened) {
      return { ...result, reason: "browser_handoff_failed" };
    }
  }
  return result;
});

ipcMain.handle("agent-grid:github-open-authorization", async () => {
  const account = githubAccount.projection();
  const opened = await handoffGitHubAuthorization(
    account,
    (code) => clipboard.writeText(code),
    (url) => shell.openExternal(url),
  );
  if (!opened) throw new Error("GitHub authorization page could not be opened");
});

ipcMain.handle("agent-grid:session-fleet-configs", async () => {
  if (!desktopConfig) throw new Error("desktop config is not ready");
  return Promise.all(
    desktopConfig.servers
      .filter((server) => server.enabled)
      .map(async (server) => {
        try {
          const capabilitiesResponse = await fleetFetch(
            server,
            "/api/capabilities",
            { method: "GET" },
            false,
          );
          const capabilities = (await capabilitiesResponse.json()) as {
            protocolVersion?: unknown;
          };
          if (capabilities.protocolVersion !== PROTOCOL_VERSION) {
            return {
              serverId: server.id,
              serverLabel: server.display_name,
              isDefaultExecution: server.default_execution,
              state: "incompatible_protocol",
              reason: protocolMismatchReason(capabilities.protocolVersion),
            };
          }
          const response = await fleetFetch(server, "/api/session-config", { method: "GET" });
          return {
            serverId: server.id,
            serverLabel: server.display_name,
            isDefaultExecution: server.default_execution,
            state: "connected",
            config: await response.json(),
          };
        } catch (error) {
          return {
            serverId: server.id,
            serverLabel: server.display_name,
            isDefaultExecution: server.default_execution,
            state: "unavailable",
            reason: connectionFailureReason(error),
          };
        }
      }),
  );
});

ipcMain.handle("agent-grid:session-fleet-subscribe", (event) => {
  if (!fleetRuntime) throw new Error("fleet runtime is not ready");
  attachFleetSession(event.sender);
});

ipcMain.handle("agent-grid:session-fleet-health-subscribe", (event) => {
  if (!fleetRuntime) throw new Error("fleet runtime is not ready");
  attachFleetHealth(event.sender);
});

ipcMain.handle("agent-grid:session-fleet-config-path", () => {
  if (!desktopConfig) throw new Error("desktop config is not ready");
  return `${desktopConfig.configDirectory.replace(/\\/g, "/")}/servers.json`;
});

ipcMain.handle("agent-grid:session-fleet-reconnect", async (_event, serverId: unknown) => {
  if (!fleetRuntime) throw new Error("fleet runtime is not ready");
  if (typeof serverId !== "string" || !serverId) {
    throw new Error("unknown or disabled server");
  }
  await fleetRuntime.reconnect(serverId);
});

ipcMain.handle(
  "agent-grid:session-fleet-create",
  async (_event, serverId: string, payload: unknown) => {
    if (!desktopConfig) throw new Error("desktop config is not ready");
    const server = desktopConfig.servers.find(
      (candidate) => candidate.enabled && candidate.id === serverId,
    );
    if (!server) throw new Error(`unknown or disabled server '${serverId}'`);
    const response = await fleetFetch(server, "/api/sessions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const result = (await response.json()) as Record<string, unknown>;
    const id = typeof result.id === "string" ? result.id : "";
    const confirmed =
      id !== "" &&
      (await fleetRuntime?.waitFor({ serverId, sessionId: id }, 10_000)) === true;
    return {
      ...result,
      fleet_status: confirmed ? "confirmed" : "created_unconfirmed",
    };
  },
);

ipcMain.handle(
  "agent-grid:session-fleet-repository-create",
  async (_event, serverId: string, payload: unknown) => {
    if (!desktopConfig) throw new Error("desktop config is not ready");
    const server = desktopConfig.servers.find(
      (candidate) => candidate.enabled && candidate.id === serverId,
    );
    if (!server) throw new Error(`unknown or disabled server '${serverId}'`);
    const repository = parseRepositoryCreatePayload(payload);
    const intentId =
      typeof repository.intent_id === "string" && repository.intent_id
        ? repository.intent_id
        : randomUUID();
    const preparationId = randomUUID();
    const capabilitiesResponse = await fleetFetch(
      server,
      "/api/capabilities",
      { method: "GET" },
      false,
    );
    const capabilities = (await capabilitiesResponse.json()) as { protocolVersion?: unknown };
    const lease = await requestRepositoryLease({
      server,
      observedProtocol:
        typeof capabilities.protocolVersion === "string" ? capabilities.protocolVersion : "",
      request: {
        repository_id: repository.repository_id,
        owner: repository.owner,
        name: repository.name,
        server_id: serverId,
      },
      auth: authorityFromStore(githubCredentialStore),
    });
    if (!lease.issue) {
      throw new Error(lease.code ?? "repository_authority_unavailable");
    }
    const response = await fleetFetch(server, "/api/repository-sessions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        intent_id: intentId,
        preparation_id: preparationId,
        server_id: serverId,
        repository_id: repository.repository_id,
        owner: repository.owner,
        name: repository.name,
        version: 2,
        profile_id: repository.profile_id,
        profile_revision: repository.profile_revision,
        ...(repository.parameters && typeof repository.parameters === "object"
          ? { parameters: repository.parameters }
          : {}),
        ...(typeof repository.cols === "number" ? { cols: repository.cols } : {}),
        ...(typeof repository.rows === "number" ? { rows: repository.rows } : {}),
        lease_id: lease.issue.lease_id,
        fingerprint: `${serverId}:${repository.repository_id}:${repository.profile_id}:${repository.profile_revision}:${intentId}`,
      }),
    });
    return response.json();
  },
);

ipcMain.handle(
  "agent-grid:session-fleet-request",
  async (
    _event,
    serverId: string,
    pathname: string,
    init: { method?: string; headers?: Record<string, string>; body?: string },
  ) => {
    if (!desktopConfig) throw new Error("desktop config is not ready");
    if (!pathname.startsWith("/api/") || pathname.includes("://")) {
      throw new Error("invalid routed API path");
    }
    const method = init.method ?? "GET";
    if (!["GET", "POST", "PUT", "DELETE"].includes(method)) {
      throw new Error("unsupported routed API method");
    }
    const server = desktopConfig.servers.find(
      (candidate) => candidate.enabled && candidate.id === serverId,
    );
    if (!server) throw new Error(`unknown or disabled server '${serverId}'`);
    const response = await fleetFetch(server, pathname, {
      method,
      headers: init.headers,
      body: init.body,
    });
    return {
      status: response.status,
      headers: Object.fromEntries(response.headers.entries()),
      body: await response.text(),
    };
  },
);

ipcMain.handle("agent-grid:session-channel-open", async (event, serverId: string) => {
  if (!desktopConfig) throw new Error("desktop config is not ready");
  const server = desktopConfig.servers.find(
    (candidate) => candidate.enabled && candidate.id === serverId,
  );
  if (!server) throw new Error(`unknown or disabled server '${serverId}'`);
  const ticketResponse = await fleetFetch(server, "/api/ws-ticket", { method: "POST" });
  const body = (await ticketResponse.json()) as { ticket?: unknown };
  if (typeof body.ticket !== "string" || !body.ticket) throw new Error("empty websocket ticket");
  const url = new URL("/ws", `${server.url.replace(/\/+$/, "")}/`);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("ticket", body.ticket);
  url.searchParams.set("protocolVersion", PROTOCOL_VERSION);
  const socket = new WebSocket(url);
  const channelId = randomUUID();
  activeChannels.set(channelId, { owner: event.sender.id, socket });
  event.sender.once("destroyed", () => {
    const channel = activeChannels.get(channelId);
    if (!channel) return;
    activeChannels.delete(channelId);
    channel.socket.close();
  });
  const publish = (kind: string, data?: unknown) => {
    if (!event.sender.isDestroyed()) {
      event.sender.send("agent-grid:session-channel-event", { channelId, kind, data });
    }
  };
  socket.addEventListener("open", () => publish("open"));
  socket.addEventListener("message", (message) => publish("message", String(message.data)));
  socket.addEventListener("error", () => publish("error"));
  socket.addEventListener("close", () => {
    activeChannels.delete(channelId);
    publish("close");
  });
  return channelId;
});

ipcMain.on("agent-grid:session-channel-send", (event, channelId: string, data: string) => {
  const channel = activeChannels.get(channelId);
  if (channel?.owner === event.sender.id) channel.socket.send(data);
});

ipcMain.on("agent-grid:session-channel-close", (event, channelId: string) => {
  const channel = activeChannels.get(channelId);
  if (channel?.owner !== event.sender.id) return;
  activeChannels.delete(channelId);
  channel.socket.close();
});

async function fleetFetch(
  server: import("./desktop-config.js").ServerConfig,
  pathname: string,
  init: RequestInit,
  includeProtocolVersion = true,
): Promise<Response> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 10_000);
  try {
    const headers = new Headers(init.headers);
    if (server.auth.mode === "bearer") headers.set("Authorization", `Bearer ${server.auth.token}`);
    if (includeProtocolVersion) {
      headers.set(PROTOCOL_VERSION_HEADER, PROTOCOL_VERSION);
    }
    const response = await fetch(new URL(pathname, `${server.url.replace(/\/+$/, "")}/`), {
      ...init,
      headers,
      redirect: "manual",
      signal: controller.signal,
    });
    if (response.status >= 300 && response.status < 400) {
      throw new Error(`redirect rejected (${response.status})`);
    }
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response;
  } finally {
    clearTimeout(timer);
  }
}

// ─── Window chrome (VSCode-style title bar) ─────────────────────────────────
// The factory registers each window's {BaseWindow, title-bar wc, content wc}
// so this entry can answer title-bar IPC with native popup menus and repaint
// the chrome when the SPA reports a theme switch.

interface WindowChromeEntry {
  window: BaseWindow;
  titleBarWebContentsId: number;
  contentWebContentsId: number;
}

const chromeByTitleBarWc = new Map<number, WindowChromeEntry>();
const chromeByContentWc = new Map<number, WindowChromeEntry>();

function menuTemplateFor(
  id: TitleBarMenuId,
  win: BaseWindow,
  contentWc: WebContents | undefined,
): MenuItemConstructorOptions[] {
  switch (id) {
    case "file":
      return [
        {
          label: "Settings",
          accelerator: "Ctrl+,",
          click: () => openSettingsWindow?.(),
        },
        { type: "separator" },
        { label: "Close Window", accelerator: "Alt+F4", click: () => win.close() },
        { type: "separator" },
        { label: "Exit", click: () => app.quit() },
      ];
    case "edit":
      return [
        { label: "Undo", accelerator: "Ctrl+Z", click: () => contentWc?.undo() },
        { label: "Redo", accelerator: "Ctrl+Y", click: () => contentWc?.redo() },
        { type: "separator" },
        { label: "Cut", accelerator: "Ctrl+X", click: () => contentWc?.cut() },
        { label: "Copy", accelerator: "Ctrl+C", click: () => contentWc?.copy() },
        { label: "Paste", accelerator: "Ctrl+V", click: () => contentWc?.paste() },
        { type: "separator" },
        { label: "Select All", accelerator: "Ctrl+A", click: () => contentWc?.selectAll() },
      ];
    case "view":
      return [
        { label: "Reload", accelerator: "Ctrl+R", click: () => contentWc?.reload() },
        { type: "separator" },
        {
          label: "Zoom In",
          accelerator: "Ctrl+Plus",
          click: () => contentWc?.setZoomLevel(contentWc.getZoomLevel() + 0.5),
        },
        {
          label: "Zoom Out",
          accelerator: "Ctrl+-",
          click: () => contentWc?.setZoomLevel(contentWc.getZoomLevel() - 0.5),
        },
        { label: "Reset Zoom", accelerator: "Ctrl+0", click: () => contentWc?.setZoomLevel(0) },
        { type: "separator" },
        {
          label: "Toggle Developer Tools",
          accelerator: "Ctrl+Shift+I",
          click: () => contentWc?.toggleDevTools(),
        },
      ];
    case "window":
      return [
        { label: "Minimize", click: () => win.minimize() },
        {
          label: win.isMaximized() ? "Restore" : "Maximize",
          click: () => (win.isMaximized() ? win.unmaximize() : win.maximize()),
        },
        { type: "separator" },
        { label: "Close", accelerator: "Alt+F4", click: () => win.close() },
      ];
  }
}

ipcMain.on("agent-grid:titlebar-menu", (event, payload: unknown) => {
  const entry = chromeByTitleBarWc.get(event.sender.id);
  if (!entry || entry.window.isDestroyed()) return;
  const { id, x, y } = (payload ?? {}) as { id?: unknown; x?: unknown; y?: unknown };
  if (!isTitleBarMenuId(id)) return;
  const contentWc = webContents.fromId(entry.contentWebContentsId);
  Menu.buildFromTemplate(menuTemplateFor(id, entry.window, contentWc)).popup({
    window: entry.window,
    x: Math.max(0, Math.round(Number(x) || 0)),
    y: Math.max(0, Math.round(Number(y) || 0)),
  });
});

// The SPA resolves the effective theme (stored choice / OS) and reports it
// (resolved value + chosen mode) so the native caption-button overlay, the
// title-bar view, and native popup menus all track data-theme.
ipcMain.on("agent-grid:titlebar-theme", (event, theme: unknown, mode: unknown) => {
  if (theme !== "dark" && theme !== "light") return;
  const entry = chromeByContentWc.get(event.sender.id);
  if (!entry || entry.window.isDestroyed()) return;
  // Native menus follow nativeTheme — mirror the SPA mode (VSCode-style
  // themed menus). Missing mode (older bridge) pins to the resolved theme.
  nativeTheme.themeSource = themeSourceFor(mode ?? theme);
  try {
    entry.window.setTitleBarOverlay(titleBarOverlayFor(theme));
  } catch {
    // Window without a controls overlay (non-WCO platform) — ignore.
  }
  webContents
    .fromId(entry.titleBarWebContentsId)
    ?.executeJavaScript(`document.documentElement.dataset.theme = ${JSON.stringify(theme)};`)
    .catch(() => {});
});

void startWorkspaceApp({
  app,
  BaseWindow: BaseWindow as unknown as ElectronBaseWindowConstructor,
  WebContentsView: WebContentsView as unknown as ElectronWebContentsViewConstructor,
  preloadPath,
  titleBarPreloadPath,
  onDesktopConfigLoaded: (config) => {
    desktopConfig = config;
    replaceFleetRuntime(config);
    replaceGitHubRuntime(config.github);
  },
  systemPrefersDark: () => nativeTheme.shouldUseDarkColors,
  registerWindowChrome: (handles) => {
    const entry: WindowChromeEntry = {
      window: handles.window as unknown as BaseWindow,
      titleBarWebContentsId: handles.titleBarWebContentsId,
      contentWebContentsId: handles.contentWebContentsId,
    };
    chromeByTitleBarWc.set(entry.titleBarWebContentsId, entry);
    chromeByContentWc.set(entry.contentWebContentsId, entry);
    entry.window.on("closed", () => {
      chromeByTitleBarWc.delete(entry.titleBarWebContentsId);
      chromeByContentWc.delete(entry.contentWebContentsId);
    });
  },
  registerRendererInfo: (webContentsId, info) => {
    rendererInfo.set(webContentsId, info);
  },
  configureServerTransport: (webContentsId, baseUrl) => {
    const contents = webContents.fromId(webContentsId);
    if (!contents) return;
    const httpOrigin = new URL(baseUrl).origin;
    const wsOrigin = httpOrigin
      .replace(/^http:/, "ws:")
      .replace(/^https:/, "wss:");
    contents.session.webRequest.onBeforeSendHeaders(
      { urls: [`${httpOrigin}/*`, `${wsOrigin}/*`] },
      (details, callback) => {
        const requestHeaders = { ...details.requestHeaders };
        for (const name of Object.keys(requestHeaders)) {
          if (name.toLowerCase() === "origin") delete requestHeaders[name];
        }
        callback({ requestHeaders });
      },
    );
  },
})
  .then((started) => {
    updateDesktopConfig = started.setDesktopConfig;
    openSettingsWindow = started.openSettings;
  })
  .catch((error: unknown) => {
    console.error("Workspace startup failed", error);
    app.quit();
    process.exitCode = 1;
  });
