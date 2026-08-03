/** Workspace GitHub device-flow account + bounded catalog. Mirrors web-host githubhost. */
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import { atomicWriteJson } from "./file-state-store.js";
import { FIXED_CLIENT_ID, SCOPE_READ_USER, SCOPE_REPO } from "./github-oauth.generated.js";

export { FIXED_CLIENT_ID, SCOPE_READ_USER, SCOPE_REPO };
export const REFRESH_DEADLINE_MS = 120_000;
export const MAX_PAGES = 100;
export const MAX_REPOSITORIES = 10_000;
export const MAX_RESPONSE_BYTES = 32 << 20;
export const MAX_CACHE_BYTES = 16 << 20;
const DEVICE_URL = "https://github.com/login/device/code";
const TOKEN_URL = "https://github.com/login/oauth/access_token";
const USER_URL = "https://api.github.com/user";
const REPOS_URL = "https://api.github.com/user/repos?per_page=100&sort=full_name";

export type AccountState =
  | "unauthenticated" | "authorizing" | "authenticated" | "denied"
  | "expired" | "cancelled" | "refresh_failed" | "host_misconfigured";
export type CatalogOutcome =
  | "empty" | "complete-current" | "refreshing-retained"
  | "authentication-required-retained" | "source-failed-retained"
  | "catalog_too_large-retained" | "refresh_deadline-retained";
export const FailCatalogTooLarge = "catalog_too_large";
export const FailRefreshDeadline = "refresh_deadline";
export const FailSourceFailed = "source_failed";
export const FailAuthRequired = "authentication_required";
export const FailTransport = "transport_failure";
export const FailRefreshToken = "refresh_failure";
export const FailHostMisconfigured = "host_misconfigured";

export type HttpHeaders = Record<string, string>;
export interface HttpRequest { method: string; url: string; headers?: HttpHeaders; body?: string }
export interface HttpResponse { status: number; headers: HttpHeaders; body: string }
export interface HttpDoer { do(req: HttpRequest): Promise<HttpResponse> }
export interface Clock { now(): Date }
export class ManualClock implements Clock {
  private t: number;
  constructor(d: Date) { this.t = d.getTime(); }
  now() { return new Date(this.t); }
  advance(ms: number) { this.t += ms; }
}
export interface RandomSource { id(prefix?: string): string }
export class SeqRandom implements RandomSource {
  private n = 0;
  id(prefix = "id") { return `${prefix}-${++this.n}`; }
}
export interface OAuthApp { schema_version: number; client_id: string; scopes: string[] }
export interface Repository { repository_id: string; owner: string; name: string }
export interface AccountProjection {
  state: AccountState; login?: string; user_code?: string;
  verification_uri?: string; failure_code?: string;
}
export interface CatalogProjection {
  outcome: CatalogOutcome; generation: number; stale: boolean;
  failure_code?: string; repositories: Repository[];
}
export interface CredentialRecord {
  access_token: string; refresh_token?: string; token_type?: string;
  scope?: string; login?: string; generation: number;
}
export interface CredentialStore {
  load(): { record: CredentialRecord; ok: boolean };
  save(record: CredentialRecord): void;
}
export interface CacheStore {
  load(): { projection: CatalogProjection; ok: boolean };
  save(projection: CatalogProjection): void;
}
export class MemoryCredentialStore implements CredentialStore {
  private r: CredentialRecord | null = null;
  load() {
    return this.r ? { record: { ...this.r }, ok: true as const }
      : { record: { access_token: "", generation: 0 }, ok: false as const };
  }
  save(r: CredentialRecord) { this.r = { ...r }; }
}
export class MemoryCacheStore implements CacheStore {
  private p: CatalogProjection | null = null;
  load() {
    return this.p ? { projection: cloneCatalog(this.p), ok: true as const }
      : { projection: emptyCat(), ok: false as const };
  }
  save(p: CatalogProjection) { this.p = cloneCatalog(p); }
}

