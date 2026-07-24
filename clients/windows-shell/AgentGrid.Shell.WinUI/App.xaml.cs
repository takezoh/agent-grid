using AgentGrid.Shell.Composition;
using AgentGrid.Shell.WinUI.Panel;
using AgentGrid.Shell.WinUI.Toast;
using AgentGrid.Shell.WinUI.Tray;
using Microsoft.UI.Xaml;
using Microsoft.Windows.AppLifecycle;

namespace AgentGrid.Shell.WinUI;

/// <summary>
/// WinUI 3 unpackaged shell host: tray + panel glance + AppNotification.
/// Composition root is framework-agnostic; this class only owns UI lifetime.
/// </summary>
public partial class App : Application
{
    private ShellCompositionRoot? _root;
    private TrayIconController? _tray;
    private PanelWindow? _panel;
    private AppNotificationToastService? _toasts;
    private Window? _hidden; // keeps message pump alive with no visible window

    public App()
    {
        InitializeComponent();
        UnhandledException += (_, e) =>
        {
            System.Diagnostics.Debug.WriteLine($"WinUI unhandled: {e.Message}");
            e.Handled = true;
        };
    }

    protected override async void OnLaunched(LaunchActivatedEventArgs args)
    {
        // Single-instance: deep-link re-activation routes to existing process.
        var mainInstance = AppInstance.FindOrRegisterForKey("agent-grid-shell");
        if (!mainInstance.IsCurrent)
        {
            var activated = AppInstance.GetCurrent().GetActivatedEventArgs();
            await mainInstance.RedirectActivationToAsync(activated);
            System.Diagnostics.Process.GetCurrentProcess().Kill();
            return;
        }
        mainInstance.Activated += OnRedirectedActivation;

        Action quit = () =>
        {
            _tray?.Dispose();
            _root?.DisposeAsync().AsTask().GetAwaiter().GetResult();
            Exit();
        };
        var opts = ShellHostOptionsFromEnvironment(quit);

        _root = ShellCompositionRoot.Build(opts);
        await _root.StartAsync();

        _hidden = new Window(); // never Activate — process stays for tray
        _panel = new PanelWindow(_root);
        _toasts = new AppNotificationToastService(_root, () => _panel);
        _toasts.Register();
        _tray = new TrayIconController(_root, _panel, quit);
        // Health tick
        _ = Task.Run(async () =>
        {
            using var timer = new PeriodicTimer(TimeSpan.FromSeconds(5));
            while (await timer.WaitForNextTickAsync())
            {
                try
                {
                    await _root.Supervisor.HealthTickAsync();
                }
                catch
                {
                    /* tray reflects degraded */
                }
            }
        });

        // Handle argv deep link on cold start
        var deep = Environment.GetCommandLineArgs()
            .FirstOrDefault(a => a.StartsWith("agent-grid://", StringComparison.OrdinalIgnoreCase));
        if (deep is not null)
            await _panel.HandleDeepLinkAsync(deep);
    }

    private void OnRedirectedActivation(object? sender, AppActivationArguments e)
    {
        if (e.Data is Windows.ApplicationModel.Activation.ProtocolActivatedEventArgs protocol)
        {
            var uri = protocol.Uri?.AbsoluteUri;
            if (uri is not null && _panel is not null)
                _ = _panel.HandleDeepLinkAsync(uri);
        }
    }

    private static ShellHostOptions ShellHostOptionsFromEnvironment(Action quit)
    {
        var local = Environment.GetEnvironmentVariable("LOCALAPPDATA")
                    ?? Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        var port = 8443;
        if (int.TryParse(Environment.GetEnvironmentVariable("AG_GATEWAY_PORT"), out var p))
            port = p;
        // e2e against scripts/run-dev.sh: AG_NO_AUTH=1 (default for local loopback e2e).
        var noAuth = IsTruthy(Environment.GetEnvironmentVariable("AG_NO_AUTH"));
        return new ShellHostOptions
        {
            GatewayBaseUri = new Uri(
                Environment.GetEnvironmentVariable("AG_GATEWAY_URL") ?? $"http://127.0.0.1:{port}"),
            TokenPath = Environment.GetEnvironmentVariable("AG_TOKEN_PATH")
                        ?? Path.Combine(local, "agent-grid", "gateway-token"),
            NoAuth = noAuth,
            TokenPathInWsl = Environment.GetEnvironmentVariable("AG_TOKEN_PATH_WSL")
                             ?? "~/.agent-grid/gateway-token",
            WslDistro = Environment.GetEnvironmentVariable("AG_WSL_DISTRO") ?? "Ubuntu-22.04",
            ServerPathInWsl = Environment.GetEnvironmentVariable("AG_SERVER_PATH")
                              ?? "/workspace/agent-grid/server",
            GatewayPort = port,
            WorkspaceExePath = Environment.GetEnvironmentVariable("AG_WORKSPACE_EXE")
                               ?? "agent-grid-workspace",
            QuitApplication = quit,
        };
    }

    private static bool IsTruthy(string? v) =>
        v is "1" or "true" or "TRUE" or "yes" or "YES";
}

/// <summary>Explicit main so we can bootstrap WinRT before Application.Start.</summary>
public static class Program
{
    [STAThread]
    public static void Main(string[] args)
    {
        WinRT.ComWrappersSupport.InitializeComWrappers();
        Application.Start(_ => new App());
    }
}
