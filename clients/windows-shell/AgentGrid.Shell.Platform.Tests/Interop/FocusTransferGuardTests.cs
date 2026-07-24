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
}
