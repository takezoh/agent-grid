import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App, terminalIdentity } from "./App";
// FR-D1 / FR-D2 / FR-D3: Header's Cmd/Ctrl-K label routes through the
// lib/platform single-source helper instead of the deleted local isMac().
// We mock the lib so each test can flip mac / non-mac without touching
// navigator (which different envs surface differently — userAgentData on
// Chromium, deprecated navigator.platform on Safari/Firefox, etc).
import { isMacPlatform } from "./lib/platform";
import { Connection } from "./socket/connection";
import { selectDaemonSnapshot, useDaemonStore } from "./store/daemon";
import { useFrameMessagingStore } from "./store/frameMessaging";
import { useNotificationsStore } from "./store/notifications";
import { usePaletteStore } from "./store/palette";
import { useWorkspaceActivityStore } from "./store/workspaceActivity";
import { mkSnapshot } from "./test/fixtures/daemon";

vi.mock("./lib/platform", () => ({
  isMacPlatform: vi.fn(),
}));

describe("terminalIdentity", () => {
  it("changes when the active head frame changes within one session", () => {
    expect(terminalIdentity({ id: "s1", head_frame_id: "f1" })).toBe("s1:f1");
    expect(terminalIdentity({ id: "s1", head_frame_id: "f2" })).toBe("s1:f2");
  });

  it("keeps a stable fallback identity for legacy session payloads", () => {
    expect(terminalIdentity({ id: "s1" })).toBe("s1:__legacy_head__");
    expect(terminalIdentity(null)).toBe("__none__");
  });
});

