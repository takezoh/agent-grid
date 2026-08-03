import { afterEach, describe, expect, it, vi } from "vitest";
import { GitHubDeviceFlowController } from "../src/main/github-device-flow.js";
import { createGitHubActionDispatcher } from "../src/main/github-action-dispatch.js";
import {
  Account,
  MemoryCredentialStore,
  fixedOAuthApp,
  type HttpDoer,
} from "../src/main/github-host.js";
import type { GitHubAction, GitHubActionResult } from "../src/main/repository-lease.js";

const result = (state: "authorizing" | "authenticated"): GitHubActionResult => ({
  outcome: state === "authenticated" ? "success" : "authorizing",
  account: { state, ...(state === "authenticated" ? { login: "alice" } : {}) },
  catalog: { outcome: "empty", generation: 0, stale: false, repositories: [] },
});

afterEach(() => vi.useRealTimers());

describe("GitHubDeviceFlowController", () => {
  it("completes the fake Device Flow and persists credentials without renderer polling", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-02T00:00:00Z"));
    let tokenPolls = 0;
    const doer: HttpDoer = {
      async do(request) {
        if (request.url.endsWith("/device/code")) {
          return response(200, {
            device_code: "fake-device",
            user_code: "TEST-CODE",
            verification_uri: "https://github.com/login/device",
            expires_in: 900,
            interval: 1,
          });
        }
        if (request.url.endsWith("/access_token")) {
          tokenPolls += 1;
          return response(
            200,
            tokenPolls === 1
              ? { error: "authorization_pending" }
              : { access_token: "gho_fake", token_type: "bearer", scope: "repo" },
          );
        }
        return response(200, { login: "fixture-user" });
      },
    };
    const store = new MemoryCredentialStore();
    const account = new Account(doer, { now: () => new Date() }, store, fixedOAuthApp());
    const catalog = {
      projection: vi.fn(() => ({
        outcome: "empty" as const,
        generation: 0,
        stale: false,
        repositories: [],
      })),
      refresh: vi.fn(async () => catalog.projection()),
    };
    const controller = new GitHubDeviceFlowController(
      createGitHubActionDispatcher(account, catalog),
    );

    await controller.action("start");
    await vi.advanceTimersByTimeAsync(2_000);

    expect(account.projection()).toEqual({ state: "authenticated", login: "fixture-user" });
    expect(account.hostCredential()).toEqual({ token: "gho_fake", ok: true });
    expect(tokenPolls).toBe(2);
    expect(catalog.refresh).toHaveBeenCalledTimes(1);
  });

  it("continues Device Flow in the main process without renderer poll actions", async () => {
    vi.useFakeTimers();
    const actions: GitHubAction[] = [];
    let polls = 0;
    const dispatch = vi.fn(async (action: GitHubAction) => {
      actions.push(action);
      if (action === "start") return result("authorizing");
      if (action === "poll") return result(++polls === 2 ? "authenticated" : "authorizing");
      return result("authorizing");
    });
    const controller = new GitHubDeviceFlowController(dispatch);

    await controller.action("start");
    await vi.advanceTimersByTimeAsync(2_000);

    expect(actions).toEqual(["start", "poll", "poll"]);
    expect(dispatch).toHaveBeenCalledTimes(3);
  });

  it("cancels the main-process polling timer", async () => {
    vi.useFakeTimers();
    const dispatch = vi.fn(async (action: GitHubAction) =>
      action === "cancel"
        ? {
            ...result("authorizing"),
            outcome: "cancel" as const,
            account: { state: "cancelled" as const },
          }
        : result("authorizing"),
    );
    const controller = new GitHubDeviceFlowController(dispatch);

    await controller.action("start");
    await controller.action("cancel");
    await vi.advanceTimersByTimeAsync(5_000);

    expect(dispatch.mock.calls.map(([action]) => action)).toEqual(["start", "cancel"]);
  });
});

function response(status: number, body: unknown) {
  return { status, headers: {}, body: JSON.stringify(body) };
}
