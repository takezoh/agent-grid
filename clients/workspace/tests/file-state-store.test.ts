import * as fs from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  FileStateStore,
  readWorkspaceStateFile,
  writeWorkspaceStateFile,
} from "../src/main/file-state-store.js";
import { loadWorkspaceState } from "../src/main/window-registry.js";

describe("file-state-store", () => {
  const tmpFiles: string[] = [];

  afterEach(async () => {
    for (const f of tmpFiles) {
      try {
        await fs.unlink(f);
      } catch {
        /* ignore */
      }
    }
    tmpFiles.length = 0;
  });

  function tmp(): string {
    const p = path.join(os.tmpdir(), `ag-ws-state-${Math.random().toString(16).slice(2)}.json`);
    tmpFiles.push(p);
    return p;
  }

  it("round-trips v1 state", async () => {
    const p = tmp();
    const state = {
      schema_version: 1 as const,
      windows: { "sess-a": { x: 10, y: 20, width: 800, height: 600 } },
    };
    await writeWorkspaceStateFile(p, state);
    const loaded = await readWorkspaceStateFile(p);
    expect(loaded).toEqual(state);

    const store = new FileStateStore(p);
    expect(store.load()).toEqual(state);
    store.save({
      schema_version: 1,
      windows: { "sess-b": { x: 0, y: 0, width: 1, height: 1 } },
    });
    expect(store.load()?.windows["sess-b"]?.width).toBe(1);
  });

  it("refuses unknown schema versions", () => {
    expect(loadWorkspaceState({ schema_version: 99, windows: {} })).toBeNull();
    expect(loadWorkspaceState(null)).toBeNull();
  });

  it("missing file loads as null", () => {
    const store = new FileStateStore(path.join(os.tmpdir(), "ag-missing-state.json"));
    expect(store.load()).toBeNull();
  });
});
