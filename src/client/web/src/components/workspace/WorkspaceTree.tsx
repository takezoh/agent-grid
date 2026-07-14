import { type ReactNode, useCallback, useEffect, useState } from "react";
import {
  type WorkspacePinnedHandle,
  type WorkspaceTreeEntry,
  makeWorkspaceApi,
} from "../../api/workspace";

export type WorkspaceTreeProps = {
  sessionId: string;
  pinned: WorkspacePinnedHandle | null;
  onSelectFile: (path: string) => void;
  reloadToken?: number;
};

type TreeState = {
  entries: WorkspaceTreeEntry[];
  expanded: ReadonlySet<string>;
  outcome: string;
  path: string;
  loading: boolean;
  error: string | null;
};

const BANNERS: Record<string, string> = {
  root_unreachable: "Workspace root is unreachable. Try refresh.",
  refresh_failed: "Tree refresh failed. Try again.",
};

export function WorkspaceTree({
  sessionId,
  pinned,
  onSelectFile,
  reloadToken = 0,
}: WorkspaceTreeProps): ReactNode {
  const [state, setState] = useState<TreeState>({
    entries: [],
    expanded: new Set(),
    outcome: "ok",
    path: "",
    loading: false,
    error: null,
  });

  const fetchTree = useCallback(
    async (path: string) => {
      if (!pinned) return;
      setState((s) => ({ ...s, loading: true, error: null }));
      try {
        const api = makeWorkspaceApi();
        const resp = await api.getTree(sessionId, path, pinned);
        setState((s) => ({
          ...s,
          loading: false,
          outcome: resp.outcome,
          path: resp.path ?? path,
          entries: resp.entries ?? [],
          error:
            resp.outcome === "ok" ? null : (BANNERS[resp.outcome] ?? `Tree error: ${resp.outcome}`),
        }));
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        setState((s) => ({
          ...s,
          loading: false,
          outcome: "refresh_failed",
          error: msg,
        }));
      }
    },
    [sessionId, pinned],
  );

  // biome-ignore lint/correctness/useExhaustiveDependencies: reloadToken is the parent-driven refetch trigger
  useEffect(() => {
    void fetchTree("");
  }, [fetchTree, reloadToken]);

  const toggleDir = (entry: WorkspaceTreeEntry) => {
    if (entry.type !== "dir") {
      onSelectFile(entry.path);
      return;
    }
    setState((s) => {
      const next = new Set(s.expanded);
      if (next.has(entry.path)) next.delete(entry.path);
      else next.add(entry.path);
      return { ...s, expanded: next };
    });
    void fetchTree(entry.path);
  };

  return (
    <div className="workspace-tree" data-testid="workspace-tree">
      <div className="workspace-tree__header">
        <span className="workspace-tree__root" aria-label="workspace root">
          Workspace
        </span>
        <button
          type="button"
          className="workspace-tree__refresh"
          aria-label="Refresh tree"
          onClick={() => void fetchTree(state.path)}
          disabled={!pinned || state.loading}
        >
          Refresh
        </button>
      </div>
      {state.error && (
        <>
          {/* biome-ignore lint/a11y/useSemanticElements: tree error banner; <output> implies form association */}
          <div className="workspace-tree__banner" role="status">
            {state.error}
          </div>
        </>
      )}
      {state.loading && <div className="workspace-tree__loading">Loading…</div>}
      <ul className="workspace-tree__list" role="tree" aria-label="Workspace files">
        {state.entries.map((entry) => (
          <li
            key={entry.path}
            role="treeitem"
            aria-expanded={entry.type === "dir" ? state.expanded.has(entry.path) : undefined}
          >
            <button
              type="button"
              className="workspace-tree__node"
              aria-label={entry.name}
              onClick={() => toggleDir(entry)}
            >
              {entry.type === "dir" ? "📁" : "📄"} {entry.name}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
