/**
 * GitHubAuthContract + GitHubCatalogContract + Web/Workspace parity + FakeVsReal.
 * rejects counterexample: SPA projection containing access/refresh tokens;
 * rejects counterexample: clearing retained repos or publishing partial pages.
 */
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import {
  Account, Catalog, FailHostMisconfigured, FailTransport, FileCacheStore, FileCredentialStore,
  MAX_PAGES, ManualClock,
  MemoryCacheStore, MemoryCredentialStore, REFRESH_DEADLINE_MS, fixedOAuthApp,
  projectionContainsCredential, resolveGitHubAuthAuthority, resolveGitHubCatalogAuthority,
  type CatalogProjection, type HttpDoer, type HttpRequest, type HttpResponse, type Repository,
} from "../src/main/github-host.js";

const CONFIG = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../config");
const EPOCH = new Date(1_700_000_000_000);
const noCred = (v: unknown) => expect(projectionContainsCredential(v)).toBe(false);
const j = (status: number, v: unknown): HttpResponse =>
  ({ status, headers: { "content-type": "application/json" }, body: JSON.stringify(v) });

/** FakeGitHubAuth — contract/fidelity fake */
class FakeGitHubAuth implements HttpDoer {
  deviceCodes: Array<Record<string, unknown>> = [];
  tokenPolls: Array<Record<string, unknown>> = [];
  userLogin = "";
  private di = 0; private ti = 0;
  async do(req: HttpRequest): Promise<HttpResponse> {
    if (req.url.includes("/login/device/code")) {
      const o = this.di < this.deviceCodes.length ? this.deviceCodes[this.di++]! : {
        device_code: "dev", user_code: "ABCD-1234",
        verification_uri: "https://github.com/login/device", expires_in: 900, interval: 1,
      };
      return j(200, o);
    }
    if (req.url.includes("/login/oauth/access_token")) {
      return j(200, this.ti < this.tokenPolls.length ? this.tokenPolls[this.ti++]! : { error: "authorization_pending" });
    }
    if (req.url.includes("/user")) return j(200, { login: this.userLogin });
    return j(404, { m: "nf" });
  }
  resetTokenIndex() { this.ti = 0; }
}

/** FakeGitHubCatalog — contract/fidelity fake */
class FakeGitHubCatalog implements HttpDoer {
  pages: Repository[][] = [];
  failAtPage = 0; authStatus = 0; pageCalls = 0; etag = "";
  async do(req: HttpRequest): Promise<HttpResponse> {
    if (!req.url.includes("/user/repos")) return j(404, { m: "nf" });
    if (this.authStatus) return j(this.authStatus, { m: "bad" });
    this.pageCalls += 1;
    const pages = this.pages.length ? this.pages : [[]];
    let idx = this.pageCalls - 1;
    if (idx >= pages.length) idx = 0;
    if (this.failAtPage > 0 && this.pageCalls === this.failAtPage) return j(500, { m: "boom" });
    let next: string | null = null;
    if (this.pageCalls < pages.length) {
      const u = new URL(req.url);
      next = `${u.origin}${u.pathname}?page=${this.pageCalls + 1}`;
    }
    const raw = (pages[idx] ?? []).map((r, i) => ({
      id: Number(r.repository_id) || i + 1, name: r.name,
      full_name: `${r.owner}/${r.name}`, owner: { login: r.owner },
    }));
    const headers: Record<string, string> = { "content-type": "application/json" };
    if (next) headers.link = `<${next}>; rel="next"`;
    if (this.etag) headers.etag = this.etag;
    return { status: 200, headers, body: JSON.stringify(raw) };
  }
}

class CombinedTransport implements HttpDoer {
  constructor(readonly auth: HttpDoer, readonly catalog: HttpDoer) {}
  async do(req: HttpRequest) {
    return req.url.includes("/user/repos") ? this.catalog.do(req) : this.auth.do(req);
  }
}

const newAcct = (auth: FakeGitHubAuth) =>
  new Account(auth, new ManualClock(EPOCH), new MemoryCredentialStore(), fixedOAuthApp());

