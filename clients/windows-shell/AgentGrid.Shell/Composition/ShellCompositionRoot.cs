using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.Configuration;
using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Core.WorkspaceLauncher;
using AgentGrid.Shell.Menu;
using AgentGrid.Shell.Platform.Engage;
using AgentGrid.Shell.Platform.Interop;
using AgentGrid.Shell.Platform.JumpBack;
using AgentGrid.Shell.Platform.Toast;
using AgentGrid.Shell.TrayIcon;

namespace AgentGrid.Shell.Composition;

/// <summary>
/// Wires Shell.Core + Platform + UI adapters. UI toolkit (WinUI/WPF) hosts this
/// root; pure logic stays framework-agnostic (implementation_decisions_remaining:
/// shell-host-framework-choice).
/// </summary>
public sealed class ShellCompositionRoot : IAsyncDisposable
{
    public required DaemonSupervisorService Supervisor { get; init; }
    public required ShellGatewayClient Gateway { get; init; }
    public required ApprovalSubmissionService Supervision { get; init; }
    public required SupervisionWsSession WsSession { get; init; }
    public required WorkspaceLauncherService WorkspaceLauncher { get; init; }
    public required ShellMenuHandlers Menu { get; init; }
    public required JumpBackService JumpBack { get; init; }
    public required EngageFocusService Engage { get; init; }
    public required ToastDecisionService ToastDecisions { get; init; }
    public required IWin32InteropService Win32 { get; init; }

    public TrayAppearance CurrentTrayAppearance() =>
        TrayAppearanceMapper.From(Supervisor.Snapshot);

    public PanelGlanceView CurrentGlance() =>
        PanelGlanceView.From(Supervision.Snapshot);

    public static ShellCompositionRoot Build(ShellHostOptions opts)
    {
        ArgumentNullException.ThrowIfNull(opts);

        ITokenSource tokens = opts.TokenSource
            ?? (opts.Auth.Mode == ServerAuthMode.None
                ? new NoAuthTokenSource()
                : new PersistedTokenSource(opts.Auth.Token!));
        var gateway = new ShellGatewayClient(opts.GatewayBaseUri, tokens, opts.HttpClient);
        var supervision = new ApprovalSubmissionService(gateway);
        var ws = new SupervisionWsSession(gateway, supervision, opts.WebSocketFactory);

        var supervisor = new DaemonSupervisorService(
            new GatewayHealthProbe(gateway),
            resubscriber: ws);

        IWin32InteropService win32 = opts.Win32
            ?? (OperatingSystem.IsWindows()
                ? new Win32InteropService()
                : new NullWin32InteropService());

        var jump = new JumpBackService(win32);
        var engage = new EngageFocusService(win32);
        var toast = new ToastDecisionService(new PanelWatchedPredicate(win32));

        IWorkspaceControlClient pipe = opts.WorkspaceControlClient
            ?? new NamedPipeWorkspaceControlClient(opts.WorkspaceControlPath);
        IWorkspaceProcessLauncher process = opts.WorkspaceProcessLauncher
            ?? new ProcessWorkspaceLauncher(opts.WorkspaceExePath, opts.WorkspaceArguments);
        // debug-run-workspace-not-activated: decision authority for cross-process foreground
        // transfer stays in this composition layer (Core cannot reference IWin32InteropService --
        // Platform references Core, not the reverse). SessionChip clicks / deep-link opens reach
        // WorkspaceLauncherService.OpenSessionAsync on the pipe-success path (workspace already
        // running); without this, Electron's own focus() is demoted to a taskbar flash by
        // Windows' foreground-lock restriction. Classified under FocusTransferGuard.ExplicitUser
        // (see FocusTransferGuard.cs) -- reached only from explicit user actions in-process
        // (SessionChip / pending-session open / WinUI deep link), not the headless Host deep-link
        // path (Program.cs), which is out of scope here (separate issue candidate).
        var workspace = new WorkspaceLauncherService(
            pipe,
            process,
            onForegroundTransferEligible: pid => win32.AllowSetForegroundWindow(pid));

        Action quit = opts.QuitApplication ?? (() => { });
        var menu = new ShellMenuHandlers(quit);

        return new ShellCompositionRoot
        {
            Supervisor = supervisor,
            Gateway = gateway,
            Supervision = supervision,
            WsSession = ws,
            WorkspaceLauncher = workspace,
            Menu = menu,
            JumpBack = jump,
            Engage = engage,
            ToastDecisions = toast,
            Win32 = win32,
        };
    }

    public async Task StartAsync(CancellationToken ct = default)
    {
        await Supervisor.StartAsync(ct).ConfigureAwait(false);
        WsSession.Start();
    }

    public async ValueTask DisposeAsync()
    {
        await WsSession.DisposeAsync().ConfigureAwait(false);
        Gateway.Dispose();
    }
}

public sealed class ShellHostOptions
{
    public required Uri GatewayBaseUri { get; init; }
    public required ServerAuthConfig Auth { get; init; }
    public string WorkspaceExePath { get; init; } = "agent-grid-workspace";
    public string[] WorkspaceArguments { get; init; } = Array.Empty<string>();
    public string? WorkspaceControlPath { get; init; }
    public ITokenSource? TokenSource { get; init; }
    public HttpClient? HttpClient { get; init; }
    public IWebSocketTransportFactory? WebSocketFactory { get; init; }
    public IWin32InteropService? Win32 { get; init; }
    public IWorkspaceControlClient? WorkspaceControlClient { get; init; }
    public IWorkspaceProcessLauncher? WorkspaceProcessLauncher { get; init; }
    public Action? QuitApplication { get; init; }
}

/// <summary>No-op Win32 for non-Windows unit tests / WSL-only CI.</summary>
public sealed class NullWin32InteropService : IWin32InteropService
{
    public nint GetForegroundWindow() => nint.Zero;
    public bool IsWindow(nint hwnd) => false;
    public bool SetForegroundWindow(nint hwnd) => false;
    public bool AllowSetForegroundWindow(int processId) => false;
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
