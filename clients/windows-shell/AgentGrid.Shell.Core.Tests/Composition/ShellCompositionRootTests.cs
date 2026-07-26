using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.Core.SessionIdentity;
using AgentGrid.Shell.Core.WorkspaceLauncher;
using AgentGrid.Shell.Platform.Interop;

namespace AgentGrid.Shell.Core.Tests.Composition;

public class ShellCompositionRootTests
{
    [Fact]
    public async Task Build_and_start_with_fakes_reaches_healthy()
    {
        var http = new ScriptedHttp(() =>
            new HttpResponseMessage(System.Net.HttpStatusCode.OK)
            {
                Content = new StringContent("[]", System.Text.Encoding.UTF8, "application/json"),
            });
        using var httpClient = new HttpClient(http) { BaseAddress = new Uri("http://127.0.0.1:9") };

        var root = ShellCompositionRoot.Build(new ShellHostOptions
        {
            GatewayBaseUri = new Uri("http://127.0.0.1:9"),
            TokenPath = Path.Combine(Path.GetTempPath(), $"ag-t-{Guid.NewGuid():N}"),
            TokenSource = new StaticToken("tok"),
            HttpClient = httpClient,
            Win32 = new NullWin32InteropService(),
            WorkspaceControlClient = new AlwaysOkPipe(),
            WorkspaceProcessLauncher = new NoopProcess(),
            WebSocketFactory = new ClosedWsFactory(),
            QuitApplication = () => { },
        });

        await using (root)
        {
            // Probe succeeds via scripted HTTP.
            await root.Supervisor.StartAsync();
            Assert.True(root.Supervisor.Snapshot.IsConnected);
            Assert.Equal(TrayIconKind.Connected, TrayAppearanceMapper.From(root.Supervisor.Snapshot).Kind);
            Assert.NotNull(root.CurrentGlance());
        }
    }