async function pollAfterInterval(account: Account, milliseconds = 5_000) {
  if (!(account.clock instanceof ManualClock)) throw new Error("manual clock required");
  account.clock.advance(milliseconds);
  return account.pollDeviceFlow();
}

async function newCat(catalog: FakeGitHubCatalog): Promise<Catalog> {
  const auth = new FakeGitHubAuth();
  auth.userLogin = "alice";
  auth.tokenPolls = [{ access_token: "gho_secret", refresh_token: "ghr_secret" }];
  const http = new CombinedTransport(auth, catalog);
  const a = new Account(http, new ManualClock(EPOCH), new MemoryCredentialStore(), fixedOAuthApp());
  await a.startDeviceFlow(); await pollAfterInterval(a);
  return new Catalog(http, new ManualClock(EPOCH), new MemoryCacheStore(), a);
}

function fetchDoer(): HttpDoer {
  return {
    async do(req) {
      const res = await fetch(req.url, { method: req.method, headers: req.headers, body: req.body });
      const headers: Record<string, string> = {};
      res.headers.forEach((v, k) => { headers[k] = v; });
      return { status: res.status, headers, body: await res.text() };
    },
  };
}

describe("GitHubAuthContract", () => {
  it("enforces the device interval and increases it after slow_down", async () => {
    const auth = new FakeGitHubAuth();
    auth.deviceCodes = [{
      device_code: "dev",
      user_code: "ABCD-1234",
      verification_uri: "https://github.com/login/device",
      expires_in: 900,
      interval: 5,
    }];
    auth.tokenPolls = [
      { error: "slow_down" },
      { access_token: "gho_secret", refresh_token: "ghr_secret" },
    ];
    const clock = new ManualClock(EPOCH);
    const account = new Account(auth, clock, new MemoryCredentialStore(), fixedOAuthApp());

    await account.startDeviceFlow();
    expect((await account.pollDeviceFlow()).state).toBe("authorizing");
    clock.advance(5_000);
    expect((await account.pollDeviceFlow()).state).toBe("authorizing");
    clock.advance(9_999);
    expect((await account.pollDeviceFlow()).state).toBe("authorizing");
    clock.advance(1);
    expect((await account.pollDeviceFlow()).state).toBe("authenticated");
  });

  it("distinct outcomes + credential sentinel non-exposure", async () => {
    const auth = new FakeGitHubAuth();
    auth.userLogin = "alice";
    auth.tokenPolls = [{ access_token: "gho_secret_token", refresh_token: "ghr_r" }];
    const a = newAcct(auth);
    await a.startDeviceFlow();
    const p = await pollAfterInterval(a);
    expect(p).toMatchObject({ state: "authenticated", login: "alice" });
    noCred(p);
    expect(a.hostCredential()).toEqual({ token: "gho_secret_token", ok: true });
    for (const [err, want] of [["access_denied", "denied"], ["expired_token", "expired"]] as const) {
      const b = new FakeGitHubAuth(); b.tokenPolls = [{ error: err }];
      const acct = newAcct(b); await acct.startDeviceFlow();
      expect((await pollAfterInterval(acct)).state).toBe(want);
    }
    const c = newAcct(new FakeGitHubAuth()); await c.startDeviceFlow();
    expect(c.cancelDeviceFlow().state).toBe("cancelled");
    const mis = new Account(new FakeGitHubAuth(), new ManualClock(new Date(1)),
      new MemoryCredentialStore(), { schema_version: 1, client_id: "", scopes: [] });
    await expect(mis.startDeviceFlow()).rejects.toThrow(/client_id/);
    expect(mis.projection().state).toBe("host_misconfigured");
    const fake = new FakeGitHubAuth();
    fake.userLogin = "a"; fake.tokenPolls = [{ access_token: "gho_a", refresh_token: "ghr_r" }];
    const d = newAcct(fake); await d.startDeviceFlow(); await pollAfterInterval(d);
    fake.tokenPolls = [{ error: "bad" }]; fake.resetTokenIndex();
    await expect(d.refreshAccessToken()).rejects.toThrow();
    expect(d.projection().state).toBe("refresh_failed");
  });

  it("types only device-code OAuth App failures as host misconfiguration", async () => {
    // Counterexample rejected: mapping every HTTP failure to a config error fails the 503 case.
    for (const [status, oauthError, failureCode] of [
      [404, "rejected", FailHostMisconfigured],
      [400, "device_flow_disabled", FailHostMisconfigured],
      [400, "incorrect_client_credentials", FailHostMisconfigured],
      [400, "authorization_pending", FailTransport],
      [503, "rejected", FailTransport],
    ] as const) {
      let transportCalls = 0;
      const account = new Account(
        {
          async do() {
            transportCalls += 1;
            return j(status, { error: oauthError });
          },
        },
        new ManualClock(EPOCH),
        new MemoryCredentialStore(),
        fixedOAuthApp(),
      );

      await expect(account.startDeviceFlow()).rejects.toThrow(`status ${status}`);
      expect(transportCalls).toBe(1);
      expect(account.projection()).toEqual({
        state: "host_misconfigured",
        failure_code: failureCode,
      });
      noCred(account.projection());
    }

    const token404: HttpDoer = {
      async do(req) {
        if (req.url.endsWith("/device/code")) {
          return j(200, {
            device_code: "synthetic-device",
            user_code: "TEST-CODE",
            verification_uri: "https://github.example/device",
            expires_in: 900,
          });
        }
        return j(404, { error: "rejected" });
      },
    };
    const tokenAccount = new Account(
      token404,
      new ManualClock(EPOCH),
      new MemoryCredentialStore(),
      fixedOAuthApp(),
    );
    await tokenAccount.startDeviceFlow();
    expect(await pollAfterInterval(tokenAccount)).toMatchObject({
      state: "host_misconfigured",
      failure_code: FailTransport,
    });
  });
});