const SENTINELS = ["access_token", "refresh_token", "client_secret", '"authorization"', "bearer ", "gho_", "ghr_", "ghu_", "ghs_"];
/** rejects counterexample: SPA projection containing access/refresh tokens */
export function projectionContainsCredential(v: unknown): boolean {
  try { const s = JSON.stringify(v).toLowerCase(); return SENTINELS.some((x) => s.includes(x)); }
  catch { return true; }
}
export function fixedOAuthApp(): OAuthApp {
  return { schema_version: 1, client_id: FIXED_CLIENT_ID, scopes: [SCOPE_READ_USER, SCOPE_REPO] };
}
export function cloneCatalog(p: CatalogProjection): CatalogProjection {
  return { ...p, repositories: (p.repositories ?? []).map((r) => ({ ...r })) };
}
function emptyCat(): CatalogProjection {
  return { outcome: "empty", generation: 0, stale: false, repositories: [] };
}

export class Account {
  private state: AccountState = "unauthenticated";
  private login = ""; private userCode = ""; private verificationUri = "";
  private deviceCode = ""; private failureCode = "";
  private expiresAt = 0; private credGen = 0;
  private pollIntervalMs = 5_000; private nextPollAt = 0;
  private pollFlight: Promise<void> | null = null;
  constructor(
    readonly doer: HttpDoer, readonly clock: Clock, readonly store: CredentialStore,
    readonly app: OAuthApp, readonly random: RandomSource = new SeqRandom(),
  ) {}
  projection() { return this.snap(); }
  hostCredential() {
    const { record, ok } = this.store.load();
    return !ok || !record.access_token ? { token: "", ok: false as const }
      : { token: record.access_token, ok: true as const };
  }
  async startDeviceFlow(): Promise<AccountProjection> {
    if (!this.app.client_id) {
      this.set("host_misconfigured", FailHostMisconfigured);
      throw new Error("github-host: missing client_id");
    }
    try {
      const out = await this.post(DEVICE_URL, { client_id: this.app.client_id, scope: this.app.scopes.join(" ") });
      this.state = "authorizing";
      this.userCode = String(out.user_code ?? "");
      this.deviceCode = String(out.device_code ?? "");
      this.failureCode = "";
      this.verificationUri = String(out.verification_uri || out.verification_uri_complete || "");
      const now = this.clock.now().getTime();
      this.pollIntervalMs = Math.max(1, Number(out.interval) || 5) * 1000;
      this.nextPollAt = now + this.pollIntervalMs;
      this.expiresAt = now + (Number(out.expires_in) || 900) * 1000;
      return this.snap();
    } catch (e) {
      this.set(
        "host_misconfigured",
        isOAuthAppConfigurationError(e)
          ? FailHostMisconfigured
          : FailTransport,
      );
      throw e instanceof Error ? e : new Error(String(e));
    }
  }
  cancelDeviceFlow(): AccountProjection {
    if (this.state === "authorizing") {
      this.state = "cancelled"; this.failureCode = "cancelled";
      this.userCode = ""; this.verificationUri = ""; this.deviceCode = "";
      this.nextPollAt = 0;
    }
    return this.snap();
  }
  async pollDeviceFlow(): Promise<AccountProjection> {
    if (this.state !== "authorizing") return this.snap();
    const now = this.clock.now().getTime();
    if (now >= this.expiresAt) return this.term("expired", "expired");
    if (now < this.nextPollAt) return this.snap();
    if (this.pollFlight) { await this.pollFlight; return this.projection(); }
    let resolve!: () => void;
    this.pollFlight = new Promise<void>((r) => { resolve = r; });
    try {
      this.nextPollAt = now + this.pollIntervalMs;
      return await this.applyToken(await this.post(TOKEN_URL, {
        client_id: this.app.client_id, device_code: this.deviceCode,
        grant_type: "urn:ietf:params:oauth:grant-type:device_code",
      }));
    } catch { return this.set("host_misconfigured", FailTransport); }
    finally { resolve(); this.pollFlight = null; }
  }
  async refreshAccessToken(): Promise<AccountProjection> {
    const { record, ok } = this.store.load();
    if (!ok || !record.refresh_token) {
      this.set("refresh_failed", FailRefreshToken);
      throw new Error("github-host: refresh unavailable");
    }
    let out: Record<string, unknown>;
    try {
      out = await this.post(TOKEN_URL, {
        client_id: this.app.client_id, grant_type: "refresh_token", refresh_token: record.refresh_token,
      });
    } catch { out = { error: "bad" }; }
    if (out.error || !out.access_token) {
      this.set("refresh_failed", FailRefreshToken);
      throw new Error("github-host: refresh failed");
    }
    const next = { ...record, access_token: String(out.access_token),
      refresh_token: out.refresh_token ? String(out.refresh_token) : record.refresh_token };
    next.generation = ++this.credGen;
    this.store.save(next);
    return this.set("authenticated", "");
  }
  private async applyToken(out: Record<string, unknown>): Promise<AccountProjection> {
    if (out.error) {
      const e = String(out.error);
      if (e === "authorization_pending") return this.projection();
      if (e === "slow_down") {
        this.pollIntervalMs += 5_000;
        this.nextPollAt = this.clock.now().getTime() + this.pollIntervalMs;
        return this.projection();
      }
      if (e === "access_denied") return this.term("denied", "denied");
      if (e === "expired_token") return this.term("expired", "expired");
      return this.term("host_misconfigured", FailTransport);
    }
    if (!out.access_token) { this.set("host_misconfigured", FailTransport); throw new Error("empty token"); }
    let login = "";
    try {
      const res = await this.doer.do({
        method: "GET", url: USER_URL,
        headers: { Authorization: `Bearer ${String(out.access_token)}`, Accept: "application/vnd.github+json" },
      });
      if (res.status >= 200 && res.status < 300) login = (JSON.parse(res.body) as { login?: string }).login ?? "";
    } catch { /* optional */ }
    this.store.save({
      access_token: String(out.access_token),
      refresh_token: out.refresh_token ? String(out.refresh_token) : undefined,
      token_type: out.token_type ? String(out.token_type) : undefined,
      scope: out.scope ? String(out.scope) : undefined,
      login, generation: ++this.credGen,
    });
    this.state = "authenticated"; this.login = login; this.failureCode = "";
    this.userCode = ""; this.verificationUri = ""; this.deviceCode = "";
    this.nextPollAt = 0;
    return this.snap();
  }
  private set(st: AccountState, code: string) { this.state = st; this.failureCode = code; return this.snap(); }
  private term(st: AccountState, code: string) {
    this.state = st; this.failureCode = code;
    this.userCode = ""; this.verificationUri = ""; this.deviceCode = "";
    this.nextPollAt = 0;
    return this.snap();
  }
  private snap(): AccountProjection {
    const p: AccountProjection = { state: this.state };
    if (this.login) p.login = this.login;
    if (this.userCode) p.user_code = this.userCode;
    if (this.verificationUri) p.verification_uri = this.verificationUri;
    if (this.failureCode) p.failure_code = this.failureCode;
    return p;
  }
  private async post(url: string, form: Record<string, string>) {
    const res = await this.doer.do({
      method: "POST", url,
      headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
      body: new URLSearchParams(form).toString(),
    });
    if (res.status < 200 || res.status >= 300) {
      let oauthError = "";
      try {
        const parsed = JSON.parse(res.body) as Record<string, unknown>;
        if (typeof parsed.error === "string") oauthError = parsed.error;
      } catch {
        // The status remains authoritative when GitHub returns a non-JSON body.
      }
      throw new HttpStatusError(res.status, oauthError);
    }
    return JSON.parse(res.body) as Record<string, unknown>;
  }
}

