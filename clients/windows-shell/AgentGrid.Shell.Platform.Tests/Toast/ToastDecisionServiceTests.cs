using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Platform.Interop;
using AgentGrid.Shell.Platform.Toast;

namespace AgentGrid.Shell.Platform.Tests.Toast;

public class ToastDecisionServiceTests
{
    [Fact]
    public void Fires_for_approval_when_unwatched()
    {
        var svc = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var outcome = svc.Decide("approval", panelFlyoutOpen: false, panelHwnd: 0);
        Assert.Equal(ToastDecisionService.ToastOutcome.Fire, outcome);
    }

    [Fact]
    public void Suppresses_when_flyout_open()
    {
        var svc = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var outcome = svc.Decide("approval", panelFlyoutOpen: true, panelHwnd: 0);
        Assert.Equal(ToastDecisionService.ToastOutcome.SuppressWatched, outcome);
    }

    [Fact]
    public void Suppresses_health_source()
    {
        var svc = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var outcome = svc.Decide("health", panelFlyoutOpen: false, panelHwnd: 0);
        Assert.Equal(ToastDecisionService.ToastOutcome.SuppressSource, outcome);
    }

    [Fact]
    public void DecideForNewPending_uses_item_kind()
    {
        var svc = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var item = new PanelGlanceItem("ap1", "s1", "approval", "cmd", null);
        Assert.Equal(
            ToastDecisionService.ToastOutcome.Fire,
            svc.DecideForNewPending(item, panelFlyoutOpen: false, panelHwnd: 0));
    }

    private sealed class OpenUnlocked : IWin32InteropService
    {
        public nint GetForegroundWindow() => nint.Zero;
        public bool IsWindow(nint hwnd) => true;
        public bool SetForegroundWindow(nint hwnd) => true;
        public bool AllowSetForegroundWindow(int processId) => true;
        public uint GetWindowThreadProcessId(nint hwnd, out uint processId)
        {
            processId = 1;
            return 1;
        }
        public bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach) => true;
        public bool SetNoActivate(nint hwnd, bool noActivate) => true;

        public string? GetWindowProcessName(nint hwnd) => null;
        public string? GetWindowTitle(nint hwnd) => null;
        public IReadOnlyList<WindowInfo> EnumerateWindows() => Array.Empty<WindowInfo>();
        public bool IsSessionLocked() => false;
        public bool IsDoNotDisturb() => false;
        public bool TryGetNotificationsAllowed(out bool allowed)
        {
            allowed = true;
            return true;
        }
    }
}
