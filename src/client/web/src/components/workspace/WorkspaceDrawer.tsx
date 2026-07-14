import {
  type MouseEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import {
  WorkspaceApiError,
  type WorkspaceDiffResponse,
  type WorkspaceFileResponse,
  makeWorkspaceApi,
} from "../../api/workspace";
import {
  selectDrawerStale,
  selectStaleAnnounceMessage,
  useWorkspaceActivityStore,
} from "../../store/workspaceActivity";
import { DiffViewer } from "./DiffViewer";
import { FileViewer } from "./FileViewer";
import { WorkspaceTree } from "./WorkspaceTree";

export type WorkspaceDrawerProps = {
  sessionId: string | null;
};

export function WorkspaceDrawer({ sessionId }: WorkspaceDrawerProps): ReactNode {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const liveId = useId();

  const open = useWorkspaceActivityStore((s) => s.drawerOpen);
  const tab = useWorkspaceActivityStore((s) => s.drawerTab);
  const target = useWorkspaceActivityStore((s) => s.drawerTarget);
  const pinnedHandle = useWorkspaceActivityStore((s) => s.pinnedHandle);
  const reloadGeneration = useWorkspaceActivityStore((s) => s.reloadGeneration);
  const staleAnnounceSeq = useWorkspaceActivityStore((s) => s.staleAnnounceSeq);

  const closeDrawer = useWorkspaceActivityStore((s) => s.closeDrawer);
  const setDrawerTab = useWorkspaceActivityStore((s) => s.setDrawerTab);
  const setPinnedHandle = useWorkspaceActivityStore((s) => s.setPinnedHandle);
  const reloadDrawerContent = useWorkspaceActivityStore((s) => s.reloadDrawerContent);
  const openDrawerFromRow = useWorkspaceActivityStore((s) => s.openDrawerFromRow);

  const stale = useWorkspaceActivityStore((s) => selectDrawerStale(s, target?.path));
  const announce = useWorkspaceActivityStore((s) => selectStaleAnnounceMessage(s, target?.path));

  const [file, setFile] = useState<WorkspaceFileResponse | null>(null);
  const [diff, setDiff] = useState<WorkspaceDiffResponse | null>(null);
  const [loadingFile, setLoadingFile] = useState(false);
  const [loadingDiff, setLoadingDiff] = useState(false);
  const [fileError, setFileError] = useState<string | null>(null);
  const [diffError, setDiffError] = useState<string | null>(null);
  const [handleStale, setHandleStale] = useState(false);

  // Pin root handle once per drawer open (never re-resolve on background frame push).
  useEffect(() => {
    if (!open) {
      setHandleStale(false);
      return;
    }
    if (!sessionId || pinnedHandle) return;
    let cancelled = false;
    void (async () => {
      try {
        const api = makeWorkspaceApi();
        const handle = await api.getRootHandle(sessionId);
        if (cancelled) return;
        setPinnedHandle({
          frameGeneration: handle.frame_generation,
          resolvedRootPath: handle.resolved_root_path,
        });
      } catch (e) {
        if (cancelled) return;
        setFileError(e instanceof Error ? e.message : String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, sessionId, pinnedHandle, setPinnedHandle]);

  const fetchContent = useCallback(async () => {
    if (!sessionId || !pinnedHandle || !target?.path) return;
    if (target.kind === "delete") {
      setFile({ path: target.path, size: 0, is_binary: false });
      setDiff(null);
      return;
    }
    setLoadingFile(true);
    setFileError(null);
    try {
      const api = makeWorkspaceApi();
      const resp = await api.getFile(sessionId, target.path, pinnedHandle);
      setFile(resp);
    } catch (e) {
      if (WorkspaceApiError.isHandleStale(e)) {
        setHandleStale(true);
        setFileError(null);
      } else {
        setFileError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLoadingFile(false);
    }
  }, [sessionId, pinnedHandle, target]);

  const fetchDiff = useCallback(async () => {
    if (!sessionId || !pinnedHandle || !target?.path) return;
    setLoadingDiff(true);
    setDiffError(null);
    try {
      const api = makeWorkspaceApi();
      const resp = await api.getDiff(sessionId, target.path, pinnedHandle);
      setDiff(resp);
    } catch (e) {
      if (WorkspaceApiError.isHandleStale(e)) {
        setHandleStale(true);
        setDiffError(null);
      } else {
        setDiffError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLoadingDiff(false);
    }
  }, [sessionId, pinnedHandle, target]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: reloadGeneration is the store-driven refetch trigger
  useEffect(() => {
    if (!open || tab === "tree") return;
    void fetchContent();
    if (target?.kind === "edit") void fetchDiff();
  }, [open, tab, fetchContent, fetchDiff, target?.kind, reloadGeneration]);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeDrawer();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, closeDrawer]);

  const onScrimClick = (e: MouseEvent<HTMLDialogElement>) => {
    if (e.target === dialogRef.current) closeDrawer();
  };

  const headerPath = target?.path ?? "Workspace";

  if (!open) return null;

  return (
    <dialog
      ref={dialogRef}
      className="workspace-drawer"
      aria-modal="true"
      aria-label="Workspace viewer"
      onClick={onScrimClick}
      onKeyDown={(e) => {
        if (e.target !== dialogRef.current) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          closeDrawer();
        }
      }}
      data-testid="workspace-drawer"
    >
      <div className="workspace-drawer__panel">
        <header className="workspace-drawer__header">
          <h2 className="workspace-drawer__title">{headerPath}</h2>
          <button
            type="button"
            className="workspace-drawer__close"
            aria-label="Close"
            onClick={closeDrawer}
          >
            ×
          </button>
        </header>

        {handleStale && (
          <>
            {/* biome-ignore lint/a11y/useSemanticElements: handle-stale banner; <output> implies form association */}
            <div className="workspace-drawer__stale workspace-drawer__stale--handle" role="status">
              <span>
                Workspace root changed while this drawer was open. Close and reopen the drawer to
                refresh the pinned root.
              </span>
            </div>
          </>
        )}

        {stale && !handleStale && (
          <>
            {/* biome-ignore lint/a11y/useSemanticElements: stale-file banner; <output> implies form association */}
            <div className="workspace-drawer__stale" role="status">
              <span>File may be stale — background edits detected.</span>
              <button type="button" onClick={reloadDrawerContent}>
                Reload
              </button>
            </div>
          </>
        )}

        <div
          id={liveId}
          className="workspace-drawer__live"
          aria-live="polite"
          // biome-ignore lint/a11y/useSemanticElements: aria-live polite region; <output> does not support live announcements
          role="status"
          data-seq={staleAnnounceSeq}
        >
          {announce}
        </div>

        <div className="workspace-drawer__tabs" role="tablist" aria-label="Workspace panels">
          <button
            type="button"
            role="tab"
            aria-selected={tab === "viewer"}
            onClick={() => setDrawerTab("viewer")}
          >
            Viewer
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === "diff"}
            onClick={() => setDrawerTab("diff")}
            disabled={target?.kind !== "edit"}
          >
            Diff
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === "tree"}
            onClick={() => setDrawerTab("tree")}
          >
            Tree
          </button>
        </div>

        <div className="workspace-drawer__body">
          {tab === "viewer" && (
            <FileViewer
              file={file}
              loading={loadingFile}
              error={fileError}
              eventKind={target?.kind}
            />
          )}
          {tab === "diff" && <DiffViewer diff={diff} loading={loadingDiff} error={diffError} />}
          {tab === "tree" && sessionId && (
            <WorkspaceTree
              sessionId={sessionId}
              pinned={pinnedHandle}
              reloadToken={reloadGeneration}
              onSelectFile={(path) => {
                openDrawerFromRow({ sessionId, path, kind: "read" }, "viewer");
              }}
            />
          )}
        </div>
      </div>
    </dialog>
  );
}
