namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Spike notes for WSL daemon detach survival
/// (adr-20260724-boundary-3-wsl-detach-spike, contract-b3-wsl-detach-mechanism).
///
/// Candidate launch line (Windows Shell Runner / WslDaemonRunner.BuildDetachCommand):
///   wsl.exe -d &lt;distro&gt; -- bash -lc 'mkdir -p /tmp/agent-grid-data &amp;&amp;
///     setsid nohup &lt;path&gt;/server -data-dir /tmp/agent-grid-data -addr 127.0.0.1:&lt;port&gt;
///     -token-file &lt;token&gt; -insecure &gt;/tmp/agent-grid-server.log 2&gt;&amp;1 &lt;/dev/null &amp; echo $!'
///
/// Acceptance (T3 fidelity): see docs/wsl-detach-spike-result.md (PASS 2026-07-24).
/// Criterion used: process remains + authenticated /api/sessions ≥5s after launcher returns.
/// Explicit -data-dir avoids non-writable ~/.agent-grid on some hosts.
///
/// On failure: supersede ADR with systemd --user unit alternative and re-run S1.
/// </summary>
public static class WslDetachNotes
{
    public const string CandidateDetachPrefix = "setsid nohup";
    public const string FallbackIfSpikeFails = "systemd --user unit";
    public const string SpikeResultDoc = "clients/windows-shell/docs/wsl-detach-spike-result.md";
    public const string SpikeVerdict = "accepted-2026-07-24";
}
