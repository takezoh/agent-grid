import { describe, expect, it } from "vitest";
import {
  WindowRegistry,
  loadWorkspaceState,
  type WindowFactory,
  type WindowHandle,
  type WorkspaceStateV2,
} from "../src/main/window-registry.js";
import type { SessionRef } from "../src/shared/session-ref.js";

function memFactory() {
  const created: string[] = [];
  const factory: WindowFactory = {
    create(session: SessionRef): WindowHandle {
      const id = `${session.serverId}:${session.sessionId}`;
      created.push(id);
      let destroyed = false;
      let bounds = { x: 0, y: 0, width: 800, height: 600 };
      return {
        id,
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
    const a = await reg.openSession({ serverId: "one", sessionId: "s1" });
    const b = await reg.openSession({ serverId: "one", sessionId: "s1" });
    expect(a).toBe(b);
    expect(created).toEqual(["one:s1"]);
    expect(reg.openCount).toBe(1);
  });

  it("converges concurrent openSession to one window", async () => {
    const { factory, created } = memFactory();
    const reg = new WindowRegistry(factory);
    const results = await Promise.all(
      Array.from({ length: 20 }, () =>
        reg.openSession({ serverId: "one", sessionId: "race" })),
    );
    expect(new Set(results).size).toBe(1);
    expect(created.filter((id) => id === "one:race")).toHaveLength(1);
    expect(reg.openCount).toBe(1);
  });

  it("closeSessionView does not block re-open (view collapse only)", async () => {
    const { factory, created } = memFactory();
    const reg = new WindowRegistry(factory);
    const session = { serverId: "one", sessionId: "s1" };
    await reg.openSession(session);
    reg.closeSessionView(session);
    expect(reg.openCount).toBe(0);
    await reg.openSession(session);
    expect(created).toEqual(["one:s1", "one:s1"]);
  });

  it("different sessions get different windows", async () => {
    const { factory, created } = memFactory();
    const reg = new WindowRegistry(factory);
    await reg.openSession({ serverId: "one", sessionId: "same" });
    await reg.openSession({ serverId: "two", sessionId: "same" });
    expect(created).toEqual(["one:same", "two:same"]);
    expect(reg.openCount).toBe(2);
  });
});

describe("workspace-state schema evolution", () => {
  it("loads v2", () => {
    const raw = {
      schema_version: 2,
      windows: { one: { s1: { x: 1, y: 2, width: 3, height: 4 } } },
    };
    const s = loadWorkspaceState(raw) as WorkspaceStateV2;
    expect(s.schema_version).toBe(2);
    expect(s.windows.one?.s1?.width).toBe(3);
  });

  it("refuses unknown version without silent corruption", () => {
    expect(loadWorkspaceState({ schema_version: 1, windows: { s: {} } })).toBeNull();
    expect(loadWorkspaceState({ schema_version: 99, windows: { s: {} } })).toBeNull();
  });

  it("refuses garbage", () => {
    expect(loadWorkspaceState(null)).toBeNull();
    expect(loadWorkspaceState("x")).toBeNull();
  });
});
