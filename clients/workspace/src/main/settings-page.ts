function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

export type SettingsSection = "connections" | "appearance" | "workspace" | "github";

export function settingsPageUrl(section: SettingsSection = "connections"): string {
  return `data:text/html;charset=utf-8,${encodeURIComponent(buildSettingsPageHtml())}#${section}`;
}

export function buildSettingsPageHtml(): string {
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Settings</title>
    <style>
      :root {
        color-scheme: dark;
        --bg: #1e1e1e;
        --panel: #252526;
        --panel-alt: #2d2d30;
        --panel-hover: #313135;
        --line: #3f3f46;
        --text: #cccccc;
        --muted: #9d9d9d;
        --accent: #0e639c;
        --accent-soft: rgba(14, 99, 156, 0.2);
        --danger: #c74e39;
        --danger-soft: rgba(199, 78, 57, 0.14);
        --ok: #73c991;
        --github: #f0f6fc;
        --github-soft: rgba(240, 246, 252, 0.08);
        --shadow: rgba(0, 0, 0, 0.28);
        font-family: "Segoe UI", "Noto Sans", sans-serif;
      }
      * { box-sizing: border-box; }
      html, body { margin: 0; min-height: 100%; background: var(--bg); color: var(--text); }
      body { font-size: 13px; }
      button, input, select {
        font: inherit;
      }
      .layout {
        display: grid;
        grid-template-columns: 280px minmax(0, 1fr);
        min-height: 100vh;
      }
      .sidebar {
        border-right: 1px solid var(--line);
        background: linear-gradient(180deg, #252526 0%, #202022 100%);
        padding: 18px 14px;
      }
      .sidebar__eyebrow {
        margin: 0 0 6px;
        color: var(--muted);
        font-size: 11px;
        letter-spacing: 0.12em;
        text-transform: uppercase;
      }
      .sidebar__title {
        margin: 0 0 18px;
        font-size: 24px;
        font-weight: 600;
      }
      .sidebar__meta {
        margin: 0 0 18px;
        padding: 10px 12px;
        border: 1px solid var(--line);
        border-radius: 8px;
        background: rgba(255, 255, 255, 0.03);
        color: var(--muted);
        line-height: 1.5;
      }
      .tree {
        display: grid;
        gap: 2px;
      }
      .tree__section {
        margin-top: 14px;
        margin-bottom: 4px;
        padding: 0 10px;
        color: var(--muted);
        font-size: 11px;
        letter-spacing: 0.08em;
        text-transform: uppercase;
      }
      .tree button {
        width: 100%;
        display: grid;
        grid-template-columns: 1fr auto;
        align-items: center;
        gap: 10px;
        padding: 8px 10px;
        border: 0;
        border-radius: 6px;
        background: transparent;
        color: inherit;
        text-align: left;
        cursor: pointer;
      }
      .tree button:hover {
        background: var(--panel-hover);
      }
      .tree button[aria-current="true"] {
        background: var(--accent-soft);
        color: #ffffff;
      }
      .tree__count {
        color: var(--muted);
        font-size: 12px;
      }
      .content {
        padding: 28px 34px 40px;
      }
      .content__header h1 {
        margin: 0 0 6px;
        font-size: 30px;
        font-weight: 600;
      }
      .content__header p {
        margin: 0 0 24px;
        max-width: 760px;
        color: var(--muted);
        line-height: 1.6;
      }
      .section {
        display: none;
      }
      .section[data-active="true"] {
        display: block;
      }
      .group {
        margin-bottom: 24px;
        border: 1px solid var(--line);
        border-radius: 10px;
        background: var(--panel);
        box-shadow: 0 10px 30px var(--shadow);
        overflow: hidden;
      }
      .group__header {
        padding: 16px 18px;
        border-bottom: 1px solid var(--line);
        background: linear-gradient(180deg, rgba(255,255,255,0.03), transparent);
      }
      .group__header h2,
      .group__header h3 {
        margin: 0;
        font-size: 17px;
        font-weight: 600;
      }
      .group__header p {
        margin: 6px 0 0;
        color: var(--muted);
        line-height: 1.5;
      }
      .group__body {
        padding: 18px;
      }
      .form-grid {
        display: grid;
        gap: 16px;
      }
      .form-row {
        display: grid;
        gap: 16px;
      }
      @media (min-width: 860px) {
        .form-row {
          grid-template-columns: repeat(2, minmax(0, 1fr));
        }
      }
      .field {
        display: grid;
        gap: 6px;
      }
      .field label {
        font-weight: 600;
      }
      .field small {
        color: var(--muted);
        line-height: 1.4;
      }
      .field input,
      .field select {
        width: 100%;
        min-height: 32px;
        padding: 6px 9px;
        border: 1px solid #3c3c3c;
        border-radius: 4px;
        background: #3c3c3c;
        color: inherit;
      }
      .field input:focus,
      .field select:focus,
      .tree button:focus,
      .btn:focus {
        outline: 1px solid #ffffff;
        outline-offset: 1px;
      }
      .server-list {
        display: grid;
        gap: 16px;
      }
      .server-card {
        border: 1px solid var(--line);
        border-radius: 8px;
        background: var(--panel-alt);
        overflow: hidden;
      }
      .server-card__header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
        padding: 14px 16px;
        border-bottom: 1px solid var(--line);
      }
      .server-card__title {
        display: grid;
        gap: 4px;
      }
      .server-card__title strong {
        font-size: 14px;
      }
      .server-card__title span {
        color: var(--muted);
      }
      .server-card__body {
        padding: 16px;
      }
      .server-card__footer {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
        margin-top: 18px;
      }
      .toggle {
        display: inline-flex;
        align-items: center;
        gap: 8px;
        color: var(--muted);
      }
      .toggle input {
        width: 14px;
        height: 14px;
      }
      .btn {
        min-height: 32px;
        padding: 0 12px;
        border: 1px solid transparent;
        border-radius: 4px;
        background: #3a3d41;
        color: inherit;
        cursor: pointer;
      }
      .btn:hover {
        background: #45494e;
      }
      .btn--primary {
        background: var(--accent);
        color: #ffffff;
      }
      .btn--primary:hover {
        background: #1177bb;
      }
      .btn--danger {
        background: transparent;
        border-color: var(--danger);
        color: #f2b8b5;
      }
      .btn--danger:hover {
        background: var(--danger-soft);
      }
      .toolbar {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
        align-items: center;
      }
      .content__actions {
        display: flex;
        justify-content: flex-end;
        gap: 10px;
        margin-bottom: 16px;
      }
      .status {
        min-height: 18px;
        color: var(--muted);
      }
      .status[data-level="success"] { color: var(--ok); }
      .status[data-level="error"] { color: #f48771; }
      .status[data-level="info"] { color: var(--muted); }
      .empty {
        margin: 0;
        color: var(--muted);
        line-height: 1.5;
      }
      .server-health {
        margin-left: auto;
        font-size: 12px;
      }
      .server-health[data-state="connected"] { color: var(--ok); }
      .server-health[data-state="unavailable"] { color: #f48771; }
      .server-health[data-state="disabled"] { color: var(--muted); }
      .server-health[data-state="checking"] { color: var(--muted); }
      .github-hero {
        display: grid;
        grid-template-columns: auto minmax(0, 1fr) auto;
        gap: 16px;
        align-items: center;
        padding: 20px;
        border: 1px solid var(--line);
        border-radius: 12px;
        background: radial-gradient(circle at 85% 0%, rgba(88,166,255,.16), transparent 42%), linear-gradient(145deg, #2a2a2d, #222225);
      }
      .github-mark {
        display: grid;
        place-items: center;
        width: 48px;
        height: 48px;
        border-radius: 14px;
        background: var(--github);
        color: #161b22;
        font-size: 20px;
        font-weight: 800;
        letter-spacing: -.08em;
        box-shadow: 0 8px 24px rgba(0,0,0,.28);
      }
      .github-hero__copy { min-width: 0; }
      .github-hero__copy h3 { margin: 0 0 5px; color: #fff; font-size: 17px; }
      .github-hero__copy p { margin: 0; color: var(--muted); line-height: 1.45; }
      .github-status-pill {
        display: inline-flex;
        align-items: center;
        gap: 7px;
        padding: 6px 10px;
        border: 1px solid var(--line);
        border-radius: 999px;
        background: rgba(0,0,0,.18);
        color: var(--muted);
        white-space: nowrap;
      }
      .github-status-pill::before { content: ""; width: 7px; height: 7px; border-radius: 50%; background: currentColor; }
      .github-status-pill[data-state="authenticated"] { color: var(--ok); }
      .github-status-pill[data-state="authorizing"] { color: #cca700; }
      .github-status-pill[data-state="host_misconfigured"],
      .github-status-pill[data-state="denied"],
      .github-status-pill[data-state="refresh_failed"] { color: #f48771; }
      .github-actions { margin-top: 16px; }
      .github-detail { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 16px; }
      .github-metric { padding: 12px 14px; border: 1px solid var(--line); border-radius: 8px; background: rgba(255,255,255,.025); }
      .github-metric span { display: block; margin-bottom: 4px; color: var(--muted); font-size: 11px; text-transform: uppercase; letter-spacing: .08em; }
      .github-metric strong { color: #fff; font-weight: 600; }
      .github-device-hint { margin: 16px 0 0; padding: 12px 14px; border-left: 3px solid var(--accent); border-radius: 4px; background: var(--accent-soft); line-height: 1.5; }
      .github-device-hint:empty { display: none; }
      .scope-list { display: flex; flex-wrap: wrap; gap: 8px; }
      .scope-chip { padding: 5px 9px; border: 1px solid var(--line); border-radius: 999px; background: var(--github-soft); color: var(--github); font-family: Consolas, monospace; font-size: 12px; }
      .security-note { margin: 16px 0 0; padding: 12px 14px; border-radius: 8px; background: rgba(115,201,145,.08); color: var(--muted); line-height: 1.5; }
      .security-note strong { color: var(--ok); }
      @media (max-width: 760px) {
        .layout { grid-template-columns: 1fr; }
        .sidebar { border-right: 0; border-bottom: 1px solid var(--line); }
        .content { padding: 22px 18px 32px; }
        .github-hero { grid-template-columns: auto minmax(0, 1fr); }
        .github-status-pill { grid-column: 1 / -1; justify-self: start; }
        .github-detail { grid-template-columns: 1fr; }
      }
    </style>
  </head>
  <body>
    <div class="layout">
      <aside class="sidebar">
        <p class="sidebar__eyebrow">Agent Grid</p>
        <h1 class="sidebar__title">Settings</h1>
        <p id="config-directory" class="sidebar__meta">Loading desktop configuration…</p>
        <nav class="tree" aria-label="Settings categories">
          <div class="tree__section">Workspace</div>
          <button type="button" data-target="connections" aria-current="true">
            <span>Connections</span>
            <span id="connections-count" class="tree__count">0</span>
          </button>
          <button type="button" data-target="appearance" aria-current="false">
            <span>Appearance</span>
            <span class="tree__count"></span>
          </button>
          <button type="button" data-target="workspace" aria-current="false">
            <span>Workspace</span>
            <span class="tree__count"></span>
          </button>
          <button type="button" data-target="shell" aria-current="false">
            <span>Shell</span>
            <span class="tree__count"></span>
          </button>
          <button type="button" data-target="github" aria-current="false"><span>GitHub</span><span id="github-status-count" class="tree__count"></span></button>
        </nav>
      </aside>

      <main class="content">
        <header class="content__header">
          <h1 id="section-title">Connections</h1>
          <p id="section-description">
            Configure the list of Agent Grid servers that Shell and Workspace connect to together.
            These entries are not a switcher; every enabled server remains part of the combined session view.
          </p>
        </header>
        <div class="content__actions">
          <button id="close-settings" type="button" class="btn">Close</button>
        </div>

        <section id="connections" class="section" data-active="true">
          <div class="group">
            <div class="group__header">
              <h2>Default execution server</h2>
              <p>New Session uses this enabled server unless you choose Change in the palette.</p>
            </div>
            <div class="group__body">
              <div class="field">
                <label for="default-execution-server">Execution server</label>
                <select id="default-execution-server"></select>
                <small>The selection is shared by Workspace and the web client through servers.json.</small>
              </div>
            </div>
          </div>
          <div class="group">
            <div class="group__header">
              <h2>Server list</h2>
              <p>
                Enabled entries are always connected by both Shell and Workspace. Add every gateway you want included in the merged session list.
              </p>
            </div>
            <div class="group__body">
              <div class="toolbar" style="margin-bottom:16px">
                <button id="add-server" type="button" class="btn">Add server</button>
              </div>
              <div id="servers" class="server-list"></div>
            </div>
          </div>
        </section>

        <section id="appearance" class="section" data-active="false">
          <div class="group">
            <div class="group__header">
              <h2>Appearance</h2>
              <p>Default look and feel for the desktop clients.</p>
            </div>
            <div class="group__body">
              <div class="form-grid">
                <div class="field">
                  <label for="theme">Theme</label>
                  <select id="theme">
                    <option value="default">Default</option>
                    <option value="system">System</option>
                    <option value="light">Light</option>
                    <option value="dark">Dark</option>
                  </select>
                </div>
                <div class="field">
                  <label for="density">Density</label>
                  <select id="density">
                    <option value="comfortable">Comfortable</option>
                    <option value="compact">Compact</option>
                  </select>
                </div>
                <div class="field">
                  <label for="font-scale">Font scale</label>
                  <input id="font-scale" type="number" min="0.8" max="1.5" step="0.1">
                  <small>Accepted range: 0.8 to 1.5.</small>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section id="workspace" class="section" data-active="false">
          <div class="group">
            <div class="group__header">
              <h2>Workspace</h2>
              <p>Desktop-only behavior for the Workspace app.</p>
            </div>
            <div class="group__body">
              <div class="form-grid">
                <div class="field">
                  <label for="idle-quit-seconds">Idle quit seconds</label>
                  <input id="idle-quit-seconds" type="number" min="0" max="86400" step="1">
                  <small>When no windows are open, the Workspace process exits after this delay.</small>
                </div>
                <div class="form-row">
                  <div class="field">
                    <label for="window-width">Default width</label>
                    <input id="window-width" type="number" min="320" max="16384" step="1">
                  </div>
                  <div class="field">
                    <label for="window-height">Default height</label>
                    <input id="window-height" type="number" min="240" max="16384" step="1">
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section id="shell" class="section" data-active="false">
          <div class="group">
            <div class="group__header">
              <h2>Shell</h2>
              <p>Behavior of the Windows Shell surface.</p>
            </div>
            <div class="group__body">
              <div class="form-grid">
                <div class="field">
                  <label for="shell-display">Display</label>
                  <select id="shell-display">
                    <option value="">System primary (default)</option>
                  </select>
                  <small>Takes effect the next time Windows Shell starts.</small>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section id="github" class="section" data-active="false">
          <div class="group">
            <div class="group__header"><h2>GitHub connection</h2><p>Connect Workspace directly with GitHub using the device authorization flow.</p></div>
            <div class="group__body">
              <div class="github-hero">
                <div class="github-mark" aria-hidden="true">GH</div>
                <div class="github-hero__copy"><h3 id="github-account-state">Checking account…</h3><p>Authentication is owned by this Workspace app. Access tokens are never exposed to the UI.</p></div>
                <span id="github-status-pill" class="github-status-pill" data-state="unauthenticated">Checking</span>
              </div>
              <div class="github-detail">
                <div class="github-metric"><span>Account</span><strong id="github-account-detail">Not connected</strong></div>
                <div class="github-metric"><span>Repository catalog</span><strong id="github-catalog-detail">Empty</strong></div>
              </div>
              <div class="toolbar github-actions">
                <button id="github-connect" type="button" class="btn btn--primary">Connect GitHub</button>
                <button id="github-open-browser" type="button" class="btn" hidden>Open authorization page</button>
                <button id="github-cancel" type="button" class="btn">Cancel</button>
                <button id="github-refresh" type="button" class="btn">Refresh repositories</button>
              </div>
              <p id="github-device-hint" class="github-device-hint"></p>
            </div>
          </div>
          <div class="group">
            <div class="group__header"><h2>GitHub App configuration</h2><p>Public identifiers for the OAuth registration used by Workspace.</p></div>
            <div class="group__body">
              <div class="form-grid">
                <div class="form-row">
                  <div class="field"><label for="github-app-id">App ID</label><input id="github-app-id" inputmode="numeric" pattern="[0-9]+" autocomplete="off" spellcheck="false" required><small>The numeric identifier shown in GitHub App settings.</small></div>
                  <div class="field"><label for="github-client-id">Client ID</label><input id="github-client-id" autocomplete="off" spellcheck="false" required><small>The public Client ID used to begin device authorization.</small></div>
                </div>
                <div class="field"><label>Requested scopes</label><div class="scope-list"><span class="scope-chip">read:user</span><span class="scope-chip">repo</span></div><small>Scopes are fixed so the client cannot silently broaden access.</small></div>
              </div>
              <p class="security-note"><strong>No client secret is required.</strong> Only public identifiers are stored in <code>github.json</code>. Credentials remain in the Workspace credential store.</p>
            </div>
          </div>
        </section>

        <div class="toolbar">
          <button id="save-button" type="button" class="btn btn--primary">Save settings</button>
          <span id="status" class="status" data-level="info">Changes are written to the shared desktop client configuration.</span>
        </div>
      </main>
    </div>

    <script>
      const workspaceApi = window.agentGridWorkspace?.desktopConfig;
      const displayApi = window.agentGridWorkspace?.displays;
      const healthApi = window.agentGridWorkspace?.serverHealth;
      const fleetApi = window.agentGridWorkspace?.sessionFleet;
      const githubApi = window.agentGridWorkspace?.github;
      const sections = {
        connections: {
          title: "Connections",
          description:
            "Configure the list of Agent Grid servers that Shell and Workspace connect to together. These entries are not a switcher; every enabled server remains part of the combined session view.",
        },
        appearance: {
          title: "Appearance",
          description: "Set the default theme, density, and font scale for the desktop clients.",
        },
        workspace: {
          title: "Workspace",
          description: "Tune Workspace-specific window behavior without affecting the server list.",
        },
        shell: {
          title: "Shell",
          description: "Configure behavior specific to the Windows Shell surface.",
        },
        github: { title: "GitHub", description: "Workspace-owned GitHub device flow. Credentials stay in the desktop client." },
      };
      const state = {
        snapshot: null,
        activeSection: "connections",
        health: new Map(),
        reconnecting: new Set(),
        displays: [],
      };

      const els = {
        configDirectory: document.getElementById("config-directory"),
        sectionTitle: document.getElementById("section-title"),
        sectionDescription: document.getElementById("section-description"),
        status: document.getElementById("status"),
        saveButton: document.getElementById("save-button"),
        navButtons: Array.from(document.querySelectorAll(".tree button[data-target]")),
        connectionsCount: document.getElementById("connections-count"),
        servers: document.getElementById("servers"),
        defaultExecutionServer: document.getElementById("default-execution-server"),
        addServer: document.getElementById("add-server"),
        closeSettings: document.getElementById("close-settings"),
        theme: document.getElementById("theme"),
        density: document.getElementById("density"),
        fontScale: document.getElementById("font-scale"),
        idleQuitSeconds: document.getElementById("idle-quit-seconds"),
        shellDisplay: document.getElementById("shell-display"),
        windowWidth: document.getElementById("window-width"),
        windowHeight: document.getElementById("window-height"),
        ghState: document.getElementById("github-account-state"),
        ghDetail: document.getElementById("github-account-detail"),
        ghCatalog: document.getElementById("github-catalog-detail"),
        ghPill: document.getElementById("github-status-pill"),
        ghHint: document.getElementById("github-device-hint"),
        ghAppId: document.getElementById("github-app-id"),
        ghClientId: document.getElementById("github-client-id"),
        ghConnect: document.getElementById("github-connect"),
        ghOpenBrowser: document.getElementById("github-open-browser"),
        ghCancel: document.getElementById("github-cancel"),
        ghRefresh: document.getElementById("github-refresh"),
        ghCount: document.getElementById("github-status-count"),
      };
      let ghPoll = null;

      function escapeHtml(value) {
        return String(value)
          .replaceAll("&", "&amp;")
          .replaceAll("<", "&lt;")
          .replaceAll(">", "&gt;")
          .replaceAll('"', "&quot;")
          .replaceAll("'", "&#39;");
      }

      function setStatus(level, message) {
        els.status.dataset.level = level;
        els.status.textContent = message;
      }

      function resolveTheme(theme) {
        if (theme === "dark") return "dark";
        if (theme === "light") return "light";
        const prefersDark =
          window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
        return prefersDark ? "dark" : "light";
      }

      function syncNativeTheme() {
        const mode = els.theme.value;
        window.agentGridWorkspace?.setTitleBarTheme?.(resolveTheme(mode), mode);
      }

      function createEmptyServer() {
        return {
          display_name: "New server",
          enabled: true,
          default_execution: false,
          url: "http://127.0.0.1:8443",
          auth: { mode: "none" },
        };
      }

      function setSection(target) {
        if (!sections[target]) target = "connections";
        state.activeSection = target;
        for (const button of els.navButtons) {
          button.setAttribute("aria-current", button.dataset.target === target ? "true" : "false");
        }
        for (const section of document.querySelectorAll(".section")) {
          section.dataset.active = section.id === target ? "true" : "false";
        }
        els.sectionTitle.textContent = sections[target].title;
        els.sectionDescription.textContent = sections[target].description;
      }

      function serverSummary(server) {
        return (server.display_name && server.display_name.trim()) || "Server";
      }

      function serverHealthPresentation(server) {
        const health = state.health.get(server.id);
        const healthState = health?.state || (server.enabled ? "checking" : "disabled");
        const healthLabel =
          healthState === "connected" ? "Connected" :
          healthState === "incompatible_protocol" ? "Incompatible protocol" :
          healthState === "disabled" ? "Disabled" :
          healthState === "checking" ? "Checking…" :
          healthState === "health_unavailable" ? "Health unavailable" : "Unavailable";
        const stale = health?.stale ? " (stale prior)" : "";
        const healthDetail = health?.reason ? " — " + health.reason + stale : stale;
        return {
          state: healthState,
          label: healthLabel + healthDetail,
          detail: healthDetail,
        };
      }

      function renderServers() {
        const servers = state.snapshot?.servers ?? [];
        renderDefaultExecutionServers();
        els.connectionsCount.textContent = String(servers.length);
        if (servers.length === 0) {
          els.servers.innerHTML = '<p class="empty">No servers configured. Add at least one server before saving.</p>';
          return;
        }
        els.servers.innerHTML = servers.map((server, index) => {
          const health = serverHealthPresentation(server);
          return [
            '<article class="server-card" data-server-index="' + index + '">',
            '<div class="server-card__header">',
            '<div class="server-card__title">',
            '<strong data-role="server-summary">' + escapeHtml(serverSummary(server)) + '</strong>',
            '<span data-role="server-id">' + escapeHtml(server.id || "Identifier is derived after saving") + '</span>',
            '</div>',
            '<span class="server-health" data-role="server-health" data-state="' + health.state +
              '" title="' + escapeHtml(health.detail) + '">' + escapeHtml(health.label) + '</span>',
            server.enabled && server.id
              ? '<button type="button" class="btn" data-action="reconnect-server" data-index="' + index +
                '" data-server-id="' + escapeHtml(server.id) + '" aria-label="Reconnect ' +
                escapeHtml(serverSummary(server)) + '"' +
                (state.reconnecting.has(server.id) ? ' disabled aria-busy="true"' : '') + '>' +
                (state.reconnecting.has(server.id) ? 'Reconnecting…' : 'Reconnect') + '</button>'
              : '',
            '<button type="button" class="btn btn--danger" data-action="remove-server" data-index="' + index + '">Remove</button>',
            '</div>',
            '<div class="server-card__body">',
            '<div class="form-grid">',
            '<div class="form-row">',
            '<div class="field"><label>Display name</label><input data-field="display_name" data-index="' + index + '" value="' + escapeHtml(server.display_name || "") + '"><small>Shown anywhere the server label is needed. This name is local to this server entry.</small></div>',
            '</div>',
            '<div class="form-row">',
            '<div class="field"><label>Gateway URL</label><input data-field="url" data-index="' + index + '" value="' + escapeHtml(server.url || "") + '"></div>',
            '<div class="field"><label>Authentication</label><select data-field="auth_mode" data-index="' + index + '"><option value="none"' + (server.auth?.mode === "none" ? " selected" : "") + '>None</option><option value="bearer"' + (server.auth?.mode === "bearer" ? " selected" : "") + '>Bearer token</option></select><small>Use None only when the server was started with -no-auth.</small></div>',
            '</div>',
            '<div class="form-row">',
            '<div class="field"><label>Bearer token</label><input type="password" autocomplete="new-password" data-field="auth_token" data-index="' + index + '" value="' + escapeHtml(server.auth?.mode === "bearer" ? server.auth.token || "" : "") + '"' + (server.auth?.mode === "bearer" ? "" : " disabled") + '><small>Stored in servers.json and masked in this UI.</small></div>',
            '</div>',
            '</div>',
            '<div class="server-card__footer">',
            '<label class="toggle"><input type="checkbox" data-field="enabled" data-index="' + index + '"' + (server.enabled ? " checked" : "") + '>Enabled</label>',
            '<span class="empty">Enabled servers are included in the combined session list.</span>',
            '</div>',
            '</div>',
            '</article>',
          ].join("");
        }).join("");
      }

      function renderDefaultExecutionServers() {
        const servers = state.snapshot?.servers ?? [];
        const enabled = servers
          .map((server, index) => ({ server, index }))
          .filter(({ server }) => server.enabled);
        let selected = enabled.find(({ server }) => server.default_execution)?.index;
        if (selected === undefined && enabled[0]) {
          selected = enabled[0].index;
          servers.forEach((server, index) => {
            server.default_execution = index === selected;
          });
        }
        els.defaultExecutionServer.innerHTML = enabled.length === 0
          ? '<option value="">No enabled servers</option>'
          : enabled.map(({ server, index }) =>
              '<option value="' + index + '"' + (index === selected ? ' selected' : '') + '>' +
              escapeHtml(serverSummary(server)) + '</option>'
            ).join("");
        els.defaultExecutionServer.disabled = enabled.length === 0;
      }

      function updateServerPresentation(index) {
        const server = state.snapshot?.servers?.[index];
        const card = els.servers.querySelector('[data-server-index="' + index + '"]');
        if (!server || !card) return;
        const summary = card.querySelector('[data-role="server-summary"]');
        const serverId = card.querySelector('[data-role="server-id"]');
        const healthElement = card.querySelector('[data-role="server-health"]');
        if (summary) summary.textContent = serverSummary(server);
        if (serverId) {
          serverId.textContent = server.id || "Identifier is derived after saving";
        }
        if (healthElement) {
          const health = serverHealthPresentation(server);
          healthElement.dataset.state = health.state;
          healthElement.title = health.detail;
          healthElement.textContent = health.label;
        }
      }

      function updateServerPresentations() {
        const servers = state.snapshot?.servers ?? [];
        servers.forEach((_server, index) => updateServerPresentation(index));
      }

      function applySnapshot(snapshot) {
        state.snapshot = JSON.parse(JSON.stringify(snapshot));
        els.configDirectory.textContent = "Config directory\\n" + snapshot.configDirectory;
        els.theme.value = snapshot.appearance.theme;
        els.density.value = snapshot.appearance.density;
        els.fontScale.value = String(snapshot.appearance.font_scale);
        els.idleQuitSeconds.value = String(snapshot.workspace.idle_quit_seconds);
        renderDisplays();
        els.windowWidth.value = String(snapshot.workspace.default_window.width);
        els.windowHeight.value = String(snapshot.workspace.default_window.height);
        els.ghAppId.value = snapshot.github.app_id;
        els.ghClientId.value = snapshot.github.client_id;
        renderServers();
        syncNativeTheme();
      }

      function renderDisplays() {
        const saved = state.snapshot?.shell?.display_anchor ?? null;
        const selected = state.displays.find((display) =>
          saved && display.anchor.x === saved.x && display.anchor.y === saved.y
        );
        const options = [
          '<option value="">System primary (default)</option>',
          ...state.displays.map((display) =>
            '<option value="' + escapeHtml(display.id) + '">' +
            escapeHtml(display.label + (display.primary ? " (Primary)" : "")) +
            '</option>'
          ),
        ];
        if (saved && !selected) {
          options.push('<option value="__saved__">Unavailable saved display (primary fallback)</option>');
        }
        els.shellDisplay.innerHTML = options.join("");
        els.shellDisplay.value = selected?.id ?? (saved ? "__saved__" : "");
      }

      function updateServer(index, field, value) {
        const server = state.snapshot?.servers?.[index];
        if (!server) return;
        if (field === "enabled") {
          server.enabled = Boolean(value);
          if (!server.enabled && server.default_execution) server.default_execution = false;
          renderServers();
          return;
        }
        if (field === "auth_mode") {
          server.auth = value === "bearer"
            ? { mode: "bearer", token: server.auth?.mode === "bearer" ? server.auth.token : "" }
            : { mode: "none" };
          const tokenInput = els.servers.querySelector(
            'input[data-field="auth_token"][data-index="' + index + '"]',
          );
          if (tokenInput) {
            tokenInput.disabled = value !== "bearer";
            if (value !== "bearer") tokenInput.value = "";
          }
          return;
        }
        if (field === "auth_token") {
          server.auth = { mode: "bearer", token: String(value) };
          return;
        }
        server[field] = String(value);
        if (field === "display_name") {
          updateServerPresentation(index);
          renderDefaultExecutionServers();
        }
      }

      function collectPayload() {
        return {
          servers: (state.snapshot?.servers ?? []).map((server) => {
            return {
              display_name: (server.display_name || "").trim(),
              enabled: Boolean(server.enabled),
              default_execution: Boolean(server.default_execution),
              url: (server.url || "").trim(),
              auth: server.auth?.mode === "bearer"
                ? { mode: "bearer", token: (server.auth.token || "").trim() }
                : { mode: "none" },
            };
          }),
          appearance: {
            theme: els.theme.value,
            density: els.density.value,
            font_scale: Number(els.fontScale.value),
          },
          shell: {
            display_anchor: els.shellDisplay.value === "__saved__"
              ? state.snapshot.shell.display_anchor
              : state.displays.find((display) => display.id === els.shellDisplay.value)?.anchor ?? null,
          },
          workspace: {
            idle_quit_seconds: Number(els.idleQuitSeconds.value),
            default_window: {
              width: Number(els.windowWidth.value),
              height: Number(els.windowHeight.value),
            },
          },
          github: {
            app_id: els.ghAppId.value.trim(),
            client_id: els.ghClientId.value.trim(),
            scopes: ["read:user", "repo"],
          },
        };
      }

      async function load() {
        if (!workspaceApi?.load) {
          setStatus("error", "Workspace settings bridge is unavailable.");
          return;
        }
        try {
          const [snapshot, displays] = await Promise.all([
            workspaceApi.load(),
            displayApi?.list ? displayApi.list() : Promise.resolve([]),
          ]);
          state.displays = Array.isArray(displays) ? displays : [];
          applySnapshot(snapshot);
          setStatus("info", "Loaded desktop client settings.");
          await refreshHealth();
        } catch (error) {
          setStatus("error", error instanceof Error ? error.message : String(error));
        }
      }

      async function refreshHealth() {
        if (!healthApi?.list) return;
        try {
          const fleet = await healthApi.list();
          state.health = new Map(fleet.servers.map((server) => [server.serverId, server]));
          updateServerPresentations();
        } catch {
          // Configuration remains editable when health observation is unavailable.
        }
      }

      async function save() {
        if (!workspaceApi?.save) {
          setStatus("error", "Workspace settings bridge is unavailable.");
          return false;
        }
        els.saveButton.disabled = true;
        setStatus("info", "Saving settings…");
        try {
          const snapshot = await workspaceApi.save(collectPayload());
          applySnapshot(snapshot);
          setStatus("success", "Settings saved.");
          return true;
        } catch (error) {
          setStatus("error", error instanceof Error ? error.message : String(error));
          return false;
        } finally {
          els.saveButton.disabled = false;
        }
      }

      els.navButtons.forEach((button) => {
        button.addEventListener("click", () => setSection(button.dataset.target));
      });
      els.theme.addEventListener("change", syncNativeTheme);
      els.addServer.addEventListener("click", () => {
        state.snapshot.servers.push(createEmptyServer());
        renderServers();
        setStatus("info", "Added a new server entry.");
      });
      els.defaultExecutionServer.addEventListener("change", () => {
        const selected = Number(els.defaultExecutionServer.value);
        (state.snapshot?.servers ?? []).forEach((server, index) => {
          server.default_execution = index === selected;
        });
        setStatus("info", "Default execution server changed.");
      });
      els.closeSettings.addEventListener("click", () => {
        window.agentGridWorkspace?.windowControls?.close?.();
      });
      els.saveButton.addEventListener("click", save);

      function renderGitHub(r) {
        if (!r || !els.ghState) return;
        const a = r.account || {}, c = r.catalog || {};
        const stateLabel =
          a.state === "authenticated" ? "Connected to GitHub" :
          a.state === "authorizing" ? "Authorization in progress" :
          a.state === "host_misconfigured" ? "GitHub App configuration required" :
          a.state === "unauthenticated" ? "Connect your GitHub account" :
          "GitHub connection needs attention";
        els.ghState.textContent = stateLabel;
        els.ghDetail.textContent = a.login || (a.state === "authenticated" ? "Connected" : "Not connected");
        els.ghCatalog.textContent = (c.repositories ? c.repositories.length : 0) + " repositories · " + (c.outcome || "empty");
        els.ghPill.dataset.state = a.state || "unauthenticated";
        els.ghPill.textContent = (a.state || "unknown").replaceAll("_", " ");
        if (els.ghCount) els.ghCount.textContent = a.state || "";
        const hostMisconfigured = a.state === "host_misconfigured"
          && a.failure_code === "host_misconfigured";
        if (els.ghOpenBrowser) els.ghOpenBrowser.hidden = a.state !== "authorizing";
        els.ghHint.textContent = hostMisconfigured
          ? "GitHub rejected this OAuth registration. Review App ID and Client ID below, save, then connect again."
          : a.state === "authorizing"
          ? ((a.user_code ? "Code " + a.user_code + " — " : "") + (a.verification_uri || "Authorize in browser."))
          : (r.outcome === "denied" || r.outcome === "cancel" || r.outcome === "failure"
            ? ("Outcome: " + r.outcome + (a.failure_code ? " (" + a.failure_code + ")" : "")) : "");
      }
      async function githubAction(action) {
        if (!githubApi?.action) { setStatus("error", "GitHub settings bridge is unavailable."); return null; }
        try { const r = await githubApi.action(action); renderGitHub(r); return r; }
        catch { setStatus("error", "GitHub action could not be completed."); return null; }
      }
      const stopGh = () => { if (ghPoll) { clearInterval(ghPoll); ghPoll = null; } };
      const observeGitHubStatus = () => {
        if (ghPoll) return;
        ghPoll = setInterval(async () => {
          const p = await githubAction("status");
          if (p?.account?.state && p.account.state !== "authorizing") {
            stopGh(); setStatus(p.outcome === "success" ? "success" : "error", "GitHub " + p.outcome);
          }
        }, 3000);
      };
      els.ghConnect?.addEventListener("click", async () => {
        stopGh();
        if (!await save()) return;
        const s = await githubAction("start");
        if (!s || s.account?.state !== "authorizing") return;
        setStatus(
          s.reason === "browser_handoff_failed" ? "error" : "info",
          s.reason === "browser_handoff_failed"
            ? "GitHub could not be opened. The authorization code was copied; use Open GitHub to retry."
            : "Authorization code copied. Waiting for GitHub authorization…",
        );
        observeGitHubStatus();
      });
      els.ghOpenBrowser?.addEventListener("click", async () => {
        if (!githubApi?.openAuthorization) {
          setStatus("error", "GitHub browser handoff is unavailable.");
          return;
        }
        try { await githubApi.openAuthorization(); }
        catch { setStatus("error", "GitHub authorization page could not be opened."); }
      });
      els.ghCancel?.addEventListener("click", async () => { stopGh(); const r = await githubAction("cancel"); if (r) setStatus("info", "GitHub " + r.outcome); });
      els.ghRefresh?.addEventListener("click", async () => { const r = await githubAction("refresh_catalog"); if (r) setStatus("info", "Catalog " + (r.catalog?.outcome || r.outcome)); });
      void githubAction("status").then((r) => {
        if (r?.account?.state === "authorizing") observeGitHubStatus();
      });
      els.servers.addEventListener("input", (event) => {
        const target = event.target;
        if (!(target instanceof HTMLInputElement)) return;
        if (target.type === "checkbox") return;
        const index = Number(target.dataset.index);
        const field = target.dataset.field;
        if (!Number.isInteger(index) || !field) return;
        updateServer(index, field, target.value);
      });
      els.servers.addEventListener("change", (event) => {
        const target = event.target;
        if (target instanceof HTMLSelectElement) {
          const index = Number(target.dataset.index);
          const field = target.dataset.field;
          if (Number.isInteger(index) && field) updateServer(index, field, target.value);
          return;
        }
        if (target instanceof HTMLInputElement && target.type === "checkbox") {
          const index = Number(target.dataset.index);
          const field = target.dataset.field;
          if (Number.isInteger(index) && field) updateServer(index, field, target.checked);
        }
      });
      els.servers.addEventListener("click", async (event) => {
        const target = event.target;
        if (!(target instanceof HTMLElement)) return;
        const index = Number(target.dataset.index);
        if (!Number.isInteger(index)) return;
        if (target.dataset.action === "reconnect-server") {
          const server = state.snapshot?.servers?.[index];
          if (!server?.enabled || !server.id || state.reconnecting.has(server.id)) return;
          if (!fleetApi?.reconnect) {
            setStatus("error", "Workspace reconnect bridge is unavailable.");
            return;
          }
          const serverId = server.id;
          const label = serverSummary(server);
          state.reconnecting.add(serverId);
          target.setAttribute("disabled", "");
          target.setAttribute("aria-busy", "true");
          target.textContent = "Reconnecting…";
          setStatus("info", "Reconnecting " + label + "…");
          try {
            await fleetApi.reconnect(serverId);
            setStatus("success", "Reconnect started for " + label + ".");
            await refreshHealth();
          } catch {
            setStatus("error", "Reconnect failed for " + label + ".");
          } finally {
            state.reconnecting.delete(serverId);
            const buttons = Array.from(
              els.servers.querySelectorAll('[data-action="reconnect-server"]'),
            );
            const button = buttons.find((candidate) => candidate.dataset.serverId === serverId);
            if (button) {
              button.removeAttribute("disabled");
              button.removeAttribute("aria-busy");
              button.textContent = "Reconnect";
            }
          }
          return;
        }
        if (target.dataset.action !== "remove-server") return;
        state.snapshot.servers.splice(index, 1);
        renderServers();
        setStatus("info", "Removed server entry.");
      });

      const sectionFromHash = () => setSection(window.location.hash.slice(1));
      window.addEventListener("hashchange", sectionFromHash);
      sectionFromHash();
      load();
      setInterval(refreshHealth, 5000);
    </script>
  </body>
</html>`;
}

export function previewServerLabel(value: string): string {
  return escapeHtml(value);
}
