// @vitest-environment node
//
// RED (debug-run-workspace-electron-terminal-dead / hypothesis-workspace-204-response)
//
// The Workspace (Electron) fleet provider re-materialises an HTTP response that
// crossed the preload IPC boundary as `new Response(result.body, {status})`
// (src/fleet/provider.ts:644). The Electron main handler always produces
// `body: await response.text()` (clients/workspace/src/main/electron-entry.ts),
// so a 204/205/304 route yields a non-null (empty-string) body for a null-body
// status. Both Chromium and Node's WHATWG `Response` reject that combination,
// so the constructor throws and api/sessions.ts doFetch() maps the throw to
// Error("network") -> "Failed to stop session (network error)".
//
// This file deliberately opts out of the project-wide happy-dom environment:
// happy-dom 20.10.6 accepts `new Response("", {status:204})` and silently
// coerces the body to null, so it cannot discriminate the defect from the fix.
// Node 24's global Response enforces the same invariant as Chromium, which
// makes the seam testable at T1.
//
// The contract these tests pin: a routed Workspace request whose upstream
// response carries a null-body status resolves to a Response instead of
// throwing, and preserves that status.

import { afterEach, describe, expect, it, vi } from "vitest";
import { makeFleetProvider } from "./provider";

type WorkspaceBridge = NonNullable<Window["agentGridWorkspace"]>;

function installWorkspaceBridge(
  request: (
    serverId: string,
    pathname: string,
    init: { method?: string; headers?: Record<string, string>; body?: string },
  ) => Promise<{ status: number; headers: Record<string, string>; body: string }>,
): void {
  const bridge = {
    hostedModeInfo: {
      hosted: true,
      serverId: "srv-a",
      baseUrl: "https://gateway.invalid",
      token: "renderer-token",
    },
    openSettings: vi.fn(),
    sessionFleet: {
      listConfigs: vi.fn().mockResolvedValue({
        schemaVersion: 1,
        revision: 1,
        servers: [],
      }),
      create: vi.fn(),
      request,
      subscribeHealth: () => () => undefined,
      subscribeSessions: () => () => undefined,
    },
  } as unknown as WorkspaceBridge;
  (globalThis as { window?: unknown }).window = { agentGridWorkspace: bridge };
}

afterEach(() => {
  (globalThis as { window?: unknown }).window = undefined;
});

describe("workspaceProvider.request over null-body statuses", () => {
  it("is a Node environment whose Response enforces the null-body-status invariant", () => {
    // Guard: if this ever stops throwing, the environment silently regressed to
    // a permissive Response and every assertion below becomes a false green.
    expect(() => new Response("", { status: 204 })).toThrow();
    expect(new Response(null, { status: 204 }).status).toBe(204);
    expect(new Response("", { status: 200 }).status).toBe(200);
  });

  it("returns a 204 Response when the upstream route has no content", async () => {
    installWorkspaceBridge(async () => ({ status: 204, headers: {}, body: "" }));
    const fleet = makeFleetProvider();

    // Counterexample this rejects: throwing a TypeError, which doFetch() maps to
    // Error("network") and the UI reports as "(network error)" for a request
    // that actually succeeded.
    const resp = await fleet.request("srv-a", "/api/sessions/s1", { method: "DELETE" });

    expect(resp.status).toBe(204);
    expect(await resp.text()).toBe("");
  });

  it("returns a 304 Response when the upstream route reports not-modified", async () => {
    installWorkspaceBridge(async () => ({ status: 304, headers: { etag: "e1" }, body: "" }));
    const fleet = makeFleetProvider();

    const resp = await fleet.request("srv-a", "/api/sessions/s1/transcript?offset=0", {});

    expect(resp.status).toBe(304);
    expect(resp.headers.get("etag")).toBe("e1");
  });

  it("still returns bodies for ordinary statuses", async () => {
    installWorkspaceBridge(async () => ({
      status: 200,
      headers: { "content-type": "application/json" },
      body: '{"ok":true}',
    }));
    const fleet = makeFleetProvider();

    const resp = await fleet.request("srv-a", "/api/sessions", {});

    expect(resp.status).toBe(200);
    expect(await resp.text()).toBe('{"ok":true}');
  });
});
