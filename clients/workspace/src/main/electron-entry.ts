import { BrowserWindow, app, ipcMain, webContents } from "electron";
import { fileURLToPath } from "node:url";
import { startWorkspaceApp } from "./app-entry.js";
import type { ElectronBrowserWindowConstructor } from "./electron-window-factory.js";

const preloadPath = fileURLToPath(new URL("../preload/electron-preload.cjs", import.meta.url));
const rendererInfo = new Map<number, import("./daemon-config.js").HostedModeInfo>();

ipcMain.on("agent-grid:renderer-bootstrap", (event) => {
  event.returnValue = rendererInfo.get(event.sender.id) ?? null;
});

void startWorkspaceApp({
  app,
  BrowserWindow: BrowserWindow as unknown as ElectronBrowserWindowConstructor,
  preloadPath,
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
}).catch((error: unknown) => {
  console.error("Workspace startup failed", error);
  app.quit();
  process.exitCode = 1;
});
