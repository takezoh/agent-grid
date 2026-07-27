"use strict";

const { contextBridge, ipcRenderer } = require("electron");

const hostedModeInfo = ipcRenderer.sendSync("agent-grid:renderer-bootstrap");
contextBridge.exposeInMainWorld("agentGridWorkspace", {
  hostedModeInfo,
  windowControls: {
    minimize: () => ipcRenderer.send("agent-grid:window-minimize"),
    maximize: () => ipcRenderer.send("agent-grid:window-maximize"),
    close: () => ipcRenderer.send("agent-grid:window-close"),
  },
  desktopConfig: {
    load: () => ipcRenderer.invoke("agent-grid:desktop-config-load"),
    save: (settings) => ipcRenderer.invoke("agent-grid:desktop-config-save", settings),
  },
  serverHealth: {
    list: () => ipcRenderer.invoke("agent-grid:server-health-list"),
  },
  // Keeps the native chrome (caption-button overlay, title-bar view, popup
  // menus) in sync with the SPA theme. `theme` is the resolved value;
  // `mode` is the user's choice ("system" keeps native UI on the OS theme).
  setTitleBarTheme: (theme, mode) => {
    if (theme !== "dark" && theme !== "light") return;
    ipcRenderer.send(
      "agent-grid:titlebar-theme",
      theme,
      mode === "system" || mode === "dark" || mode === "light" ? mode : undefined,
    );
  },
});
