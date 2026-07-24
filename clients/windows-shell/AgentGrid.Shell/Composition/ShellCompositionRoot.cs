using AgentGrid.Shell.Core.DaemonSupervisor;
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
            ?? (opts.NoAuth
                ? new NoAuthTokenSource()
                : new FileTokenSource(opts.TokenPath));
        var gateway = new ShellGatewayClient(opts.GatewayBaseUri, tokens, opts.HttpClient);
        var supervision = new ApprovalSubmissionService(gateway);
        var ws = new SupervisionWsSession(gateway, supervision, opts.WebSocketFactory);

        IDaemonRunner runner = opts.DaemonRunner
            ?? ProcessRunner.CreateWslRunner(
                opts.WslDistro,
                opts.ServerPathInWsl,
                opts.GatewayPort,
                opts.TokenPathInWsl);

        var supervisor = new DaemonSupervisorService(
            runner,
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
        var workspace = new WorkspaceLauncherService(pipe, process);

        Action quit = opts.QuitApplication ?? (() => { });
        var menu = new ShellMenuHandlers(supervisor, quit);

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
    public required string TokenPath { get; init; }
    /// <summary>
    /// Loopback e2e against scripts/run-dev.sh (-no-auth). Skips bearer file.
    /// </summary>
    public bool NoAuth { get; init; }
    public string TokenPathInWsl { get; init; } = "~/.agent-grid/gateway-token";
    public string WslDistro { get; init; } = "Ubuntu-22.04";
    public string ServerPathInWsl { get; init; } = "~/agent-grid/server";
    public int GatewayPort { get; init; } = 8443;
    public string WorkspaceExePath { get; init; } = "agent-grid-workspace";
    public string[] WorkspaceArguments { get; init; } = Array.Empty<string>();
    public string? WorkspaceControlPath { get; init; }
    public ITokenSource? TokenSource { get; init; }
    public HttpClient? HttpClient { get; init; }
    public IWebSocketTransportFactory? WebSocketFactory { get; init; }
    public IDaemonRunner? DaemonRunner { get; init; }
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
