import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  loadOrCreateDesktopConfig,
  resolveConfigDirectory,
  saveEditableDesktopSettings,
  serverIdentifier,
} from "../src/main/desktop-config.js";

const directories: string[] = [];

function tempDir(): string {
  const value = fs.mkdtempSync(path.join(os.tmpdir(), "agent-grid-config-"));
  directories.push(value);
  return value;
}

afterEach(() => {
  for (const directory of directories.splice(0)) {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});

describe("desktop config", () => {
  it("creates and loads desktop defaults including editable GitHub registration", () => {
    const directory = tempDir();
    const config = loadOrCreateDesktopConfig(directory);

    expect(config.servers.map((server) => server.id)).toEqual([
      "srv_ca5ed80c77e645b7c7ec83189f36bfa835128debf18096e46c6300fdc17e8c3a",
    ]);
    expect(config.servers[0]?.default_execution).toBe(true);
    expect(config.appearance).toEqual({
      theme: "default",
      density: "comfortable",
      font_scale: 1,
    });
    expect(config.shell.display_anchor).toBeNull();
    expect(config.github).toEqual({
      app_id: "4460068",
      client_id: "Iv23li471HWU8jxRwlK7",
      scopes: ["read:user", "repo"],
    });
    expect(fs.readdirSync(directory).sort()).toEqual([
      "appearance.json",
      "github.json",
      "servers.json",
      "shell.json",
      "workspace.json",
    ]);
    expect(JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8")).servers[0])
      .not.toHaveProperty("launch");
    expect(JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8")).servers[0])
      .not.toHaveProperty("id");
    if (process.platform !== "win32") {
      expect(fs.statSync(path.join(directory, "servers.json")).mode & 0o777).toBe(0o600);
    }
  });

  it("resolves --config-dir for isolated test configuration", () => {
    const directory = tempDir();
    expect(resolveConfigDirectory(["--config-dir", directory])).toBe(path.resolve(directory));
  });

  it("loads all enabled and disabled server entries", () => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    fs.writeFileSync(
      path.join(directory, "servers.json"),
      JSON.stringify({
        schema_version: 2,
        servers: [
          server("One", true, "http://127.0.0.1:8443"),
          server("Two", false, "https://example.test"),
        ],
      }),
    );

    const config = loadOrCreateDesktopConfig(directory);
    expect(config.servers.map(({ id, enabled }) => ({ id, enabled }))).toEqual([
      {
        id: "srv_ca5ed80c77e645b7c7ec83189f36bfa835128debf18096e46c6300fdc17e8c3a",
        enabled: true,
      },
      {
        id: "srv_f1e2c7da767e6e0fc3eac0819fdd3a622967f18748c6f0a875c3a5b0a165031e",
        enabled: false,
      },
    ]);
    expect(config.servers.map((entry) => entry.default_execution)).toEqual([true, false]);
  });

  it("rejects a disabled or duplicate default execution server", () => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    const file = path.join(directory, "servers.json");
    const one = server("One", true, "http://127.0.0.1:8443");
    const two = server("Two", true, "https://example.test");
    fs.writeFileSync(file, JSON.stringify({
      schema_version: 2,
      servers: [
        { ...one, default_execution: true },
        { ...two, default_execution: true },
      ],
    }));
    expect(() => loadOrCreateDesktopConfig(directory)).toThrow(/exactly one enabled server/);

    fs.writeFileSync(file, JSON.stringify({
      schema_version: 2,
      servers: [{ ...one, enabled: false, default_execution: true }, two],
    }));
    expect(() => loadOrCreateDesktopConfig(directory)).toThrow(/must be enabled/);
  });

  it("does not replace invalid user configuration", () => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    const file = path.join(directory, "appearance.json");
    fs.writeFileSync(file, '{"schema_version":1,"theme":"blue"}');

    expect(() => loadOrCreateDesktopConfig(directory)).toThrow(/appearance\.json/);
    expect(fs.readFileSync(file, "utf8")).toBe('{"schema_version":1,"theme":"blue"}');
  });

  it("rejects legacy token_path configuration without rewriting it", () => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    const file = path.join(directory, "servers.json");
    const legacy = JSON.stringify({
      schema_version: 1,
      servers: [{
        display_name: "Legacy",
        enabled: true,
        url: "http://127.0.0.1:8443",
        token_path: "gateway-token",
      }],
    });
    fs.writeFileSync(file, legacy);

    expect(() => loadOrCreateDesktopConfig(directory)).toThrow(/token_path is no longer supported/);
    expect(fs.readFileSync(file, "utf8")).toBe(legacy);
  });

  it.each([
    { auth: { mode: "bearer", token: " " }, message: /non-empty token/ },
    { auth: { mode: "none", token: "leak" }, message: /cannot contain a token/ },
    { auth: { mode: "unknown" }, message: /auth must be/ },
  ])("rejects malformed auth variant %#", ({ auth, message }) => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    fs.writeFileSync(path.join(directory, "servers.json"), JSON.stringify({
      schema_version: 2,
      servers: [{
        display_name: "Invalid",
        enabled: true,
        url: "http://127.0.0.1:8443",
        auth,
      }],
    }));

    expect(() => loadOrCreateDesktopConfig(directory)).toThrow(message);
  });

  it("saves editable workspace settings without disturbing server config", () => {
    const directory = tempDir();
    const initial = loadOrCreateDesktopConfig(directory);

    const updated = saveEditableDesktopSettings(directory, {
      servers: [
        {
          ...initial.servers[0]!,
          display_name: "Updated Local",
        },
      ],
      appearance: {
        theme: "dark",
        density: "compact",
        font_scale: 1.2,
      },
      shell: {
        display_anchor: { x: 3000, y: 700 },
      },
      workspace: {
        idle_quit_seconds: 42,
        default_window: { width: 1440, height: 900 },
      },
      github: {
        app_id: "9000001",
        client_id: "Iv23liUpdatedClient99",
        scopes: ["read:user", "repo"],
      },
    });

    expect(updated.servers).toMatchObject([
      {
        id: "srv_ca5ed80c77e645b7c7ec83189f36bfa835128debf18096e46c6300fdc17e8c3a",
        display_name: "Updated Local",
        default_execution: true,
      },
    ]);
    expect(updated.appearance).toEqual({
      theme: "dark",
      density: "compact",
      font_scale: 1.2,
    });
    expect(updated.workspace).toEqual({
      idle_quit_seconds: 42,
      default_window: { width: 1440, height: 900 },
    });
    expect(updated.shell.display_anchor).toEqual({ x: 3000, y: 700 });
    expect(updated.github).toEqual({
      app_id: "9000001",
      client_id: "Iv23liUpdatedClient99",
      scopes: ["read:user", "repo"],
    });
    expect(JSON.parse(fs.readFileSync(path.join(directory, "appearance.json"), "utf8"))).toMatchObject({
      schema_version: 1,
      theme: "dark",
      density: "compact",
      font_scale: 1.2,
    });
    expect(JSON.parse(fs.readFileSync(path.join(directory, "workspace.json"), "utf8"))).toMatchObject({
      schema_version: 1,
      idle_quit_seconds: 42,
      default_window: { width: 1440, height: 900 },
    });
    expect(JSON.parse(fs.readFileSync(path.join(directory, "shell.json"), "utf8"))).toEqual({
      schema_version: 1,
      workspace_executable: "agent-grid-workspace",
      health_poll_interval_seconds: 5,
      display_anchor: { x: 3000, y: 700 },
    });
    expect(JSON.parse(fs.readFileSync(path.join(directory, "github.json"), "utf8"))).toEqual({
      schema_version: 1,
      app_id: "9000001",
      client_id: "Iv23liUpdatedClient99",
      scopes: ["read:user", "repo"],
    });
    expect(JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8"))).toMatchObject({
      schema_version: 2,
      servers: [
        {
          display_name: "Updated Local",
          default_execution: true,
        },
      ],
    });
  });

  it("save/reload path strips runtime health from a projection-bearing settings payload", () => {
    // rejects counterexample: building a separate health-free payload that bypasses conversion
    const directory = tempDir();
    const initial = loadOrCreateDesktopConfig(directory);
    const runtimeProjection = {
      ...initial.servers[0]!,
      health: {
        kind: "connected",
        observedAt: "2026-08-03T00:00:00.000Z",
      },
      observation: {
        kind: "unavailable",
        reason: "http_error",
      },
      hostInstanceId: "host-runtime",
      revision: 9,
    };

    saveEditableDesktopSettings(directory, {
      servers: [runtimeProjection as (typeof initial.servers)[number]],
      appearance: initial.appearance,
      workspace: initial.workspace,
      github: initial.github,
    });

    const persisted = JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8")) as {
      servers: Array<Record<string, unknown>>;
    };
    expect(persisted.servers[0]).not.toHaveProperty("health");
    expect(persisted.servers[0]).not.toHaveProperty("observation");
    expect(persisted.servers[0]).not.toHaveProperty("hostInstanceId");
    expect(persisted.servers[0]).not.toHaveProperty("revision");
    expect(JSON.stringify(persisted)).not.toContain("connected");
    expect(JSON.stringify(persisted)).not.toContain("http_error");

    const reloaded = loadOrCreateDesktopConfig(directory);
    expect(JSON.stringify(reloaded.servers)).not.toContain("health");
    expect(JSON.stringify(reloaded.servers)).not.toContain("observation");
    expect(reloaded.servers[0]).toMatchObject({
      display_name: initial.servers[0]!.display_name,
      enabled: true,
    });
  });

  it("never persists runtime health fields into servers.json after save/reload", () => {
    // rejects counterexample: health projection fields written into configuration
    const directory = tempDir();
    const initial = loadOrCreateDesktopConfig(directory);
    // Runtime may project health alongside config, but editable save only accepts
    // configuration fields. After save/reload, servers.json stays health-free.
    const runtimeProjection = {
      ...initial.servers[0]!,
      health: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
      observation: { kind: "unavailable" },
      hostInstanceId: "host-1",
      revision: 9,
    };
    expect(runtimeProjection).toHaveProperty("health");
    const saved = saveEditableDesktopSettings(directory, {
      servers: [
        {
          id: initial.servers[0]!.id,
          display_name: "Local",
          enabled: true,
          default_execution: true,
          url: initial.servers[0]!.url,
          auth: initial.servers[0]!.auth,
        },
      ],
      appearance: initial.appearance,
      workspace: initial.workspace,
      github: initial.github,
    });
    const serversJson = JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8"));
    expect(JSON.stringify(serversJson)).not.toMatch(/health|observation|hostInstanceId|"revision"/);
    const reloaded = loadOrCreateDesktopConfig(directory);
    expect(reloaded.servers[0]).not.toHaveProperty("health");
    expect(reloaded.servers[0]).not.toHaveProperty("observation");
    expect(saved.servers[0]).not.toHaveProperty("health");
  });

  it("derives the cross-client identifier from the canonical URL", () => {
    expect(serverIdentifier("HTTP://Example.TEST:80/api///?token=ignored#fragment")).toBe(
      "srv_36edcc69555ffb4873eb2d2b461c03839096e08cbb4dd60ba5ecc53f8175a5d0",
    );
  });

  it.each([
    "servers-valid-none.json",
    "servers-valid-bearer.json",
  ])("accepts shared fixture %s", (fixture) => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    fs.copyFileSync(
      path.resolve("../config/fixtures", fixture),
      path.join(directory, "servers.json"),
    );

    expect(() => loadOrCreateDesktopConfig(directory)).not.toThrow();
  });

  it.each([
    "servers-invalid-v1-token-path.json",
    "servers-invalid-empty-bearer.json",
  ])("rejects shared fixture %s", (fixture) => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    fs.copyFileSync(
      path.resolve("../config/fixtures", fixture),
      path.join(directory, "servers.json"),
    );

    expect(() => loadOrCreateDesktopConfig(directory)).toThrow();
  });

  it("ignores a legacy client-selected id and removes it on save", () => {
    const directory = tempDir();
    const initial = loadOrCreateDesktopConfig(directory);
    const serversPath = path.join(directory, "servers.json");
    const persisted = JSON.parse(fs.readFileSync(serversPath, "utf8"));
    persisted.servers[0].id = "client-chosen";
    fs.writeFileSync(serversPath, JSON.stringify(persisted));

    const loaded = loadOrCreateDesktopConfig(directory);
    expect(loaded.servers[0]?.id).toBe(initial.servers[0]?.id);
    saveEditableDesktopSettings(directory, {
      servers: loaded.servers,
      appearance: loaded.appearance,
      workspace: loaded.workspace,
      github: loaded.github,
    });

    expect(JSON.parse(fs.readFileSync(serversPath, "utf8")).servers[0]).not.toHaveProperty("id");
  });

  it.each([
    { github: { app_id: "not-numeric", client_id: "Iv23li471HWU8jxRwlK7", scopes: ["read:user", "repo"] }, message: /app_id/ },
    { github: { app_id: "4460068", client_id: "short", scopes: ["read:user", "repo"] }, message: /client_id/ },
    { github: { app_id: "4460068", client_id: "Iv23li471HWU8jxRwlK7", scopes: ["repo"] }, message: /scopes/ },
  ])("rejects invalid editable GitHub registration without rewriting it %#", ({ github, message }) => {
    const directory = tempDir();
    const initial = loadOrCreateDesktopConfig(directory);
    const file = path.join(directory, "github.json");
    const before = fs.readFileSync(file, "utf8");

    expect(() => saveEditableDesktopSettings(directory, {
      servers: initial.servers,
      appearance: initial.appearance,
      workspace: initial.workspace,
      github,
    })).toThrow(message);
    expect(fs.readFileSync(file, "utf8")).toBe(before);
  });
});

function server(displayName: string, enabled: boolean, url: string) {
  return {
    display_name: displayName,
    enabled,
    url,
    auth: { mode: "bearer", token: "token" },
  };
}
