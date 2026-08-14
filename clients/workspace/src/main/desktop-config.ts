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
/**
 * How an entry may be used. A malformed entry costs that entry and the entries
 * it collides with, not the fleet
 * (adr-20260814-server-display-name-naming-contract).
 */
export type AdmissionState = "usable" | "disabled" | "configuration_invalid";

/** Why an entry is unusable: the rule it broke, and enough detail to find it. */
export interface ServerViolation {
  rule: string;
  detail: string;
}

// Rule identifiers, shared with clients/config/servers.schema.json
// x-contract-rules and with the expected-outcome corpus.
export const RULE_DISPLAY_NAME_REQUIRED = "display-name-required";
export const RULE_DISPLAY_NAME_CHARSET = "display-name-charset";
export const RULE_DISPLAY_NAME_UNIQUE = "display-name-unique";
export const RULE_ENTRY_FIELD_TYPES = "entry-field-types";
export const RULE_UNKNOWN_ENTRY_FIELD = "unknown-entry-field-rejected";
export const RULE_URL_SCHEME = "url-http-or-https";
export const RULE_URL_BARE_AUTHORITY = "url-carries-no-credentials-query-or-fragment";
export const RULE_DUPLICATE_URL = "duplicate-url-rejected";
export const RULE_AUTH_VARIANT = "auth-variant-exact";
export const RULE_AT_MOST_ONE_DEFAULT = "at-most-one-default-execution";
export const RULE_DEFAULT_MUST_BE_ENABLED = "default-execution-must-be-enabled";

export interface ServerConfig {
  /**
   * Deterministic routing identifier derived from the canonical gateway URL.
   * Empty on an entry whose URL could not be canonicalised.
   */
  id: string;
  /** Position in the file's servers array, so an unnamed entry is still findable. */
  index: number;
  display_name: string;
  enabled: boolean;
  default_execution: boolean;
  url: string;
  auth: ServerAuth;
  admission: AdmissionState;
  /** Absent on an admissible entry. */
  violation?: ServerViolation;
}

/** An entry a client may connect to. Invalid and disabled entries are not. */
export function isUsable(server: ServerConfig): boolean {
  return server.admission === "usable";
}

type ServerDocument = {
  display_name: string;
  enabled: boolean;
  default_execution?: boolean;
  url: string;
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
  display_anchor?: { x: number; y: number } | null;
}

export interface EditableShellSettings {
  display_anchor: { x: number; y: number } | null;
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
  shell?: EditableShellSettings;
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

