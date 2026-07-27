import {
  BrowserWindow,
  BaseWindow,
  Menu,
  WebContentsView,
  app,
  ipcMain,
  nativeTheme,
  webContents,
  type MenuItemConstructorOptions,
  type WebContents,
} from "electron";
import { fileURLToPath } from "node:url";
import { startWorkspaceApp } from "./app-entry.js";
import {
  loadEditableDesktopSettings,
  saveEditableDesktopSettings,
  type DesktopConfig,
  type EditableDesktopSettings,
} from "./desktop-config.js";
import type {
  ElectronBaseWindowConstructor,
  ElectronWebContentsViewConstructor,
} from "./electron-window-factory.js";
import { FleetHealthMonitor } from "./server-health.js";
import {
  isTitleBarMenuId,
  themeSourceFor,
  titleBarOverlayFor,
  type TitleBarMenuId,
} from "./window-chrome.js";

const preloadPath = fileURLToPath(new URL("../preload/electron-preload.cjs", import.meta.url));
const titleBarPreloadPath = fileURLToPath(
  new URL("../preload/titlebar-preload.cjs", import.meta.url),
);
const rendererInfo = new Map<number, import("./daemon-config.js").HostedModeInfo>();
let desktopConfig: DesktopConfig | null = null;
let updateDesktopConfig: ((config: DesktopConfig) => void) | null = null;
let openSettingsWindow: (() => void) | null = null;
const serverHealth = new FleetHealthMonitor(() => desktopConfig?.servers ?? []);

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
  const next = saveEditableDesktopSettings(desktopConfig.configDirectory, settings);
  desktopConfig = next;
  serverHealth.invalidate();
  updateDesktopConfig?.(next);
  return next;
});

ipcMain.handle("agent-grid:server-health-list", () => {
  if (!desktopConfig) throw new Error("desktop config is not ready");
  return serverHealth.list();
});

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
