// @vitest-environment node
//
// adr-20260727-host-neutral-fleet-provider says the browser host and the
// Workspace host may differ in transport and in nothing else. They did differ
// in behavior: only the Workspace path threw on a null-body status, because the
// IPC envelope could not say "no body".
//
// This file pins the parity as an observable claim — same status, same headers,
// same text on both providers — over the status matrix that reaches these
// routes. It runs in the node environment on purpose: happy-dom accepts
// `new Response("", {status: 204})`, so under it a permissive fake and a
// correct one are indistinguishable and every assertion here is a false green.

import { afterEach, describe, expect, it, vi } from "vitest";
import { makeFleetProvider } from "./provider";

type WorkspaceBridge = NonNullable<Window["agentGridWorkspace"]>;

/** Statuses the WHATWG Fetch Standard forbids from carrying a body. */
const NULL_BODY = new Set([204, 205, 304]);

const MATRIX = [200, 201, 204, 205, 304, 400, 404, 500];

function bodyFor(status: number): string | null {
  // What the real envelope builder produces. A fake that returned "" for a
  // null-body status would be more permissive than the real one and would hide
  // a producer regression, so this mirrors the builder rather than the wire.
  return NULL_BODY.has(status) ? null : `{"status":${status}}`;
}

function installWorkspaceBridge(status: number): void {
  const bridge = {
    hostedModeInfo: {
      hosted: true,
      serverId: "srv-a",
      baseUrl: "https://gateway.invalid",
      token: "renderer-token",
    },
    openSettings: vi.fn(),
    sessionFleet: {
      listConfigs: vi.fn().mockResolvedValue({ schemaVersion: 1, revision: 1, servers: [] }),
      create: vi.fn(),
      request: async () => ({ status, headers: { "x-probe": "1" }, body: bodyFor(status) }),
      subscribeHealth: () => () => undefined,
      subscribeSessions: () => () => undefined,
    },
  } as unknown as WorkspaceBridge;
  (globalThis as { window?: unknown }).window = { agentGridWorkspace: bridge };
}

function installBrowserWindow(): void {
  (globalThis as { window?: unknown }).window = {};
}

/** The browser host returns whatever fetch produced, untouched. */
function browserFetch(status: number): typeof fetch {
  return (async () =>
    new Response(bodyFor(status), {
      status,
      headers: { "x-probe": "1" },
    })) as unknown as typeof fetch;
}

async function observe(resp: Response) {
  return {
    status: resp.status,
    ok: resp.ok,
    probe: resp.headers.get("x-probe"),
    text: await resp.text(),
  };
}

afterEach(() => {
  (globalThis as { window?: unknown }).window = undefined;
});

describe("FleetProvider.request host parity", () => {
  it.each(MATRIX)("observes the same response on both hosts for status %i", async (status) => {
    installBrowserWindow();
    const web = await observe(
      await makeFleetProvider(browserFetch(status)).request("srv-a", "/api/sessions/s1", {}),
    );

    installWorkspaceBridge(status);
    const workspace = await observe(
      await makeFleetProvider().request("srv-a", "/api/sessions/s1", {}),
    );

    expect(workspace).toEqual(web);
    expect(workspace.status).toBe(status);
  });

  // Fidelity backstop: if the fake bridge is ever made more permissive than the
  // real envelope builder (returning "" where the builder returns null), this
  // is the assertion that fails. Without it the permissive fake would keep the
  // suite green while the real producer regressed.
  it("rejects a fake bridge that sends a body with a null-body status", () => {
    for (const status of NULL_BODY) {
      expect(bodyFor(status)).toBeNull();
      expect(() => new Response("", { status })).toThrow();
    }
    expect(bodyFor(200)).not.toBeNull();
  });
});
