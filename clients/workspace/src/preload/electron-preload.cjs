"use strict";

const { contextBridge, ipcRenderer } = require("electron");

const hostedModeInfo = ipcRenderer.sendSync("agent-grid:renderer-bootstrap");
contextBridge.exposeInMainWorld("agentGridWorkspace", { hostedModeInfo });
