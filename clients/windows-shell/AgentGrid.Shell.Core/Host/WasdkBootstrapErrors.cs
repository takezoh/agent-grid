namespace AgentGrid.Shell.Core.Host;

/// <summary>
/// Formats Windows App SDK bootstrap HRESULTs into stable, testable text.
/// MessageBox dialogs from auto-bootstrap are not on stdout — production
/// must call Bootstrap.TryInitialize and route failures through here.
/// </summary>
public static class WasdkBootstrapErrors
{
    // Common deployment HRESULTs (Windows App SDK / packaging).
    public const int RpcSServerUnavailable = unchecked((int)0x800706BA);
    public const int ClassNotRegistered = unchecked((int)0x80040154);
    public const int FileNotFound = unchecked((int)0x80070002);
    public const int PathNotFound = unchecked((int)0x80070003);
    public const int PackageNotFound = unchecked((int)0x80073D54);
    public const int NoApplicablePackage = unchecked((int)0x80073CF3);
    public const int ServiceDoesNotExist = unchecked((int)0x80070424);
    /// <summary>Observed when Bootstrap looks for MSIX 1.6 framework (self-contained must skip).</summary>
    public const int BootstrapVersionLookupFailed = unchecked((int)0x80670016);

    /// <summary>0x00010006 = Windows App SDK 1.6 major.minor.</summary>
    public const uint MajorMinor_1_6 = 0x00010006;

    public static bool Failed(int hr) => hr < 0;

    public static string FormatHr(int hr) =>
        $"0x{unchecked((uint)hr):X8} ({hr})";

    public static string Classify(int hr) =>
        unchecked((uint)hr) switch
        {
            0x80073D54 => "package-not-found",
            0x80073CF3 => "no-applicable-package",
            0x80040154 => "class-not-registered",
            0x80070002 => "file-not-found",
            0x80070003 => "path-not-found",
            0x800706BA => "rpc-server-unavailable",
            0x80070424 => "service-does-not-exist",
            0x80670016 => "bootstrap-msix-version-lookup-failed",
            _ when hr < 0 => "bootstrap-failed",
            _ => "ok",
        };

    public static bool LooksLikeMissingRuntime(int hr) =>
        Classify(hr) is "package-not-found" or "no-applicable-package"
            or "class-not-registered" or "file-not-found" or "path-not-found"
            or "service-does-not-exist" or "bootstrap-msix-version-lookup-failed";

    /// <summary>
    /// Self-contained WASDK layout ships natives next to the EXE; MddBootstrap
    /// MSIX lookup must not run (HRESULT 0x80670016 on this machine).
    /// </summary>
    public static bool LooksLikeSelfContainedLayout(string baseDir)
    {
        if (string.IsNullOrEmpty(baseDir))
            return false;
        return File.Exists(Path.Combine(baseDir, "Microsoft.WindowsAppRuntime.dll"))
               && File.Exists(Path.Combine(baseDir, "Microsoft.ui.xaml.dll"))
               && File.Exists(Path.Combine(baseDir, "Microsoft.WindowsAppRuntime.Bootstrap.dll"));
    }

    /// <summary>
    /// Multi-line report suitable for log file, stderr, and assertions.
    /// </summary>
    public static string FormatReport(
        int hr,
        uint majorMinor = MajorMinor_1_6,
        string? exePath = null,
        string? logPath = null)
    {
        var major = (majorMinor >> 16) & 0xFFFF;
        var minor = majorMinor & 0xFFFF;
        var lines = new List<string>
        {
            "AgentGrid.Shell.WinUI startup failed: Windows App SDK bootstrap",
            $"classification: {Classify(hr)}",
            $"hresult: {FormatHr(hr)}",
            $"requested_wasdk: {major}.{minor} (0x{majorMinor:X8})",
            "user_visible_hint: This application requires the Windows App Runtime "
                + $"Version {major}.{minor} (MSIX package) or a self-contained build "
                + "that ships Bootstrap + native runtime next to the EXE.",
        };
        if (!string.IsNullOrEmpty(exePath))
            lines.Add($"exe: {exePath}");
        if (!string.IsNullOrEmpty(logPath))
            lines.Add($"log: {logPath}");
        lines.Add("fix: rebuild with WindowsAppSDKSelfContained=true and launch the "
                  + "win-x64 output folder (not a stripped copy of the EXE alone).");
        return string.Join(Environment.NewLine, lines);
    }

    public static string DefaultLogPath()
    {
        var local = Environment.GetEnvironmentVariable("LOCALAPPDATA")
                    ?? Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        return Path.Combine(local, "agent-grid", "logs", "winui-startup-error.txt");
    }

    public static string DefaultOkMarkerPath()
    {
        var local = Environment.GetEnvironmentVariable("LOCALAPPDATA")
                    ?? Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        return Path.Combine(local, "agent-grid", "logs", "winui-startup-ok.txt");
    }

    public static void WriteReport(string path, string report)
    {
        var dir = Path.GetDirectoryName(path);
        if (!string.IsNullOrEmpty(dir))
            Directory.CreateDirectory(dir);
        File.WriteAllText(path, report + Environment.NewLine);
    }
}
