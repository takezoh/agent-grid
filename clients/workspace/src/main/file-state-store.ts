/**
 * Persist workspace window layout and last display to disk (schema_version: 3).
 * Default path: %APPDATA%/agent-grid/workspace-state.json on Windows.
 * contract-workspace-state-schema-evolution via loadWorkspaceState.
 *
 * Also owns atomic protected JSON persistence (temp write, fsync, rename,
 * 0o600) used by GitHub credential/catalog stores (NFR-GH-004).
 */

import * as fs from "node:fs";
import * as fsp from "node:fs/promises";
import * as path from "node:path";
import * as os from "node:os";
import {
  loadWorkspaceState,
  type StateStore,
  type WorkspaceStateV3,
} from "./window-registry.js";

export function defaultWorkspaceStatePath(): string {
  if (process.platform === "win32") {
    const appData = process.env.APPDATA ?? path.join(os.homedir(), "AppData", "Roaming");
    return path.join(appData, "agent-grid", "workspace-state.json");
  }
  const xdg = process.env.XDG_STATE_HOME ?? path.join(os.homedir(), ".local", "state");
  return path.join(xdg, "agent-grid", "workspace-state.json");
}

/**
 * Same-filesystem temp write, optional mode, fsync, atomic rename.
 * Used for workspace state and protected credential/catalog records.
 */
export function atomicWriteFile(
  filePath: string,
  contents: string,
  mode = 0o600,
): void {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  const tempPath = `${filePath}.tmp-${process.pid}-${Date.now()}`;
  fs.writeFileSync(tempPath, contents, { encoding: "utf8", mode });
  try {
    fs.chmodSync(tempPath, mode);
  } catch {
    /* Windows / exotic FS may ignore mode */
  }
  const fd = fs.openSync(tempPath, "r+");
  try {
    fs.fsyncSync(fd);
  } finally {
    fs.closeSync(fd);
  }
  fs.renameSync(tempPath, filePath);
  try {
    const dirFd = fs.openSync(path.dirname(filePath), "r");
    try {
      fs.fsyncSync(dirFd);
    } finally {
      fs.closeSync(dirFd);
    }
  } catch {
    /* parent fsync not always supported */
  }
}

export function atomicWriteJson(filePath: string, value: unknown, mode = 0o600): void {
  atomicWriteFile(filePath, `${JSON.stringify(value, null, 2)}\n`, mode);
}

export class FileStateStore implements StateStore {
  constructor(private readonly filePath: string = defaultWorkspaceStatePath()) {}

  get path(): string {
    return this.filePath;
  }

  load(): WorkspaceStateV3 | null {
    // Sync read: registry hot paths already hold in-flight maps; failure → empty.
    try {
      const raw = fs.readFileSync(this.filePath, "utf8");
      return loadWorkspaceState(JSON.parse(raw));
    } catch {
      return null;
    }
  }

  save(state: WorkspaceStateV3): void {
    try {
      atomicWriteJson(this.filePath, state, 0o600);
    } catch {
      /* best-effort persistence; do not throw into window close path */
    }
  }
}

/** Async helpers for tests and bootstrap. */
export async function readWorkspaceStateFile(
  filePath: string,
): Promise<WorkspaceStateV3 | null> {
  try {
    const raw = await fsp.readFile(filePath, "utf8");
    return loadWorkspaceState(JSON.parse(raw));
  } catch {
    return null;
  }
}

export async function writeWorkspaceStateFile(
  filePath: string,
  state: WorkspaceStateV3,
): Promise<void> {
  atomicWriteJson(filePath, state, 0o600);
}
