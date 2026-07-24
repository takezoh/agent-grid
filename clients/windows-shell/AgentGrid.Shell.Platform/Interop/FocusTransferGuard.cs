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
        /// <summary>Explicit user-initiated action (e.g. clicking the panel title).</summary>
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
