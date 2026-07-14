// Read-only workspace HTTP client for the agent workspace viewer.
// Mirrors src/server/web/workspace.go response shapes.

import { readBearerTokenFromHash } from "../auth";

export type WorkspaceRootHandle = {
  session_id: string;
  frame_generation: number;
  resolved_root_path: string;
};

export type WorkspaceTreeEntry = {
  name: string;
  path: string;
  type: "file" | "dir";
};

export type WorkspaceTreeResponse = {
  outcome: "ok" | "root_unreachable" | "refresh_failed" | string;
  path?: string;
  entries?: WorkspaceTreeEntry[];
};

export type WorkspaceFileResponse = {
  path: string;
  size: number;
  is_binary: boolean;
  content_type?: string;
  content?: string;
};

export type WorkspaceDiffOutcome =
  | "ok"
  | "not_a_repo"
  | "git_metadata_corrupted"
  | "git_binary_missing"
  | string;

export type WorkspaceDiffResponse = {
  outcome: WorkspaceDiffOutcome;
  diff?: string;
};

export type WorkspacePinnedHandle = {
  frameGeneration: number;
  resolvedRootPath: string;
};

export type WorkspaceHandleStaleBody = {
  error: "handle_stale";
  session_id: string;
  frame_generation: number;
  pinned_frame_generation: number;
  resolved_root_path?: string;
};

export class WorkspaceApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly body: unknown;

  constructor(status: number, code: string, message: string, body?: unknown) {
    super(message);
    this.name = "WorkspaceApiError";
    this.status = status;
    this.code = code;
    this.body = body;
  }

  static isHandleStale(err: unknown): err is WorkspaceApiError {
    return err instanceof WorkspaceApiError && err.code === "handle_stale";
  }
}

export interface WorkspaceApi {
  getRootHandle(sessionId: string): Promise<WorkspaceRootHandle>;
  getTree(
    sessionId: string,
    path: string,
    pinned: WorkspacePinnedHandle,
  ): Promise<WorkspaceTreeResponse>;
  getFile(
    sessionId: string,
    path: string,
    pinned: WorkspacePinnedHandle,
  ): Promise<WorkspaceFileResponse>;
  getDiff(
    sessionId: string,
    path: string,
    pinned: WorkspacePinnedHandle,
  ): Promise<WorkspaceDiffResponse>;
}

function authHeaders(): Record<string, string> {
  const token = readBearerTokenFromHash();
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
}

function pinnedQuery(pinned: WorkspacePinnedHandle): string {
  const params = new URLSearchParams();
  params.set("handle", String(pinned.frameGeneration));
  params.set("root", pinned.resolvedRootPath);
  return params.toString();
}

export function makeWorkspaceApi(fetchFn: typeof fetch = fetch): WorkspaceApi {
  const get = async <T>(url: string): Promise<T> => {
    const resp = await fetchFn(url, { headers: authHeaders() });
    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      if (resp.status === 409) {
        try {
          const parsed = JSON.parse(text) as { error?: string };
          if (parsed.error === "handle_stale") {
            throw new WorkspaceApiError(409, "handle_stale", "workspace root handle stale", parsed);
          }
        } catch (e) {
          if (WorkspaceApiError.isHandleStale(e)) throw e;
        }
      }
      throw new WorkspaceApiError(
        resp.status,
        "http_error",
        `workspace ${resp.status}: ${text || resp.statusText}`,
        text,
      );
    }
    return (await resp.json()) as T;
  };

  return {
    async getRootHandle(sessionId) {
      return get<WorkspaceRootHandle>(
        `/api/sessions/${encodeURIComponent(sessionId)}/workspace/root-handle`,
      );
    },
    async getTree(sessionId, path, pinned) {
      const params = new URLSearchParams(pinnedQuery(pinned));
      if (path) params.set("path", path);
      return get<WorkspaceTreeResponse>(
        `/api/sessions/${encodeURIComponent(sessionId)}/workspace/tree?${params}`,
      );
    },
    async getFile(sessionId, path, pinned) {
      const params = new URLSearchParams(pinnedQuery(pinned));
      params.set("path", path);
      return get<WorkspaceFileResponse>(
        `/api/sessions/${encodeURIComponent(sessionId)}/workspace/file?${params}`,
      );
    },
    async getDiff(sessionId, path, pinned) {
      const params = new URLSearchParams(pinnedQuery(pinned));
      if (path) params.set("path", path);
      return get<WorkspaceDiffResponse>(
        `/api/sessions/${encodeURIComponent(sessionId)}/workspace/diff?${params}`,
      );
    },
  };
}
