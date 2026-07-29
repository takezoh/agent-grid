import type { ServerConfig } from "./desktop-config.js";
import {
  projectionContainsCredential,
  type AccountProjection,
  type CatalogProjection,
  type CredentialStore,
  type Repository,
} from "./github-host.js";
import { PROTOCOL_VERSION, PROTOCOL_VERSION_HEADER } from "./protocol-version.js";

export const LEASE_FAIL_AUTH_MODE = "authenticated_transport_required";
export const LEASE_FAIL_PROTOCOL = "incompatible_protocol";
export const LEASE_FAIL_NO_CRED = "authentication_required";
export const LEASE_FAIL_TRANSPORT = "transport_failure";
export const LEASE_FAIL_INVALID = "invalid_identity";

export interface HostSource { id: string; available: boolean; status?: string }
export interface HostProjectTarget { server_id: string; project: string }
export interface HostRepositoryIdentity { repository_id: string; owner: string; name: string }
export interface HostTarget { source: "project" | "repository"; project?: HostProjectTarget; repository?: HostRepositoryIdentity }
export interface HostPreparation { preparation_id: string; intent_id: string; server_id: string; repository: HostRepositoryIdentity; stage: string; revision: number; failure_code?: string; retry_origin?: string; environment_disposition?: string; session_id?: string; clone_path?: string; environment_id?: string }
export interface HostRepositoryProjection { sources: HostSource[]; account: AccountProjection; catalog: CatalogProjection; targets: HostTarget[]; preparations: HostPreparation[] }

export function buildHostRepositoryProjection(
  projects: HostProjectTarget[], account: AccountProjection,
  catalog: CatalogProjection, preparations: HostPreparation[] = [],
): HostRepositoryProjection {
  const repos = catalog.repositories ?? [];
  const githubAvailable =
    catalog.outcome === "complete-current" || catalog.outcome === "refreshing-retained" ||
    (catalog.stale === true && repos.length > 0) ||
    account.state === "authenticated" || account.state === "authorizing";
  let githubStatus: string = catalog.outcome || account.state;
  if (!githubStatus || githubStatus === "empty") githubStatus = account.state;
  const targets: HostTarget[] = [];
  for (const p of projects) {
    if (p.server_id && p.project) targets.push({ source: "project", project: { ...p } });
  }
  for (const r of repos) {
    if (r.repository_id && r.owner && r.name) {
      targets.push({ source: "repository", repository: { ...r } });
    }
  }
  return {
    sources: [
      { id: "projects", available: true },
      { id: "github", available: githubAvailable, status: githubStatus },
    ],
    account: { ...account },
    catalog: { ...catalog, repositories: repos.map((r) => ({ ...r })) },
    targets,
    preparations: preparations.map((p) => ({ ...p, repository: { ...p.repository } })),
  };
}

export function preflightRepositoryLease(authMode: string, observedProtocol: string, serverUrl: string) {
  if (!authMode || authMode === "none") return { code: LEASE_FAIL_AUTH_MODE };
  if (observedProtocol !== PROTOCOL_VERSION) return { code: LEASE_FAIL_PROTOCOL };
  if (!isSecureServerUrl(serverUrl)) return { code: LEASE_FAIL_AUTH_MODE };
  return null;
}

export function isSecureServerUrl(raw: string): boolean {
  try {
    const u = new URL(raw);
    if (u.protocol === "https:") return true;
    const h = u.hostname;
    return h === "localhost" || h === "127.0.0.1" || h === "::1" || h === "[::1]" || /^127\.\d+\.\d+\.\d+$/.test(h);
  } catch { return false; }
}

