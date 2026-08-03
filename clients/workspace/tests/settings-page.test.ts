import { describe, expect, it, vi } from "vitest";
import { Browser, type BrowserWindow } from "happy-dom";
import {
  buildSettingsPageHtml,
  previewServerLabel,
  settingsPageUrl,
} from "../src/main/settings-page.js";

describe("settings page", () => {
  it("builds a VSCode-style settings document with a category tree", () => {
    const html = buildSettingsPageHtml();
    expect(html).toContain(">Connections<");
    expect(html).toContain(">Appearance<");
    expect(html).toContain(">Workspace<");
    expect(html).toContain("Save settings");
    expect(html).toContain("Default execution server");
    expect(html).toContain('id="default-execution-server"');
    expect(html).toContain("GitHub App configuration");
    expect(html).toContain('id="github-app-id"');
    expect(html).toContain('id="github-client-id"');
    expect(html).toContain("No client secret is required");
    expect(html).toContain(">Close<");
    expect(html).toContain("desktopConfig");
    expect(html).toContain("These entries are not a switcher");
    expect(html).toContain("windowControls?.close");
    expect(html).toContain("<label>Display name</label>");
    expect(html).not.toContain("<label>Identifier</label>");
    expect(html).not.toContain('data-field="id"');
    expect(html).toContain('<input type="password"');
    expect(html).not.toContain("Token path");
  });

  it("packs the page as a data URL", () => {
    const url = settingsPageUrl();
    expect(url.startsWith("data:text/html;charset=utf-8,")).toBe(true);
    expect(decodeURIComponent(url)).toContain("Settings");
  });

  it("targets the requested Settings section in the data URL", () => {
    expect(settingsPageUrl("github")).toMatch(/#github$/);
  });

  it("escapes preview labels used inside the server list", () => {
    expect(previewServerLabel('<img src=x onerror="pwn">')).toBe(
      "&lt;img src=x onerror=&quot;pwn&quot;&gt;",
    );
  });

  it.each(["display_name", "url", "auth_token"])(
    "preserves the focused %s input while editing and refreshing health",
    async (field) => {
      const { window, runHealthRefresh, close } = await loadSettingsPage();
      const input = window.document.querySelector<HTMLInputElement>(
        `input[data-field="${field}"]`,
      );
      expect(input, window.document.getElementById("status")?.textContent ?? "").not.toBeNull();
      input!.focus();
      input!.value += "x";
      input!.dispatchEvent(new window.Event("input", { bubbles: true }));

      expect(window.document.activeElement).toBe(input);
      expect(window.document.querySelector(`input[data-field="${field}"]`)).toBe(input);
      if (field === "display_name") {
        expect(window.document.querySelector('[data-role="server-summary"]')?.textContent)
          .toBe("Localx");
      }

      await runHealthRefresh();

      expect(window.document.activeElement).toBe(input);
      expect(window.document.querySelector(`input[data-field="${field}"]`)).toBe(input);
      expect(input!.value.endsWith("x")).toBe(true);
      expect(window.document.querySelector('[data-role="server-health"]')?.textContent)
        .toBe("Connected");
      await close();
    },
  );

  it("presents protocol mismatch as typed incompatibility", async () => {
    const { window, close } = await loadSettingsPage({
      state: "incompatible_protocol",
      reason: "incompatible protocol: client protocol-3, server protocol-1",
    });

    expect(window.document.querySelector('[data-role="server-health"]')?.textContent).toBe(
      "Incompatible protocol — incompatible protocol: client protocol-3, server protocol-1",
    );
    await close();
  });

  it("presents a credential-free typed GitHub host configuration error", async () => {
    const unsafeReason = "status 404 client_id=Ov23liAgentGridDeviceFlow";
    const unsafeDetail = "access_token=gho_renderer_leak";
    const unsafeCode = "LEAK-CODE";
    const unsafeUri = "https://attacker.invalid/device";
    const { window, close } = await loadSettingsPage(undefined, async () => ({
      outcome: "failure",
      account: {
        state: "host_misconfigured",
        failure_code: "host_misconfigured",
        user_code: unsafeCode,
        verification_uri: unsafeUri,
      },
      catalog: { outcome: "empty", generation: 0, stale: false, repositories: [] },
      reason: unsafeReason,
      detail: unsafeDetail,
    }));

    window.document.querySelector<HTMLButtonElement>("#github-connect")?.click();
    await new Promise<void>((resolve) => setImmediate(resolve));

    expect(window.document.querySelector("#github-device-hint")?.textContent).toBe(
      "GitHub rejected this OAuth registration. Review App ID and Client ID below, save, then connect again.",
    );
    const visible = window.document.body.textContent ?? "";
    for (const unsafe of [unsafeReason, unsafeDetail, unsafeCode, unsafeUri]) {
      expect(visible).not.toContain(unsafe);
    }
    expect(visible).not.toMatch(/status 404|access[_-]?token/i);
    await close();
  });

  it("observes main-owned GitHub status after start without renderer token polling", async () => {
    const actions: string[] = [];
    let started = false;
    const syntheticCode = "TEST-CODE";
    const syntheticUri = "https://oauth.test.invalid/device";
    const githubAction = async (action: string): Promise<unknown> => {
      actions.push(action);
      if (action === "start") {
        started = true;
        return {
          outcome: "authorizing",
          account: {
            state: "authorizing",
            user_code: syntheticCode,
            verification_uri: syntheticUri,
          },
          catalog: { outcome: "empty", generation: 0, stale: false, repositories: [] },
        };
      }
      if (action === "status" && started) {
        return {
          outcome: "success",
          account: { state: "authenticated", login: "fixture-user" },
          catalog: {
            outcome: "complete-current",
            generation: 1,
            stale: false,
            repositories: [{ repository_id: "fixture-repo", owner: "fixture", name: "demo" }],
          },
        };
      }
      return {
        outcome: "unauthenticated",
        account: { state: "unauthenticated" },
        catalog: { outcome: "empty", generation: 0, stale: false, repositories: [] },
      };
    };
    const { window, runGitHubPoll, close } = await loadSettingsPage(undefined, githubAction);

    expect(actions).toEqual(["status"]);
    window.document.querySelector<HTMLButtonElement>("#github-connect")?.click();
    await new Promise<void>((resolve) => setImmediate(resolve));

    const authorizing = window.document.querySelector("#github-device-hint")?.textContent ?? "";
    expect(actions).toEqual(["status", "start"]);
    expect(authorizing).toContain(syntheticCode);
    expect(authorizing).toContain(syntheticUri);

    await runGitHubPoll();

    // Counterexample rejection: renderer-owned token polling would emit "poll";
    // Settings now observes the main-process-owned state machine via "status".
    expect(actions).toEqual(["status", "start", "status"]);
    expect(actions).not.toContain("poll");
    expect(window.document.querySelector("#github-account-state")?.textContent).toBe("Connected to GitHub");
    expect(window.document.querySelector("#github-account-detail")?.textContent).toBe(
      "fixture-user",
    );
    expect(window.document.querySelector("#github-catalog-detail")?.textContent).toBe(
      "1 repositories · complete-current",
    );
    const authenticatedBody = window.document.body.textContent ?? "";
    expect(authenticatedBody).not.toContain(syntheticCode);
    expect(authenticatedBody).not.toContain(syntheticUri);
    expect(authenticatedBody).not.toMatch(
      /access[_-]?token|refresh[_-]?token|client[_-]?(?:id|secret)|github_pat_|gh[opusr]_/i,
    );
    await close();
  });

  it("provides a safe browser retry action while device authorization is pending", async () => {
    const openAuthorization = vi.fn(async () => undefined);
    const { window, close } = await loadSettingsPage(
      undefined,
      async (action) => action === "start"
        ? {
            outcome: "authorizing",
            account: {
              state: "authorizing",
              user_code: "TEST-CODE",
              verification_uri: "https://github.com/login/device",
            },
            catalog: { outcome: "empty", generation: 0, stale: false, repositories: [] },
          }
        : {
            outcome: "unauthenticated",
            account: { state: "unauthenticated" },
            catalog: { outcome: "empty", generation: 0, stale: false, repositories: [] },
          },
      openAuthorization,
    );

    window.document.querySelector<HTMLButtonElement>("#github-connect")?.click();
    await new Promise<void>((resolve) => setImmediate(resolve));
    const openButton = window.document.querySelector<HTMLButtonElement>("#github-open-browser");
    expect(openButton?.hidden).toBe(false);
    openButton?.click();
    await new Promise<void>((resolve) => setImmediate(resolve));
    expect(openAuthorization).toHaveBeenCalledTimes(1);
    await close();
  });

  it("loads and saves editable public GitHub App identifiers with fixed scopes", async () => {
    const { window, savedSettings, close } = await loadSettingsPage();
    const appId = window.document.querySelector<HTMLInputElement>("#github-app-id")!;
    const clientId = window.document.querySelector<HTMLInputElement>("#github-client-id")!;
    expect(appId.value).toBe("4460068");
    expect(clientId.value).toBe("Iv23li471HWU8jxRwlK7");

    appId.value = "9000001";
    clientId.value = "Iv23liUpdatedClient99";
    window.document.querySelector<HTMLButtonElement>("#save-button")?.click();
    await new Promise<void>((resolve) => setImmediate(resolve));

    expect(savedSettings()).toMatchObject({
      github: {
        app_id: "9000001",
        client_id: "Iv23liUpdatedClient99",
        scopes: ["read:user", "repo"],
      },
    });
    await close();
  });

  it("saves the selected default execution server", async () => {
    const { window, savedSettings, close } = await loadSettingsPage();
    const select = window.document.querySelector<HTMLSelectElement>("#default-execution-server")!;
    expect(select.value).toBe("0");

    select.value = "1";
    select.dispatchEvent(new window.Event("change", { bubbles: true }));
    window.document.querySelector<HTMLButtonElement>("#save-button")?.click();
    await new Promise<void>((resolve) => setImmediate(resolve));

    expect(savedSettings()).toMatchObject({
      servers: [
        { display_name: "Local", default_execution: false },
        { display_name: "Remote", default_execution: true },
      ],
    });
    await close();
  });
});

async function loadSettingsPage(
  initialHealth: { state: string; reason: string } = {
    state: "unavailable",
    reason: "starting",
  },
  githubAction: (action: string) => Promise<unknown> = async () => ({
    outcome: "unauthenticated",
    account: { state: "unauthenticated" },
    catalog: { outcome: "empty", generation: 0, stale: false, repositories: [] },
  }),
  openAuthorization: () => Promise<void> = async () => undefined,
): Promise<{
  window: BrowserWindow;
  runHealthRefresh: () => Promise<void>;
  runGitHubPoll: () => Promise<void>;
  savedSettings: () => unknown;
  close: () => Promise<void>;
}> {
  const browser = new Browser({
    settings: { disableJavaScriptEvaluation: false },
    console,
  });
  const page = browser.newPage();
  const window = page.mainFrame.window;
  const intervalCallbacks = new Map<number, () => Promise<void>>();
  let healthCalls = 0;
  let lastSavedSettings: unknown = null;
  window.setInterval = ((callback: TimerHandler, delay?: number) => {
    intervalCallbacks.set(Number(delay), async () => {
      if (typeof callback === "function") await callback();
    });
    return Number(delay) || 1;
  }) as typeof window.setInterval;
  Object.assign(window, {
    agentGridWorkspace: {
      desktopConfig: {
        load: async () => ({
          configDirectory: "C:\\agent-grid\\config",
          servers: [{
            id: "srv_local",
            display_name: "Local",
            enabled: true,
            default_execution: true,
            url: "http://127.0.0.1:8443",
            auth: { mode: "bearer", token: "test-token" },
          }, {
            id: "srv_remote",
            display_name: "Remote",
            enabled: true,
            default_execution: false,
            url: "https://example.test",
            auth: { mode: "none" },
          }],
          appearance: { theme: "dark", density: "comfortable", font_scale: 1 },
          workspace: {
            idle_quit_seconds: 300,
            default_window: { width: 1280, height: 800 },
          },
          github: {
            app_id: "4460068",
            client_id: "Iv23li471HWU8jxRwlK7",
            scopes: ["read:user", "repo"],
          },
        }),
        save: async (settings: unknown) => {
          lastSavedSettings = settings;
          return {
            ...(settings as object),
            configDirectory: "C:\\agent-grid\\config",
            shell: { workspace_executable: "agent-grid-workspace", health_poll_interval_seconds: 5 },
          };
        },
      },
      serverHealth: {
        list: async () => {
          healthCalls += 1;
          return {
            servers: [{
              serverId: "srv_local",
              state: healthCalls === 1 ? initialHealth.state : "connected",
              reason: healthCalls === 1 ? initialHealth.reason : "",
            }],
          };
        },
      },
      github: { action: githubAction, openAuthorization },
      setTitleBarTheme: () => undefined,
      windowControls: {
        close: () => undefined,
      },
    },
  });
  const html = buildSettingsPageHtml();
  const scriptMatch = html.match(/<script>([\s\S]*)<\/script>/);
  if (!scriptMatch) throw new Error("settings page script is missing");
  page.content = html.replace(scriptMatch[0], "");
  page.evaluate(scriptMatch[1]);
  await new Promise<void>((resolve) => setImmediate(resolve));
  return {
    window,
    runHealthRefresh: async () => {
      const callback = intervalCallbacks.get(5000);
      expect(callback).toBeTypeOf("function");
      await callback!();
    },
    runGitHubPoll: async () => {
      const callback = intervalCallbacks.get(3000);
      expect(callback).toBeTypeOf("function");
      await callback!();
    },
    savedSettings: () => lastSavedSettings,
    close: () => browser.close(),
  };
}
