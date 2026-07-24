using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.Configuration;
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
    private ShellFleet? _root;
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
        var configDirectory = DesktopConfigLoader.ResolveConfigDirectory(
            Environment.GetCommandLineArgs().Skip(1));
        var config = DesktopConfigLoader.LoadOrCreate(configDirectory);
        _root = ShellFleet.Build(config, quit);
        await _root.StartAsync();

        _panel = new PanelWindow(_root);
        // Keep a window in the process; show glance so launch is visible (tray alone is easy to miss).
        _hidden = _panel;
        _toasts = new AppNotificationToastService(_root, () => _panel);
        try
        {
            _toasts.Register();
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"toast register: {ex}");
        }

        try
        {
            _tray = new TrayIconController(_root, _panel, quit);
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"tray create: {ex}");
        }

        _panel.ShowGlance();

        // Health tick
        _ = Task.Run(async () =>
        {
            using var timer = new PeriodicTimer(
                TimeSpan.FromSeconds(config.Shell.HealthPollIntervalSeconds));
            while (await timer.WaitForNextTickAsync())
            {
                try
                {
                    await _root.HealthTickAsync();
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

}

/// <summary>Explicit main: capture WASDK bootstrap errors before XAML starts.</summary>
public static class Program
{
    [STAThread]
    public static int Main(string[] args)
    {
        // Structured capture — do not rely on auto MessageBox text (not on stdout).
        if (!Host.WasdkBootstrapHost.TryStart(out var hr, out var report))
        {
            // Automated smoke sets AG_WINUI_NO_MSGBOX=1 so the process exits
            // with the HRESULT and launch-smoke can read the log without hanging.
            var noMsg = Environment.GetEnvironmentVariable("AG_WINUI_NO_MSGBOX");
            if (noMsg is not ("1" or "true" or "TRUE"))
            {
                _ = NativeMessageBox(report, "Agent Grid Shell — startup failed");
            }
            return hr != 0 ? hr : 1;
        }

        WinRT.ComWrappersSupport.InitializeComWrappers();
        Application.Start(p =>
        {
            var context = new Microsoft.UI.Dispatching.DispatcherQueueSynchronizationContext(
                Microsoft.UI.Dispatching.DispatcherQueue.GetForCurrentThread());
            SynchronizationContext.SetSynchronizationContext(context);
            _ = new App();
        });
        return 0;
    }

    [System.Runtime.InteropServices.DllImport("user32.dll", CharSet = System.Runtime.InteropServices.CharSet.Unicode)]
    private static extern int MessageBoxW(nint hWnd, string text, string caption, uint type);

    private static int NativeMessageBox(string text, string caption) =>
        MessageBoxW(0, text, caption, 0x00000010 /* MB_ICONERROR */);
}
