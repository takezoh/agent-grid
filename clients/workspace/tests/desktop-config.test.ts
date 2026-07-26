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
  it("creates and loads the four defaults", () => {
    const directory = tempDir();
    const config = loadOrCreateDesktopConfig(directory);

    expect(config.servers.map((server) => server.id)).toEqual([
      "srv_ca5ed80c77e645b7c7ec83189f36bfa835128debf18096e46c6300fdc17e8c3a",
    ]);
    expect(config.appearance).toEqual({
      theme: "default",
      density: "comfortable",
      font_scale: 1,
    });
    expect(fs.readdirSync(directory).sort()).toEqual([
      "appearance.json",
      "servers.json",
      "shell.json",
      "workspace.json",
    ]);
    expect(JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8")).servers[0])
      .not.toHaveProperty("launch");
    expect(JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8")).servers[0])
      .not.toHaveProperty("id");
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
        schema_version: 1,
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
  });

  it("does not replace invalid user configuration", () => {
    const directory = tempDir();
    loadOrCreateDesktopConfig(directory);
    const file = path.join(directory, "appearance.json");
    fs.writeFileSync(file, '{"schema_version":1,"theme":"blue"}');

    expect(() => loadOrCreateDesktopConfig(directory)).toThrow(/appearance\.json/);
    expect(fs.readFileSync(file, "utf8")).toBe('{"schema_version":1,"theme":"blue"}');
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
      workspace: {
        idle_quit_seconds: 42,
        default_window: { width: 1440, height: 900 },
      },
    });

    expect(updated.servers).toMatchObject([
      {
        id: "srv_ca5ed80c77e645b7c7ec83189f36bfa835128debf18096e46c6300fdc17e8c3a",
        display_name: "Updated Local",
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
    expect(JSON.parse(fs.readFileSync(path.join(directory, "servers.json"), "utf8"))).toMatchObject({
      schema_version: 1,
      servers: [
        {
          display_name: "Updated Local",
        },
      ],
    });
  });

  it("derives the cross-client identifier from the canonical URL", () => {
    expect(serverIdentifier("HTTP://Example.TEST:80/api///?token=ignored#fragment")).toBe(
      "srv_36edcc69555ffb4873eb2d2b461c03839096e08cbb4dd60ba5ecc53f8175a5d0",
    );
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
    });

    expect(JSON.parse(fs.readFileSync(serversPath, "utf8")).servers[0]).not.toHaveProperty("id");
  });
});

function server(displayName: string, enabled: boolean, url: string) {
  return {
    display_name: displayName,
    enabled,
    url,
    token_path: "token",
  };
}
