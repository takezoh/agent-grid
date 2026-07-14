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
  selectAriaLiveMessage,
  selectBufferDirty,
  selectConflictBannerVisible,
  selectDrawerStale,
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
  const reconnectResyncGeneration = useWorkspaceActivityStore((s) => s.reconnectResyncGeneration);
  const rootDisappeared = useWorkspaceActivityStore((s) => s.rootDisappeared);
  const closeWarningOpen = useWorkspaceActivityStore((s) => s.closeWarningOpen);
  const liveAnnounceSeq = useWorkspaceActivityStore((s) => s.liveAnnounceSeq);

  const requestCloseDrawer = useWorkspaceActivityStore((s) => s.requestCloseDrawer);
  const confirmDiscardAndClose = useWorkspaceActivityStore((s) => s.confirmDiscardAndClose);
  const cancelCloseWarning = useWorkspaceActivityStore((s) => s.cancelCloseWarning);
  const setDrawerTab = useWorkspaceActivityStore((s) => s.setDrawerTab);
  const setPinnedHandle = useWorkspaceActivityStore((s) => s.setPinnedHandle);
  const reloadDrawerContent = useWorkspaceActivityStore((s) => s.reloadDrawerContent);
  const openDrawerFromRow = useWorkspaceActivityStore((s) => s.openDrawerFromRow);
  const registerDirtyBuffer = useWorkspaceActivityStore((s) => s.registerDirtyBuffer);
  const setBufferDirty = useWorkspaceActivityStore((s) => s.setBufferDirty);
  const clearDirtyBuffer = useWorkspaceActivityStore((s) => s.clearDirtyBuffer);
  const setConflictOutcome = useWorkspaceActivityStore((s) => s.setConflictOutcome);
  const setRootDisappeared = useWorkspaceActivityStore((s) => s.setRootDisappeared);
  const resolveConflict = useWorkspaceActivityStore((s) => s.resolveConflict);

  const stale = useWorkspaceActivityStore((s) => selectDrawerStale(s, target?.path));
  const dirty = useWorkspaceActivityStore((s) => selectBufferDirty(s, target?.path));
  const conflictVisible = useWorkspaceActivityStore((s) =>
    selectConflictBannerVisible(s, target?.path),
  );
  const announce = useWorkspaceActivityStore((s) => selectAriaLiveMessage(s, target?.path));

  const [file, setFile] = useState<WorkspaceFileResponse | null>(null);
  const [diff, setDiff] = useState<WorkspaceDiffResponse | null>(null);
  const [loadingFile, setLoadingFile] = useState(false);
  const [loadingDiff, setLoadingDiff] = useState(false);
  const [fileError, setFileError] = useState<string | null>(null);
  const [diffError, setDiffError] = useState<string | null>(null);
  const [handleStale, setHandleStale] = useState(false);
  const [skipPrecondition, setSkipPrecondition] = useState(false);
  const [clipboardContent, setClipboardContent] = useState<string | null>(null);

  // Pin root handle once per drawer open (never re-resolve on background frame push).
  useEffect(() => {
    if (!open) {
      setHandleStale(false);
      setSkipPrecondition(false);
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
      if (resp.mtime) {
        registerDirtyBuffer(target.path, resp.mtime);
      }
    } catch (e) {
      if (WorkspaceApiError.isHandleStale(e)) {
        setHandleStale(true);
        if (dirty) {
          setRootDisappeared(true);
        }
        setFileError(null);
      } else {
        setFileError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLoadingFile(false);
    }
  }, [sessionId, pinnedHandle, target, registerDirtyBuffer, dirty, setRootDisappeared]);

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
        if (dirty) {
          setRootDisappeared(true);
        }
        setDiffError(null);
      } else {
        setDiffError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLoadingDiff(false);
    }
  }, [sessionId, pinnedHandle, target, dirty, setRootDisappeared]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: reloadGeneration is the store-driven refetch trigger
  useEffect(() => {
    if (!open || tab === "tree") return;
    void fetchContent();
    if (target?.kind === "edit") void fetchDiff();
  }, [open, tab, fetchContent, fetchDiff, target?.kind, reloadGeneration]);

  // Reconnect: re-fetch mtime for dirty buffers.
  useEffect(() => {
    if (!open || reconnectResyncGeneration === 0 || !sessionId || !pinnedHandle || !target?.path) {
      return;
    }
    const buffer = useWorkspaceActivityStore.getState().dirtyBuffers[target.path];
    if (!buffer?.dirty || !buffer.ifUnmodifiedSince) return;

    let cancelled = false;
    void (async () => {
      try {
        const api = makeWorkspaceApi();
        const resp = await api.getFile(sessionId, target.path, pinnedHandle);
        if (cancelled) return;
        if (resp.mtime && resp.mtime !== buffer.ifUnmodifiedSince) {
          setConflictOutcome(target.path, "reconnect_mtime_differs");
        }
      } catch {
        // connectivity degraded — save remains disabled via transportDegraded elsewhere
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [
    open,
    reconnectResyncGeneration,
    sessionId,
    pinnedHandle,
    target?.path,
    setConflictOutcome,
  ]);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") requestCloseDrawer();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, requestCloseDrawer]);

  const onScrimClick = (e: MouseEvent<HTMLDialogElement>) => {
    if (e.target === dialogRef.current) requestCloseDrawer();
  };

  const onDirtyChange = useCallback(
    (isDirty: boolean) => {
      if (!target?.path) return;
      setBufferDirty(target.path, isDirty);
    },
    [target?.path, setBufferDirty],
  );

  const onSaveSuccess = useCallback(() => {
    if (!target?.path) return;
    clearDirtyBuffer(target.path);
    setSkipPrecondition(false);
    reloadDrawerContent();
  }, [target?.path, clearDirtyBuffer, reloadDrawerContent]);

  const onSaveError = useCallback(
    (err: WorkspaceApiError) => {
      if (WorkspaceApiError.isHandleStale(err)) {
        setHandleStale(true);
        if (dirty) setRootDisappeared(true);
        return;
      }
      if (WorkspaceApiError.isPreconditionFailed(err)) {
        if (target?.path) {
          setConflictOutcome(target.path, "background_touch_dirty_buffer");
        }
        return;
      }
      setFileError((err as WorkspaceApiError).message);
    },
    [dirty, target?.path, setConflictOutcome, setRootDisappeared],
  );

  const onKeepMine = () => {
    if (!target?.path) return;
    resolveConflict(target.path, "keep_mine");
    setSkipPrecondition(true);
  };

  const onTakeTheirs = () => {
    if (!target?.path) return;
    resolveConflict(target.path, "take_theirs");
    setSkipPrecondition(false);
  };

  const onMerge = () => {
    if (!target?.path) return;
    resolveConflict(target.path, "merge");
    void fetchDiff();
  };

  const onExportClipboard = async () => {
    const text = clipboardContent ?? file?.content ?? "";
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // fallback for test env
    }
  };

  const headerPath = target?.path ?? "Workspace";
  const saveDisabled = rootDisappeared || handleStale;

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
          requestCloseDrawer();
        }
      }}
      data-testid="workspace-drawer"
    >
      <div className="workspace-drawer__panel">
        <header className="workspace-drawer__header">
          <h2 className="workspace-drawer__title">
            {headerPath}
            {dirty && (
              <span
                className="workspace-drawer__dirty"
                data-testid="dirty-indicator"
                aria-label="Unsaved changes"
              >
                {" "}
                •
              </span>
            )}
          </h2>
          <button
            type="button"
            className="workspace-drawer__close"
            aria-label="Close"
            onClick={requestCloseDrawer}
          >
            ×
          </button>
        </header>

        {rootDisappeared && (
          <div
            className="workspace-drawer__root-disappeared"
            role="status"
            data-testid="root-disappeared-banner"
          >
            <span>Workspace root disappeared. Buffer kept in memory; save is disabled.</span>
            <button type="button" onClick={() => void onExportClipboard()}>
              Copy buffer to clipboard
            </button>
          </div>
        )}

        {conflictVisible && !rootDisappeared && (
          <div
            className="workspace-drawer__conflict"
            role="status"
            data-testid="conflict-banner"
          >
            <span>Write conflict detected for this file.</span>
            <button type="button" onClick={onKeepMine}>
              Keep mine
            </button>
            <button type="button" onClick={onTakeTheirs}>
              Take theirs
            </button>
            <button type="button" onClick={onMerge}>
              Merge
            </button>
          </div>
        )}

        {handleStale && !rootDisappeared && (
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

        {stale && !handleStale && !conflictVisible && (
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
          data-seq={liveAnnounceSeq}
        >
          {announce}
        </div>

        {closeWarningOpen && (
          <dialog
            open
            className="workspace-drawer__close-warning"
            data-testid="close-warning-dialog"
            aria-label="Unsaved changes"
          >
            <p>You have unsaved changes. Discard and close?</p>
            <button type="button" onClick={confirmDiscardAndClose}>
              Discard
            </button>
            <button type="button" onClick={cancelCloseWarning}>
              Cancel
            </button>
          </dialog>
        )}

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
              sessionId={sessionId ?? undefined}
              pinnedHandle={pinnedHandle ?? undefined}
              onDirtyChange={onDirtyChange}
              onBufferChange={setClipboardContent}
              onSaveSuccess={onSaveSuccess}
              onSaveError={onSaveError}
              saveDisabled={saveDisabled}
              skipPrecondition={skipPrecondition}
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