export type RepositoryCreatePayload = {
  repository_id: string;
  owner: string;
  name: string;
  intent_id?: string;
  version: 2;
  profile_id: string;
  profile_revision: string;
  parameters?: Record<string, unknown>;
  cols?: number;
  rows?: number;
};

export function parseRepositoryCreatePayload(payload: unknown): RepositoryCreatePayload {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    throw new Error("repository identity and versioned command profile are required");
  }
  const value = payload as Record<string, unknown>;
  if (
    typeof value.repository_id !== "string" ||
    !value.repository_id ||
    typeof value.owner !== "string" ||
    !value.owner ||
    typeof value.name !== "string" ||
    !value.name ||
    value.version !== 2 ||
    typeof value.profile_id !== "string" ||
    !value.profile_id ||
    typeof value.profile_revision !== "string" ||
    !value.profile_revision
  ) {
    throw new Error("repository identity and versioned command profile are required");
  }
  return value as RepositoryCreatePayload;
}
