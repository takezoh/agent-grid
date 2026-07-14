import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { makeSessionsApi } from "./api/sessions";
import type { ApiHttpError } from "./api/sessions";
import { readBearerTokenFromHash } from "./auth";
import { AppShell } from "./components/AppShell";
import { CommandSearchTrigger } from "./components/CommandSearchTrigger";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { DriverViewPanel } from "./components/DriverViewPanel";
import { MainTabs } from "./components/MainTabs";
import { NotificationToast } from "./components/NotificationToast";
import { SessionList } from "./components/SessionList";
import { StatusBanner } from "./components/StatusBanner";
import { TerminalPane } from "./components/TerminalPane";
import { ThemeSegmentedControl } from "./components/ThemeSegmentedControl";
import { CommandPalette } from "./components/palette/CommandPalette";
import { ActivityRail } from "./components/workspace/ActivityRail";
import { WorkspaceDrawer } from "./components/workspace/WorkspaceDrawer";
import { useFavicon } from "./hooks/useFavicon";
import { useGlobalHotkey } from "./hooks/useGlobalHotkey";
import { useMobileGate } from "./hooks/useMobileGate";
import { useTerminateSession } from "./hooks/useTerminateSession";
import { Connection } from "./socket/connection";
import { useDaemonStore } from "./store/daemon";
import {
  normalizeFrameMessagingSummary,
  selectFrameMessagingSummary,
  useFrameMessagingStore,
} from "./store/frameMessaging";
import { useNotificationsStore } from "./store/notifications";
import { useWorkspaceActivityStore } from "./store/workspaceActivity";
import "./css/workspace.css";

export function App() {
  // ADR-0037 / FR-001: intercept Cmd/Ctrl+K on the capture phase.
  // Invariant: mount this exactly once across the App tree. Do not call from
  // multiple sites.
  useGlobalHotkey();

  // Browser tab favicon reflects the loudest status across all sessions
  // (priority: running > pending > waiting > idle > stopped). Mount once.
  useFavicon();

  const token = useMemo(() => readBearerTokenFromHash(), []);
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

  useEffect(() => {
    useWorkspaceActivityStore.getState().setScopedSession(activeSessionID);
  }, [activeSessionID]);
  const activeSession = useDaemonStore((s) =>
    s.activeSessionID ? (s.sessions.find((x) => x.id === s.activeSessionID) ?? null) : null,
  );
  const activeFrameMessagingSummary = useFrameMessagingStore((s) =>
    activeSessionID ? selectFrameMessagingSummary(s, activeSessionID) : undefined,
  );

  // セッション終了 confirm dialog の state. 単一インスタンスを App 直下で
  // 持ち overlays に mount する. dialog の variant は mobile gate に応じて
  // sheet / modal を出し分け.
  const [terminationTarget, setTerminationTarget] = useState<{
    id: string;
    label: string;
  } | null>(null);
  // dialog close 時に focus を戻す opener (× button) を保持.
  // ConfirmDialog は openerRef.current?.focus() で復元する.
  const terminationOpenerRef = useRef<HTMLElement | null>(null);
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

  // B3 / ADR-0062: the legacy 'Command (⌘K)' + 'New Session' header buttons
  // are absorbed into the single CommandSearchTrigger (search-bar style) +
  // ThemeSegmentedControl. The palette's tool list surfaces new-session at
  // the top, so the discoverability of new-session is preserved without a
  // second CTA — keeping the header free of competing affordances on
  // narrow viewports (UAC-008 mobile counterexample target).
  const headerContent = (
    <>
      <CommandSearchTrigger />
      <ThemeSegmentedControl />
    </>
  );

  const sidebarContent = <SessionList conn={conn} />;

  const openDrawerTree = useCallback(() => {
    if (!activeSessionID) return;
    useWorkspaceActivityStore.getState().openDrawerTree(activeSessionID);
  }, [activeSessionID]);

  const mainContent = (
    <div className="main-with-activity-rail" data-testid="main-with-activity-rail">
      <ActivityRail onOpenTree={openDrawerTree} />
      <div className="main-with-activity-rail__tabs">
        {activeSession && (
          <DriverViewPanel
            view={activeSession.view}
            sessionId={activeSession.id}
            onRequestTerminate={handleRequestTerminate}
          />
        )}
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
            <TerminalPane
              key={activeSessionID ?? "__none__"}
              conn={conn}
              sessionId={activeSessionID}
            />
          }
        />
      </div>
    </div>
  );

  return (
    <AppShell
      banner={<StatusBanner />}
      header={headerContent}
      sidebar={sidebarContent}
      main={mainContent}
      overlays={
        <>
          <NotificationToast />
          {/* Mounted via portal directly under <body>, so the placement of
              this element in the tree is irrelevant (ADR-0036). */}
          <CommandPalette />
          <WorkspaceDrawer sessionId={activeSessionID} />
          <ConfirmDialog
            open={terminationTarget !== null}
            variant={isMobile ? "sheet" : "modal"}
            title="セッションを終了"
            body={
              terminationTarget !== null
                ? `「${terminationTarget.label}」を終了します。実行中のプロセスは停止されます。`
                : ""
            }
            confirmLabel="終了する"
            cancelLabel="キャンセル"
            destructive
            pending={terminatePending}
            pendingLabel="終了中…"
            onConfirm={handleConfirmTerminate}
            onCancel={closeTermination}
            openerRef={terminationOpenerRef}
          />
        </>
      }
    />
  );
}
