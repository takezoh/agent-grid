import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import { createHash } from "node:crypto";
import {
  FIXED_APP_ID,
  FIXED_CLIENT_ID,
  SCOPE_READ_USER,
  SCOPE_REPO,
} from "./github-oauth.generated.js";

export const CONFIG_SCHEMA_VERSION = 1;
export const SERVERS_SCHEMA_VERSION = 2;
export const CONFIG_DIR_ARGUMENT = "--config-dir";

export type Theme = "default" | "system" | "light" | "dark";
export type Density = "compact" | "comfortable";
export type ServerAuth =
  | { mode: "none" }
  | { mode: "bearer"; token: string };
export interface ServerConfig {
  /** Deterministic routing identifier derived from the canonical gateway URL. */
  id: string;
  display_name: string;
  enabled: boolean;
  default_execution: boolean;
  url: string;
  auth: ServerAuth;
}

type ServerDocument = Omit<ServerConfig, "id" | "auth"> & {
  auth: unknown;
  id?: unknown;
};

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

export interface GitHubAppConfig {
  app_id: string;
  client_id: string;
  scopes: string[];
}

export interface DesktopConfig {
  configDirectory: string;
  servers: readonly ServerConfig[];
  appearance: AppearanceConfig;
  shell: ShellAppConfig;
  workspace: WorkspaceAppConfig;
  github: GitHubAppConfig;
}

export interface EditableDesktopSettings {
  servers: readonly ServerConfig[];
  appearance: AppearanceConfig;
  workspace: WorkspaceAppConfig;
  github: GitHubAppConfig;
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

  const serversDoc = readJson<Versioned<{ servers: ServerDocument[] }>>(directory, "servers.json");
  const appearanceDoc = readJson<Versioned<AppearanceConfig>>(directory, "appearance.json");
  const shellDoc = readJson<Versioned<ShellAppConfig>>(directory, "shell.json");
  const workspaceDoc = readJson<Versioned<WorkspaceAppConfig>>(directory, "workspace.json");
  const githubDoc = readJson<Versioned<GitHubAppConfig>>(directory, "github.json");
  checkVersion(serversDoc, "servers.json", SERVERS_SCHEMA_VERSION);
  checkVersion(appearanceDoc, "appearance.json");
  checkVersion(shellDoc, "shell.json");
  checkVersion(workspaceDoc, "workspace.json");
  checkVersion(githubDoc, "github.json");

  validateServers(serversDoc.servers);
  const servers = serversDoc.servers.map((server, index) =>
    mapServer(server, index, serversDoc.servers),
  );
  validateAppearance(appearanceDoc);
  validateShell(shellDoc);
  validateWorkspace(workspaceDoc);
  validateGitHub(githubDoc);
  return {
    configDirectory: directory,
    servers,
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
    github: {
      app_id: githubDoc.app_id,
      client_id: githubDoc.client_id,
      scopes: [...githubDoc.scopes],
    },
  };
}

export function loadEditableDesktopSettings(configDirectory: string): DesktopConfig {
  return loadOrCreateDesktopConfig(configDirectory);
}

export function saveEditableDesktopSettings(
  configDirectory: string,
  settings: EditableDesktopSettings,
): DesktopConfig {
  const directory = path.resolve(configDirectory);
  loadOrCreateDesktopConfig(directory);
  const serversDoc: Versioned<{ servers: readonly ServerDocument[] }> = {
    schema_version: SERVERS_SCHEMA_VERSION,
    servers: settings.servers.map(({ id: _id, ...server }) => server),
  };
  const appearanceDoc: Versioned<AppearanceConfig> = {
    schema_version: CONFIG_SCHEMA_VERSION,
    theme: settings.appearance.theme,
    density: settings.appearance.density,
    font_scale: settings.appearance.font_scale,
  };
  const workspaceDoc: Versioned<WorkspaceAppConfig> = {
    schema_version: CONFIG_SCHEMA_VERSION,
    idle_quit_seconds: settings.workspace.idle_quit_seconds,
    default_window: {
      width: settings.workspace.default_window.width,
      height: settings.workspace.default_window.height,
    },
  };
  const githubDoc: Versioned<GitHubAppConfig> = {
    schema_version: CONFIG_SCHEMA_VERSION,
    app_id: settings.github.app_id.trim(),
    client_id: settings.github.client_id.trim(),
    scopes: [...settings.github.scopes],
  };
  validateServers([...serversDoc.servers]);
  validateAppearance(appearanceDoc);
  validateWorkspace(workspaceDoc);
  validateGitHub(githubDoc);
  writeJson(path.join(directory, "servers.json"), serversDoc);
  writeJson(path.join(directory, "appearance.json"), appearanceDoc);
  writeJson(path.join(directory, "workspace.json"), workspaceDoc);
  writeJson(path.join(directory, "github.json"), githubDoc);
  return loadOrCreateDesktopConfig(directory);
}

