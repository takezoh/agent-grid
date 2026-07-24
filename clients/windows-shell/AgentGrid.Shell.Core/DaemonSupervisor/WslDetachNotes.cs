namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Spike notes for WSL daemon detach survival
/// (adr-20260724-boundary-3-wsl-detach-spike, contract-b3-wsl-detach-mechanism).
///
/// Candidate launch line (Windows Shell Runner):
///   wsl.exe -d &lt;distro&gt; -- bash -lc 'setsid nohup &lt;path&gt;/server -addr 127.0.0.1:&lt;port&gt;
///     -token-file ~/.agent-grid/gateway-token &gt;/tmp/agent-grid-server.log 2&gt;&amp;1 &lt;/dev/null &amp;'
///
/// Acceptance (T3 fidelity, opt-in):
/// 1. Spawn via candidate mechanism.
/// 2. taskkill /F the Windows-side wsl.exe launcher.
/// 3. /api/sessions continues to respond ≥5s later; Linux PID reparented to pid 1.
///
/// On failure: supersede ADR with systemd --user unit alternative and re-run S1.
/// This type exists so the spike criterion is discoverable from the code tree;
/// the real Runner lives behind IDaemonRunner and is Windows-only.
/// </summary>
public static class WslDetachNotes
{
    public const string CandidateDetachPrefix = "setsid nohup";
    public const string FallbackIfSpikeFails = "systemd --user unit";
}
