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
