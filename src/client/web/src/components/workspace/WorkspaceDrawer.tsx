import { type ReactNode, useCallback, useEffect, useId, useState } from "react";
import {
  WorkspaceApiError,
  type WorkspaceDiffResponse,
  type WorkspaceFileResponse,
  makeWorkspaceApi,
} from "../../api/workspace";
import {
  isWorkspaceRequestCurrent,
  selectAriaLiveMessage,
  selectBufferDirty,
  selectConflictBannerVisible,
  selectDrawerStale,
  useWorkspaceActivityStore,
} from "../../store/workspaceActivity";
import { UnderlineTab, UnderlineTabList } from "../primitives/UnderlineTab";
import { ChangesDegradedNotice, ChangesRowsList } from "./ChangesRows";
import { DiffViewer } from "./DiffViewer";
import { FileViewer } from "./FileViewer";
import { WorkspaceTree } from "./WorkspaceTree";

export type WorkspaceDrawerProps = {
  sessionId: string | null;
};

function PathBreadcrumb({ path, dirty }: { path: string; dirty: boolean }): ReactNode {
  const segments = path ? path.split("/").filter(Boolean) : ["Workspace"];
  return (
    <nav className="workspace-drawer__breadcrumb" aria-label="File path">
      <ol className="workspace-drawer__breadcrumb-list">
        {segments.map((seg, i) => (
          <li
            key={segments.slice(0, i + 1).join("/")}
            className="workspace-drawer__breadcrumb-item"
          >
            {i > 0 && (
              <span className="workspace-drawer__breadcrumb-sep" aria-hidden="true">
                /
              </span>
            )}
            <span className="workspace-drawer__breadcrumb-seg">{seg}</span>
          </li>
        ))}
      </ol>
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
    </nav>
  );
}

