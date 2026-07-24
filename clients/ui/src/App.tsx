import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { makeSessionsApi } from "./api/sessions";
import type { ApiHttpError } from "./api/sessions";
import { hostedSessionId, isHostedMode, readBearerTokenFromHash } from "./auth";
import { AppShell } from "./components/AppShell";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { HeaderBar } from "./components/HeaderBar";
import { LastPromptBar } from "./components/LastPromptBar";
import { MainTabs } from "./components/MainTabs";
import { NewSessionButton } from "./components/NewSessionButton";
import { NotificationToast } from "./components/NotificationToast";
import { SessionList } from "./components/SessionList";
import { SidebarBrandRow } from "./components/SidebarBrandRow";
import { StatusBanner } from "./components/StatusBanner";
import { StatusBar } from "./components/StatusBar";
import { TerminalPane } from "./components/TerminalPane";
import { CommandPalette } from "./components/palette/CommandPalette";
import { WorkspaceDrawer } from "./components/workspace/WorkspaceDrawer";
import { useFavicon } from "./hooks/useFavicon";
import { useGlobalHotkey } from "./hooks/useGlobalHotkey";
import { useMobileGate } from "./hooks/useMobileGate";
import { useTerminateSession } from "./hooks/useTerminateSession";
import type { TerminalGeometry } from "./lib/terminalGeometry";
import { Connection } from "./socket/connection";
import { useDaemonStore } from "./store/daemon";
import {
  normalizeFrameMessagingSummary,
  selectFrameMessagingSummary,
  useFrameMessagingStore,
} from "./store/frameMessaging";
import { useNotificationsStore } from "./store/notifications";
import { useWorkspaceActivityStore } from "./store/workspaceActivity";
import { truncateLabel } from "./util/truncate";
import "./css/workspace.css";

export function terminalIdentity(session: { id: string; head_frame_id?: string } | null): string {
  if (!session) return "__none__";
  return `${session.id}:${session.head_frame_id ?? "__legacy_head__"}`;
}

