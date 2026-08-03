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
  openSettings: (section) => ipcRenderer.invoke("agent-grid:open-settings", section),
  github: {
    action: (action) => ipcRenderer.invoke("agent-grid:github-action", action),
    openAuthorization: () => ipcRenderer.invoke("agent-grid:github-open-authorization"),
  },
  sessionFleet: {
    listConfigs: () => ipcRenderer.invoke("agent-grid:session-fleet-configs"),
    subscribe: (listener) => {
      const receive = (_event, snapshot) => listener(snapshot);
      ipcRenderer.on("agent-grid:session-fleet-snapshot", receive);
      void ipcRenderer.invoke("agent-grid:session-fleet-subscribe");
      return () => ipcRenderer.removeListener("agent-grid:session-fleet-snapshot", receive);
    },
    subscribeHealth: (listener) => {
      const receive = (_event, snapshot) => listener(snapshot);
      ipcRenderer.on("agent-grid:session-fleet-health", receive);
      void ipcRenderer.invoke("agent-grid:session-fleet-health-subscribe");
      return () => ipcRenderer.removeListener("agent-grid:session-fleet-health", receive);
    },
    configPath: () => ipcRenderer.invoke("agent-grid:session-fleet-config-path"),
    openChannel: async (serverId, listener) => {
      let channelId = null;
      const pending = [];
      const receive = (_event, envelope) => {
        if (channelId === null) {
          pending.push(envelope);
          return;
        }
        if (envelope.channelId === channelId) listener(envelope);
      };
      ipcRenderer.on("agent-grid:session-channel-event", receive);
      channelId = await ipcRenderer.invoke("agent-grid:session-channel-open", serverId);
      for (const envelope of pending) {
        if (envelope.channelId === channelId) listener(envelope);
      }
      return {
        channelId,
        send: (data) => ipcRenderer.send("agent-grid:session-channel-send", channelId, data),
        close: () => {
          ipcRenderer.removeListener("agent-grid:session-channel-event", receive);
          ipcRenderer.send("agent-grid:session-channel-close", channelId);
        },
      };
    },
    create: (serverId, payload) =>
      ipcRenderer.invoke("agent-grid:session-fleet-create", serverId, payload),
    createRepositorySession: (serverId, payload) =>
      ipcRenderer.invoke("agent-grid:session-fleet-repository-create", serverId, payload),
    reconnect: (serverId) => ipcRenderer.invoke("agent-grid:session-fleet-reconnect", serverId),
    request: (serverId, pathname, init) =>
      ipcRenderer.invoke("agent-grid:session-fleet-request", serverId, pathname, init),
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
