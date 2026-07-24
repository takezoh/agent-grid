using System.Diagnostics;

namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Windows-side process runner used by WslDaemonRunner to invoke wsl.exe via
/// System.Diagnostics.Process. Callable from PowerShell/cmd-hosted Shell.
/// </summary>
public static class ProcessRunner
{
    /// <summary>
    /// Run fileName with args; return exit code. Captures no domain data —
    /// stdout/stderr discarded (daemon logs land inside WSL).
    /// </summary>
    public static async Task<int> RunAsync(
        string fileName,
        string arguments,
        CancellationToken ct = default)
    {
        var psi = new ProcessStartInfo
        {
            FileName = fileName,
            Arguments = arguments,
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
        };
        using var proc = new Process { StartInfo = psi };
        if (!proc.Start())
            throw new InvalidOperationException($"failed to start {fileName}");

        // Drain to avoid pipe fills; content not used (domain data stays in WSL).
        var stdout = proc.StandardOutput.ReadToEndAsync(ct);
        var stderr = proc.StandardError.ReadToEndAsync(ct);
        await proc.WaitForExitAsync(ct).ConfigureAwait(false);
        await Task.WhenAll(stdout, stderr).ConfigureAwait(false);
        return proc.ExitCode;
    }

    /// <summary>
    /// Build a WslDaemonRunner wired to real Process.Start (Windows host).
    /// From WSL, prefer injecting a test runner — this path is for the WinUI Shell.
    /// </summary>
    public static WslDaemonRunner CreateWslRunner(
        string distro,
        string serverPath,
        int port,
        string tokenFileInWsl = "~/.agent-grid/gateway-token",
        string dataDirInWsl = "/tmp/agent-grid-data",
        Func<Task>? shutdown = null) =>
        new(
            distro,
            serverPath,
            port,
            tokenFileInWsl,
            dataDirInWsl,
            run: (file, args) => RunAsync(file, args),
            shutdown: shutdown);
}
