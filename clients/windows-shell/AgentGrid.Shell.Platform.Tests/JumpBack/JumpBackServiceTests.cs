using AgentGrid.Shell.Platform.Interop;
using AgentGrid.Shell.Platform.JumpBack;

namespace AgentGrid.Shell.Platform.Tests.JumpBack;

public class JumpBackServiceTests
{
    [Fact]
    public void Cached_hwnd_identity_ok_activates()
    {
        var win32 = new FakeWin32();
        win32.Windows.Add(new WindowInfo(42, "WindowsTerminal", "agent-grid", 100));
        win32.Alive.Add(42);
        var svc = new JumpBackService(win32);

        var r = svc.Jump(new JumpBackTarget(42, "WindowsTerminal", "agent-grid", null));
        Assert.Equal(JumpBackOutcome.Activated, r.Outcome);
        Assert.Equal(42, win32.LastSetForeground);
    }

    [Fact]
    public void Cached_hwnd_identity_mismatch_falls_through_to_enumeration()
    {
        var win32 = new FakeWin32();
        win32.Alive.Add(42);
        win32.ProcessNames[42] = "other";
        win32.Titles[42] = "nope";
        // Matching window elsewhere
        win32.Windows.Add(new WindowInfo(99, "WindowsTerminal", "sess-cwd", 7));
        var svc = new JumpBackService(win32);

        var r = svc.Jump(new JumpBackTarget(42, "WindowsTerminal", "sess-cwd", null));
        Assert.Equal(JumpBackOutcome.Activated, r.Outcome);
        Assert.Equal(99, win32.LastSetForeground);
    }

    [Fact]
    public void No_match_returns_not_found_without_activation()
    {
        var win32 = new FakeWin32();
        win32.Windows.Add(new WindowInfo(1, "notepad", "readme", 1));
        var svc = new JumpBackService(win32);

        var r = svc.Jump(new JumpBackTarget(null, "WindowsTerminal", "foo", null));
        Assert.Equal(JumpBackOutcome.NotFound, r.Outcome);
        Assert.Null(win32.LastSetForeground);
    }

    [Fact]
    public void Ambiguous_matches_return_conflicting()
    {
        var win32 = new FakeWin32();
        win32.Windows.Add(new WindowInfo(1, "code", "proj-a", 1));
        win32.Windows.Add(new WindowInfo(2, "code", "proj-a", 2));
        var svc = new JumpBackService(win32);

        var r = svc.Jump(new JumpBackTarget(null, "code", "proj-a", null));
        Assert.Equal(JumpBackOutcome.Conflicting, r.Outcome);
        Assert.Null(win32.LastSetForeground);
    }

    private sealed class FakeWin32 : IWin32InteropService
    {
        public List<WindowInfo> Windows { get; } = new();
        public HashSet<nint> Alive { get; } = new();
        public Dictionary<nint, string> ProcessNames { get; } = new();
        public Dictionary<nint, string> Titles { get; } = new();
        public nint? LastSetForeground { get; private set; }

        public nint GetForegroundWindow() => 0;
        public bool IsWindow(nint hwnd) => Alive.Contains(hwnd);
        public bool SetForegroundWindow(nint hwnd)
        {
            LastSetForeground = hwnd;
            return true;
        }
        public bool AllowSetForegroundWindow(int processId) => true;
        public string? GetWindowProcessName(nint hwnd) =>
            ProcessNames.TryGetValue(hwnd, out var n) ? n
            : Windows.FirstOrDefault(w => w.Hwnd == hwnd)?.ProcessName;
        public string? GetWindowTitle(nint hwnd) =>
            Titles.TryGetValue(hwnd, out var t) ? t
            : Windows.FirstOrDefault(w => w.Hwnd == hwnd)?.Title;
        public IReadOnlyList<WindowInfo> EnumerateWindows() => Windows;
        public bool IsSessionLocked() => false;
        public bool IsDoNotDisturb() => false;
        public bool TryGetNotificationsAllowed(out bool allowed)
        {
            allowed = true;
            return true;
        }
    }
}
