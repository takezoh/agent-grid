import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";

export const CONFIG_SCHEMA_VERSION = 1;
export const CONFIG_DIR_ARGUMENT = "--config-dir";

export type Theme = "default" | "system" | "light" | "dark";
export type Density = "compact" | "comfortable";
export type LaunchMode = "managed_wsl" | "connect_only";

export interface ServerConfig {
  id: string;
  display_name: string;
  enabled: boolean;
  url: string;
  token_path: string;
  launch: {
    mode: LaunchMode;
    wsl_distro?: string;
    server_path_in_wsl?: string;
    token_path_in_wsl?: string;
  };
}

export interface AppearanceConfig {
  theme: Theme;
  density: Density;
  font_scale: number;
}

export interface ShellAppConfig {
  workspace_executable: string;
  health_poll_interval_seconds: number;
}

export interface WorkspaceAppConfig {
  idle_quit_seconds: number;
  default_window: { width: number; height: number };
}

export interface DesktopConfig {
  configDirectory: string;
  servers: readonly ServerConfig[];
  appearance: AppearanceConfig;
  shell: ShellAppConfig;
  workspace: WorkspaceAppConfig;
}

type Versioned<T> = T & { schema_version: number };

export function defaultConfigDirectory(): string {
  if (process.platform === "win32") {
    const appData = process.env.APPDATA ?? path.join(os.homedir(), "AppData", "Roaming");
    return path.join(appData, "agent-grid", "config");
  }
  const xdg = process.env.XDG_CONFIG_HOME ?? path.join(os.homedir(), ".config");
  return path.join(xdg, "agent-grid");
}

export function resolveConfigDirectory(args: readonly string[]): string {
  const index = args.indexOf(CONFIG_DIR_ARGUMENT);
  if (index < 0) return defaultConfigDirectory();
  const value = args[index + 1];
  if (!value) throw new Error("--config-dir requires a path");
  return path.resolve(value);
}

export function loadOrCreateDesktopConfig(configDirectory: string): DesktopConfig {
  const directory = path.resolve(configDirectory);
  fs.mkdirSync(directory, { recursive: true });
  const defaults = defaultDocuments();
  for (const [name, value] of Object.entries(defaults)) {
    createIfMissing(path.join(directory, name), value);
  }

  const serversDoc = readJson<Versioned<{ servers: ServerConfig[] }>>(directory, "servers.json");
  const appearanceDoc = readJson<Versioned<AppearanceConfig>>(directory, "appearance.json");
  const shellDoc = readJson<Versioned<ShellAppConfig>>(directory, "shell.json");
  const workspaceDoc = readJson<Versioned<WorkspaceAppConfig>>(directory, "workspace.json");
  checkVersion(serversDoc, "servers.json");
  checkVersion(appearanceDoc, "appearance.json");
  checkVersion(shellDoc, "shell.json");
  checkVersion(workspaceDoc, "workspace.json");

  validateServers(serversDoc.servers);
  validateAppearance(appearanceDoc);
  validateShell(shellDoc);
  validateWorkspace(workspaceDoc);
  return {
    configDirectory: directory,
    servers: serversDoc.servers.map((server) => ({
      ...server,
      token_path: expandWindowsEnvironment(server.token_path),
    })),
    appearance: {
      theme: appearanceDoc.theme,
      density: appearanceDoc.density,
      font_scale: appearanceDoc.font_scale,
    },
    shell: {
      workspace_executable: shellDoc.workspace_executable,
      health_poll_interval_seconds: shellDoc.health_poll_interval_seconds,
    },
    workspace: {
      idle_quit_seconds: workspaceDoc.idle_quit_seconds,
      default_window: { ...workspaceDoc.default_window },
    },
  };
}

function createIfMissing(filePath: string, value: unknown): void {
  try {
    const fd = fs.openSync(filePath, "wx");
    try {
      fs.writeFileSync(fd, `${JSON.stringify(value, null, 2)}\n`, "utf8");
      fs.fsyncSync(fd);
    } finally {
      fs.closeSync(fd);
    }
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "EEXIST") throw error;
  }
}

function readJson<T>(directory: string, name: string): T {
  try {
    return JSON.parse(fs.readFileSync(path.join(directory, name), "utf8")) as T;
  } catch (error) {
    throw new Error(`${name}: invalid or unreadable JSON: ${(error as Error).message}`);
  }
}

function checkVersion(value: { schema_version?: unknown }, name: string): void {
  if (value.schema_version !== CONFIG_SCHEMA_VERSION) {
    throw new Error(
      `${name}: unsupported schema_version ${String(value.schema_version)}; expected ${CONFIG_SCHEMA_VERSION}`,
    );
  }
}

