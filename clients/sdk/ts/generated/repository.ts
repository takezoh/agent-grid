/**
 * Fully-qualified repository session identities and preparation snapshots.
 * Generated from protocol/repository.schema.json (flag-day exact-version surface).
 */
export interface GitHubRepositoryIdentity {
  repository_id: string;
  owner: string;
  name: string;
}

export interface ProjectTarget {
  server_id: string;
  project: string;
}

export type SessionTarget =
  | { source: "project"; project: ProjectTarget }
  | { source: "repository"; repository: GitHubRepositoryIdentity };

/** Runtime oneOf guard matching protocol/repository.schema.json SessionTarget. */
export function isSessionTarget(value: unknown): value is SessionTarget {
  if (!value || typeof value !== "object") return false;
  const v = value as Record<string, unknown>;
  if (v.source === "project") {
    const p = v.project as Record<string, unknown> | undefined;
    return (
      !!p &&
      typeof p.server_id === "string" &&
      p.server_id.length > 0 &&
      typeof p.project === "string" &&
      p.project.length > 0 &&
      v.repository === undefined
    );
  }
  if (v.source === "repository") {
    const r = v.repository as Record<string, unknown> | undefined;
    return (
      !!r &&
      typeof r.repository_id === "string" &&
      r.repository_id.length > 0 &&
      typeof r.owner === "string" &&
      r.owner.length > 0 &&
      typeof r.name === "string" &&
      r.name.length > 0 &&
      v.project === undefined
    );
  }
  return false;
}

export type PreparationStage =
  | "confirmed"
  | "clone-in-progress"
  | "clone-ready"
  | "devcontainer-in-progress"
  | "failed"
  | "cancelled"
  | "interrupted"
  | "internal_corruption"
  | "available"
  | "abandoned";

export interface RepositoryPreparation {
  preparation_id: string;
  intent_id: string;
  server_id: string;
  repository: GitHubRepositoryIdentity;
  stage: PreparationStage;
  revision: number;
  failure_code?: string;
  retry_origin?: "" | "clone" | "devcontainer";
  environment_disposition?: "none" | "ensuring" | "ready" | "reused" | "failed";
  session_id?: string;
  clone_path?: string;
  environment_id?: string;
}