    /// <summary>
    /// RED (debug-run-shell-zero-sessions round 2): a SnapshotChanged subscriber
    /// that is only safe to invoke on the thread that constructed the root (e.g.
    /// a WinUI TrayIconController touching a DependencyObject — TrayIconController.cs:57,
    /// wired via ShellFleet's `root.Supervisor.SnapshotChanged += _ =>
    /// HealthChanged?.Invoke(...)`) throws when DaemonSupervisorService.Raise()
    /// fires from the background-thread continuation that follows the health
    /// probe's real async I/O gap. That throw currently propagates all the way
    /// up through DaemonSupervisorService.ApplyAsync → DaemonSupervisorService
    /// .StartAsync → ShellCompositionRoot.StartAsync, aborting it BEFORE the
    /// next line (WsSession.Start()) ever runs — so the WS session is never
    /// started and the shell can never learn about sessions, independent of
    /// the k="h" hello-frame parser fix. StartAsync (and therefore
    /// WsSession.Start()) must be robust to a subscriber failing off-thread.
    /// </summary>
    [Fact]
    public async Task StartAsync_starts_ws_session_even_if_a_snapshot_subscriber_throws_off_thread()
    {
        var callingThread = Environment.CurrentManagedThreadId;
        var http = new AsyncScriptedHttp(async request =>
        {
            // Force a genuine async gap (like real network I/O) so the
            // continuation after `await _probe.ProbeAsync()` resumes on a
            // thread-pool thread instead of running inline on the caller's
            // thread — this is what actually happens against a real gateway.
            await Task.Delay(10);

            // MintWsTicketAsync (called by SupervisionWsSession.RunOnceAsync,
            // reached once StartAsync's ApplyAsync/Raise chain no longer aborts
            // on the throwing subscriber) needs a real ticket envelope; the
            // health-probe's GET /api/sessions only cares about the status code.
            if (request.RequestUri?.AbsolutePath == "/api/ws-ticket")
            {
                return new HttpResponseMessage(System.Net.HttpStatusCode.OK)
                {
                    Content = new StringContent(
                        """{"ticket":"t","client_instance_id":"ci"}""",
                        System.Text.Encoding.UTF8,
                        "application/json"),
                };
            }

            return new HttpResponseMessage(System.Net.HttpStatusCode.OK)
            {
                Content = new StringContent("[]", System.Text.Encoding.UTF8, "application/json"),
            };
        });
        using var httpClient = new HttpClient(http) { BaseAddress = new Uri("http://127.0.0.1:9") };

        var wsFactory = new RecordingWsFactory();
        var root = ShellCompositionRoot.Build(new ShellHostOptions
        {
            GatewayBaseUri = new Uri("http://127.0.0.1:9"),
            TokenPath = Path.Combine(Path.GetTempPath(), $"ag-t-{Guid.NewGuid():N}"),
            TokenSource = new StaticToken("tok"),
            HttpClient = httpClient,
            Win32 = new NullWin32InteropService(),
            WorkspaceControlClient = new AlwaysOkPipe(),
            WorkspaceProcessLauncher = new NoopProcess(),
            WebSocketFactory = wsFactory,
            QuitApplication = () => { },
        });

        // Mirrors ShellFleet's constructor wiring:
        //   root.Supervisor.SnapshotChanged += _ => HealthChanged?.Invoke(...);
        //   (TrayIconController.Apply then sets a WinUI DependencyProperty)
        root.Supervisor.SnapshotChanged += _ =>
        {
            if (Environment.CurrentManagedThreadId != callingThread)
                throw new InvalidOperationException(
                    "simulated cross-thread DependencyObject write (TrayIconController.Apply)");
        };

        // Companion assertion (critique-2 finding 1 / finding-red-test-does-not-
        // pin-subscriber-still-notified): a second, non-throwing subscriber must
        // still be invoked. This rejects a "fix" that silences the event
        // entirely (removing the ShellFleet forwarding lambda, or swallowing
        // SnapshotChanged without invoking anyone) — that would also make
        // StartAsync succeed and the WS session start, but no legitimate
        // subscriber (throwing or not) would ever be notified again.
        var wellBehavedSubscriberCalls = 0;
        root.Supervisor.SnapshotChanged += _ =>
            Interlocked.Increment(ref wellBehavedSubscriberCalls);

        await using (root)
        {
            await root.StartAsync();

            Assert.True(
                await wsFactory.CreatedWithin(TimeSpan.FromSeconds(2)),
                "WsSession.Start() must run the WS receive loop even when a " +
                "SnapshotChanged subscriber throws off-thread.");

            Assert.True(
                Volatile.Read(ref wellBehavedSubscriberCalls) > 0,
                "A non-throwing SnapshotChanged subscriber must still be " +
                "notified — the throwing subscriber must be isolated, not " +
                "have the event source silenced for everyone.");
        }
    }

