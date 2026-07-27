import { describe, expect, it } from "vitest";
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
      reason: "incompatible protocol: client protocol-2, server protocol-1",
    });

    expect(window.document.querySelector('[data-role="server-health"]')?.textContent).toBe(
      "Incompatible protocol — incompatible protocol: client protocol-2, server protocol-1",
    );
    await close();
  });
});

async function loadSettingsPage(
  initialHealth: { state: string; reason: string } = {
    state: "unavailable",
    reason: "starting",
  },
): Promise<{
  window: BrowserWindow;
  runHealthRefresh: () => Promise<void>;
  close: () => Promise<void>;
}> {
  const browser = new Browser({
    settings: { disableJavaScriptEvaluation: false },
    console,
  });
  const page = browser.newPage();
  const window = page.mainFrame.window;
  let intervalCallback: (() => Promise<void>) | undefined;
  let healthCalls = 0;
  window.setInterval = ((callback: TimerHandler) => {
    intervalCallback = async () => {
      if (typeof callback === "function") await callback();
    };
    return 1;
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
            url: "http://127.0.0.1:8443",
            auth: { mode: "bearer", token: "test-token" },
          }],
          appearance: { theme: "dark", density: "comfortable", font_scale: 1 },
          workspace: {
            idle_quit_seconds: 300,
            default_window: { width: 1280, height: 800 },
          },
        }),
        save: async (settings: unknown) => settings,
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
      expect(intervalCallback).toBeTypeOf("function");
      await intervalCallback!();
    },
    close: () => browser.close(),
  };
}
