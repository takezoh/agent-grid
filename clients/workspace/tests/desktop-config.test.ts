import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  loadOrCreateDesktopConfig,
  resolveConfigDirectory,
  saveEditableDesktopSettings,
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

    expect(config.servers.map((server) => server.id)).toEqual(["local"]);
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
          server("one", true),
          server("two", false),
        ],
      }),
    );

    const config = loadOrCreateDesktopConfig(directory);
    expect(config.servers.map(({ id, enabled }) => ({ id, enabled }))).toEqual([
      { id: "one", enabled: true },
      { id: "two", enabled: false },
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
        id: "local",
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
          id: "local",
          display_name: "Updated Local",
        },
      ],
    });
  });
});

function server(id: string, enabled: boolean) {
  return {
    id,
    display_name: id,
    enabled,
    url: "http://127.0.0.1:8443",
    token_path: "token",
  };
}