    /// <summary>
    /// RED (debug-run-workspace-not-activated round 2, finding-red-punt-underinvests-contract-seam,
    /// severity: blocker). Responsibility pin, not an OS-fidelity check: Shell already owns
    /// SetForegroundWindow / AllowSetForegroundWindow (IWin32InteropService, wired into this same
    /// composition root as <see cref="ShellCompositionRoot.Win32"/>) and already uses them for
    /// JumpBack/Engage (JumpBackService.cs, EngageFocusService.cs) -- but nothing wires that
    /// capability to WorkspaceLauncherService.OpenSessionAsync's pipe-success path
    /// (debug-run-workspace-not-activated investigation.json rca.why_chain Why2). Per the bug
    /// report, this only manifests on the *pipe* path (workspace already running); the spawn path
    /// activates for free via Windows' CreateProcess foreground grace period, so it is not pinned
    /// here.
    ///
    /// This test intentionally does not assert WHICH Win32 primitive is used
    /// (SetForegroundWindow(hwnd) -- fix option (b) -- vs AllowSetForegroundWindow(pid) -- fix
    /// option (a); still an open design choice) or which HWND/PID is passed (PID capture is a
    /// separate unresolved gap in ProcessWorkspaceLauncher.DefaultStartAsync, which discards
    /// Process.Start's return value -- see NamedPipeWorkspaceControlClient.cs). It pins only that
    /// *some* cross-process foreground-transfer call happens by the time a successful pipe
    /// openSession returns, so a future fix cannot silently regress to "pipe replies ok but nobody
    /// asked Windows for permission to foreground the target". Real-OS fidelity (does Windows
    /// actually honor the call against the concrete FocusTransferGuard-approved caller) is
    /// deliberately out of scope for a unit test -- FakeWin32/RecordingWin32-style doubles return
    /// unconditional success (see JumpBackServiceTests.cs's FakeWin32.SetForegroundWindow/
    /// AllowSetForegroundWindow) and cannot model Windows' foreground-lock arbitration, so fidelity
    /// is covered separately by the T3 FlaUI/UIA harness (AG_E2E_WINUI_UI=1,
    /// docs/note/note-20260724-windows-shell-ui-e2e-authoring.md) against the real workspace/shell
    /// processes, per investigation.json red_evidence.
    /// </summary>
    [Fact]
    public async Task OpenSession_over_pipe_success_path_must_delegate_foreground_to_workspace()
    {
        var win32 = new RecordingWin32();
        var root = ShellCompositionRoot.Build(new ShellHostOptions
        {
            GatewayBaseUri = new Uri("http://127.0.0.1:9"),
            TokenPath = Path.Combine(Path.GetTempPath(), $"ag-t-{Guid.NewGuid():N}"),
            TokenSource = new StaticToken("tok"),
            Win32 = win32,
            // AlwaysOkPipe simulates the pipe-success path: workspace is already running and
            // replies {"ok":true} without ever being spawned -- exactly the reproduction in
            // bug-report.json (second SessionChip click after focus moved elsewhere).
            WorkspaceControlClient = new AlwaysOkPipe(),
            WorkspaceProcessLauncher = new NoopProcess(),
            WebSocketFactory = new ClosedWsFactory(),
            QuitApplication = () => { },
        });

        await using (root)
        {
            var reply = await root.WorkspaceLauncher.OpenSessionAsync(new ServerSessionId("local", "s1"));

            Assert.True(reply.Ok);
            Assert.True(
                win32.ForegroundTransferCalls > 0,
                "pipe openSession replied ok, but Shell never called SetForegroundWindow or " +
                "AllowSetForegroundWindow on the (already-running) workspace target -- Electron's " +
                "own BrowserWindow.focus() will be demoted to a taskbar flash by Windows' " +
                "foreground-lock restriction because the workspace process is not the caller's " +
                "foreground process at click time (debug-run-workspace-not-activated).");
        }
    }

    /// <summary>
    /// Invocation-recording double for the responsibility pin above -- deliberately does NOT
    /// simulate Windows' foreground-lock arbitration (both foreground-transfer methods return
    /// unconditional success, mirroring JumpBackServiceTests.cs's FakeWin32). It only records
    /// whether Shell asked Win32 to transfer foreground at all, and to which target.
    /// </summary>
    private sealed class RecordingWin32 : IWin32InteropService
    {
        public int ForegroundTransferCalls { get; private set; }
        public nint? LastSetForegroundHwnd { get; private set; }
        public int? LastAllowSetForegroundPid { get; private set; }

