using AgentGrid.Shell.Platform.Interop;

namespace AgentGrid.Shell.Platform.Tests.Interop;

public class FocusTransferGuardTests
{
    [Theory]
    [InlineData(FocusTransferGuard.Caller.JumpBack)]
    [InlineData(FocusTransferGuard.Caller.EngageRestore)]
    [InlineData(FocusTransferGuard.Caller.ExplicitUser)]
    public void Allowed_callers_pass(FocusTransferGuard.Caller caller)
    {
        Assert.True(FocusTransferGuard.IsAllowed(caller));
        FocusTransferGuard.EnsureAllowed(caller); // does not throw
    }

    // NOTE (contract pin, 2026-07-26): a test asserting that
    // Enum.GetValues<Caller>().Where(IsAllowed) equals {JumpBack, EngageRestore,
    // ExplicitUser} was deliberately NOT added as a witness for NFR-002
    // ("到着・表示・解決のいずれでもフォーカスを奪わない") / UAC-011.
    //
    // It would be a tautology over two hardcoded lists in the same production file
    // (the enum has exactly those three members and the private Allowed set contains
    // all three), and — decisively — FocusTransferGuard.EnsureAllowed/IsAllowed have
    // NO production call sites: JumpBackService and EngageFocusService call
    // _win32.SetForegroundWindow directly. So an arrival/toast path that stole focus
    // by calling SetForegroundWindow directly would leave such a test green.
    //
    // Per adr-20260725-vibe-island-verification-tiers (no fake green on T3-manual
    // rows), focus non-steal stays T3-observable-only. That the guard is documented
    // as "used by production Win32 adapter" while having no call site is a
    // production-side gap recorded for a follow-up change, not fixable here
    // (adr-20260725-vibe-island-asis-contract-pin forbids production changes).
}
