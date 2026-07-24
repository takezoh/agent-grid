/**
 * Persist workspace window layout to disk (schema_version: 2).
 * Default path: %APPDATA%/agent-grid/workspace-state.json on Windows.
 * contract-workspace-state-schema-evolution via loadWorkspaceState.
 */

import * as fs from "node:fs";
import * as fsp from "node:fs/promises";
import * as path from "node:path";
import * as os from "node:os";
import {
  loadWorkspaceState,
  type StateStore,
  type WorkspaceStateV2,
} from "./window-registry.js";

export function defaultWorkspaceStatePath(): string {
  if (process.platform === "win32") {
    const appData = process.env.APPDATA ?? path.join(os.homedir(), "AppData", "Roaming");
    return path.join(appData, "agent-grid", "workspace-state.json");
  }
  const xdg = process.env.XDG_STATE_HOME ?? path.join(os.homedir(), ".local", "state");
  return path.join(xdg, "agent-grid", "workspace-state.json");
}

export class FileStateStore implements StateStore {
  constructor(private readonly filePath: string = defaultWorkspaceStatePath()) {}

  get path(): string {
    return this.filePath;
  }

  load(): WorkspaceStateV2 | null {
    // Sync read: registry hot paths already hold in-flight maps; failure → empty.
    try {
      const raw = fs.readFileSync(this.filePath, "utf8");
      return loadWorkspaceState(JSON.parse(raw));
    } catch {
      return null;
    }
  }

  save(state: WorkspaceStateV2): void {
    try {
      fs.mkdirSync(path.dirname(this.filePath), { recursive: true });
      fs.writeFileSync(this.filePath, JSON.stringify(state, null, 2), "utf8");
    } catch {
      /* best-effort persistence; do not throw into window close path */
    }
  }
}

/** Async helpers for tests and bootstrap. */
export async function readWorkspaceStateFile(
  filePath: string,
): Promise<WorkspaceStateV2 | null> {
  try {
    const raw = await fsp.readFile(filePath, "utf8");
    return loadWorkspaceState(JSON.parse(raw));
  } catch {
    return null;
  }
}

export async function writeWorkspaceStateFile(
  filePath: string,
  state: WorkspaceStateV2,
): Promise<void> {
  await fsp.mkdir(path.dirname(filePath), { recursive: true });
  await fsp.writeFile(filePath, JSON.stringify(state, null, 2), "utf8");
}