class HttpStatusError extends Error {
  constructor(readonly status: number, readonly oauthError: string) {
    super(`status ${status}`);
    this.name = "HttpStatusError";
  }
}

function isOAuthAppConfigurationError(error: unknown): boolean {
  return error instanceof HttpStatusError && (
    error.status === 404 ||
    error.oauthError === "device_flow_disabled" ||
    error.oauthError === "incorrect_client_credentials"
  );
}

const errDeadline = new Error("refresh_deadline");
const errTooLarge = new Error("catalog_too_large");
const errAuth = new Error("authentication_required");

export class Catalog {
  proj: CatalogProjection = emptyCat();
  generation = 0;
  private flight: { done: Promise<void>; resolve: () => void; proj: CatalogProjection; err: Error | null } | null = null;
  reposUrl = REPOS_URL;
  doer: HttpDoer;
  constructor(
    doer: HttpDoer, readonly clock: Clock, readonly cache: CacheStore,
    readonly account: Account, readonly random: RandomSource = new SeqRandom(),
  ) {
    this.doer = doer;
    const l = cache.load();
    if (l.ok) { this.proj = cloneCatalog(l.projection); this.generation = l.projection.generation; }
  }
  projection() { return cloneCatalog(this.proj); }
  async refresh(): Promise<CatalogProjection> {
    if (this.flight) {
      const f = this.flight; await f.done;
      if (f.err) throw f.err;
      return cloneCatalog(f.proj);
    }
    let resolve!: () => void;
    const done = new Promise<void>((r) => { resolve = r; });
    const flight = { done, resolve, proj: cloneCatalog(this.proj), err: null as Error | null };
    this.flight = flight;
    if (this.proj.repositories.length > 0 || this.proj.generation > 0) {
      this.proj.outcome = "refreshing-retained"; this.proj.stale = true;
    }
    try {
      flight.proj = await this.once();
      return cloneCatalog(flight.proj);
    } catch (e) { flight.err = e as Error; throw e; }
    finally { this.flight = null; resolve(); }
  }
  private async once(): Promise<CatalogProjection> {
    const cred = this.account.hostCredential();
    if (!cred.ok) return this.fail("authentication-required-retained", FailAuthRequired);
    const deadline = this.clock.now().getTime() + REFRESH_DEADLINE_MS;
    try {
      const g = await this.gen(cred.token, deadline);
      if ((await this.sig1(cred.token)) !== g.first) return this.fail("source-failed-retained", FailSourceFailed);
      if (g.pages > MAX_PAGES || g.repos.length > MAX_REPOSITORIES || g.bytes > MAX_RESPONSE_BYTES
        || JSON.stringify(g.repos).length > MAX_CACHE_BYTES) {
        return this.fail("catalog_too_large-retained", FailCatalogTooLarge);
      }
      this.generation += 1;
      const next: CatalogProjection = {
        outcome: "complete-current", generation: this.generation, stale: false, repositories: g.repos,
      };
      this.proj = next; this.cache.save(next);
      return cloneCatalog(next);
    } catch (e) {
      if (e === errDeadline) return this.fail("refresh_deadline-retained", FailRefreshDeadline);
      if (e === errTooLarge) return this.fail("catalog_too_large-retained", FailCatalogTooLarge);
      if (e === errAuth) return this.fail("authentication-required-retained", FailAuthRequired);
      return this.fail("source-failed-retained", FailSourceFailed);
    }
  }
  private fail(outcome: CatalogOutcome, code: string): CatalogProjection {
    this.proj.outcome = outcome; this.proj.failure_code = code;
    this.proj.stale = this.proj.repositories.length > 0 || this.proj.generation > 0;
    return cloneCatalog(this.proj);
  }
  private async gen(token: string, deadline: number) {
    let u: string | null = this.reposUrl;
    const repos: Repository[] = [];
    let bytes = 0, pages = 0, first = "";
    while (u) {
      if (this.clock.now().getTime() >= deadline) throw errDeadline;
      if (pages >= MAX_PAGES) throw errTooLarge;
      const page = await this.page(token, u);
      bytes += page.n; pages += 1;
      if (pages === 1) first = pageSig(page.repos, page.etag);
      repos.push(...page.repos);
      if (repos.length > MAX_REPOSITORIES || bytes > MAX_RESPONSE_BYTES) throw errTooLarge;
      u = page.next;
    }
    return { repos, bytes, pages, first };
  }
  private async sig1(token: string) {
    const p = await this.page(token, this.reposUrl);
    return pageSig(p.repos, p.etag);
  }
  private async page(token: string, pageUrl: string) {
    const res = await this.doer.do({
      method: "GET", url: pageUrl,
      headers: { Authorization: `Bearer ${token}`, Accept: "application/vnd.github+json" },
    });
    if (res.body.length > MAX_RESPONSE_BYTES) throw errTooLarge;
    if (res.status === 401 || res.status === 403) throw errAuth;
    if (res.status !== 200) throw new Error(`status ${res.status}`);
    const items = JSON.parse(res.body) as Array<{ id?: number; name?: string; full_name?: string; owner?: { login?: string } }>;
    const repos: Repository[] = [];
    for (const it of items) {
      let o = it.owner?.login ?? "", n = it.name ?? "";
      if ((!o || !n) && it.full_name) {
        const p = it.full_name.split("/", 2);
        if (p.length === 2) { o = p[0]!; n = p[1]!; }
      }
      if (o && n) repos.push({ repository_id: String(it.id ?? repos.length + 1), owner: o, name: n });
    }
    return { repos, next: nextLink(hdr(res.headers, "link")), n: res.body.length, etag: hdr(res.headers, "etag") };
  }
}
function pageSig(repos: Repository[], etag: string) {
  return etag ? `etag:${etag}` : repos.map((r) => r.repository_id).join("|");
}
function nextLink(link: string): string | null {
  for (const part of link.split(",")) {
    const p = part.trim();
    if (!p.includes('rel="next"')) continue;
    const s = p.indexOf("<"), e = p.indexOf(">");
    if (s >= 0 && e > s) return p.slice(s + 1, e);
  }
  return null;
}
function hdr(h: HttpHeaders, name: string) {
  const w = name.toLowerCase();
  for (const [k, v] of Object.entries(h)) if (k.toLowerCase() === w) return v;
  return "";
}

