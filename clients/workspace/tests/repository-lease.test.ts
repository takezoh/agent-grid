/** Host projection parity + lease preflight/no-transfer + Settings IPC boundary. */
import { describe, expect, it, vi } from "vitest";
import type { ServerConfig } from "../src/main/desktop-config.js";
import type { AccountProjection, CatalogProjection } from "../src/main/github-host.js";
import { PROTOCOL_VERSION } from "../src/main/protocol-version.js";
import {
  LEASE_FAIL_AUTH_MODE, LEASE_FAIL_NO_CRED, LEASE_FAIL_PROTOCOL, authorityFromStore,
  buildHostRepositoryProjection, githubActionResult, projectionContainsCredential,
  requestRepositoryLease,
} from "../src/main/repository-lease.js";

const SENTINEL = "gho_SENTINEL_WORKSPACE_7f3a9c";
const projects = [{ server_id: "srv-a", project: "/repo/demo" }];
const account: AccountProjection = { state: "authenticated", login: "example-user" };
const catalogFailed: CatalogProjection = {
  outcome: "source-failed-retained", generation: 1, stale: true, failure_code: "source_failed",
  repositories: [{ repository_id: "1", owner: "example-user", name: "demo" }],
};

describe("HostRepositoryProjection parity", () => {
  it("deep-equals Web shape and keeps Projects on GitHub failure", () => {
    const got = buildHostRepositoryProjection(projects, account, catalogFailed, [{
      preparation_id: "prep-1", intent_id: "intent-1", server_id: "srv-a",
      repository: { repository_id: "1", owner: "example-user", name: "demo" },
      stage: "clone-in-progress", revision: 2, environment_disposition: "none",
    }]);
    expect(projectionContainsCredential(got)).toBe(false);
    expect(got.sources).toEqual([
      { id: "projects", available: true },
      { id: "github", available: true, status: "source-failed-retained" },
    ]);
    expect(got.targets.map((t) => t.source)).toEqual(["project", "repository"]);
    const hard = buildHostRepositoryProjection(
      projects, { state: "refresh_failed", failure_code: "refresh_failure" },
      { outcome: "authentication-required-retained", generation: 0, stale: false,
        failure_code: "authentication_required", repositories: [] },
    );
    expect(hard.sources[0]).toEqual({ id: "projects", available: true });
    expect(hard.targets).toHaveLength(1);
  });
});

describe("Repository lease preflight / no-transfer", () => {
  const okServer: ServerConfig = {
    id: "s", display_name: "s", enabled: true, url: "http://127.0.0.1:9",
    auth: { mode: "bearer", token: "server-secret" },
  };
  const store = {
    load: () => ({ record: { access_token: SENTINEL, generation: 3 }, ok: true as const }),
    save: () => {},
  };

  it("rejects auth.mode=none / protocol mismatch / insecure before transfer", async () => {
    const fetchFn = vi.fn(async () => { throw new Error("no"); }) as unknown as typeof fetch;
    for (const [server, proto, code] of [
      [{ ...okServer, auth: { mode: "none" as const } }, PROTOCOL_VERSION, LEASE_FAIL_AUTH_MODE],
      [okServer, "protocol-legacy", LEASE_FAIL_PROTOCOL],
      [{ ...okServer, url: "http://example.com" }, PROTOCOL_VERSION, LEASE_FAIL_AUTH_MODE],
    ] as const) {
      expect(await requestRepositoryLease({
        server, observedProtocol: proto,
        request: { repository_id: "1", owner: "o", name: "n" },
        auth: authorityFromStore(store), fetchFn,
      })).toEqual({ code });
    }
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it("transfers after preflight and returns credential-free issue", async () => {
    const fetchFn = vi.fn(async (_u: string, init?: RequestInit) => {
      expect(String(init?.body)).toContain(SENTINEL);
      return new Response(JSON.stringify({ lease_id: "lease-1", expires_at: "t" }), { status: 201 });
    }) as unknown as typeof fetch;
    const empty = { load: () => ({ record: { access_token: "", generation: 0 }, ok: false as const }), save: () => {} };
    expect(await requestRepositoryLease({
      server: okServer, observedProtocol: PROTOCOL_VERSION,
      request: { repository_id: "1", owner: "o", name: "n" },
      auth: authorityFromStore(empty), fetchFn,
    })).toEqual({ code: LEASE_FAIL_NO_CRED });
    const ok = await requestRepositoryLease({
      server: okServer, observedProtocol: PROTOCOL_VERSION,
      request: { repository_id: "1", owner: "o", name: "n" },
      auth: authorityFromStore(store), hostId: "host-1", fetchFn,
    });
    expect(ok.issue?.lease_id).toBe("lease-1");
    expect(projectionContainsCredential(ok)).toBe(false);
    expect(JSON.stringify(ok)).not.toContain(SENTINEL);
    expect(githubActionResult({ state: "denied", failure_code: "denied" },
      { outcome: "empty", generation: 0, stale: false, repositories: [] }).outcome).toBe("denied");
    expect(githubActionResult({ state: "cancelled", failure_code: "cancelled" },
      { outcome: "empty", generation: 0, stale: false, repositories: [] }).outcome).toBe("cancel");
    expect(githubActionResult({ state: "refresh_failed" },
      { outcome: "empty", generation: 0, stale: false, repositories: [] }).outcome).toBe("failure");
    expect(githubActionResult({ state: "authenticated", login: "u" },
      { outcome: "complete-current", generation: 1, stale: false,
        repositories: [{ repository_id: "1", owner: "u", name: "r" }] }).outcome).toBe("success");
  });
});

describe("repository Settings IPC / preload surface", () => {
  it("exposes credential-free success/denied/cancel/failure outcomes", async () => {
    const empty = { outcome: "empty" as const, generation: 0, stale: false, repositories: [] };
    const outcomes = [
      githubActionResult({ state: "authenticated", login: "a" }, {
        outcome: "complete-current", generation: 1, stale: false,
        repositories: [{ repository_id: "1", owner: "a", name: "r" }],
      }),
      githubActionResult({ state: "denied", failure_code: "denied" }, empty),
      githubActionResult({ state: "cancelled", failure_code: "cancelled" }, empty),
      githubActionResult({ state: "refresh_failed" }, empty),
    ];
    expect(outcomes.map((o) => o.outcome)).toEqual(["success", "denied", "cancel", "failure"]);
    const { buildPreloadApi } = await import("../src/preload/index.js");
    let i = 0;
    const api = buildPreloadApi({
      hostedModeInfo: null,
      windowControls: { minimize() {}, maximize() {}, close() {} },
      requestJumpBack: async () => {},
      github: { action: async () => outcomes[i++]! },
    });
    for (const expected of outcomes) {
      const got = await api.github!.action("status");
      expect(got.outcome).toBe(expected.outcome);
      expect(projectionContainsCredential(got)).toBe(false);
    }
  });
});