describe("GitHubCatalogContract", () => {
  it("limits, partial non-publish, stale fallback, singleflight", async () => {
    const fake = new FakeGitHubCatalog();
    fake.pages = [[{ repository_id: "1", owner: "o", name: "a" }, { repository_id: "2", owner: "o", name: "b" }]];
    fake.etag = `"g"`;
    let release!: () => void; const gate = new Promise<void>((r) => { release = r; });
    let entered!: () => void; const enteredP = new Promise<void>((r) => { entered = r; });
    let first = false;
    const blocked: HttpDoer = {
      async do(req) {
        if (!first) { first = true; entered(); await gate; }
        return fake.do(req);
      },
    };
    const cat = await newCat(fake);
    cat.doer = new CombinedTransport(new FakeGitHubAuth(), blocked);
    cat.account.store.save({ access_token: "gho_a", generation: 1 });
    fake.pageCalls = 0;
    const inflight = Array.from({ length: 8 }, () => cat.refresh());
    await enteredP; await new Promise((r) => setTimeout(r, 15)); release();
    await Promise.all(inflight);
    expect(cat.projection()).toMatchObject({ outcome: "complete-current", generation: 1 });
    expect(cat.projection().repositories).toHaveLength(2);
    expect(fake.pageCalls).toBe(2);
    noCred(cat.projection());

    let c = await newCat(Object.assign(new FakeGitHubCatalog(), {
      pages: [[{ repository_id: "1", owner: "o", name: "kept" }]] as Repository[][], etag: `"g1"`,
    }));
    await c.refresh();
    c.doer = new CombinedTransport(new FakeGitHubAuth(), Object.assign(new FakeGitHubCatalog(), {
      pages: [[{ repository_id: "9", owner: "o", name: "x" }], [{ repository_id: "10", owner: "o", name: "y" }]] as Repository[][],
      failAtPage: 2, etag: `"g2"`,
    }));
    c.account.store.save({ access_token: "gho_a", generation: 1 });
    let p = await c.refresh();
    expect(p).toMatchObject({ outcome: "source-failed-retained", stale: true, generation: 1 });
    expect(p.repositories[0]?.name).toBe("kept");

    const pages = Array.from({ length: MAX_PAGES + 1 }, (_, i) => [
      { repository_id: String(i + 1), owner: "o", name: "r" },
    ]);
    c = await newCat(Object.assign(new FakeGitHubCatalog(), { pages, etag: `"big"` }));
    c.proj = { outcome: "complete-current", generation: 3, stale: false,
      repositories: [{ repository_id: "old", owner: "o", name: "old" }] };
    c.generation = 3;
    p = await c.refresh();
    expect(p).toMatchObject({ outcome: "catalog_too_large-retained", generation: 3 });
    expect(p.repositories[0]?.name).toBe("old");

    const clock = new ManualClock(EPOCH);
    const df = new FakeGitHubCatalog();
    df.pages = [[{ repository_id: "1", owner: "o", name: "a" }], [{ repository_id: "2", owner: "o", name: "b" }]];
    df.etag = `"g"`;
    const adv: HttpDoer = { async do(req) { clock.advance(REFRESH_DEADLINE_MS); return df.do(req); } };
    const httpC = new CombinedTransport(new FakeGitHubAuth(), adv);
    const acct = new Account(httpC, clock, new MemoryCredentialStore(), fixedOAuthApp());
    acct.store.save({ access_token: "gho_a", generation: 1 });
    const cache = new MemoryCacheStore();
    cache.save({ outcome: "complete-current", generation: 1, stale: false,
      repositories: [{ repository_id: "k", owner: "o", name: "kept" }] });
    p = await new Catalog(httpC, clock, cache, acct).refresh();
    expect(p.outcome).toBe("refresh_deadline-retained");
    expect(p.repositories[0]?.name).toBe("kept");

    c = await newCat(Object.assign(new FakeGitHubCatalog(), {
      pages: [[{ repository_id: "1", owner: "o", name: "a" }]] as Repository[][], etag: `"g"`,
    }));
    await c.refresh();
    c.doer = new CombinedTransport(new FakeGitHubAuth(), Object.assign(new FakeGitHubCatalog(), { authStatus: 500 }));
    c.account.store.save({ access_token: "gho_a", generation: 1 });
    p = await c.refresh();
    expect(p).toMatchObject({ outcome: "source-failed-retained", stale: true });
    expect(p.repositories).toHaveLength(1);
  });
});

