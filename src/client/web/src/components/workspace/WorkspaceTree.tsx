import { type ReactNode, useCallback, useEffect, useState } from "react";
import {
  type WorkspacePinnedHandle,
  type WorkspaceTreeEntry,
  makeWorkspaceApi,
} from "../../api/workspace";
import { Icon } from "../icons/Icon";

export type WorkspaceTreeProps = {
  sessionId: string;
  pinned: WorkspacePinnedHandle | null;
  onSelectFile: (path: string) => void;
  reloadToken?: number;
};

type TreeState = {
  childrenByPath: Record<string, WorkspaceTreeEntry[]>;
  expanded: ReadonlySet<string>;
  outcome: string;
  error: string | null;
  loadingPaths: ReadonlySet<string>;
};

const BANNERS: Record<string, string> = {
  root_unreachable: "Workspace root is unreachable. Try refresh.",
  refresh_failed: "Tree refresh failed. Try again.",
};

const ROOT_PATH = "";

export function WorkspaceTree({
  sessionId,
  pinned,
  onSelectFile,
  reloadToken = 0,
}: WorkspaceTreeProps): ReactNode {
  const [state, setState] = useState<TreeState>({
    childrenByPath: {},
    expanded: new Set(),
    outcome: "ok",
    error: null,
    loadingPaths: new Set(),
  });

  const fetchTree = useCallback(
    async (path: string) => {
      if (!pinned) return;
      setState((s) => ({
        ...s,
        loadingPaths: new Set(s.loadingPaths).add(path),
        error: null,
      }));
      try {
        const api = makeWorkspaceApi();
        const resp = await api.getTree(sessionId, path, pinned);
        setState((s) => {
          const loadingPaths = new Set(s.loadingPaths);
          loadingPaths.delete(path);
          return {
            ...s,
            loadingPaths,
            outcome: resp.outcome,
            childrenByPath:
              resp.outcome === "ok" && resp.entries
                ? { ...s.childrenByPath, [path]: resp.entries }
                : s.childrenByPath,
            error:
              resp.outcome === "ok"
                ? null
                : (BANNERS[resp.outcome] ?? `Tree error: ${resp.outcome}`),
          };
        });
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        setState((s) => {
          const loadingPaths = new Set(s.loadingPaths);
          loadingPaths.delete(path);
          return {
            ...s,
            loadingPaths,
            outcome: "refresh_failed",
            error: msg,
          };
        });
      }
    },
    [sessionId, pinned],
  );

  // biome-ignore lint/correctness/useExhaustiveDependencies: reloadToken is the parent-driven refetch trigger
  useEffect(() => {
    setState({
      childrenByPath: {},
      expanded: new Set(),
      outcome: "ok",
      error: null,
      loadingPaths: new Set(),
    });
    void fetchTree(ROOT_PATH);
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
    if (!state.childrenByPath[entry.path]) {
      void fetchTree(entry.path);
    }
  };

  const rootEntries = state.childrenByPath[ROOT_PATH] ?? [];
  const isRootLoading = state.loadingPaths.has(ROOT_PATH);

  return (
    <div className="workspace-tree" data-testid="workspace-tree">
      <div className="workspace-tree__header">
        <span className="workspace-tree__root" aria-label="workspace root">
          Files
        </span>
        <button
          type="button"
          className="workspace-tree__refresh"
          aria-label="Refresh tree"
          onClick={() => void fetchTree(ROOT_PATH)}
          disabled={!pinned || isRootLoading}
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
      {isRootLoading && rootEntries.length === 0 && (
        <div className="workspace-tree__loading">Loading…</div>
      )}
      <ul className="workspace-tree__list" role="tree" aria-label="Workspace files">
        {rootEntries.map((entry) => (
          <TreeBranch key={entry.path} entry={entry} depth={0} state={state} onToggle={toggleDir} />
        ))}
      </ul>
    </div>
  );
}

type TreeBranchProps = {
  entry: WorkspaceTreeEntry;
  depth: number;
  state: TreeState;
  onToggle: (entry: WorkspaceTreeEntry) => void;
};

function TreeBranch({ entry, depth, state, onToggle }: TreeBranchProps): ReactNode {
  const isDir = entry.type === "dir";
  const isExpanded = isDir && state.expanded.has(entry.path);
  const children = state.childrenByPath[entry.path] ?? [];
  const isLoading = state.loadingPaths.has(entry.path);

  return (
    <li
      role="treeitem"
      aria-expanded={isDir ? isExpanded : undefined}
      className="workspace-tree__item"
      data-depth={depth}
    >
      <button
        type="button"
        className="workspace-tree__node"
        aria-label={entry.name}
        style={{ paddingLeft: `calc(var(--space-3) + ${depth} * var(--workspace-tree-indent))` }}
        onClick={() => onToggle(entry)}
      >
        {isDir ? (
          <span
            className={
              isExpanded
                ? "workspace-tree__chevron workspace-tree__chevron--expanded"
                : "workspace-tree__chevron"
            }
            aria-hidden="true"
          >
            <Icon name="chevron-right" size={12} />
          </span>
        ) : (
          <span
            className="workspace-tree__chevron workspace-tree__chevron--spacer"
            aria-hidden="true"
          />
        )}
        <span className="workspace-tree__icon" aria-hidden="true">
          <Icon name={isDir ? "folder" : "file"} size={14} />
        </span>
        <span className="workspace-tree__label">{entry.name}</span>
      </button>
      {isDir && isExpanded && (
        <ul className="workspace-tree__group">
          {isLoading && children.length === 0 && (
            <li className="workspace-tree__loading workspace-tree__loading--nested">Loading…</li>
          )}
          {children.map((child) => (
            <TreeBranch
              key={child.path}
              entry={child}
              depth={depth + 1}
              state={state}
              onToggle={onToggle}
            />
          ))}
        </ul>
      )}
    </li>
  );
}