export interface LeaseRequest { host_id?: string; repository_id: string; owner: string; name: string; server_id?: string }
export interface LeaseIssue { lease_id: string; expires_at: string }
export interface LeaseResult { issue?: LeaseIssue; code?: string }
export interface CredentialAuthority { hostCredential(): { token: string; ok: boolean }; credentialGeneration(): { generation: string; ok: boolean } }
export function authorityFromStore(store: CredentialStore): CredentialAuthority {
  return {
    hostCredential() {
      const { record, ok } = store.load();
      return !ok || !record.access_token ? { token: "", ok: false } : { token: record.access_token, ok: true };
    },
    credentialGeneration() {
      const { record, ok } = store.load();
      return !ok || !record.access_token
        ? { generation: "", ok: false }
        : { generation: String(record.generation ?? 0), ok: true };
    },
  };
}

export async function requestRepositoryLease(opts: {
  server: ServerConfig; observedProtocol: string; request: LeaseRequest;
  auth: CredentialAuthority; hostId?: string; fetchFn?: typeof fetch;
}): Promise<LeaseResult> {
  const fail = preflightRepositoryLease(opts.server.auth.mode, opts.observedProtocol, opts.server.url);
  if (fail) return { code: fail.code };
  const { repository_id, owner, name } = opts.request;
  if (!repository_id || !owner || !name) return { code: LEASE_FAIL_INVALID };
  const cred = opts.auth.hostCredential();
  if (!cred.ok || !cred.token) return { code: LEASE_FAIL_NO_CRED };
  const gen = opts.auth.credentialGeneration();
  const body = JSON.stringify({
    host_id: opts.request.host_id || opts.hostId || "workspace-host",
    repository_id, owner, name, token: cred.token,
    token_generation: gen.ok ? gen.generation : "0",
  });
  const headers: Record<string, string> = {
    "Content-Type": "application/json", [PROTOCOL_VERSION_HEADER]: PROTOCOL_VERSION,
  };
  if (opts.server.auth.mode === "bearer" && "token" in opts.server.auth) {
    headers.Authorization = `Bearer ${opts.server.auth.token}`;
  }
  try {
    const res = await (opts.fetchFn ?? fetch)(
      `${opts.server.url.replace(/\/+$/, "")}/api/repository/leases`,
      { method: "POST", headers, body, redirect: "manual" },
    );
    const text = await res.text();
    if (!res.ok) {
      try { return { code: (JSON.parse(text) as { code?: string }).code || LEASE_FAIL_TRANSPORT }; }
      catch { return { code: LEASE_FAIL_TRANSPORT }; }
    }
    const issue = JSON.parse(text) as LeaseIssue;
    if (!issue.lease_id) return { code: LEASE_FAIL_TRANSPORT };
    return { issue: { lease_id: issue.lease_id, expires_at: String(issue.expires_at ?? "") } };
  } catch { return { code: LEASE_FAIL_TRANSPORT }; }
}

export type GitHubAction = "status" | "start" | "poll" | "cancel" | "refresh_catalog";
export type GitHubActionOutcome = "success" | "denied" | "cancel" | "failure" | "authorizing" | "unauthenticated";
export interface GitHubActionResult { outcome: GitHubActionOutcome; account: AccountProjection; catalog: CatalogProjection; reason?: string }

export function classifyAccountOutcome(account: AccountProjection): GitHubActionOutcome {
  switch (account.state) {
    case "authenticated": return "success";
    case "authorizing": return "authorizing";
    case "denied": return "denied";
    case "cancelled": return "cancel";
    case "unauthenticated": return "unauthenticated";
    default: return "failure";
  }
}

export function githubActionResult(
  account: AccountProjection, catalog: CatalogProjection, reason?: string,
): GitHubActionResult {
  const result: GitHubActionResult = {
    outcome: classifyAccountOutcome(account),
    account: { ...account },
    catalog: { ...catalog, repositories: (catalog.repositories ?? []).map((r: Repository) => ({ ...r })) },
  };
  if (reason) result.reason = reason;
  if (projectionContainsCredential(result)) throw new Error("github action result must be credential-free");
  return result;
}

export { projectionContainsCredential };
