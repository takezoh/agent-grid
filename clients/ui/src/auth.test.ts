import { afterEach, describe, expect, it } from "vitest";
import {
  hostedSessionId,
  isHostedMode,
  readBearerTokenFromHash,
} from "./auth";

describe("auth hosted mode", () => {
  afterEach(() => {
    delete window.hostedModeInfo;
    delete window.agentGridWorkspace;
    window.location.hash = "";
    // happy-dom: reset search by replacing location is awkward; set via history when available
    window.history.replaceState({}, "", "/");
  });

  it("reads token from hash in browser mode", () => {
    window.location.hash = "#token=from-hash";
    expect(readBearerTokenFromHash()).toBe("from-hash");
    expect(isHostedMode()).toBe(false);
  });

  it("prefers preload token over hash", () => {
    window.location.hash = "#token=hash-tok";
    window.hostedModeInfo = {
      hosted: true,
      sessionId: "s1",
      baseUrl: "http://127.0.0.1:8443",
      token: "preload-tok",
    };
    expect(readBearerTokenFromHash()).toBe("preload-tok");
    expect(isHostedMode()).toBe(true);
    expect(hostedSessionId()).toBe("s1");
  });

  it("detects hosted query flag without token in URL", () => {
    window.history.replaceState({}, "", "/?hosted=1&session=sess-x");
    expect(isHostedMode()).toBe(true);
    expect(hostedSessionId()).toBe("sess-x");
    expect(readBearerTokenFromHash()).toBe("");
  });
});