function createIfMissing(filePath: string, value: unknown): void {
  try {
    const fd = fs.openSync(filePath, "wx", 0o600);
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

function writeJson(filePath: string, value: unknown): void {
  const directory = path.dirname(filePath);
  fs.mkdirSync(directory, { recursive: true });
  const tempPath = `${filePath}.tmp-${process.pid}`;
  fs.writeFileSync(tempPath, `${JSON.stringify(value, null, 2)}\n`, "utf8");
  fs.chmodSync(tempPath, 0o600);
  fs.renameSync(tempPath, filePath);
}

function checkVersion(
  value: { schema_version?: unknown },
  name: string,
  expected = CONFIG_SCHEMA_VERSION,
): void {
  if (value.schema_version !== expected) {
    throw new Error(
      `${name}: unsupported schema_version ${String(value.schema_version)}; expected ${expected}. ` +
        (name === "servers.json"
          ? 'Use auth {"mode":"none"} or {"mode":"bearer","token":"..."}; token_path is no longer supported.'
          : ""),
    );
  }
}

function validateServers(servers: unknown): asserts servers is ServerDocument[] {
  if (!Array.isArray(servers) || servers.length === 0) {
    throw new Error("servers.json: at least one server is required");
  }
  const ids = new Set<string>();
  let enabled = 0;
  let defaultExecution = 0;
  for (const raw of servers) {
    if (!isRecord(raw)) throw new Error("servers.json: every server must be an object");
    const server = raw as unknown as ServerDocument;
    const extraKeys = Object.keys(raw).filter(
      (key) =>
        !["id", "display_name", "enabled", "default_execution", "url", "auth"].includes(key),
    );
    if (extraKeys.length > 0) {
      throw new Error(`servers.json: server contains unsupported field '${extraKeys[0]}'`);
    }
    const label = typeof server.display_name === "string" && server.display_name.trim()
      ? server.display_name
      : "unnamed server";
    if (typeof server.display_name !== "string" || !server.display_name.trim()) {
      throw new Error("servers.json: every server needs display_name");
    }
    if (typeof server.enabled !== "boolean") {
      throw new Error(`servers.json: server '${label}' enabled must be boolean`);
    }
    if (server.enabled) enabled++;
    if (server.default_execution === true) {
      if (!server.enabled) {
        throw new Error(`servers.json: default execution server '${label}' must be enabled`);
      }
      defaultExecution++;
    } else if (server.default_execution !== undefined && server.default_execution !== false) {
      throw new Error(`servers.json: server '${label}' default_execution must be boolean`);
    }
    assertHttpUrl(server.url, `servers.json: server '${label}' url`);
    const id = serverIdentifier(server.url);
    if (ids.has(id)) throw new Error(`servers.json: duplicate server url '${server.url}'`);
    ids.add(id);
    validateServerAuth(server.auth, label);
  }
  if (enabled === 0) throw new Error("servers.json: at least one server must be enabled");
  if (defaultExecution > 1) {
    throw new Error("servers.json: exactly one enabled server may be the default execution server");
  }
}

function mapServer(server: ServerDocument, index: number, servers: readonly ServerDocument[]): ServerConfig {
  const hasExplicitDefault = servers.some((candidate) => candidate.default_execution === true);
  const firstEnabledIndex = servers.findIndex((candidate) => candidate.enabled);
  return {
    id: serverIdentifier(server.url),
    display_name: server.display_name,
    enabled: server.enabled,
    default_execution: hasExplicitDefault
      ? server.default_execution === true
      : server.enabled && index === firstEnabledIndex,
    url: canonicalServerUrl(server.url),
    auth: mapServerAuth(server.auth),
  };
}

export function canonicalServerUrl(value: string): string {
  const url = new URL(value);
  const pathname = url.pathname.replace(/\/+$/, "");
  return `${url.protocol}//${url.host}${pathname}`;
}

export function serverIdentifier(value: string): string {
  const digest = createHash("sha256").update(canonicalServerUrl(value), "utf8").digest("hex");
  return `srv_${digest}`;
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

function validateGitHub(value: Versioned<GitHubAppConfig>): void {
  if (typeof value.app_id !== "string" || !/^\d+$/.test(value.app_id.trim())) {
    throw new Error("github.json: app_id must be a numeric GitHub App ID");
  }
  if (typeof value.client_id !== "string" || !/^[A-Za-z0-9]{20,64}$/.test(value.client_id.trim())) {
    throw new Error("github.json: client_id must be a valid public GitHub Client ID");
  }
  if (JSON.stringify(value.scopes) !== JSON.stringify([SCOPE_READ_USER, SCOPE_REPO])) {
    throw new Error('github.json: scopes must be ["read:user","repo"]');
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

function validateServerAuth(value: unknown, label: string): asserts value is ServerAuth {
  if (!isRecord(value) || (value.mode !== "none" && value.mode !== "bearer")) {
    throw new Error(
      `servers.json: server '${label}' auth must be {"mode":"none"} or ` +
        '{"mode":"bearer","token":"..."}',
    );
  }
  const keys = Object.keys(value).sort();
  if (value.mode === "none") {
    if (keys.length !== 1 || keys[0] !== "mode") {
      throw new Error(`servers.json: server '${label}' auth mode none cannot contain a token`);
    }
    return;
  }
  if (
    keys.length !== 2 ||
    keys[0] !== "mode" ||
    keys[1] !== "token" ||
    typeof value.token !== "string" ||
    !value.token.trim()
  ) {
    throw new Error(`servers.json: server '${label}' bearer auth needs a non-empty token`);
  }
}

function mapServerAuth(value: unknown): ServerAuth {
  validateServerAuth(value, "server");
  return value.mode === "none"
    ? { mode: "none" }
    : { mode: "bearer", token: value.token.trim() };
}

function defaultDocuments(): Record<string, unknown> {
  return {
    "servers.json": {
      schema_version: SERVERS_SCHEMA_VERSION,
      servers: [{
        display_name: "Local",
        enabled: true,
        default_execution: true,
        url: "http://127.0.0.1:8443",
        auth: { mode: "none" },
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
    "github.json": {
      schema_version: CONFIG_SCHEMA_VERSION,
      app_id: FIXED_APP_ID,
      client_id: FIXED_CLIENT_ID,
      scopes: [SCOPE_READ_USER, SCOPE_REPO],
    },
  };
}
