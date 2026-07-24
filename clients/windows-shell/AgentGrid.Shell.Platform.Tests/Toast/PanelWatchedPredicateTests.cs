using AgentGrid.Shell.Platform.Interop;
using AgentGrid.Shell.Platform.Toast;

namespace AgentGrid.Shell.Platform.Tests.Toast;

public class PanelWatchedPredicateTests
{
    [Fact]
    public void Flyout_open_suppresses_toast()
    {
        var p = new PanelWatchedPredicate(new FakeWin32());
        Assert.False(p.ShouldFireToast(panelFlyoutOpen: true, panelHwnd: 1));
    }

    [Fact]
    public void Flyout_closed_fires_toast()
    {
        var p = new PanelWatchedPredicate(new FakeWin32());
        Assert.True(p.ShouldFireToast(panelFlyoutOpen: false, panelHwnd: 1));
    }

    [Fact]
    public void Dnd_suppresses_toast()
    {
        var p = new PanelWatchedPredicate(new FakeWin32 { Dnd = true });
        Assert.False(p.ShouldFireToast(panelFlyoutOpen: false, panelHwnd: 1));
    }

    [Fact]
    public void Locked_suppresses_toast()
    {
        var p = new PanelWatchedPredicate(new FakeWin32 { Locked = true });
        Assert.False(p.ShouldFireToast(panelFlyoutOpen: false, panelHwnd: 1));
    }

    [Fact]
    public void Notification_query_fail_opens_allows_toast()
    {
        // Fail-open: TryGetNotificationsAllowed returns false → toast still allowed.
        var p = new PanelWatchedPredicate(new FakeWin32 { NotificationQueryOk = false });
        Assert.True(p.ShouldFireToast(panelFlyoutOpen: false, panelHwnd: 1));
    }

    [Fact]
    public void Notifications_disallowed_suppresses()
    {
        var p = new PanelWatchedPredicate(new FakeWin32 { NotificationsAllowed = false });
        Assert.False(p.ShouldFireToast(panelFlyoutOpen: false, panelHwnd: 1));
    }

    private sealed class FakeWin32 : IWin32InteropService
    {
        public bool Dnd { get; set; }
        public bool Locked { get; set; }
        public bool NotificationQueryOk { get; set; } = true;
        public bool NotificationsAllowed { get; set; } = true;

        public nint GetForegroundWindow() => 0;
        public bool IsWindow(nint hwnd) => false;
        public bool SetForegroundWindow(nint hwnd) => false;
        public bool AllowSetForegroundWindow(int processId) => true;
        public string? GetWindowProcessName(nint hwnd) => null;
        public string? GetWindowTitle(nint hwnd) => null;
        public IReadOnlyList<WindowInfo> EnumerateWindows() => Array.Empty<WindowInfo>();
        public bool IsSessionLocked() => Locked;
        public bool IsDoNotDisturb() => Dnd;
        public bool TryGetNotificationsAllowed(out bool allowed)
        {
            allowed = NotificationsAllowed;
            return NotificationQueryOk;
        }
    }
}
