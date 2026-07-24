using AgentGrid.Shell.Core.Host;
using Microsoft.Windows.ApplicationModel.DynamicDependency;

namespace AgentGrid.Shell.WinUI.Host;

/// <summary>
/// Explicit Windows App SDK startup with machine-readable failure capture.
/// Self-contained builds skip MddBootstrap (avoids 0x80670016 MSIX lookup).
/// Framework-dependent builds call Bootstrap.TryInitialize and log HRESULT.
/// </summary>
public static class WasdkBootstrapHost
{
    public static bool TryStart(out int hr, out string report)
    {
        var exe = Environment.ProcessPath ?? "AgentGrid.Shell.WinUI.exe";
        var baseDir = AppContext.BaseDirectory;
        var logPath = Environment.GetEnvironmentVariable("AG_WINUI_STARTUP_LOG")
                      ?? WasdkBootstrapErrors.DefaultLogPath();
        var okPath = Environment.GetEnvironmentVariable("AG_WINUI_STARTUP_OK")
                     ?? WasdkBootstrapErrors.DefaultOkMarkerPath();

        // Self-contained: natives are already beside the EXE. Calling
        // Bootstrap.TryInitialize searches for the MSIX framework package and
        // fails with 0x80670016 even when the layout is complete.
        if (WasdkBootstrapErrors.LooksLikeSelfContainedLayout(baseDir))
        {
            hr = 0;
            report =
                $"bootstrap_ok mode=self-contained-skip-mdd hr={WasdkBootstrapErrors.FormatHr(0)} " +
                $"exe={exe} baseDir={baseDir}";
            WriteOk(okPath, report);
            return true;
        }

        try
        {
            if (Bootstrap.TryInitialize(WasdkBootstrapErrors.MajorMinor_1_6, out hr))
            {
                report = $"bootstrap_ok mode=framework-dependent hr={WasdkBootstrapErrors.FormatHr(hr)} exe={exe}";
                WriteOk(okPath, report);
                return true;
            }
        }
        catch (Exception ex)
        {
            hr = ex.HResult != 0 ? ex.HResult : unchecked((int)0x80004005);
            report = WasdkBootstrapErrors.FormatReport(hr, exePath: exe, logPath: logPath)
                     + Environment.NewLine
                     + $"exception: {ex.GetType().FullName}: {ex.Message}"
                     + Environment.NewLine
                     + $"baseDir={baseDir}";
            TryWriteError(logPath, report);
            return false;
        }

        report = WasdkBootstrapErrors.FormatReport(hr, exePath: exe, logPath: logPath)
                 + Environment.NewLine
                 + $"baseDir={baseDir}"
                 + Environment.NewLine
                 + "note: if this is a self-contained build, Bootstrap should have been skipped; "
                 + "verify Microsoft.WindowsAppRuntime.dll and Microsoft.ui.xaml.dll sit next to the EXE.";
        TryWriteError(logPath, report);
        return false;
    }

    private static void WriteOk(string path, string report)
    {
        try
        {
            WasdkBootstrapErrors.WriteReport(path, report);
            WasdkBootstrapErrors.WriteReport(
                Path.Combine(AppContext.BaseDirectory, "winui-startup-ok.txt"),
                report);
        }
        catch
        {
            /* non-fatal */
        }
    }

    private static void TryWriteError(string path, string report)
    {
        try
        {
            WasdkBootstrapErrors.WriteReport(path, report);
            WasdkBootstrapErrors.WriteReport(
                Path.Combine(AppContext.BaseDirectory, "winui-startup-error.txt"),
                report);
        }
        catch
        {
            /* best-effort */
        }

        try
        {
            Console.Error.WriteLine(report);
        }
        catch
        {
            /* WinExe may have no console */
        }
    }
}
