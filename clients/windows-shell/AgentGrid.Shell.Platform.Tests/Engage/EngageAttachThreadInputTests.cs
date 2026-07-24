using AgentGrid.Shell.Platform.Engage;
using AgentGrid.Shell.Platform.Interop;

namespace AgentGrid.Shell.Platform.Tests.Engage;

public class EngageAttachThreadInputTests
{
    [Fact]
    public void Restore_uses_AttachThreadInput_when_threads_differ()
    {
        var win32 = new FakeWin32 { Foreground = 100 };
        win32.Alive.Add(100);
        win32.ThreadOf[100] = 10;
        win32.ThreadOf[200] = 20; // panel thread after engage
        var svc = new EngageFocusService(win32);

        svc.EnterEngage();
        win32.Foreground = 200;
        var r = svc.ExitEngage(shellHwnd: 200);

        Assert.Equal(EngageRestoreOutcome.RestoreLive, r.Outcome);
        Assert.True(win32.AttachCalls >= 2); // attach + detach
        Assert.Equal(100, win32.LastSetForeground);
    }

    private sealed class FakeWin32 : IWin32InteropService
    {
        public nint Foreground { get; set; }
        public HashSet<nint> Alive { get; } = new();
        public Dictionary<nint, uint> ThreadOf { get; } = new();
        public int AttachCalls { get; private set; }
        public nint? LastSetForeground { get; private set; }

        public nint GetForegroundWindow() => Foreground;
        public bool IsWindow(nint hwnd) => Alive.Contains(hwnd);
        public bool SetForegroundWindow(nint hwnd)
        {
            LastSetForeground = hwnd;
            return true;
        }
        public bool AllowSetForegroundWindow(int processId) => true;
        public uint GetWindowThreadProcessId(nint hwnd, out uint processId)
        {
            processId = 1;
            return ThreadOf.TryGetValue(hwnd, out var t) ? t : 1;
        }
        public bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach)
        {
            AttachCalls++;
            return true;
        }
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