describe("shared fixture parity", () => {
  it("Web/Workspace credential-free projection deep-equals shared fixture", () => {
    const production = JSON.parse(fs.readFileSync(path.join(CONFIG, "github.production.json"), "utf8"));
    const oauth = JSON.parse(fs.readFileSync(path.join(CONFIG, "fixtures/github-oauth-valid.json"), "utf8"));
    expect(oauth).toEqual(production);
    expect(oauth).toMatchObject({ client_id: fixedOAuthApp().client_id, scopes: fixedOAuthApp().scopes });
    expect(production.client_id).not.toBe("Ov23liAgentGridDeviceFlow");
    noCred(oauth);
    const fixture = JSON.parse(fs.readFileSync(path.join(CONFIG, "fixtures/github-projection-credential-free.json"), "utf8"));
    const workspace = {
      account: { state: "authenticated", login: "example-user" },
      catalog: {
        outcome: "complete-current", generation: 1, stale: false,
        repositories: [{ repository_id: "1", owner: "example-user", name: "demo" }],
      },
    };
    expect(workspace).toEqual(fixture);
    noCred(workspace);
    expect(fs.readFileSync(path.join(CONFIG, "github.schema.json"), "utf8")).not.toContain('"client_secret"');
  });
});

describe("Windows GitHub OAuth generation input contract", () => {
  it("stages every client tree required by the OAuth generator", () => {
    const generator = fs.readFileSync(path.join(CONFIG, "generate-github-oauth.mjs"), "utf8");
    const buildScript = fs.readFileSync(
      path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../scripts/build-windows.ps1"),
      "utf8",
    );
    const generatedClientTrees = Array.from(
      generator.matchAll(/resolve\(root, "clients\/([^/"]+)\//g),
      (match) => match[1]!,
    );
    const stagedTrees = new Set(
      buildScript.match(/foreach \(\$name in @\(([^)]+)\)\)/)?.[1]
        ?.match(/"([^"]+)"/g)
        ?.map((name) => name.slice(1, -1)) ?? [],
    );

    expect(generatedClientTrees.length).toBeGreaterThan(0);
    expect([...stagedTrees]).toEqual(expect.arrayContaining(["config", ...generatedClientTrees]));
  });
});

