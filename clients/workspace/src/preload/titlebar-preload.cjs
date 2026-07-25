"use strict";

// Title-bar view bridge: clicking a menu-bar label asks the main process to
// pop the matching native menu at the label's position (window coordinates).
const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("agentGridTitleBar", {
  openMenu: (id, x, y) => {
    if (typeof id !== "string") return;
    ipcRenderer.send("agent-grid:titlebar-menu", {
      id,
      x: Math.round(Number(x) || 0),
      y: Math.round(Number(y) || 0),
    });
  },
});
