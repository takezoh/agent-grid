namespace AgentGrid.Shell.WinUI.UiTests.E2E;

/// <summary>
/// Opt-in T3 UI automation against the self-contained WinUI exe via UIA (FlaUI).
///
/// Enable with <c>AG_E2E_WINUI_UI=1</c> on Windows. The exe path comes from
/// <c>AG_E2E_WINUI_EXE</c> (the harness passes the smoke-stage build output);
/// without it the launch-smoke default locations are probed.
/// </summary>
public static class WinUiUi
{
    public const string EnableEnv = "AG_E2E_WINUI_UI";
    public const string ExeEnv = "AG_E2E_WINUI_EXE";
    public const string GatewayUrlEnv = "AG_E2E_GATEWAY_URL";
    public const string DefaultGatewayUrl = "http://127.0.0.1:8443";

    public static bool Enabled
    {
        get
        {
            if (!OperatingSystem.IsWindows())
                return false;
            var v = Environment.GetEnvironmentVariable(EnableEnv);
            return v is "1" or "true" or "TRUE" or "yes" or "YES";
        }
    }

    public static string GatewayUrl =>
        Environment.GetEnvironmentVariable(GatewayUrlEnv) ?? DefaultGatewayUrl;

    /// <summary>Skip message for xUnit when the UI fixture is not requested.</summary>
    public static string SkipUnlessEnabled()
    {
        if (Enabled)
            return string.Empty;
        return $"set {EnableEnv}=1 on Windows and build the WinUI exe first " +
               "(clients/windows-shell/scripts/e2e.sh runs this stage)";
    }

    public static string ResolveExe()
    {
        var fromEnv = Environment.GetEnvironmentVariable(ExeEnv);
        if (!string.IsNullOrEmpty(fromEnv) && File.Exists(fromEnv))
            return fromEnv;

        var local = Environment.GetEnvironmentVariable("LOCALAPPDATA")
                    ?? Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        // Same candidates as scripts/launch-smoke.ps1 Find-WinUiExe.
        var candidates = new[]
        {
            Path.Combine(local, "Temp", "ag-shell-src", "AgentGrid.Shell.WinUI", "bin", "x64",
                "Debug", "net8.0-windows10.0.19041.0", "win-x64", "AgentGrid.Shell.WinUI.exe"),
            Path.Combine(local, "agent-grid", "shell-winui", "AgentGrid.Shell.WinUI.exe"),
        };
        foreach (var c in candidates)
        {
            if (File.Exists(c))
                return c;
        }

        throw new FileNotFoundException(
            $"WinUI exe not found. Set {ExeEnv} or build via scripts/win-build-winui.ps1. " +
            $"Probed: {string.Join("; ", candidates)}");
    }
}

/// <summary>
/// xUnit Fact that is skipped unless <c>AG_E2E_WINUI_UI=1</c> (Windows only).
/// </summary>
[AttributeUsage(AttributeTargets.Method, AllowMultiple = false)]
public sealed class WinUiUiFactAttribute : FactAttribute
{
    public WinUiUiFactAttribute()
    {
        if (!WinUiUi.Enabled)
            Skip = WinUiUi.SkipUnlessEnabled();
    }
}
