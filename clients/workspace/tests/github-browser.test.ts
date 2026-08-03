import { describe, expect, it, vi } from "vitest";
import {
  githubAuthorizationUrl,
  handoffGitHubAuthorization,
  openGitHubAuthorization,
} from "../src/main/github-browser.js";

describe("GitHub browser handoff", () => {
  it("opens only the GitHub device verification page", async () => {
    const openExternal = vi.fn(async () => undefined);
    await openGitHubAuthorization(
      {
        state: "authorizing",
        user_code: "TEST-CODE",
        verification_uri: "https://github.com/login/device",
      },
      openExternal,
    );
    expect(openExternal).toHaveBeenCalledWith("https://github.com/login/device");
  });

  it("rejects non-GitHub, non-HTTPS, and non-authorizing projections", () => {
    for (const account of [
      { state: "authorizing" as const, verification_uri: "https://attacker.invalid/login/device" },
      { state: "authorizing" as const, verification_uri: "http://github.com/login/device" },
      { state: "authorizing" as const, verification_uri: "https://github.com/settings/apps" },
      { state: "unauthenticated" as const, verification_uri: "https://github.com/login/device" },
    ]) {
      expect(githubAuthorizationUrl(account)).toBeNull();
    }
  });

  it("copies the code, waits for browser completion, and reports opener failure", async () => {
    const writeCode = vi.fn();
    let finishOpen!: () => void;
    const opening = new Promise<void>((resolve) => { finishOpen = resolve; });
    const openExternal = vi.fn(() => opening);
    const handoff = handoffGitHubAuthorization({
      state: "authorizing",
      user_code: "ABCD-EFGH",
      verification_uri: "https://github.com/login/device",
    }, writeCode, openExternal);
    let settled = false;
    void handoff.then(() => { settled = true; });
    await Promise.resolve();

    expect(writeCode).toHaveBeenCalledWith("ABCD-EFGH");
    expect(settled).toBe(false);
    finishOpen();
    await expect(handoff).resolves.toBe(true);
    await expect(handoffGitHubAuthorization({
      state: "authorizing",
      user_code: "ABCD-EFGH",
      verification_uri: "https://github.com/login/device",
    }, writeCode, async () => { throw new Error("no browser"); })).resolves.toBe(false);
  });
});
