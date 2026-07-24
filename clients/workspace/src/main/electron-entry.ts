import { BrowserWindow, app } from "electron";
import { fileURLToPath } from "node:url";
import { startWorkspaceApp } from "./app-entry.js";
import type { ElectronBrowserWindowConstructor } from "./electron-window-factory.js";

const preloadPath = fileURLToPath(new URL("../preload/electron-preload.cjs", import.meta.url));

void startWorkspaceApp({
  app,
  BrowserWindow: BrowserWindow as unknown as ElectronBrowserWindowConstructor,
  preloadPath,
}).catch((error: unknown) => {
  console.error("Workspace startup failed", error);
  app.quit();
  process.exitCode = 1;
});