        public nint GetForegroundWindow() => 0;
        public bool IsWindow(nint hwnd) => false;
        public bool SetForegroundWindow(nint hwnd)
        {
            ForegroundTransferCalls++;
            LastSetForegroundHwnd = hwnd;
            return true;
        }
        public bool AllowSetForegroundWindow(int processId)
        {
            ForegroundTransferCalls++;
            LastAllowSetForegroundPid = processId;
            return true;
        }
        public uint GetWindowThreadProcessId(nint hwnd, out uint processId)
        {
            processId = 0;
            return 0;
        }
        public bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach) => false;
        public bool SetNoActivate(nint hwnd, bool noActivate) => false;
        public string? GetWindowProcessName(nint hwnd) => null;
        public string? GetWindowTitle(nint hwnd) => null;
        public IReadOnlyList<WindowInfo> EnumerateWindows() => Array.Empty<WindowInfo>();
        public bool IsSessionLocked() => false;
        public bool IsDoNotDisturb() => false;
        public bool TryGetNotificationsAllowed(out bool allowed)
        {
            allowed = true;
            return true;
        }
    }

    private sealed class StaticToken : ITokenSource
    {
        private readonly string _t;
        public StaticToken(string t) => _t = t;
        public Task<string> ReadFreshAsync(CancellationToken ct = default) => Task.FromResult(_t);
    }

    private sealed class ScriptedHttp : HttpMessageHandler
    {
        private readonly Func<HttpResponseMessage> _fn;
        public ScriptedHttp(Func<HttpResponseMessage> fn) => _fn = fn;
        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken) =>
            Task.FromResult(_fn());
    }

    /// <summary>Like ScriptedHttp but completes via a real async gap (Task.Delay),
    /// so continuations after the await resume on a thread-pool thread instead
    /// of running inline — matching real network I/O.</summary>
    private sealed class AsyncScriptedHttp : HttpMessageHandler
    {
        private readonly Func<HttpRequestMessage, Task<HttpResponseMessage>> _fn;
        public AsyncScriptedHttp(Func<HttpRequestMessage, Task<HttpResponseMessage>> fn) => _fn = fn;
        protected override async Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken) =>
            await _fn(request).ConfigureAwait(false);
    }

    private sealed class RecordingWsFactory : IWebSocketTransportFactory
    {
        private readonly TaskCompletionSource _created =
            new(TaskCreationOptions.RunContinuationsAsynchronously);

        public IWebSocketTransport Create()
        {
            _created.TrySetResult();
            return new ClosedWs();
        }

        public async Task<bool> CreatedWithin(TimeSpan timeout)
        {
            var completed = await Task.WhenAny(_created.Task, Task.Delay(timeout)).ConfigureAwait(false);
            return completed == _created.Task;
        }
    }

    private sealed class AlwaysOkPipe : IWorkspaceControlClient
    {
        /// <summary>
        /// Arbitrary non-zero sentinel PID. This fake models the pipe-success path (workspace
        /// already running, connect always succeeds), so a concrete server process id is always
        /// "known" by the time SendAsync would write the envelope -- see
        /// NamedPipeWorkspaceControlClient's real GetNamedPipeServerProcessId-backed
        /// implementation, not exercised by this composition-root-level fake.
        /// </summary>
        private const int FakeServerProcessId = 4321;

        public Task<ControlReply> SendAsync(
            ControlEnvelope envelope,
            CancellationToken ct = default,
            Action<int>? onConnectedServerProcessId = default)
        {
            onConnectedServerProcessId?.Invoke(FakeServerProcessId);
            return Task.FromResult(ControlReply.Success());
        }
    }

    private sealed class NoopProcess : IWorkspaceProcessLauncher
    {
        public Task SpawnAsync(CancellationToken ct = default) => Task.CompletedTask;
    }

    private sealed class ClosedWsFactory : IWebSocketTransportFactory
    {
        public IWebSocketTransport Create() => new ClosedWs();
    }

    private sealed class ClosedWs : IWebSocketTransport
    {
        public Task ConnectAsync(Uri uri, CancellationToken ct = default) => Task.CompletedTask;
        public Task<string?> ReceiveTextAsync(CancellationToken ct = default) =>
            Task.FromResult<string?>(null);
        public Task CloseAsync(CancellationToken ct = default) => Task.CompletedTask;
        public ValueTask DisposeAsync() => ValueTask.CompletedTask;
    }
}
