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
      serverId: "local",
      tokenPath,
      baseUrl: "http://127.0.0.1:8443",
      webOrigin: "http://127.0.0.1:5173",
    });
    expect((await r.resolve("local")).token).toBe("v1");
    await fs.writeFile(tokenPath, "v2\n");
    expect((await r.resolve("local")).token).toBe("v2");
  });

  it("throws explicit error when unreadable", async () => {
    const r = new DaemonConfigResolver({
      serverId: "local",
      tokenPath: path.join(os.tmpdir(), `missing-${Date.now()}`),
      baseUrl: "http://127.0.0.1:8443",
      webOrigin: "http://127.0.0.1:5173",
    });
    await expect(r.resolve("local")).rejects.toThrow(/unreadable/);
  });

  it("selects the configured connection by serverId", async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), "ag-tok-multi-"));
    const one = path.join(dir, "one");
    const two = path.join(dir, "two");
    await fs.writeFile(one, "token-one");
    await fs.writeFile(two, "token-two");
    const resolver = new DaemonConfigResolver([
      {
        serverId: "one",
        tokenPath: one,
        baseUrl: "http://one.test",
        webOrigin: "http://one-ui.test",
      },
      {
        serverId: "two",
        tokenPath: two,
        baseUrl: "http://two.test",
        webOrigin: "http://two-ui.test",
      },
    ]);

    expect(await resolver.resolve("one")).toMatchObject({
      baseUrl: "http://one.test",
      token: "token-one",
    });
    expect(await resolver.resolve("two")).toMatchObject({
      baseUrl: "http://two.test",
      token: "token-two",
    });
  });

  it("hosted URL never includes token", () => {
    const r = new DaemonConfigResolver({
      serverId: "local",
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
