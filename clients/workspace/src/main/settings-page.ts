function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

export function settingsPageUrl(): string {
  return `data:text/html;charset=utf-8,${encodeURIComponent(buildSettingsPageHtml())}`;
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

        <section id="github" class="section" data-active="false"><div class="group">
          <div class="group__header"><h2>GitHub account</h2><p>Host-owned device flow; credentials stay in main.</p></div>
          <div class="group__body"><p id="github-account-state" class="sidebar__meta">Checking…</p>
            <p id="github-account-detail" class="sidebar__meta"></p>
            <div class="toolbar" style="margin-top:12px">
              <button id="github-connect" type="button" class="btn btn--primary">Connect</button>
              <button id="github-cancel" type="button" class="btn">Cancel</button>
              <button id="github-refresh" type="button" class="btn">Refresh catalog</button></div>
            <p id="github-device-hint" class="sidebar__meta" style="margin-top:12px"></p></div></div></section>

        <div class="toolbar">
          <button id="save-button" type="button" class="btn btn--primary">Save settings</button>
          <span id="status" class="status" data-level="info">Changes are written to the shared desktop client configuration.</span>
        </div>
      </main>
    </div>

    <script>
      const workspaceApi = window.agentGridWorkspace?.desktopConfig;
      const healthApi = window.agentGridWorkspace?.serverHealth;
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
        github: { title: "GitHub", description: "Host-owned GitHub device flow. Credentials stay in the main process." },
      };
      const state = {
        snapshot: null,
        activeSection: "connections",
        health: new Map(),
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
        addServer: document.getElementById("add-server"),
        closeSettings: document.getElementById("close-settings"),
        theme: document.getElementById("theme"),
        density: document.getElementById("density"),
        fontScale: document.getElementById("font-scale"),
        idleQuitSeconds: document.getElementById("idle-quit-seconds"),
        windowWidth: document.getElementById("window-width"),
        windowHeight: document.getElementById("window-height"),
        ghState: document.getElementById("github-account-state"),
        ghDetail: document.getElementById("github-account-detail"),
        ghHint: document.getElementById("github-device-hint"),
        ghConnect: document.getElementById("github-connect"),
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
          url: "http://127.0.0.1:8443",
          auth: { mode: "none" },
        };
      }

      function setSection(target) {
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
          healthState === "checking" ? "Checking…" : "Unavailable";
        const healthDetail = health?.reason ? " — " + health.reason : "";
        return {
          state: healthState,
          label: healthLabel + healthDetail,
          detail: healthDetail,
        };
      }

      function renderServers() {
        const servers = state.snapshot?.servers ?? [];
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
        els.windowWidth.value = String(snapshot.workspace.default_window.width);
        els.windowHeight.value = String(snapshot.workspace.default_window.height);
        renderServers();
        syncNativeTheme();
      }

      function updateServer(index, field, value) {
        const server = state.snapshot?.servers?.[index];
        if (!server) return;
        if (field === "enabled") {
          server.enabled = Boolean(value);
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
        }
      }

      function collectPayload() {
        return {
          servers: (state.snapshot?.servers ?? []).map((server) => {
            return {
              display_name: (server.display_name || "").trim(),
              enabled: Boolean(server.enabled),
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
          workspace: {
            idle_quit_seconds: Number(els.idleQuitSeconds.value),
            default_window: {
              width: Number(els.windowWidth.value),
              height: Number(els.windowHeight.value),
            },
          },
        };
      }

      async function load() {
        if (!workspaceApi?.load) {
          setStatus("error", "Workspace settings bridge is unavailable.");
          return;
        }
        try {
          const snapshot = await workspaceApi.load();
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
          return;
        }
        els.saveButton.disabled = true;
        setStatus("info", "Saving settings…");
        try {
          const snapshot = await workspaceApi.save(collectPayload());
          applySnapshot(snapshot);
          setStatus("success", "Settings saved.");
        } catch (error) {
          setStatus("error", error instanceof Error ? error.message : String(error));
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
      els.closeSettings.addEventListener("click", () => {
        window.agentGridWorkspace?.windowControls?.close?.();
      });
      els.saveButton.addEventListener("click", save);

      function renderGitHub(r) {
        if (!r || !els.ghState) return;
        const a = r.account || {}, c = r.catalog || {};
        els.ghState.textContent = "Account: " + (a.state || "?") + (a.login ? " (" + a.login + ")" : "");
        els.ghDetail.textContent = "Catalog: " + (c.outcome || "empty") + (c.repositories ? " · " + c.repositories.length : "");
        if (els.ghCount) els.ghCount.textContent = a.state || "";
        els.ghHint.textContent = a.state === "authorizing"
          ? ((a.user_code ? "Code " + a.user_code + " — " : "") + (a.verification_uri || "Authorize in browser."))
          : (r.outcome === "denied" || r.outcome === "cancel" || r.outcome === "failure"
            ? ("Outcome: " + r.outcome + (a.failure_code ? " (" + a.failure_code + ")" : "")) : "");
      }
      async function githubAction(action) {
        if (!githubApi?.action) { setStatus("error", "GitHub settings bridge is unavailable."); return null; }
        try { const r = await githubApi.action(action); renderGitHub(r); return r; }
        catch (e) { setStatus("error", e instanceof Error ? e.message : String(e)); return null; }
      }
      const stopGh = () => { if (ghPoll) { clearInterval(ghPoll); ghPoll = null; } };
      els.ghConnect?.addEventListener("click", async () => {
        stopGh();
        const s = await githubAction("start");
        if (!s || s.account?.state !== "authorizing") return;
        setStatus("info", "Waiting for GitHub authorization…");
        ghPoll = setInterval(async () => {
          const p = await githubAction("poll");
          if (p?.account?.state && p.account.state !== "authorizing") {
            stopGh(); setStatus(p.outcome === "success" ? "success" : "error", "GitHub " + p.outcome);
          }
        }, 3000);
      });
      els.ghCancel?.addEventListener("click", async () => { stopGh(); const r = await githubAction("cancel"); if (r) setStatus("info", "GitHub " + r.outcome); });
      els.ghRefresh?.addEventListener("click", async () => { const r = await githubAction("refresh_catalog"); if (r) setStatus("info", "Catalog " + (r.catalog?.outcome || r.outcome)); });
      void githubAction("status");
      els.servers.addEventListener("input", (event) => {
        const target = event.target;
        if (!(target instanceof HTMLInputElement)) return;
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
      els.servers.addEventListener("click", (event) => {
        const target = event.target;
        if (!(target instanceof HTMLElement)) return;
        if (target.dataset.action !== "remove-server") return;
        const index = Number(target.dataset.index);
        if (!Number.isInteger(index)) return;
        state.snapshot.servers.splice(index, 1);
        renderServers();
        setStatus("info", "Removed server entry.");
      });

      load();
      setInterval(refreshHealth, 5000);
    </script>
  </body>
</html>`;
}

export function previewServerLabel(value: string): string {
  return escapeHtml(value);
}