export function WorkspaceDrawer({ sessionId }: WorkspaceDrawerProps): ReactNode {
  const liveId = useId();

  const open = useWorkspaceActivityStore((s) => s.drawerOpen);
  const visible = useWorkspaceActivityStore((s) => s.mainMode === "workspace");
  const setMainMode = useWorkspaceActivityStore((s) => s.setMainMode);
  const tab = useWorkspaceActivityStore((s) => s.drawerTab);
  const target = useWorkspaceActivityStore((s) => s.drawerTarget);
  const pinnedHandle = useWorkspaceActivityStore((s) => s.pinnedHandle);
  const workspaceEpoch = useWorkspaceActivityStore((s) => s.workspaceEpoch);
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
  const completeOrphanedRecovery = useWorkspaceActivityStore((s) => s.completeOrphanedRecovery);

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

  const requestIsCurrent = useCallback(() => {
    if (!sessionId) return false;
    return isWorkspaceRequestCurrent({ sessionId, epoch: workspaceEpoch });
  }, [sessionId, workspaceEpoch]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: session/epoch identity change must clear component-local async state
  useEffect(() => {
    setFile(null);
    setDiff(null);
    setLoadingFile(false);
    setLoadingDiff(false);
    setFileError(null);
    setDiffError(null);
    setHandleStale(false);
    setSkipPrecondition(false);
    setClipboardContent(null);
  }, [sessionId, workspaceEpoch]);

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
        if (cancelled || !requestIsCurrent()) return;
        setPinnedHandle({
          sessionId: handle.session_id,
          frameGeneration: handle.frame_generation,
          resolvedRootPath: handle.resolved_root_path,
        });
      } catch (e) {
        if (cancelled || !requestIsCurrent()) return;
        setFileError(e instanceof Error ? e.message : String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, sessionId, pinnedHandle, setPinnedHandle, requestIsCurrent]);

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
      if (!requestIsCurrent()) return;
      setFile(resp);
      if (resp.mtime) {
        registerDirtyBuffer(target.path, resp.mtime);
      }
    } catch (e) {
      if (!requestIsCurrent()) return;
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
      if (requestIsCurrent()) setLoadingFile(false);
    }
  }, [
    sessionId,
    pinnedHandle,
    target,
    registerDirtyBuffer,
    dirty,
    setRootDisappeared,
    requestIsCurrent,
  ]);

  const fetchDiff = useCallback(async () => {
    if (!sessionId || !pinnedHandle || !target?.path) return;
    setLoadingDiff(true);
    setDiffError(null);
    try {
      const api = makeWorkspaceApi();
      const resp = await api.getDiff(sessionId, target.path, pinnedHandle);
      if (!requestIsCurrent()) return;
      setDiff(resp);
    } catch (e) {
      if (!requestIsCurrent()) return;
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
      if (requestIsCurrent()) setLoadingDiff(false);
    }
  }, [sessionId, pinnedHandle, target, dirty, setRootDisappeared, requestIsCurrent]);

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
        if (cancelled || !requestIsCurrent()) return;
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
    requestIsCurrent,
  ]);

  // Esc returns to Terminal mode. Pure visibility switch — the workspace
  // session (open file, dirty buffer) survives, so no discard guard is
  // needed. Skipped while focus is inside the CodeMirror editor, where Esc
  // is vim currency and must never leave the mode.
  useEffect(() => {
    if (!open || !visible) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      const el = e.target;
      if (el instanceof HTMLElement && el.closest(".cm-editor") !== null) return;
      setMainMode("terminal");
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, visible, setMainMode]);

  const onDirtyChange = useCallback(
    (isDirty: boolean) => {
      if (!target?.path) return;
      setBufferDirty(target.path, isDirty);
    },
    [target?.path, setBufferDirty],
  );

  const onSaveSuccess = useCallback(() => {
    if (!target?.path || !requestIsCurrent()) return;
    clearDirtyBuffer(target.path);
    setSkipPrecondition(false);
    reloadDrawerContent();
  }, [target?.path, clearDirtyBuffer, reloadDrawerContent, requestIsCurrent]);

  const onSaveError = useCallback(
    (err: WorkspaceApiError) => {
      if (!requestIsCurrent()) return;
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
    [dirty, target?.path, setConflictOutcome, setRootDisappeared, requestIsCurrent],
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
      completeOrphanedRecovery();
    } catch {
      // fallback for test env
    }
  };

  const headerPath = target?.path ?? "Workspace";
  const saveDisabled = rootDisappeared || handleStale;
  const readOnlyReason = rootDisappeared
    ? "Workspace root disappeared. Buffer kept in memory; save is disabled."
    : handleStale
      ? "Workspace root changed while this drawer was open. Close and reopen the drawer to refresh the pinned root."
      : null;

  if (!open) return null;

  const contentTab = tab === "diff" ? "diff" : "viewer";

  return (
    <section className="workspace-view" aria-label="Workspace" data-testid="workspace-drawer">
      <div className="workspace-view__main workspace-drawer__panel">
        <header className="workspace-drawer__header">
          <h2 className="workspace-drawer__title">
            <PathBreadcrumb path={headerPath} dirty={dirty} />
          </h2>
          <UnderlineTabList aria-label="Workspace panels" className="workspace-drawer__tabs">
            <UnderlineTab selected={contentTab === "viewer"} onClick={() => setDrawerTab("viewer")}>
              Viewer
            </UnderlineTab>
            <UnderlineTab
              selected={contentTab === "diff"}
              onClick={() => setDrawerTab("diff")}
              disabled={target?.kind !== "edit"}
            >
              Diff
            </UnderlineTab>
          </UnderlineTabList>
          <button
            type="button"
            className="workspace-drawer__close"
            aria-label="Close"
            title="Back to terminal (Esc)"
            onClick={requestCloseDrawer}
          >
            ×
          </button>
        </header>

        {rootDisappeared && (
          <div
            className="workspace-drawer__root-disappeared"
            // biome-ignore lint/a11y/useSemanticElements: status banner is not form output; tests pin getByRole('status')
            role="status"
            data-testid="root-disappeared-banner"
          >
            <span>Workspace root disappeared. Buffer kept in memory; save is disabled.</span>
            <button type="button" onClick={() => void onExportClipboard()}>
              Copy buffer to clipboard
            </button>
            <button type="button" onClick={completeOrphanedRecovery}>
              Discard buffer
            </button>
          </div>
        )}

        {conflictVisible && !rootDisappeared && (
          <div
            className="workspace-drawer__conflict"
            // biome-ignore lint/a11y/useSemanticElements: status banner is not form output; tests pin getByRole('status')
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

        <div className="workspace-drawer__body">
          {target === null ? (
            <div className="workspace-view__empty" data-testid="workspace-empty">
              <p>Select a file from the tree to view or edit it.</p>
            </div>
          ) : (
            <>
              <div
                className="workspace-drawer__panel-view"
                hidden={contentTab !== "viewer" ? true : undefined}
              >
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
                  readOnlyReason={readOnlyReason}
                  skipPrecondition={skipPrecondition}
                />
              </div>
              <div
                className="workspace-drawer__panel-view"
                hidden={contentTab !== "diff" ? true : undefined}
              >
                <DiffViewer diff={diff} loading={loadingDiff} error={diffError} />
              </div>
            </>
          )}
        </div>
      </div>
      <aside className="workspace-view__tree" aria-label="Workspace files">
        <div className="workspace-view__side-changes" data-testid="workspace-changes">
          <div className="workspace-view__side-head">Changes</div>
          <ChangesDegradedNotice />
          <ChangesRowsList />
        </div>
        {sessionId && (
          <div className="workspace-view__tree-body">
            <WorkspaceTree
              sessionId={sessionId}
              workspaceEpoch={workspaceEpoch}
              pinned={pinnedHandle}
              reloadToken={reloadGeneration}
              onSelectFile={(path) => {
                openDrawerFromRow({ sessionId, path, kind: "read" }, "viewer");
              }}
            />
          </div>
        )}
      </aside>
    </section>
  );
}
