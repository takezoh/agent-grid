import * as fs from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";
import { describe, expect, it } from "vitest";
import { DaemonConfigResolver } from "../src/main/daemon-config.js";
import { assertTokenNotInUrl } from "../src/preload/index.js";

describe("daemon-config", () => {
  it("reads token fresh each resolve", async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), "ag-tok-"));
    const tokenPath = path.join(dir, "gateway-token");
    await fs.writeFile(tokenPath, "v1\n");
    const r = new DaemonConfigResolver({
      tokenPath,
      baseUrl: "http://127.0.0.1:8443",
      webOrigin: "http://127.0.0.1:5173",
    });
    expect((await r.resolve()).token).toBe("v1");
    await fs.writeFile(tokenPath, "v2\n");
    expect((await r.resolve()).token).toBe("v2");
  });

  it("throws explicit error when unreadable", async () => {
    const r = new DaemonConfigResolver({
      tokenPath: path.join(os.tmpdir(), `missing-${Date.now()}`),
      baseUrl: "http://127.0.0.1:8443",
      webOrigin: "http://127.0.0.1:5173",
    });
    await expect(r.resolve()).rejects.toThrow(/unreadable/);
  });

  it("hosted URL never includes token", () => {
    const r = new DaemonConfigResolver({
      tokenPath: "/x",
      baseUrl: "http://127.0.0.1:8443",
      webOrigin: "http://127.0.0.1:5173",
    });
    const url = r.hostedUrl("http://127.0.0.1:5173", "sess-9");
    expect(url).toContain("hosted=1");
    expect(url).toContain("session=sess-9");
    expect(url).not.toContain("token=");
    assertTokenNotInUrl(url, "super-secret-token");
  });
});

describe("assertTokenNotInUrl", () => {
  it("throws when token leaks", () => {
    expect(() =>
      assertTokenNotInUrl("http://x/?token=abc", "abc"),
    ).toThrow(/token/);
  });
});
