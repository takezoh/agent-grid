import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { makeSessionsApi } from "./api/sessions";
import type { ApiHttpError } from "./api/sessions";
import { hostedSessionId, isHostedMode, readBearerTokenFromHash } from "./auth";
import { AppShell } from "./components/AppShell";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { ConnectionsPanel } from "./components/ConnectionsPanel";
import { HeaderBar } from "./components/HeaderBar";
import { LastPromptBar } from "./components/LastPromptBar";
import { MainTabs } from "./components/MainTabs";
import { NewSessionButton } from "./components/NewSessionButton";
import { NotificationToast } from "./components/NotificationToast";
import { SessionList } from "./components/SessionList";
import { SidebarBrandRow } from "./components/SidebarBrandRow";
import { StatusBar } from "./components/StatusBar";
import { TerminalPane } from "./components/TerminalPane";
import { CognitivePanel } from "./components/cognitive/CognitivePanel";
import {
  WORKSPACE_MODE_STORAGE_KEY,
  WorkspaceModeSwitch,
  type WorkspaceUiMode,
  parseWorkspaceUiMode,
  validateWorkspaceUiMode,
} from "./components/cognitive/WorkspaceModeSwitch";
import { CommandPalette } from "./components/palette/CommandPalette";
import { WorkspaceDrawer } from "./components/workspace/WorkspaceDrawer";
import { makeFleetProvider } from "./fleet/provider";
import {
  fleetSessionIdFor,
  resolveSessionRoute,
  toFleetSessionRow,
} from "./fleet/session-identity";
import type { FleetHealthView } from "./fleet/session-protocol";
import { useFavicon } from "./hooks/useFavicon";
import { useGlobalHotkey } from "./hooks/useGlobalHotkey";
import { useMobileGate } from "./hooks/useMobileGate";
import { usePersistedValue } from "./hooks/usePersistedValue";
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
import "./css/cognitive.css";

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
  const fleet = useMemo(() => makeFleetProvider(), []);
  const [fleetHealth, setFleetHealth] = useState<FleetHealthView | null>(null);
  const [connectionsOpen, setConnectionsOpen] = useState(false);
  const [configPath, setConfigPath] = useState<string | null>(null);
  const pendingExternalSelection = useRef<string | null>(null);
  const configMounted = useRef(true);
  const configRequest = useRef<Promise<void> | null>(null);
  const priorHealthKinds = useRef(new Map<string, string>());
  const activeSessionID = useDaemonStore((state) => state.activeSessionID);
  const activeSessionServerID = useDaemonStore((state) =>
    state.activeSessionID
      ? state.sessions.find((session) => session.id === state.activeSessionID)?.serverId
      : undefined,
  );
  const serverId = useMemo(() => {
    if (!activeSessionID) return fleet.initialServerId() ?? "local";
    // Row first, id second — the resolver's own precedence. The id decode is
    // not a fallback for laziness: a session can be selected before its row
    // arrives (a hosted deep-link selects on mount, and creating a session
    // selects it before the next fleet snapshot), and resolving to the default
    // server in that window would point the Connection at a server that does
    // not own the session.
    const route = resolveSessionRoute(activeSessionID);
    const fromId = route.ok ? route.serverId : undefined;
    return activeSessionServerID ?? fromId ?? "local";
  }, [activeSessionID, activeSessionServerID, fleet]);
  const conn = useMemo(() => {
    const transport = fleet.activeConnection(serverId, token);
    return new Connection({
      ...transport,
      bearerToken: token,
      publishSessions: false,
      serverId,
    });
  }, [fleet, serverId, token]);

  const loadSessionConfig = useCallback((): Promise<void> => {
    if (configRequest.current) return configRequest.current;

    const request = makeSessionsApi()
      .getSessionConfig()
      .then((cfg) => {
        if (!configMounted.current) return;
        useDaemonStore.getState().setSessionConfig(cfg);
      })
      .catch((e: unknown) => {
        if (!configMounted.current) return;
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
    configRequest.current = request;
    void request.finally(() => {
      if (configRequest.current === request) configRequest.current = null;
    });
    return request;
  }, []);

  useEffect(() => {
    void conn.start();
    return () => conn.close();
  }, [conn]);

  useEffect(
    () =>
      fleet.subscribe((snapshot) => {
        const sessions = (snapshot.sessions ?? []).map(({ ref, serverLabel, session }) =>
          toFleetSessionRow(ref, session, serverLabel),
        );
        useDaemonStore.getState().applyViewUpdate({ k: "v", sessions });
        const state = useDaemonStore.getState();
        const pending = pendingExternalSelection.current;
        if (pending && sessions.some((session) => session.id === pending)) {
          pendingExternalSelection.current = null;
          state.selectSession(pending);
          return;
        }
        if (state.activeSessionID === null && sessions[0]) {
          state.selectSession(sessions[0].id);
        }
      }),
    [fleet],
  );

  // Application-lifecycle singleton health subscription (one writer path into SPA state).
  // A server recovering from unavailable state also rehydrates session config:
  // the initial request may have failed while that server was unreachable.
  useEffect(
    () =>
      fleet.subscribeHealth((view) => {
        let recovered = false;
        for (const server of view.servers) {
          const prior = priorHealthKinds.current.get(server.serverId);
          const current = server.observation.kind;
          if (
            server.enabled &&
            current === "connected" &&
            (prior === "unavailable" || prior === "health_unavailable")
          ) {
            recovered = true;
          }
          priorHealthKinds.current.set(server.serverId, current);
        }
        setFleetHealth(view);
        if (recovered) void loadSessionConfig();
      }),
    [fleet, loadSessionConfig],
  );

  useEffect(() => {
    void fleet.configPath?.().then((path) => setConfigPath(path));
  }, [fleet]);

  useEffect(() => {
    const select = (event: Event) => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (!detail || typeof detail !== "object") return;
      const { serverId, sessionId } = detail as Record<string, unknown>;
      if (typeof serverId !== "string" || typeof sessionId !== "string") return;
      const id = fleetSessionIdFor({ serverId, sessionId });
      if (useDaemonStore.getState().sessions.some((session) => session.id === id)) {
        useDaemonStore.getState().selectSession(id);
      } else {
        pendingExternalSelection.current = id;
      }
    };
    window.addEventListener("agent-grid-select-session", select);
    return () => window.removeEventListener("agent-grid-select-session", select);
  }, []);

  // visibilitychange safety net: the daemon expires terminal leases after
  // 12s (ADR terminal-lifecycle-bounds). The 4s renewal timer inside
  // TerminalSubscriptionController normally keeps the lease alive, but
  // browsers throttle setTimeout in hidden tabs and a throttled renewal
  // can miss the deadline — the tab then returns to visibility with a
  // silently dead subscription. Firing a forced renewal on visible
  // guarantees a fresh `ld` is sent (which the daemon accepts, replaces
  // the binding, and resets the TTL), so output resumes without the user
  // having to switch sessions to trigger a fresh subscribe.
  useEffect(() => {
    const handler = () => {
      if (document.visibilityState === "visible") {
        conn.forceTerminalRenewal();
      }
    };
    document.addEventListener("visibilitychange", handler);
    return () => document.removeEventListener("visibilitychange", handler);
  }, [conn]);

  // Hosted mode (Electron Workspace): token comes from preload (never URL)
  // and a deep-linked session is pre-selected. The sidebar is NOT suppressed —
  // it renders identically in browser and hosted modes (user decision
  // 2026-07-25: default visible, toggleable, persisted in AppShell).
  useEffect(() => {
    if (!hosted || !hostedSession) return;
    useDaemonStore
      .getState()
      .selectSession(
        serverId ? fleetSessionIdFor({ serverId, sessionId: hostedSession }) : hostedSession,
      );
  }, [hosted, hostedSession, serverId]);

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
    configMounted.current = true;
    void loadSessionConfig();
    return () => {
      configMounted.current = false;
    };
  }, [loadSessionConfig]);

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

  // Classic ⇄ Cognitive workspace switcher (plan §2.1): Classic is the
  // default and the permanent one-click fallback; the choice persists per
  // client. The Cognitive layer mounts lazily on first visit and then stays
  // mounted (same layering pattern as Terminal/Workspace modes).
  const [workspaceUiMode, setWorkspaceUiMode] = usePersistedValue<WorkspaceUiMode>({
    key: WORKSPACE_MODE_STORAGE_KEY,
    parse: parseWorkspaceUiMode,
    serialize: String,
    fallback: "classic",
    validate: validateWorkspaceUiMode,
  });
  const [cognitiveVisited, setCognitiveVisited] = useState(workspaceUiMode === "cognitive");
  const handleWorkspaceUiModeChange = useCallback(
    (next: WorkspaceUiMode) => {
      if (next === "cognitive") setCognitiveVisited(true);
      setWorkspaceUiMode(next);
    },
    [setWorkspaceUiMode],
  );

  const classicHeader = activeSession ? (
    <HeaderBar
      project={activeSession.project}
      card={activeSession.view.card}
      status={activeSession.status}
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

  const headerContent = (
    <div className="workspace-mode-header">
      <WorkspaceModeSwitch value={workspaceUiMode} onChange={handleWorkspaceUiModeChange} />
      <div className="workspace-mode-header__rest">
        {workspaceUiMode === "classic" ? (
          classicHeader
        ) : (
          <span className="cognitive-header-title">Cognitive Workspace</span>
        )}
      </div>
    </div>
  );

  const sidebarContent = (
    <div className="sidebar-shell">
      <SidebarBrandRow />
      <SessionList conn={conn} onRequestTerminate={handleRequestTerminate} />
      <NewSessionButton />
    </div>
  );

  const classicMain = (
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
        <StatusBar
          statusLine={activeSession?.view.status_line}
          statusChangedAt={activeSession?.status.changed_at}
          onRetry={() => conn.retry()}
          health={fleetHealth}
          onOpenConnections={() => {
            if (fleet.openConnections) {
              void fleet.openConnections();
              return;
            }
            setConnectionsOpen(true);
          }}
        />
      </div>
    </div>
  );

  const mainContent = (
    <div className="workspace-ui-modes" data-testid="workspace-ui-modes">
      <div
        className="workspace-ui-modes__layer"
        data-active={workspaceUiMode === "classic" ? "true" : "false"}
      >
        {classicMain}
      </div>
      <div
        className="workspace-ui-modes__layer"
        data-active={workspaceUiMode === "cognitive" ? "true" : "false"}
      >
        {cognitiveVisited && <CognitivePanel active={workspaceUiMode === "cognitive"} />}
      </div>
    </div>
  );

  return (
    <AppShell
      banner={null}
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
          <ConnectionsPanel
            open={connectionsOpen}
            health={fleetHealth}
            configPath={configPath}
            onClose={() => setConnectionsOpen(false)}
            onReconnect={(serverId) => fleet.reconnect(serverId)}
          />
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