export function App() {
  // ADR-0037 / FR-001: intercept Cmd/Ctrl+K on the capture phase.
  // Invariant: mount this exactly once across the App tree. Do not call from
  // multiple sites.
  useGlobalHotkey();

  // Browser tab favicon reflects the loudest status across all sessions
  // (priority: running > pending > waiting > idle > stopped). Mount once.
  useFavicon();

  const token = useMemo(() => readBearerTokenFromHash(), []);
  const hosted = useMemo(() => isHostedMode(), []);
  const hostedSession = useMemo(() => hostedSessionId(), []);
  const conn = useMemo(
    () =>
      new Connection({
        ticketEndpoint: "/api/ws-ticket",
        wsUrl: (ticket) => {
          const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
          return `${proto}//${window.location.host}/ws?ticket=${encodeURIComponent(ticket)}`;
        },
        bearerToken: token,
      }),
    [token],
  );

  useEffect(() => {
    void conn.start();
    return () => conn.close();
  }, [conn]);

  // Hosted mode (Electron Workspace): 1-window-1-session view.
  // Token comes from preload (never URL); sidebar/new-session chrome is suppressed.
  useEffect(() => {
    if (!hosted || !hostedSession) return;
    useDaemonStore.getState().selectSession(hostedSession);
  }, [hosted, hostedSession]);

  // Blocker T1: hydrate daemon.sessionConfig at mount so the command
  // palette has projects + pushCommands available. Without this call,
  // GET /api/session-config never fires from production code path and
  // ParamSelectPhase / scope gating see empty lists forever (FR-013 /
  // FR-014 toggles never light up, push scope stays fail-closed).
  //
  // - We swallow 401 silently: the auth bootstrap (Connection.start)
  //   handles the missing-token path with its own toast; surfacing it
  //   twice would be noisy.
  // - Other HTTP / network failures surface as a single error toast and
  //   leave sessionConfig=null. The CommandPalette already logs a
  //   breadcrumb when sessionConfig is missing at open-time, so the
  //   diagnostic chain stays intact.
  // - We deliberately do NOT block UI rendering on this fetch: the
  //   palette can still surface the standard-scope tools (new-session,
  //   stop-session) with empty project lists while the request is in
  //   flight; the hydrate fires-and-fills as soon as it lands.
  useEffect(() => {
    const api = makeSessionsApi();
    let cancelled = false;
    api
      .getSessionConfig()
      .then((cfg) => {
        if (cancelled) return;
        useDaemonStore.getState().setSessionConfig(cfg);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        // 401 = token missing / stale. Connection.start surfaces this
        // separately; we stay quiet here so the user does not see two
        // toasts for the same auth gap.
        if (e instanceof Error && (e as ApiHttpError).status === 401) {
          return;
        }
        const msg = e instanceof Error ? e.message : String(e);
        useNotificationsStore.getState().add({
          level: "error",
          message: `Failed to load session config: ${msg}`,
        });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  // TerminalPane is the sole geometry measurement owner. App only retains the
  // latest snapshot so the independently-rendered palette overlay can read it
  // at submit time without duplicating xterm/DOM measurement.
  const terminalGeometryRef = useRef<TerminalGeometry | null>(null);
  const handleTerminalGeometryChange = useCallback((geometry: TerminalGeometry) => {
    terminalGeometryRef.current = geometry;
  }, []);
  const getTerminalGeometry = useCallback(() => terminalGeometryRef.current, []);

  const activeSession = useDaemonStore((s) =>
    s.activeSessionID ? (s.sessions.find((x) => x.id === s.activeSessionID) ?? null) : null,
  );
  const activeFrameMessagingSummary = useFrameMessagingStore((s) =>
    activeSessionID ? selectFrameMessagingSummary(s, activeSessionID) : undefined,
  );

  // Session-terminate confirm dialog state. A single instance lives directly
  // under App and mounts into overlays. The dialog variant switches between
  // sheet / modal based on the mobile gate.
  const [terminationTarget, setTerminationTarget] = useState<{
    id: string;
    label: string;
  } | null>(null);
  // Holds the opener (× button) that focus returns to on dialog close.
  // ConfirmDialog restores it via openerRef.current?.focus().
  const terminationOpenerRef = useRef<HTMLElement | null>(null);
  const workspaceSwitchOpenerRef = useRef<HTMLElement | null>(null);
  const handleRequestTerminate = useCallback((id: string, label: string, opener: HTMLElement) => {
    terminationOpenerRef.current = opener;
    setTerminationTarget({ id, label });
  }, []);
  const closeTermination = useCallback(() => setTerminationTarget(null), []);
  const { terminate, pending: terminatePending } = useTerminateSession();
  const isMobile = useMobileGate();
  const handleConfirmTerminate = useCallback(async () => {
    if (!terminationTarget) return;
    const ok = await terminate(terminationTarget.id);
    if (ok) closeTermination();
  }, [terminationTarget, terminate, closeTermination]);

  const pendingWorkspaceSessionId = useWorkspaceActivityStore((s) => s.pendingSessionSwitchId);
  const workspaceSessionSwitchError = useWorkspaceActivityStore((s) => s.sessionSwitchError);
  const workspaceEpoch = useWorkspaceActivityStore((s) => s.workspaceEpoch);
  const workspaceScopedSessionId = useWorkspaceActivityStore((s) => s.scopedSessionId);
  const workspaceOrphanedRecovery = useWorkspaceActivityStore((s) => s.orphanedRecovery);
  const pendingWorkspaceSession = useDaemonStore((s) =>
    pendingWorkspaceSessionId
      ? (s.sessions.find((session) => session.id === pendingWorkspaceSessionId) ?? null)
      : null,
  );
  const cancelWorkspaceSessionSwitch = useCallback(() => {
    useWorkspaceActivityStore.getState().cancelPendingSessionSwitch();
  }, []);
  const confirmWorkspaceSessionSwitch = useCallback(() => {
    const workspace = useWorkspaceActivityStore.getState();
    const pending = workspace.pendingSessionSwitchId;
    if (pending === null) return;
    if (!useDaemonStore.getState().sessions.some((session) => session.id === pending)) {
      workspace.markPendingSessionMissing();
      return;
    }
    const target = workspace.discardPendingSessionSwitch();
    if (target !== null) useDaemonStore.getState().selectSession(target);
  }, []);

  useEffect(() => {
    if (workspaceSessionSwitchError === null) return;
    useNotificationsStore.getState().add({
      level: "error",
      message:
        workspaceSessionSwitchError === "pending_target_disappeared"
          ? "The selected session disappeared. Unsaved Workspace changes were kept."
          : "The active session disappeared. Unsaved Workspace content is available for recovery.",
    });
  }, [workspaceSessionSwitchError]);

  const openDrawerTree = useCallback(() => {
    if (!activeSessionID) return;
    useWorkspaceActivityStore.getState().openDrawerTree(activeSessionID);
  }, [activeSessionID]);

  // Main-area mode: Terminal (tabs + xterm) vs Workspace (tree + editor).
  // mainMode is pure visibility — switching modes never closes the workspace
  // session, so the open file / dirty buffer / editor state survive.
  const workspaceMode = useWorkspaceActivityStore((s) => s.mainMode === "workspace");
  const workspaceSessionOpen = useWorkspaceActivityStore((s) => s.drawerOpen);
  const workspaceDisplaySessionId = workspaceOrphanedRecovery
    ? workspaceScopedSessionId
    : activeSessionID;
  const handleModeChange = useCallback(
    (next: "terminal" | "workspace") => {
      if (next === "workspace") {
        openDrawerTree();
        return;
      }
      useWorkspaceActivityStore.getState().setMainMode("terminal");
    },
    [openDrawerTree],
  );

  const headerContent = activeSession ? (
    <HeaderBar
      project={activeSession.project}
      card={activeSession.view.card}
      status={activeSession.view.status}
      model={activeSession.view.model}
      effort={activeSession.view.effort}
      driver={activeSession.root_driver}
      sessionId={activeSession.id}
      onRequestTerminate={handleRequestTerminate}
      mobile={isMobile}
      mode={workspaceMode ? "workspace" : "terminal"}
      onModeChange={handleModeChange}
    />
  ) : (
    <HeaderBar mobile={isMobile} />
  );

  const sidebarContent = hosted ? null : (
    <div className="sidebar-shell">
      <SidebarBrandRow />
      <SessionList conn={conn} onRequestTerminate={handleRequestTerminate} />
      <NewSessionButton />
    </div>
  );

  const mainContent = (
    <div className="main-with-changes" data-testid="main-with-changes">
      <div className="main-with-changes__terminal">
        {/* Layered modes (ADR-0065 pattern): MainTabs + xterm stay mounted
            with visibility toggled so scrollback / subscriptions survive
            Workspace mode; WorkspaceDrawer mounts lazily on open. */}
        <div className="main-modes">
          <div className="main-modes__layer" data-active={workspaceMode ? "false" : "true"}>
            <MainTabs
              tabs={activeSession?.view.log_tabs ?? []}
              messagesSummary={
                activeFrameMessagingSummary ??
                normalizeFrameMessagingSummary(activeSession?.view.frame_messaging_summary)
              }
              sessionId={activeSession?.id}
              bearerToken={token}
              suppressInfo={activeSession?.view.suppress_info ?? false}
              terminalSlot={
                <>
                  {/* First flex child of .terminal-slot: TERMINAL-only header
                      (hidden with the slot when a log tab is active). */}
                  <LastPromptBar
                    driver={activeSession?.root_driver}
                    prompt={activeSession?.view.last_user_prompt}
                  />
                  <TerminalPane
                    key={terminalIdentity(activeSession)}
                    conn={conn}
                    sessionId={activeSessionID}
                    onGeometryChange={handleTerminalGeometryChange}
                  />
                </>
              }
            />
          </div>
          <div className="main-modes__layer" data-active={workspaceMode ? "true" : "false"}>
            {workspaceSessionOpen && (
              <WorkspaceDrawer
                key={`${workspaceDisplaySessionId ?? "__none__"}:${workspaceEpoch}`}
                sessionId={workspaceDisplaySessionId}
              />
            )}
          </div>
        </div>
        {activeSession && (
          <StatusBar
            statusLine={activeSession.view.status_line}
            statusChangedAt={activeSession.view.status_changed_at}
          />
        )}
      </div>
    </div>
  );

  return (
    <AppShell
      banner={<StatusBanner />}
      header={headerContent}
      sidebar={sidebarContent}
      main={mainContent}
      hosted={hosted}
      overlays={
        <>
          <NotificationToast />
          {/* Mounted via portal directly under <body>, so the placement of
              this element in the tree is irrelevant (ADR-0036). */}
          <CommandPalette getTerminalGeometry={getTerminalGeometry} />
          <ConfirmDialog
            open={terminationTarget !== null}
            variant={isMobile ? "sheet" : "modal"}
            title="Stop session"
            body={
              terminationTarget !== null
                ? `"${truncateLabel(terminationTarget.label)}" will be stopped. Any running process will be terminated.`
                : ""
            }
            confirmLabel="Stop session"
            cancelLabel="Cancel"
            destructive
            pending={terminatePending}
            pendingLabel="Stopping…"
            onConfirm={handleConfirmTerminate}
            onCancel={closeTermination}
            openerRef={terminationOpenerRef}
          />
          <ConfirmDialog
            open={pendingWorkspaceSessionId !== null}
            variant={isMobile ? "sheet" : "modal"}
            title="Switch session"
            body={
              pendingWorkspaceSessionId !== null
                ? `Switch to "${pendingWorkspaceSession?.view.card.title ?? pendingWorkspaceSession?.project ?? pendingWorkspaceSessionId}"? Unsaved Workspace changes will be discarded.`
                : ""
            }
            confirmLabel="Discard and switch"
            cancelLabel="Cancel"
            destructive
            onConfirm={confirmWorkspaceSessionSwitch}
            onCancel={cancelWorkspaceSessionSwitch}
            openerRef={workspaceSwitchOpenerRef}
          />
        </>
      }
    />
  );
}