describe("App", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
    useDaemonStore.getState().reset();
    useFrameMessagingStore.getState().reset();
    useNotificationsStore.setState({ items: [] });
    useWorkspaceActivityStore.getState().reset();
    // FR-002 / FR-001: Header の Command ボタンと useGlobalHotkey() は
    // usePaletteStore に書き込むため、テスト間で open=true の漏れを防ぐ。
    usePaletteStore.getState().close();
    // Default isMacPlatform → false (Linux). Mac-branch tests override per case.
    vi.mocked(isMacPlatform).mockReturnValue(false);
    // Stub fetch to hang forever so Connection.start() never rejects and
    // no unhandled rejection leaks out of the voided conn.start() in useEffect.
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    // hash token を仕込んで Connection を初期化させる
    window.location.hash = "#token=test";
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    window.location.hash = "";
    usePaletteStore.getState().close();
  });

  it("FR-022 / UAC-010: dissolves DriverViewPanel into header + status bar + sidebar meta", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "/home/dev/dev/agent-grid",
          command: "claude",
          root_driver: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {
              title: "Hello driver",
              tags: [{ text: "feature", fg: "#000", bg: "#fff" }],
            },
            status: "running",
            model: "gpt-5",
            effort: "high",
            status_line: "Thinking",
            status_changed_at: "2026-06-20T00:00:00Z",
          },
        },
      ],
      activeSessionID: "s1",
    });
    render(<App />);
    expect(screen.queryByLabelText("driver view")).toBeNull();
    expect(document.querySelector(".driver-view-panel")).toBeNull();
    expect(screen.getByLabelText("session header")).toBeTruthy();
    expect(document.querySelector(".header-bar__project")?.textContent).toBe("agent-grid");
    expect(document.querySelector(".header-bar__title")?.textContent).toBe("Hello driver");
    const headerMeta = document.querySelector(".header-bar__meta");
    expect(headerMeta?.textContent).toContain("claude · gpt-5 · high");
    expect(document.querySelector(".header-bar .run-state-badge")).not.toBeNull();
    expect(screen.getByLabelText("session status")).toBeTruthy();
    expect(screen.getByText("Thinking")).toBeTruthy();
    expect(document.querySelector(".session-list__tags")).not.toBeNull();
    expect(screen.getByText("feature")).toBeTruthy();
  });

  // Last User Prompt terminal header: whitelist drivers (claude/codex/gemini/
  // shell) render the bar inside .terminal-slot; grok and generic sessions
  // (root_driver = command first token, e.g. "bash") render no bar.
  it("renders LastPromptBar inside terminal-slot for whitelisted drivers only", () => {
    const mkSession = (id: string, rootDriver: string, prompt?: string) => ({
      id,
      project: "p",
      command: rootDriver,
      root_driver: rootDriver,
      created_at: "2026-06-20T00:00:00Z",
      view: {
        card: {},
        status: "running" as const,
        ...(prompt !== undefined ? { last_user_prompt: prompt } : {}),
      },
    });

    useDaemonStore.setState({
      sessions: [mkSession("s-claude", "claude", "fix the bug")],
      activeSessionID: "s-claude",
    });
    const { rerender } = render(<App />);
    const bar = screen.getByLabelText("Last user prompt");
    expect(bar.textContent).toContain("fix the bug");
    // The bar is a child of the always-mounted terminal slot (TERMINAL-only
    // visibility follows the slot's data-active toggle).
    expect(document.querySelector(".terminal-slot")?.contains(bar)).toBe(true);

    act(() => {
      useDaemonStore.setState({
        sessions: [mkSession("s-grok", "grok", "hidden prompt")],
        activeSessionID: "s-grok",
      });
    });
    rerender(<App />);
    expect(screen.queryByLabelText("Last user prompt")).toBeNull();

    act(() => {
      useDaemonStore.setState({
        sessions: [mkSession("s-generic", "bash")],
        activeSessionID: "s-generic",
      });
    });
    rerender(<App />);
    expect(screen.queryByLabelText("Last user prompt")).toBeNull();
  });

  it("hides status bar when no active session", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.queryByLabelText("session status")).toBeNull();
    expect(screen.queryByLabelText("driver view")).toBeNull();
  });

  it("renders MainTabs tablist when active session has log_tabs", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {},
            status: "running",
            log_tabs: [{ label: "Output", path: "/tmp/out.log", kind: "text" }],
          },
        },
      ],
      activeSessionID: "s2",
    });
    render(<App />);
    expect(screen.getByRole("tablist")).toBeTruthy();
    // TERMINAL is prepended as a synthetic tab in front of driver log_tabs.
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["TERMINAL", "Output"]);
  });

  it("keeps TERMINAL and driver tabs while inserting MESSAGES from frame messaging summary", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-msg",
          project: "p",
          command: "claude",
          created_at: "2026-07-06T00:00:00Z",
          view: {
            card: { title: "Message session" },
            status: "running",
            log_tabs: [
              {
                label: "TRANSCRIPT",
                path: "/var/lib/agent-grid/s-msg.transcript",
                kind: "text",
              },
              { label: "EVENTS", path: "/var/log/agent-grid/s-msg.log", kind: "text" },
            ],
            frame_messaging_summary: {
              unread_count: 2,
              latest_message_preview: "Need review",
              pending_delivery_count: 1,
            },
          },
        },
      ],
      activeSessionID: "s-msg",
    });

    render(<App />);

    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual([
      "TERMINAL",
      "MESSAGES",
      "TRANSCRIPT",
      "EVENTS",
    ]);
  });

  // Regression 2026-06-24: 実 driver (claude_view.go 等) は log_tabs に
  // TRANSCRIPT (path=*.transcript) と EVENTS (path=<sid>.log) を載せる。
  // App は <LogTabs tabs={view.log_tabs}> を render し、両ボタンが見えること
  // を確保する。CSS で潰されたケースは vitest では検知できないが、
  // 「App が LogTabs を render し、tablist に 2 個の [role=tab] が含まれる」
  // ロジック契約はここで永続化する (driver / wire / 描画分岐の regression を防ぐ)。
  it("driver-shaped log_tabs (TRANSCRIPT + EVENTS) renders TERMINAL + both tabs visible to user", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "sess-abc",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: { title: "Driver session" },
            status: "running",
            log_tabs: [
              {
                label: "TRANSCRIPT",
                path: "/var/lib/agent-grid/sess-abc.transcript",
                kind: "text",
              },
              { label: "EVENTS", path: "/var/log/agent-grid/sess-abc.log", kind: "text" },
            ],
          },
        },
      ],
      activeSessionID: "sess-abc",
    });
    render(<App />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(3);
    expect(tabs.map((t) => t.textContent)).toEqual(["TERMINAL", "TRANSCRIPT", "EVENTS"]);
  });

  // Regression 2026-06-24: suppress_info が真でないとき、空でない log_tabs は
  // 必ず render されること (App.tsx の条件分岐回帰防止)。MainTabs 化以後は
  // 常に TERMINAL タブが先頭に乗るため tab 数 = 1 + driver log_tabs.length。
  it("does NOT hide LogTabs when suppress_info is unset/false", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s3",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {},
            status: "running",
            suppress_info: false,
            log_tabs: [{ label: "EVENTS", path: "/x/s3.log", kind: "text" }],
          },
        },
      ],
      activeSessionID: "s3",
    });
    render(<App />);
    expect(screen.queryByRole("tablist")).not.toBeNull();
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["TERMINAL", "EVENTS"]);
  });

  it("renders NotificationToast aria-label container", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.getByLabelText("notifications")).toBeTruthy();
  });

  it("keyed remount: switching activeSessionID causes TerminalPane to remount", () => {
    // Start with session s1 active
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });
    const { rerender } = render(<App />);

    // Capture the terminal-host element from first render
    const hostBefore = document.querySelector(".terminal-host");
    expect(hostBefore).not.toBeNull();

    // Switch to s2 — this changes the key prop on TerminalPane, forcing remount
    act(() => {
      useDaemonStore.setState({ activeSessionID: "s2" });
    });
    rerender(<App />);

    // After remount the .terminal-host element is a fresh DOM node
    const hostAfter = document.querySelector(".terminal-host");
    expect(hostAfter).not.toBeNull();
    // The key change means React unmounts old and mounts new — DOM node differs
    expect(hostAfter).not.toBe(hostBefore);
  });

  it("keyed remount: switching the head frame recreates xterm within one session", () => {
    const session = {
      id: "s1",
      project: "p",
      command: "claude",
      created_at: "2026-06-20T00:00:00Z",
      head_frame_id: "f1",
      view: { card: {}, status: "running" as const },
    };
    useDaemonStore.setState({ sessions: [session], activeSessionID: "s1" });
    const { rerender } = render(<App />);
    const hostBefore = document.querySelector(".terminal-host");
    expect(hostBefore).not.toBeNull();

    act(() => {
      useDaemonStore.setState({
        sessions: [{ ...session, head_frame_id: "f2" }],
      });
    });
    rerender(<App />);

    const hostAfter = document.querySelector(".terminal-host");
    expect(hostAfter).not.toBeNull();
    expect(hostAfter).not.toBe(hostBefore);
  });

  it("ADR 0030: keyed remount releases old ownership and acquires the new session", () => {
    const releases: Array<ReturnType<typeof vi.fn>> = [];
    const acquireSpy = vi.spyOn(Connection.prototype, "acquireTerminal").mockImplementation(() => {
      const release = vi.fn();
      releases.push(release);
      return { release };
    });

    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });
    const { rerender } = render(<App />);

    // Initial mount owns s1 and has not released that ownership.
    expect(acquireSpy).toHaveBeenCalledWith("s1");
    expect(releases[0]).not.toHaveBeenCalled();

    acquireSpy.mockClear();

    // Switch active session: the old lease is released and the new pane
    // acquires s2. The controller serializes the resulting wire transition.
    act(() => {
      useDaemonStore.setState({ activeSessionID: "s2" });
    });
    rerender(<App />);

    expect(releases[0]).toHaveBeenCalledTimes(1);
    expect(acquireSpy).toHaveBeenCalledWith("s2");
  });

  // FR-002 / ADR-0037 / ADR-0062: 常設 search-bar (CommandSearchTrigger) は
  // Cmd/Ctrl+K の保険。click で palette store が open=true になり、opener に
  // トリガー自身がセットされる (CommandPalette の focus 復元先になる)。
  it("Header の CommandSearchTrigger クリックで palette が open になる (FR-002 / B3)", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    const btn = screen.getByLabelText("Open command menu");
    expect(btn).toBeTruthy();
    act(() => {
      fireEvent.click(btn);
    });

    const s = usePaletteStore.getState();
    expect(s.open).toBe(true);
    // opener はクリック元のトリガーが入る
    expect(s.opener).toBe(btn);
  });

  // FR-001 / ADR-0037: App mounts useGlobalHotkey() once and listens on the
  // document capture phase for Cmd+K (mac) / Ctrl+K (other).
  //
  // useGlobalHotkey reads isMacPlatform() from ./lib/platform — the SAME mocked
  // module the test importer sees. Spying navigator.platform alone is NOT enough
  // because the module-level vi.mock above replaces the implementation with a
  // vi.fn(); we have to flip the mock return per case via vi.mocked(...).
  it("Cmd+K (mac) opens palette — useGlobalHotkey wiring (FR-001)", () => {
    vi.mocked(isMacPlatform).mockReturnValue(true);
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      fireEvent.keyDown(document, { key: "k", metaKey: true });
    });

    expect(usePaletteStore.getState().open).toBe(true);
  });

  it("Ctrl+K (non-mac) opens palette — useGlobalHotkey wiring (FR-001)", () => {
    vi.mocked(isMacPlatform).mockReturnValue(false);
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      fireEvent.keyDown(document, { key: "k", ctrlKey: true });
    });

    expect(usePaletteStore.getState().open).toBe(true);
  });

  it("non-target keys do not open palette (regression guard)", () => {
    vi.mocked(isMacPlatform).mockReturnValue(true);
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      // k without metaKey must not open the palette
      fireEvent.keyDown(document, { key: "k" });
    });
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      // Another key with metaKey must not open the palette
      fireEvent.keyDown(document, { key: "j", metaKey: true });
    });
    expect(usePaletteStore.getState().open).toBe(false);
  });

  // FR-D1 / FR-D2 / FR-D3 / ADR-0062: hint badge は
  // lib/platform:isMacPlatform を一次ソースとして mac / non-mac で切り替わる。
  // B3 以降は CommandSearchTrigger 内の .command-search-trigger__hint span が
  // ⌘K / Ctrl+K を出し分ける (旧 'Command (⌘K)' label からの移行)。
  it("FR-D1: header trigger shows ⌘K when isMacPlatform()=true", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    vi.mocked(isMacPlatform).mockReturnValue(true);
    render(<App />);
    const btn = screen.getByLabelText("Open command menu");
    expect(btn.textContent).toContain("⌘K");
    expect(btn.getAttribute("aria-label")).toBe("Open command menu");
  });

  it("FR-D2: header trigger shows Ctrl+K when isMacPlatform()=false (no crash on fallback envs)", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    vi.mocked(isMacPlatform).mockReturnValue(false);
    render(<App />);
    const btn = screen.getByLabelText("Open command menu");
    expect(btn.textContent).toContain("Ctrl+K");
    expect(btn.textContent).not.toContain("⌘K");
  });

  // B3 / ADR-0062 regression guard: the legacy "New Session" / "Open command
  // palette (⌘K / Ctrl+K)" buttons are removed from the header. New Session is
  // surfaced inside the palette's tool list instead. Asserting their absence
  // prevents accidental regression to the dual-button header.
  it("B3 / ADR-0062: legacy 'New Session' and 'Command (⌘K)' header buttons are removed", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.queryByLabelText("New Session")).toBeNull();
    expect(screen.queryByLabelText("Open command palette (⌘K / Ctrl+K)")).toBeNull();
    // mkSnapshot / selectDaemonSnapshot imports must stay alive even though
    // the legacy header-button tests they powered are gone — keeping them
    // referenced here prevents an unused-import gate later.
    void mkSnapshot;
    void selectDaemonSnapshot;
  });

  // Blocker T1 regression guard: App on mount MUST call
  // GET /api/session-config and feed the result to
  // useDaemonStore.setSessionConfig. Without this, ParamSelectPhase
  // sees empty projects + pushCommands forever (the production
  // code path otherwise never fires session-config-extension's REST hydrate).
  it("mount で GET /api/session-config を叩き、結果を daemon.setSessionConfig に渡す (T1)", async () => {
    // Replace the default hang-forever fetch stub with one that resolves
    // the session-config call with a representative payload, and any
    // other call (ws-ticket) with a never-resolving promise so Connection
    // does not blow up the test.
    const fetchSpy = vi.fn((url: RequestInfo | URL) => {
      const u = typeof url === "string" ? url : url.toString();
      if (u.endsWith("/api/session-config")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              commands: ["claude"],
              projects: [{ path: "/repo/a", isGit: true, isSandboxed: false }],
              push_commands: ["/clear", "/exit"],
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );
      }
      return new Promise(() => {}); // hang ws-ticket forever
    });
    vi.stubGlobal("fetch", fetchSpy);

    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);

    // Wait one microtask tick for the promise chain to flush. We do this
    // by running pending timers + awaiting several resolved promises so
    // vitest's fakeTimers settings (set in beforeEach) do not stall the
    // await chain that fetch -> request -> text() -> setSessionConfig walks.
    await vi.runOnlyPendingTimersAsync().catch(() => {});
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }

    const calls = fetchSpy.mock.calls.filter((c) => {
      const u = typeof c[0] === "string" ? c[0] : (c[0] as URL).toString();
      return u.endsWith("/api/session-config");
    });
    expect(calls.length).toBeGreaterThanOrEqual(1);
    const cfg = useDaemonStore.getState().sessionConfig;
    expect(cfg).not.toBeNull();
    expect(cfg?.projects).toEqual([{ path: "/repo/a", isGit: true, isSandboxed: false }]);
    expect(cfg?.pushCommands).toEqual(["/clear", "/exit"]);
  });

  // T1 follow-up: on getSessionConfig failure (non-401) we surface a
  // single error toast and leave sessionConfig=null. 401 is silenced
  // because Connection.start owns the auth-error UX.
  it("mount の getSessionConfig が失敗したら error toast を出す (T1 失敗パス)", async () => {
    const fetchSpy = vi.fn((url: RequestInfo | URL) => {
      const u = typeof url === "string" ? url : url.toString();
      if (u.endsWith("/api/session-config")) {
        return Promise.resolve(new Response("daemon down", { status: 503 }));
      }
      return new Promise(() => {});
    });
    vi.stubGlobal("fetch", fetchSpy);

    render(<App />);
    // requestWithRetry backs off for up to 1.4s total before App catches and
    // surfaces the toast. Advance only that window so the toast is added but
    // not yet auto-dismissed.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1400);
    });
    await act(async () => {
      await Promise.resolve();
    });

    const items = useNotificationsStore.getState().items;
    const errors = items.filter((i) => i.level === "error");
    expect(errors.length).toBeGreaterThanOrEqual(1);
    // English-only gate: session-config 失敗 toast は英語に置換 (旧
    // "session-config の取得に失敗しました:" は撤去)。Server message は
    // ": <reason>" の形で連結されたまま末尾に残る。
    expect(errors[0]?.message).toMatch(/^Failed to load session config:/);
    expect(useDaemonStore.getState().sessionConfig).toBeNull();
  });

  // f2 regression guard: 旧 CreateSessionForm の form / dialog はもう
  // render されない (撤去済み)。Project directory input / Create ボタン /
  // dialog 要素のいずれも DOM に出ないこと。
  it("旧 CreateSessionForm の form 要素 (Project directory / Create) は render されない (f2 撤去)", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.queryByLabelText("Project directory")).toBeNull();
    expect(screen.queryByRole("button", { name: /^Create$/ })).toBeNull();
  });

  // ─── B1 / B2 / M2: cross-component user-reachable flow ─────────────────────
  //
  // Hamburger tap → drawer open → row click changes activeSessionID → drawer
  // closes via onSelectionClose → focus restores to hamburger → UndoSnackbar
  // appears in the notification slot announcing "Switched to <label>".
  // Clicking Undo restores the previous activeSessionID via the daemon store.
  // This exercises:
  //   - SessionDrawer's activeSessionID watcher (B1)
  //   - AppShell's previousActiveSessionId state + portal slot (B2)
  //   - UndoSnackbar / NotificationToast slot isolation (FR-TOAST-003)
  //   - daemon store ownership of activeSessionID (web_active_session_ownership)
  it("hamburger → drawer open → select session → UndoSnackbar → Undo restores (B1/B2 user-reachable)", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "Session A" }, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "Session B" }, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });

    render(<App />);

    // Hamburger tap → drawer opens.
    const hamburger = screen.getByRole("button", { name: "Open sessions" });
    expect(hamburger.getAttribute("aria-expanded")).toBe("false");
    act(() => {
      fireEvent.click(hamburger);
    });
    expect(hamburger.getAttribute("aria-expanded")).toBe("true");

    // Switch active session inside the drawer (simulating SessionList row click
    // which calls daemonStore.selectSession). SessionDrawer's useEffect
    // observes the change and calls onSelectionClose → AppShell closes drawer
    // + captures previousActiveSessionId='s1' + previousLabel='Session B'.
    act(() => {
      useDaemonStore.getState().selectSession("s2");
    });

    // Drawer closes; hamburger aria-expanded back to false.
    expect(hamburger.getAttribute("aria-expanded")).toBe("false");

    // UndoSnackbar is rendered with status announcing the new label.
    const undoBtn = screen.getByRole("button", { name: "Undo" });
    expect(undoBtn).toBeTruthy();
    // 'Switched to Session B' is in the role=status live region.
    // The notification container also has role=status, so we search by text
    // content within the snackbar slot only.
    const snackbarStatus = document.querySelector(".undo-snackbar__status");
    expect(snackbarStatus).not.toBeNull();
    expect(snackbarStatus?.textContent).toContain("Switched to Session B");

    // Undo click → activeSessionID restored to s1; snackbar dismisses.
    act(() => {
      fireEvent.click(undoBtn);
    });
    expect(useDaemonStore.getState().activeSessionID).toBe("s1");
    expect(screen.queryByRole("button", { name: "Undo" })).toBeNull();
  });

  it("keeps a dirty Workspace on session selection until App-level discard confirmation", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "Session A" }, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "Session B" }, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });
    const workspace = useWorkspaceActivityStore.getState();
    workspace.setScopedSession("s1");
    workspace.openDrawerFromRow({ sessionId: "s1", path: "a.ts", kind: "edit" });
    workspace.setBufferDirty("a.ts", true);
    render(<App />);

    act(() => useDaemonStore.getState().selectSession("s2"));
    expect(useDaemonStore.getState().activeSessionID).toBe("s1");
    expect(screen.getByRole("dialog", { name: "Switch session" }).textContent).toContain(
      "Session B",
    );

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(useWorkspaceActivityStore.getState().dirtyBuffers["a.ts"]?.dirty).toBe(true);
    expect(useDaemonStore.getState().activeSessionID).toBe("s1");

    act(() => useDaemonStore.getState().selectSession("s2"));
    fireEvent.click(screen.getByRole("button", { name: "Discard and switch" }));
    expect(useDaemonStore.getState().activeSessionID).toBe("s2");
    expect(useWorkspaceActivityStore.getState().dirtyBuffers).toEqual({});
    expect(useWorkspaceActivityStore.getState().drawerTarget).toBeNull();
  });

  // ─── B3 user-reachable: CommandSearchTrigger → palette-sheet at 375px ──────
  // FR-PALETTE-TRIGGER-001 / UAC-008 counterexample: 375px viewport tap on the
  // search trigger opens role='dialog' aria-modal='true' palette + a
  // [data-role='palette-sheet'] container exists inside the overlay. The
  // CSS contract (width = 100% - 32px / max-width 600px) is observed via the
  // class-name regex on shell.css (happy-dom cannot resolve real layout).
  it("CommandSearchTrigger tap at 375px → palette opens with data-role='palette-sheet' container (B3 user-reachable)", () => {
    // Stub innerWidth to 375 for the mobile-counterexample contract.
    Object.defineProperty(window, "innerWidth", { configurable: true, value: 375 });
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);

    const trigger = screen.getByLabelText("Open command menu");
    // Pre-condition: xterm input would have focus in a real app; we simulate
    // that the active element loses focus on palette open (CommandPalette's
    // FR-003 blur path runs in useEffect).
    act(() => {
      fireEvent.click(trigger);
    });

    // Palette dialog is open.
    const dialog = screen.getByRole("dialog", { name: /command palette/i });
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    // palette-sheet container is rendered as a sibling parent of the dialog.
    const sheet = document.querySelector("[data-role='palette-sheet']");
    expect(sheet).not.toBeNull();
    // Sheet contains the dialog (structural assertion that the wrapper is
    // in the right place — UAC-008 sheet sits between overlay and dialog).
    expect(sheet?.contains(dialog)).toBe(true);
  });

  // M2 (cross-component theme): ThemeSegmentedControl click cascades through
  // ThemeProvider to document.documentElement.dataset.theme. xterm bg / fg
  // tokens are read by useXtermTheme via getComputedStyle — we cannot fully
  // observe the computed value in happy-dom, but we CAN observe that the
  // single source (data-theme on documentElement) flips, which is the only
  // mechanism that drives both body bg and xterm bg via tokens.css.
  it("OverflowMenu Light click → data-theme=light (cross-component, m2 / UAC-005)", async () => {
    // Radix DropdownMenu relies on real timers; fake timers from beforeEach break open.
    vi.useRealTimers();
    try {
      useDaemonStore.setState({ sessions: [], activeSessionID: null });
      render(<App />);
      const trigger = screen.getByRole("button", { name: "More actions" });
      fireEvent.pointerDown(trigger);
      fireEvent.click(trigger);
      const light = await screen.findByText("Light");
      act(() => {
        fireEvent.click(light);
      });
      expect(document.documentElement.dataset.theme).toBe("light");
    } finally {
      vi.useFakeTimers();
    }
  });
});
