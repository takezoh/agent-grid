import { describe, expect, it } from "vitest";
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
});
