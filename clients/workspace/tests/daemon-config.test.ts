import * as fs from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";
import { describe, expect, it } from "vitest";
import { DaemonConfigResolver } from "../src/main/daemon-config.js";
import { sessionPageUrl } from "../src/main/electron-window-factory.js";
import { assertTokenNotInUrl, buildPreloadApi } from "../src/preload/index.js";

describe("daemon-config", () => {
  it("reads token fresh each resolve", async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), "ag-tok-"));
    const tokenPath = path.join(dir, "gateway-token");
    await fs.writeFile(tokenPath, "v1\n");
    const r = new DaemonConfigResolver({
      serverId: "local",
      tokenPath,
      baseUrl: "http://127.0.0.1:8443",
    });
    expect((await r.resolve("local")).token).toBe("v1");
    await fs.writeFile(tokenPath, "v2\n");
    expect((await r.resolve("local")).token).toBe("v2");
  });

  it("throws explicit error when unreadable", async () => {
    const r = new DaemonConfigResolver({
      serverId: "local",
      tokenPath: path.join(os.tmpdir(), `missing-${Date.now()}`),
      baseUrl: "http://127.0.0.1:8443",
    });
    await expect(r.resolve("local")).rejects.toThrow(/unreadable/);
  });

  it("selects the configured connection by serverId", async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), "ag-tok-multi-"));
    const one = path.join(dir, "one");
    const two = path.join(dir, "two");
    await fs.writeFile(one, "token-one");
    await fs.writeFile(two, "token-two");
    const resolver = new DaemonConfigResolver([
      {
        serverId: "one",
        tokenPath: one,
        baseUrl: "http://one.test",
      },
      {
        serverId: "two",
        tokenPath: two,
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
        shell: { workspace_executable: "agent-grid-workspace", health_poll_interval_seconds: 5 },
        workspace: { idle_quit_seconds: 300, default_window: { width: 1280, height: 800 } },
      });
    const save = () => load();
    const api = buildPreloadApi({
      hostedModeInfo: null,
      windowControls: { minimize() {}, maximize() {}, close() {} },
      requestJumpBack: async () => {},
      desktopConfig: { load, save },
    });

    expect(api.desktopConfig).toBeDefined();
    await expect(api.desktopConfig?.load()).resolves.toMatchObject({
      configDirectory: "C:\\config",
      appearance: { theme: "dark" },
    });
  });
});