export type RealAuthorityOutcome = "configured" | "unavailable";
export interface RealAuthAuthority { outcome: RealAuthorityOutcome; client_id?: string; reason?: string }
export interface RealCatalogAuthority { outcome: RealAuthorityOutcome; token?: string; reason?: string }
export function resolveGitHubAuthAuthority(env: NodeJS.ProcessEnv = process.env): RealAuthAuthority {
  const id = (env.AG_E2E_GITHUB_OAUTH_CLIENT_ID ?? "").trim();
  return id ? { outcome: "configured", client_id: id }
    : { outcome: "unavailable", reason: "AG_E2E_GITHUB_OAUTH_CLIENT_ID unset" };
}
export function resolveGitHubCatalogAuthority(env: NodeJS.ProcessEnv = process.env): RealCatalogAuthority {
  const token = (env.AG_E2E_GITHUB_CATALOG_TOKEN ?? "").trim();
  return token ? { outcome: "configured", token }
    : { outcome: "unavailable", reason: "AG_E2E_GITHUB_CATALOG_TOKEN unset" };
}

function stateDir() {
  if (process.platform === "win32") {
    return path.join(process.env.APPDATA ?? path.join(os.homedir(), "AppData", "Roaming"), "agent-grid");
  }
  return path.join(process.env.XDG_STATE_HOME ?? path.join(os.homedir(), ".local", "state"), "agent-grid");
}
export const defaultGitHubCredentialPath = () => path.join(stateDir(), "github-credential.json");
export const defaultGitHubCatalogCachePath = () => path.join(stateDir(), "github-catalog.json");

export class FileCredentialStore implements CredentialStore {
  constructor(private readonly filePath = defaultGitHubCredentialPath()) {}
  get path() { return this.filePath; }
  load() {
    try {
      const record = JSON.parse(fs.readFileSync(this.filePath, "utf8")) as CredentialRecord;
      if (!record?.access_token) return { record: { access_token: "", generation: 0 }, ok: false as const };
      return { record, ok: true as const };
    } catch { return { record: { access_token: "", generation: 0 }, ok: false as const }; }
  }
  save(record: CredentialRecord) { atomicWriteJson(this.filePath, record, 0o600); }
}
export class FileCacheStore implements CacheStore {
  constructor(private readonly filePath = defaultGitHubCatalogCachePath()) {}
  get path() { return this.filePath; }
  load() {
    try {
      const projection = JSON.parse(fs.readFileSync(this.filePath, "utf8")) as CatalogProjection;
      if (!projection?.repositories) return { projection: emptyCat(), ok: false as const };
      return { projection: cloneCatalog(projection), ok: true as const };
    } catch { return { projection: emptyCat(), ok: false as const }; }
  }
  save(p: CatalogProjection) { atomicWriteJson(this.filePath, cloneCatalog(p), 0o600); }
}
