using System.Diagnostics;
using FlaUI.Core;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Capturing;
using FlaUI.Core.Tools;
using FlaUI.UIA3;

namespace AgentGrid.Shell.WinUI.UiTests.E2E;

/// <summary>
/// Lazy per-class UI session: launches the self-contained WinUI exe under UIA
/// on first access (so skipped runs never start the app) and kills it on dispose.
///
/// Pre-existing shell instances are terminated first — the app is
/// single-instance (AppInstance redirect) and would kill our freshly
/// launched child otherwise.
/// </summary>
public sealed class PanelUiSession : IDisposable
{
    private readonly object _gate = new();
    private Application? _app;
    private UIA3Automation? _automation;
    private Window? _window;
    private string? _configDirectory;

    public Window Window
    {
        get
        {
            lock (_gate)
            {
                EnsureStarted();
                return _window!;
            }
        }
    }

    /// <summary>Find by AutomationId with retry (UIA tree populates async in WinUI 3).</summary>
    public AutomationElement Find(string automationId, TimeSpan? timeout = null)
    {
        var window = Window;
        var found = Retry.WhileNull(
            () => window.FindFirstDescendant(cf => cf.ByAutomationId(automationId)),
            timeout ?? TimeSpan.FromSeconds(10),
            interval: TimeSpan.FromMilliseconds(250)).Result;
        if (found is null)
        {
            var shot = TryCaptureWindow($"missing-{automationId}");
            throw new InvalidOperationException(
                $"AutomationId '{automationId}' not found in panel window" +
                (shot is null ? "" : $" (screenshot: {shot})"));
        }
        return found;
    }

    /// <summary>Best-effort screenshot for failure diagnostics; returns file path.</summary>
    public string? TryCaptureWindow(string name)
    {
        try
        {
            var dir = Path.Combine(LocalAppData(), "agent-grid", "logs");
            Directory.CreateDirectory(dir);
            var path = Path.Combine(dir, $"ui-e2e-{name}.png");
            Capture.Element(_window ?? throw new InvalidOperationException("no window"))
                .ToFile(path);
            return path;
        }
        catch
        {
            return null;
        }
    }

    private void EnsureStarted()
    {
        if (_window is not null)
            return;

        foreach (var stale in Process.GetProcessesByName("AgentGrid.Shell.WinUI"))
        {
            try
            {
                stale.Kill(entireProcessTree: true);
                stale.WaitForExit(5000);
            }
            catch
            {
                /* already gone */
            }
        }

        var exe = WinUiUi.ResolveExe();
        var psi = new ProcessStartInfo(exe)
        {
            WorkingDirectory = Path.GetDirectoryName(exe)!,
            UseShellExecute = false,
        };
        psi.Environment["AG_WINUI_NO_MSGBOX"] = "1";
        _configDirectory = WriteTestConfig(WinUiUi.GatewayUrl);
        psi.ArgumentList.Add("--config-dir");
        psi.ArgumentList.Add(_configDirectory);

        _app = Application.Launch(psi);
        _automation = new UIA3Automation();
        _window = _app.GetMainWindow(_automation, TimeSpan.FromSeconds(30));
        if (_window is null)
            throw new InvalidOperationException(
                "WinUI main window did not appear within 30s." + StartupErrorHint());
    }

    private static string WriteTestConfig(string gatewayUrl)
    {
        var directory = Path.Combine(
            Path.GetTempPath(),
            $"agent-grid-winui-e2e-{Guid.NewGuid():N}");
        Directory.CreateDirectory(directory);
        var tokenPath = Path.Combine(directory, "gateway-token");
        File.WriteAllText(tokenPath, "test-no-auth-token");
        var options = new System.Text.Json.JsonSerializerOptions { WriteIndented = true };
        File.WriteAllText(
            Path.Combine(directory, "servers.json"),
            System.Text.Json.JsonSerializer.Serialize(new
            {
                schema_version = 1,
                servers = new[]
                {
                    new
                    {
                        id = "test",
                        display_name = "Test",
                        enabled = true,
                        base_url = gatewayUrl,
                        web_origin = gatewayUrl,
                        token_path = tokenPath,
                        launch = new { mode = "connect_only" },
                    },
                },
            }, options));
        File.WriteAllText(
            Path.Combine(directory, "appearance.json"),
            """{"schema_version":1,"theme":"system","density":"comfortable","font_scale":1.0}""");
        File.WriteAllText(
            Path.Combine(directory, "shell.json"),
            """{"schema_version":1,"workspace_executable":"agent-grid-workspace","health_poll_interval_seconds":1}""");
        File.WriteAllText(
            Path.Combine(directory, "workspace.json"),
            """{"schema_version":1,"idle_quit_seconds":30,"default_window":{"width":1280,"height":800}}""");
        return directory;
    }

    private static string StartupErrorHint()
    {
        // Same structured SoT as launch-smoke.ps1 / docs/e2e.md.
        var log = Path.Combine(LocalAppData(), "agent-grid", "logs", "winui-startup-error.txt");
        if (!File.Exists(log))
            return "";
        return $"\nwinui-startup-error.txt:\n{File.ReadAllText(log)}";
    }

    private static string LocalAppData() =>
        Environment.GetEnvironmentVariable("LOCALAPPDATA")
        ?? Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);

    public void Dispose()
    {
        _automation?.Dispose();
        try
        {
            _app?.Kill();
        }
        catch
        {
            /* already exited */
        }
        _app?.Dispose();
        if (_configDirectory is not null)
        {
            try { Directory.Delete(_configDirectory, recursive: true); }
            catch { /* best-effort test cleanup */ }
        }
    }
}