  const servers = admitServers(serversDoc.servers);
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
      display_anchor: shellDoc.display_anchor ? { ...shellDoc.display_anchor } : null,
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
  const current = loadOrCreateDesktopConfig(directory);
  // Persist only config fields. Runtime health projections must never be written
  // into servers.json even when a settings payload accidentally carries them.
  const serversDoc: Versioned<{ servers: readonly ServerDocument[] }> = {
    schema_version: SERVERS_SCHEMA_VERSION,
    servers: settings.servers.map((server) => {
      const auth =
        server.auth.mode === "bearer"
          ? { mode: "bearer" as const, token: server.auth.token }
          : { mode: "none" as const };
      return {
        display_name: server.display_name,
        enabled: server.enabled,
        default_execution: server.default_execution,
        url: server.url,
        auth,
      };
    }),
  };
  const appearanceDoc: Versioned<AppearanceConfig> = {
    schema_version: CONFIG_SCHEMA_VERSION,
    theme: settings.appearance.theme,
    density: settings.appearance.density,
    font_scale: settings.appearance.font_scale,
  };
  const shellDoc: Versioned<ShellAppConfig> = {
    schema_version: CONFIG_SCHEMA_VERSION,
    workspace_executable: current.shell.workspace_executable,
    health_poll_interval_seconds: current.shell.health_poll_interval_seconds,
    display_anchor: settings.shell === undefined
      ? current.shell.display_anchor ? { ...current.shell.display_anchor } : null
      : settings.shell.display_anchor ? { ...settings.shell.display_anchor } : null,
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
  assertSavedServersAreAdmissible([...serversDoc.servers]);
  validateAppearance(appearanceDoc);
  validateShell(shellDoc);
  validateWorkspace(workspaceDoc);
  validateGitHub(githubDoc);
  writeJson(path.join(directory, "servers.json"), serversDoc);
  writeJson(path.join(directory, "appearance.json"), appearanceDoc);
  writeJson(path.join(directory, "shell.json"), shellDoc);
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

// Admission is per entry. A malformed entry costs that entry and the entries it
// collides with, not the fleet: an operator's typo in a server they are not
// using must not take away every other server on every host
// (adr-20260814-server-display-name-naming-contract). Only an unreadable entry
// list stops the host, because that is the one case where no entry can be
// named. Nothing here renames, normalises, suffixes, disables or drops an entry
// to resolve a violation.
const ENTRY_FIELDS = ["id", "display_name", "enabled", "default_execution", "url", "auth"];

function assertServerList(servers: unknown): asserts servers is unknown[] {
  if (!Array.isArray(servers)) {
    throw new Error("servers.json: servers must be an array");
  }
}

function displayNameKey(name: string): string {
  return name.trim();
}

function checkDisplayName(name: string): ServerViolation | null {
  const trimmed = displayNameKey(name);
  if (!trimmed) {
    return { rule: RULE_DISPLAY_NAME_REQUIRED, detail: "display_name must not be blank" };
  }
  for (const character of trimmed) {
    const code = character.codePointAt(0) ?? 0;
    if (code <= 0x1f || code === 0x7f) {
      return {
        rule: RULE_DISPLAY_NAME_CHARSET,
        detail: `display_name contains control character U+${code.toString(16).padStart(4, "0").toUpperCase()}`,
      };
    }
  }
  if (trimmed.startsWith("-")) {
    return {
      rule: RULE_DISPLAY_NAME_CHARSET,
      detail: "display_name must not begin with '-', which a command line reads as a flag",
    };
  }
  return null;
}

function admitEntry(raw: unknown, index: number): ServerConfig {
  const blank: ServerConfig = {
    id: "",
    index,
    display_name: "",
    enabled: false,
    default_execution: false,
    url: "",
    auth: { mode: "none" },
    admission: "configuration_invalid",
  };
  if (!isRecord(raw)) {
    return { ...blank, violation: { rule: RULE_ENTRY_FIELD_TYPES, detail: "entry must be an object" } };
  }
  const extraKeys = Object.keys(raw).filter((key) => !ENTRY_FIELDS.includes(key));
  const named = typeof raw.display_name === "string" ? raw.display_name : "";
  if (extraKeys.length > 0) {
    return {
      ...blank,
      display_name: named,
      violation: {
        rule: RULE_UNKNOWN_ENTRY_FIELD,
        detail: `unsupported field '${extraKeys[0]}'`,
      },
    };
  }
  if (
    typeof raw.display_name !== "string" ||
    typeof raw.url !== "string" ||
    typeof raw.enabled !== "boolean" ||
    (raw.default_execution !== undefined && typeof raw.default_execution !== "boolean")
  ) {
    return {
      ...blank,
      display_name: named,
      violation: {
        rule: RULE_ENTRY_FIELD_TYPES,
        detail: "display_name and url must be strings; enabled and default_execution must be booleans",
      },
    };
  }

  const entry: ServerConfig = {
    ...blank,
    display_name: raw.display_name,
    enabled: raw.enabled,
    default_execution: raw.default_execution === true,
    url: raw.url,
  };

  const nameViolation = checkDisplayName(raw.display_name);
  if (nameViolation) return { ...entry, violation: nameViolation };

  const urlViolation = checkServerUrl(raw.url);
  if (urlViolation) return { ...entry, violation: urlViolation };

  const authViolation = checkServerAuth(raw.auth);
  if (authViolation) return { ...entry, violation: authViolation };

  const canonical = canonicalServerUrl(raw.url);
  const admitted: ServerConfig = {
    ...entry,
    id: serverIdentifier(raw.url),
    url: canonical,
    auth: mapServerAuth(raw.auth),
    admission: raw.enabled ? "usable" : "disabled",
  };
  if (admitted.default_execution && !admitted.enabled) {
    return {
      ...admitted,
      admission: "configuration_invalid",
      violation: {
        rule: RULE_DEFAULT_MUST_BE_ENABLED,
        detail: "a default execution entry must be enabled",
      },
    };
  }
  return admitted;
}

// markCollisions invalidates every member of a collision group. No winner is
// chosen: a duplicate is a property of a pair, so blaming one member requires
// choosing which, and every choice rule silently redirects an operator's
// existing reference to a different machine.
function markCollisions(
  servers: ServerConfig[],
  key: (server: ServerConfig) => string | null,
  rule: string,
  detail: string,
): void {
  const groups = new Map<string, number[]>();
  servers.forEach((server, index) => {
    if (server.violation) return;
    const value = key(server);
    if (!value) return;
    groups.set(value, [...(groups.get(value) ?? []), index]);
  });
  for (const members of groups.values()) {
    if (members.length < 2) continue;
    for (const index of members) {
      servers[index] = {
        ...servers[index]!,
        admission: "configuration_invalid",
        violation: { rule, detail },
      };
    }
  }
}

function markDefaultExecution(servers: ServerConfig[]): void {
  const marked = servers
    .map((server, index) => ({ server, index }))
    .filter((item) => !item.server.violation && item.server.default_execution)
    .map((item) => item.index);
  if (marked.length > 1) {
    for (const index of marked) {
      servers[index] = {
        ...servers[index]!,
        admission: "configuration_invalid",
        violation: {
          rule: RULE_AT_MOST_ONE_DEFAULT,
          detail: "more than one entry is marked as the default execution server",
        },
      };
    }
    // Every marker is now unusable, so the legacy fallback below applies.
  } else if (marked.length === 1) {
    return;
  }
  const first = servers.findIndex((server) => server.admission === "usable");
  if (first >= 0) {
    servers[first] = { ...servers[first]!, default_execution: true };
  }
}

// assertSavedServersAreAdmissible keeps the settings UI from writing a document
// that would come back configuration-invalid. Saving is the one moment an
// operator is editing the entries directly, so a violation is reported to them
// rather than admitted silently.
function assertSavedServersAreAdmissible(servers: readonly ServerDocument[]): void {
  const admitted = admitServers(servers);
  const offender = admitted.find((server) => server.violation);
  if (offender) {
    throw new Error(
      `servers.json: server #${offender.index + 1} '${offender.display_name}' ` +
        `violates ${offender.violation!.rule}: ${offender.violation!.detail}`,
    );
  }
}

function admitServers(raw: unknown): ServerConfig[] {
  assertServerList(raw);
  const servers = raw.map((entry, index) => admitEntry(entry, index));
  markCollisions(
    servers,
    (server) => server.id || null,
    RULE_DUPLICATE_URL,
    "two entries resolve to the same canonical url",
  );
  markCollisions(
    servers,
    (server) => (server.enabled ? displayNameKey(server.display_name) : null),
    RULE_DISPLAY_NAME_UNIQUE,
    "two enabled entries share a display name",
  );
  markDefaultExecution(servers);
  return servers;
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
  if (value.display_anchor !== null && value.display_anchor !== undefined &&
      (!isRecord(value.display_anchor) ||
       !Number.isInteger(value.display_anchor.x) ||
       !Number.isInteger(value.display_anchor.y))) {
    throw new Error("shell.json: display_anchor must contain integer x and y coordinates");
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
  let url: URL;
  try {
    url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("bad scheme");
  } catch {
    throw new Error(`${label} must be an absolute HTTP(S) URL`);
  }
  // Rejected, not stripped. All three are things an operator wrote on purpose;
  // discarding them silently leaves the operator believing the client is doing
  // something it is not — most sharply for a credential, where the belief is
  // that authentication is configured
  // (adr-20260814-shared-go-client-config-module).
  if (url.username || url.password || url.search || url.hash) {
    throw new Error(`${label} must not contain credentials, a query, or a fragment`);
  }
}

// checkServerUrl and checkServerAuth are the entry-scoped forms of the two
// assertions above: they name a rule instead of throwing, so one bad entry
// costs one entry.
function checkServerUrl(value: string): ServerViolation | null {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return { rule: RULE_URL_SCHEME, detail: "url must be an absolute HTTP(S) URL" };
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return { rule: RULE_URL_SCHEME, detail: `unsupported scheme '${url.protocol}'` };
  }
  if (url.username || url.password || url.search || url.hash) {
    return {
      rule: RULE_URL_BARE_AUTHORITY,
      detail: "url must not contain credentials, a query, or a fragment",
    };
  }
  return null;
}

function checkServerAuth(value: unknown): ServerViolation | null {
  try {
    validateServerAuth(value, "server");
    return null;
  } catch (error) {
    return { rule: RULE_AUTH_VARIANT, detail: (error as Error).message };
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
      display_anchor: null,
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
