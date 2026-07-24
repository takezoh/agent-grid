import { describe, expect, it } from "vitest";
import {
  WindowRegistry,
  loadWorkspaceState,
  type WindowFactory,
  type WindowHandle,
  type WorkspaceStateV1,
} from "../src/main/window-registry.js";

function memFactory() {
  const created: string[] = [];
  const factory: WindowFactory = {
    create(sessionId: string): WindowHandle {
      created.push(sessionId);
      let destroyed = false;
      let bounds = { x: 0, y: 0, width: 800, height: 600 };
      return {
        id: sessionId,
        focus() {},
        close() {
          destroyed = true;
        },
        isDestroyed() {
          return destroyed;
        },
        getBounds() {
          return { ...bounds };
        },
        setBounds(b) {
          bounds = { ...b };
        },
      };
    },
  };
  return { factory, created };
}

describe("window-registry", () => {
  it("reuses existing window for same session", async () => {
    const { factory, created } = memFactory();
    const reg = new WindowRegistry(factory);
    const a = await reg.openSession("s1");
    const b = await reg.openSession("s1");
    expect(a).toBe(b);
    expect(created).toEqual(["s1"]);
    expect(reg.openCount).toBe(1);
  });

  it("converges concurrent openSession to one window", async () => {
    const { factory, created } = memFactory();
    const reg = new WindowRegistry(factory);
    const results = await Promise.all(
      Array.from({ length: 20 }, () => reg.openSession("race")),
    );
    expect(new Set(results).size).toBe(1);
    expect(created.filter((id) => id === "race")).toHaveLength(1);
    expect(reg.openCount).toBe(1);
  });

  it("closeSessionView does not block re-open (view collapse only)", async () => {
    const { factory, created } = memFactory();
    const reg = new WindowRegistry(factory);
    await reg.openSession("s1");
    reg.closeSessionView("s1");
    expect(reg.openCount).toBe(0);
    await reg.openSession("s1");
    expect(created).toEqual(["s1", "s1"]);
  });

  it("different sessions get different windows", async () => {
    const { factory, created } = memFactory();
    const reg = new WindowRegistry(factory);
    await reg.openSession("a");
    await reg.openSession("b");
    expect(created).toEqual(["a", "b"]);
    expect(reg.openCount).toBe(2);
  });
});

describe("workspace-state schema evolution", () => {
  it("loads v1", () => {
    const raw = {
      schema_version: 1,
      windows: { s1: { x: 1, y: 2, width: 3, height: 4 } },
    };
    const s = loadWorkspaceState(raw) as WorkspaceStateV1;
    expect(s.schema_version).toBe(1);
    expect(s.windows.s1?.width).toBe(3);
  });

  it("refuses unknown version without silent corruption", () => {
    expect(loadWorkspaceState({ schema_version: 99, windows: { s: {} } })).toBeNull();
  });

  it("refuses garbage", () => {
    expect(loadWorkspaceState(null)).toBeNull();
    expect(loadWorkspaceState("x")).toBeNull();
  });
});
