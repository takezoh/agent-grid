using AgentGrid.Shell.Platform.Engage;
using AgentGrid.Shell.Platform.Interop;

namespace AgentGrid.Shell.Platform.Tests.Engage;

public class EngageFocusServiceTests
{
    [Fact]
    public void Restore_live_when_target_still_exists()
    {
        var win32 = new FakeWin32 { Foreground = 100 };
        win32.Alive.Add(100);
        var svc = new EngageFocusService(win32);

        svc.EnterEngage();
        win32.Foreground = 200; // panel took focus
        var r = svc.ExitEngage(shellHwnd: 200);

        Assert.Equal(EngageRestoreOutcome.RestoreLive, r.Outcome);
        Assert.Equal(100, win32.LastSetForeground);
    }

    [Fact]
    public void Target_destroyed_does_not_activate_shell()
    {
        var win32 = new FakeWin32 { Foreground = 100 };
        win32.Alive.Add(100);
        var svc = new EngageFocusService(win32);

        svc.EnterEngage();
        win32.Alive.Remove(100); // target gone
        var r = svc.ExitEngage(shellHwnd: 200);

        Assert.Equal(EngageRestoreOutcome.TargetDestroyed, r.Outcome);
        Assert.Null(win32.LastSetForeground);
    }

    [Fact]
    public void Restore_denied_when_os_refuses()
    {
        var win32 = new FakeWin32 { Foreground = 100, AllowSetForeground = false };
        win32.Alive.Add(100);
        var svc = new EngageFocusService(win32);

        svc.EnterEngage();
        var r = svc.ExitEngage();

        Assert.Equal(EngageRestoreOutcome.RestoreDenied, r.Outcome);
    }

    private sealed class FakeWin32 : IWin32InteropService
    {
        public nint Foreground { get; set; }
        public HashSet<nint> Alive { get; } = new();
        public bool AllowSetForeground { get; set; } = true;
        public nint? LastSetForeground { get; private set; }

        public nint GetForegroundWindow() => Foreground;
        public bool IsWindow(nint hwnd) => Alive.Contains(hwnd);
        public bool SetForegroundWindow(nint hwnd)
        {
            if (!AllowSetForeground) return false;
            LastSetForeground = hwnd;
            return true;
        }
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
