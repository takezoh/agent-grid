using AgentGrid.Shell.Core.Host;

namespace AgentGrid.Shell.Core.Tests.Host;

/// <summary>
/// T0: bootstrap failure text is structured and assertion-friendly.
/// (Dialog MessageBox text is not on stdout — this report is the SoT for tests.)
/// </summary>
public class WasdkBootstrapErrorsTests
{
    [Theory]
    [InlineData(WasdkBootstrapErrors.PackageNotFound, "package-not-found")]
    [InlineData(WasdkBootstrapErrors.NoApplicablePackage, "no-applicable-package")]
    [InlineData(WasdkBootstrapErrors.ClassNotRegistered, "class-not-registered")]
    [InlineData(WasdkBootstrapErrors.BootstrapVersionLookupFailed, "bootstrap-msix-version-lookup-failed")]
    [InlineData(0, "ok")]
    public void Classify_known_hresults(int hr, string expected) =>
        Assert.Equal(expected, WasdkBootstrapErrors.Classify(hr));

    [Fact]
    public void LooksLikeSelfContainedLayout_requires_natives()
    {
        var dir = Path.Combine(Path.GetTempPath(), $"ag-sc-{Guid.NewGuid():N}");
        Directory.CreateDirectory(dir);
        try
        {
            Assert.False(WasdkBootstrapErrors.LooksLikeSelfContainedLayout(dir));
            File.WriteAllText(Path.Combine(dir, "Microsoft.WindowsAppRuntime.dll"), "x");
            File.WriteAllText(Path.Combine(dir, "Microsoft.ui.xaml.dll"), "x");
            File.WriteAllText(Path.Combine(dir, "Microsoft.WindowsAppRuntime.Bootstrap.dll"), "x");
            Assert.True(WasdkBootstrapErrors.LooksLikeSelfContainedLayout(dir));
        }
        finally
        {
            try { Directory.Delete(dir, true); } catch { /* ignore */ }
        }
    }

    [Fact]
    public void FormatReport_includes_hresult_and_runtime_hint()
    {
        var report = WasdkBootstrapErrors.FormatReport(
            WasdkBootstrapErrors.PackageNotFound,
            majorMinor: WasdkBootstrapErrors.MajorMinor_1_6,
            exePath: @"C:\app\AgentGrid.Shell.WinUI.exe",
            logPath: @"C:\logs\winui-startup-error.txt");

        Assert.Contains("0x80073D54", report, StringComparison.OrdinalIgnoreCase);
        Assert.Contains("package-not-found", report, StringComparison.Ordinal);
        Assert.Contains("Windows App Runtime Version 1.6", report, StringComparison.Ordinal);
        Assert.Contains("MSIX package", report, StringComparison.Ordinal);
        Assert.Contains(@"C:\app\AgentGrid.Shell.WinUI.exe", report, StringComparison.Ordinal);
        Assert.Contains("self-contained", report, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public void WriteReport_round_trips_for_launch_smoke()
    {
        var path = Path.Combine(Path.GetTempPath(), $"ag-wasdk-{Guid.NewGuid():N}.txt");
        try
        {
            var report = WasdkBootstrapErrors.FormatReport(WasdkBootstrapErrors.PackageNotFound);
            WasdkBootstrapErrors.WriteReport(path, report);
            var read = File.ReadAllText(path);
            Assert.Contains("0x80073D54", read, StringComparison.OrdinalIgnoreCase);
            Assert.True(WasdkBootstrapErrors.LooksLikeMissingRuntime(WasdkBootstrapErrors.PackageNotFound));
        }
        finally
        {
            try { File.Delete(path); } catch { /* ignore */ }
        }
    }
}
