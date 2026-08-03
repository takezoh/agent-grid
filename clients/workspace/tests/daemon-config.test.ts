import { describe, expect, it } from "vitest";
import { DaemonConfigResolver } from "../src/main/daemon-config.js";
import { sessionPageUrl } from "../src/main/electron-window-factory.js";
import { assertTokenNotInUrl, buildPreloadApi } from "../src/preload/index.js";

describe("daemon-config", () => {
  it("resolves the persisted bearer token", async () => {
    const r = new DaemonConfigResolver({
      serverId: "local",
      auth: { mode: "bearer", token: "v1" },
      baseUrl: "http://127.0.0.1:8443",
    });
    expect((await r.resolve("local")).token).toBe("v1");
  });

  it("resolves no-auth without fabricating a token", async () => {
    const r = new DaemonConfigResolver({
      serverId: "local",
      auth: { mode: "none" },
      baseUrl: "http://127.0.0.1:8443",
    });
    await expect(r.resolve("local")).resolves.toMatchObject({ token: "" });
  });

  it("selects the configured connection by serverId", async () => {
    const resolver = new DaemonConfigResolver([
      {
        serverId: "one",
        auth: { mode: "bearer", token: "token-one" },
        baseUrl: "http://one.test",
      },
      {
        serverId: "two",
        auth: { mode: "bearer", token: "token-two" },
        baseUrl: "http://two.test",
      },
    ]);

    expect(await resolver.resolve("one")).toMatchObject({
      baseUrl: "http://one.test",
      token: "token-one",
    });
    expect(await resolver.resolve("two")).toMatchObject({
      baseUrl: "http://two.test",
      token: "token-two",
    });
  });

  it("builds a local bundled UI URL without the token", () => {
    const url = sessionPageUrl("file:///app/ui/index.html", "sess-9");
    expect(url).toMatch(/^file:/);
    expect(url).toContain("hosted=1");
    expect(url).toContain("session=sess-9");
    expect(url).not.toContain("token=");
    assertTokenNotInUrl(url, "super-secret-token");
  });

  it("rejects a remote UI entry", () => {
    expect(() => sessionPageUrl("https://ui.example.test", "sess-9")).toThrow(/file:/);
  });
});

describe("assertTokenNotInUrl", () => {
  it("throws when token leaks", () => {
    expect(() =>
      assertTokenNotInUrl("http://x/?token=abc", "abc"),
    ).toThrow(/token/);
  });
});

describe("buildPreloadApi", () => {
  it("can expose the desktop settings bridge without widening Node access", async () => {
    const load = () =>
      Promise.resolve({
        configDirectory: "C:\\config",
        servers: [],
        appearance: { theme: "dark", density: "compact", font_scale: 1 },
        shell: {
          workspace_executable: "agent-grid-workspace",
          health_poll_interval_seconds: 5,
          display_anchor: null,
        },
        workspace: { idle_quit_seconds: 300, default_window: { width: 1280, height: 800 } },
        github: { app_id: "4460068", client_id: "Iv23li471HWU8jxRwlK7", scopes: ["read:user", "repo"] },
      });
    const save = () => load();
    const api = buildPreloadApi({
      hostedModeInfo: null,
      windowControls: { minimize() {}, maximize() {}, close() {} },
      requestJumpBack: async () => {},
      desktopConfig: { load, save },
      displays: { list: async () => [] },
    });

    expect(api.desktopConfig).toBeDefined();
    expect(api.displays).toBeDefined();
    await expect(api.desktopConfig?.load()).resolves.toMatchObject({
      configDirectory: "C:\\config",
      appearance: { theme: "dark" },
    });
  });
});
