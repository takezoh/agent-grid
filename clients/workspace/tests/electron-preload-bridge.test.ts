// The preload bridge is a .cjs file that runs in Electron's isolated world, so
// it is outside tsc's reach — a `?? ""` there would silently turn "no body"
// back into "empty body" and no type check would notice. This test evaluates
// the real bridge source with a stubbed `require("electron")` and pins that it
// forwards the envelope verbatim.
//
// It loads the file through a require shim rather than `vi.mock("electron")`
// because the bridge is CommonJS: vitest externalizes the bare `electron`
// specifier for a plain `require`, so the module mock never applies and the
// bridge would receive the real (absent) module.

import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { beforeEach, describe, expect, it, vi } from "vitest";

const BRIDGE_PATH = fileURLToPath(
  new URL("../src/preload/electron-preload.cjs", import.meta.url),
);

type Envelope = { status: number; headers: Record<string, string>; body: string | null };

type Bridge = {
  sessionFleet: {
    request(
      serverId: string,
      pathname: string,
      init: Record<string, unknown>,
    ): Promise<Envelope>;
  };
};

const invoke = vi.fn();

function loadBridge(): Bridge {
  const exposed: Record<string, unknown> = {};
  const electron = {
    contextBridge: {
      exposeInMainWorld: (key: string, value: unknown) => {
        exposed[key] = value;
      },
    },
    ipcRenderer: {
      invoke,
      send: vi.fn(),
      sendSync: vi.fn(() => null),
      on: vi.fn(),
      removeListener: vi.fn(),
    },
  };
  const realRequire = createRequire(BRIDGE_PATH);
  const shim = (id: string) => (id === "electron" ? electron : realRequire(id));
  const module = { exports: {} as Record<string, unknown> };
  const factory = new Function("require", "module", "exports", readFileSync(BRIDGE_PATH, "utf8"));
  factory(shim, module, module.exports);
  return exposed.agentGridWorkspace as Bridge;
}

beforeEach(() => {
  invoke.mockReset();
});

describe("preload session-fleet request bridge", () => {
  it("forwards a null body without coercing it to a string", async () => {
    invoke.mockResolvedValueOnce({ status: 204, headers: {}, body: null });

    const result = await loadBridge().sessionFleet.request("srv-a", "/api/sessions/s1", {
      method: "DELETE",
    });

    // Counterexample this rejects: `body: ""`, which the renderer would hand to
    // `new Response("", {status: 204})` and Chromium would reject.
    expect(result.body).toBeNull();
    expect(result.status).toBe(204);
  });

  it("keeps an empty 200 body distinct from an absent one", async () => {
    invoke.mockResolvedValueOnce({ status: 200, headers: {}, body: "" });

    const result = await loadBridge().sessionFleet.request("srv-a", "/api/sessions", {});

    expect(result.body).toBe("");
  });

  it("passes the call through to the session-fleet-request channel unchanged", async () => {
    invoke.mockResolvedValueOnce({ status: 200, headers: {}, body: "{}" });
    const init = { method: "POST", headers: { "content-type": "application/json" }, body: "{}" };

    await loadBridge().sessionFleet.request("srv-b", "/api/sessions", init);

    expect(invoke).toHaveBeenLastCalledWith(
      "agent-grid:session-fleet-request",
      "srv-b",
      "/api/sessions",
      init,
    );
  });
});