function validateServers(servers: unknown): asserts servers is ServerConfig[] {
  if (!Array.isArray(servers) || servers.length === 0) {
    throw new Error("servers.json: at least one server is required");
  }
  const ids = new Set<string>();
  let enabled = 0;
  for (const raw of servers) {
    if (!isRecord(raw)) throw new Error("servers.json: every server must be an object");
    const server = raw as unknown as ServerConfig;
    if (typeof server.id !== "string" || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(server.id)) {
      throw new Error(`servers.json: invalid server id '${String(server.id)}'`);
    }
    if (ids.has(server.id)) throw new Error(`servers.json: duplicate server id '${server.id}'`);
    ids.add(server.id);
    if (typeof server.display_name !== "string" || !server.display_name.trim()) {
      throw new Error(`servers.json: server '${server.id}' needs display_name`);
    }
    if (typeof server.enabled !== "boolean") {
      throw new Error(`servers.json: server '${server.id}' enabled must be boolean`);
    }
    if (server.enabled) enabled++;
    assertHttpUrl(server.url, `servers.json: server '${server.id}' url`);
    if (typeof server.token_path !== "string" || !server.token_path.trim()) {
      throw new Error(`servers.json: server '${server.id}' needs token_path`);
    }
    if (!isRecord(server.launch) ||
        (server.launch.mode !== "managed_wsl" && server.launch.mode !== "connect_only")) {
      throw new Error(`servers.json: server '${server.id}' has invalid launch.mode`);
    }
    if (server.launch.mode === "managed_wsl" &&
        (!server.launch.wsl_distro ||
         !server.launch.server_path_in_wsl ||
         !server.launch.token_path_in_wsl)) {
      throw new Error(`servers.json: managed_wsl server '${server.id}' needs all WSL fields`);
    }
  }
  if (enabled === 0) throw new Error("servers.json: at least one server must be enabled");
}

function validateAppearance(value: Versioned<AppearanceConfig>): void {
  if (!["default", "system", "light", "dark"].includes(value.theme)) {
    throw new Error("appearance.json: invalid theme");
  }
  if (!["compact", "comfortable"].includes(value.density)) {
    throw new Error("appearance.json: invalid density");
  }
  if (typeof value.font_scale !== "number" || value.font_scale < 0.8 || value.font_scale > 1.5) {
    throw new Error("appearance.json: font_scale must be between 0.8 and 1.5");
  }
}

function validateShell(value: Versioned<ShellAppConfig>): void {
  if (typeof value.workspace_executable !== "string" || !value.workspace_executable.trim()) {
    throw new Error("shell.json: workspace_executable is required");
  }
  if (!Number.isInteger(value.health_poll_interval_seconds) ||
      value.health_poll_interval_seconds < 1 ||
      value.health_poll_interval_seconds > 300) {
    throw new Error("shell.json: health_poll_interval_seconds must be between 1 and 300");
  }
}

function validateWorkspace(value: Versioned<WorkspaceAppConfig>): void {
  if (!Number.isInteger(value.idle_quit_seconds) ||
      value.idle_quit_seconds < 0 ||
      value.idle_quit_seconds > 86400) {
    throw new Error("workspace.json: idle_quit_seconds must be between 0 and 86400");
  }
  const size = value.default_window;
  if (!isRecord(size) ||
      !Number.isInteger(size.width) ||
      !Number.isInteger(size.height) ||
      size.width < 320 ||
      size.width > 16384 ||
      size.height < 240 ||
      size.height > 16384) {
    throw new Error("workspace.json: default_window size is out of range");
  }
}

function assertHttpUrl(value: unknown, label: string): void {
  if (typeof value !== "string") throw new Error(`${label} must be a URL`);
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("bad scheme");
  } catch {
    throw new Error(`${label} must be an absolute HTTP(S) URL`);
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function expandWindowsEnvironment(value: string): string {
  return value.replace(/%([^%]+)%/g, (match, name: string) => process.env[name] ?? match);
}

function defaultDocuments(): Record<string, unknown> {
  const localAppData =
    process.env.LOCALAPPDATA ?? path.join(os.homedir(), "AppData", "Local");
  return {
    "servers.json": {
      schema_version: CONFIG_SCHEMA_VERSION,
      servers: [{
        id: "local",
        display_name: "Local",
        enabled: true,
        url: "http://127.0.0.1:8443",
        token_path: path.join(localAppData, "agent-grid", "gateway-token"),
        launch: {
          mode: "managed_wsl",
          wsl_distro: "Ubuntu-22.04",
          server_path_in_wsl: "~/agent-grid/server",
          token_path_in_wsl: "~/.agent-grid/gateway-token",
        },
      }],
    },
    "appearance.json": {
      schema_version: CONFIG_SCHEMA_VERSION,
      theme: "default",
      density: "comfortable",
      font_scale: 1,
    },
    "shell.json": {
      schema_version: CONFIG_SCHEMA_VERSION,
      workspace_executable: "agent-grid-workspace",
      health_poll_interval_seconds: 5,
    },
    "workspace.json": {
      schema_version: CONFIG_SCHEMA_VERSION,
      idle_quit_seconds: 300,
      default_window: { width: 1280, height: 800 },
    },
  };
}
