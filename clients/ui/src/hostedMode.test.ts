import { afterEach, describe, expect, it } from "vitest";
import { hostedSessionId, isHostedMode, readBearerTokenFromHash } from "./auth";

/**
 * Hosted-mode SPA contracts (plan §4.3 / contract-b2-hosted-mode-token-injection /
 * contract-hosted-mode-existing-spa-compat). Browser path remains the default.
 */
describe("hosted mode SPA flags", () => {
  afterEach(() => {
    delete window.hostedModeInfo;
    delete window.agentGridWorkspace;
    window.location.hash = "";
    window.history.replaceState({}, "", "/");
    delete document.documentElement.dataset.hosted;
    delete document.body.dataset.hosted;
  });

  it("does not treat browser mode as hosted", () => {
    expect(isHostedMode()).toBe(false);
    expect(hostedSessionId()).toBeNull();
  });

  it("never reads token from hosted URL query", () => {
    window.history.replaceState({}, "", "/?hosted=1&session=s1&token=leaked");
    // Query token is ignored; only hash (browser) or preload counts.
    expect(readBearerTokenFromHash()).toBe("");
    expect(isHostedMode()).toBe(true);
    expect(hostedSessionId()).toBe("s1");
  });

  it("preload token wins and session is fixed", () => {
    window.hostedModeInfo = {
      hosted: true,
      sessionId: "sess-hosted",
      baseUrl: "http://127.0.0.1:8443",
      token: "preload-secret",
    };
    window.location.hash = "#token=hash-should-lose";
    expect(readBearerTokenFromHash()).toBe("preload-secret");
    expect(hostedSessionId()).toBe("sess-hosted");
  });
});