describe("FakeVsRealGitHubAuth", () => {
  it("typed unavailable without authority; shape parity when configured", async () => {
    const auth = resolveGitHubAuthAuthority();
    if (auth.outcome !== "configured") {
      expect(auth).toEqual({ outcome: "unavailable", reason: "AG_E2E_GITHUB_OAUTH_CLIENT_ID unset" });
      return;
    }
    const rp = await new Account(fetchDoer(), new ManualClock(new Date()), new MemoryCredentialStore(), {
      schema_version: 1, client_id: auth.client_id!, scopes: ["read:user", "repo"],
    }).startDeviceFlow();
    expect(rp.state).toBe("authorizing");
    expect(rp.user_code).toBeTruthy();
    const f = new FakeGitHubAuth();
    f.deviceCodes = [{ device_code: "d", user_code: rp.user_code, verification_uri: rp.verification_uri, expires_in: 900 }];
    expect((await newAcct(f).startDeviceFlow()).state).toBe(rp.state);
    noCred(rp);
  });
});

describe("FakeVsRealGitHubCatalog", () => {
  it("typed unavailable without authority; shape parity when configured", async () => {
    const auth = resolveGitHubCatalogAuthority();
    if (auth.outcome !== "configured") {
      expect(auth).toEqual({ outcome: "unavailable", reason: "AG_E2E_GITHUB_CATALOG_TOKEN unset" });
      return;
    }
    const clock = new ManualClock(new Date());
    const real = fetchDoer();
    const store = new MemoryCredentialStore();
    store.save({ access_token: auth.token!, generation: 1 });
    const rp = await new Catalog(real, clock, new MemoryCacheStore(),
      new Account(real, clock, store, fixedOAuthApp())).refresh();
    expect(rp.outcome).toBe("complete-current");
    const page = rp.repositories.slice(0, 3);
    const catFake = new FakeGitHubCatalog(); catFake.pages = [page]; catFake.etag = `"r"`;
    const httpC = new CombinedTransport(new FakeGitHubAuth(), catFake);
    const fsStore = new MemoryCredentialStore(); fsStore.save({ access_token: "gho_f", generation: 1 });
    const fp = await new Catalog(httpC, clock, new MemoryCacheStore(),
      new Account(httpC, clock, fsStore, fixedOAuthApp())).refresh();
    expect(fp.repositories).toHaveLength(page.length);
    noCred(rp); noCred(fp);
  });
});

describe("protected file persistence", () => {
  const tmp: string[] = [];
  afterEach(() => { for (const f of tmp) try { fs.unlinkSync(f); } catch { /* */ } tmp.length = 0; });
  it("atomically persists credential and catalog without SPA leak", async () => {
    const credPath = path.join(os.tmpdir(), `ag-gh-c-${Math.random().toString(16).slice(2)}.json`);
    const catPath = path.join(os.tmpdir(), `ag-gh-k-${Math.random().toString(16).slice(2)}.json`);
    tmp.push(credPath, catPath);
    const cred = new FileCredentialStore(credPath);
    cred.save({ access_token: "gho_secret_token", refresh_token: "ghr_secret", generation: 1, login: "alice" });
    expect(cred.load().record.access_token).toBe("gho_secret_token");
    expect(fs.statSync(credPath).mode & 0o777).toBe(0o600);
    const cache = new FileCacheStore(catPath);
    const proj: CatalogProjection = {
      outcome: "complete-current", generation: 2, stale: false,
      repositories: [{ repository_id: "1", owner: "o", name: "r" }],
    };
    cache.save(proj);
    expect(cache.load().projection).toEqual(proj);
    const auth = new FakeGitHubAuth();
    auth.userLogin = "alice";
    auth.tokenPolls = [{ access_token: "gho_secret_token", refresh_token: "ghr_secret" }];
    const a = new Account(auth, new ManualClock(EPOCH), cred, fixedOAuthApp());
    await a.startDeviceFlow();
    noCred(await pollAfterInterval(a));
  });
});
