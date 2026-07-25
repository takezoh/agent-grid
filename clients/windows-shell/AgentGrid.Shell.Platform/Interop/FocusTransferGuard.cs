namespace AgentGrid.Shell.Platform.Interop;

/// <summary>
/// Documents and enforces the cross-flow focus-transfer invariant (FR-FOCUS-INV):
/// only JumpBackService and EngageFocusService may call SetForegroundWindow for
/// automated recovery. Any third automated path is a contract violation.
///
/// Runtime guard used by production Win32 adapter; unit tests assert call-site policy.
/// </summary>
public static class FocusTransferGuard
{
    public enum Caller
    {
        JumpBack,
        EngageRestore,
        /// <summary>
        /// Explicit user-initiated action (e.g. clicking the panel title). Also covers
        /// composition-layer delegation of cross-process foreground permission on
        /// WorkspaceLauncherService's pipe-success openSession path
        /// (ShellCompositionRoot.Build's <c>win32.AllowSetForegroundWindow</c> wiring,
        /// debug-run-workspace-not-activated): reached only from explicit in-process user
        /// actions -- SessionChip click, pending-session open, WinUI-hosted deep link
        /// (PanelWindow.xaml.cs) -- NOT the headless Host deep-link path (Program.cs), which does
        /// not have a foreground-authority caller and is intentionally out of scope (tracked as a
        /// separate issue candidate; see investigation.json open_questions).
        /// </summary>
        ExplicitUser,
    }

    private static readonly HashSet<Caller> Allowed = new()
    {
        Caller.JumpBack,
        Caller.EngageRestore,
        Caller.ExplicitUser,
    };

    public static void EnsureAllowed(Caller caller)
    {
        if (!Allowed.Contains(caller))
            throw new InvalidOperationException(
                $"focus transfer by {caller} violates FR-FOCUS-INV / contract-cross-flow-focus-invariant");
    }

    public static bool IsAllowed(Caller caller) => Allowed.Contains(caller);
}
