using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;

namespace AgentGrid.Shell.Platform.Interop;

/// <summary>
/// Real Win32 interop for Windows Shell. Only load on Windows; unit tests use fakes.
/// Focus transfers MUST go through FocusTransferGuard (contract-cross-flow-focus-invariant).
/// </summary>
public sealed class Win32InteropService : IWin32InteropService
{
    public nint GetForegroundWindow() => Native.GetForegroundWindow();

    public bool IsWindow(nint hwnd) => Native.IsWindow(hwnd);

    public bool SetForegroundWindow(nint hwnd) => Native.SetForegroundWindow(hwnd);

    public bool AllowSetForegroundWindow(int processId) =>
        Native.AllowSetForegroundWindow(processId);

    public uint GetWindowThreadProcessId(nint hwnd, out uint processId) =>
        Native.GetWindowThreadProcessId(hwnd, out processId);

    public bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach) =>
        Native.AttachThreadInput(idAttach, idAttachTo, attach);

    public bool SetNoActivate(nint hwnd, bool noActivate)
    {
        const int GWL_EXSTYLE = -20;
        const uint WS_EX_NOACTIVATE = 0x08000000;
        var style = Native.GetWindowLongPtr(hwnd, GWL_EXSTYLE);
        if (style == nint.Zero && Marshal.GetLastWin32Error() != 0)
            return false;
        var next = noActivate
            ? (nint)((ulong)style | WS_EX_NOACTIVATE)
            : (nint)((ulong)style & ~WS_EX_NOACTIVATE);
        Native.SetWindowLongPtr(hwnd, GWL_EXSTYLE, next);
        // Force frame change so style sticks.
        const uint SWP_NOMOVE = 0x0002, SWP_NOSIZE = 0x0001, SWP_NOZORDER = 0x0004;
        const uint SWP_FRAMECHANGED = 0x0020, SWP_NOACTIVATE = 0x0010;
        return Native.SetWindowPos(
            hwnd, nint.Zero, 0, 0, 0, 0,
            SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_FRAMECHANGED | SWP_NOACTIVATE);
    }

    public string? GetWindowProcessName(nint hwnd)
    {
        _ = Native.GetWindowThreadProcessId(hwnd, out var pid);
        if (pid == 0)
            return null;
        try
        {
            using var p = Process.GetProcessById((int)pid);
            return p.ProcessName;
        }
        catch
        {
            return null;
        }
    }

    public string? GetWindowTitle(nint hwnd)
    {
        var len = Native.GetWindowTextLength(hwnd);
        if (len <= 0)
            return string.Empty;
        var sb = new StringBuilder(len + 1);
        _ = Native.GetWindowText(hwnd, sb, sb.Capacity);
        return sb.ToString();
    }

    public IReadOnlyList<WindowInfo> EnumerateWindows()
    {
        var list = new List<WindowInfo>();
        Native.EnumWindows((hwnd, _) =>
        {
            if (!Native.IsWindowVisible(hwnd))
                return true;
            Native.GetWindowThreadProcessId(hwnd, out var pid);
            string? name = null;
            try
            {
                if (pid != 0)
                {
                    using var p = Process.GetProcessById((int)pid);
                    name = p.ProcessName;
                }
            }
            catch
            {
                /* access denied / exited */
            }
            list.Add(new WindowInfo(hwnd, name, GetWindowTitle(hwnd), (int)pid));
            return true;
        }, nint.Zero);
        return list;
    }

    public bool IsSessionLocked()
    {
        // WTSGetActiveConsoleSessionId + WTSQuerySessionInformation is heavier;
        // use OpenInputDesktop as a practical lock probe.
        var desk = Native.OpenInputDesktop(0, false, 0x0100 /* DESKTOP_READOBJECTS */);
        if (desk == nint.Zero)
            return true; // cannot open input desktop → treat as locked
        Native.CloseDesktop(desk);
        return false;
    }

    public bool IsDoNotDisturb()
    {
        // QUERY_USER_NOTIFICATION_STATE via shell32
        if (!TryQueryUserNotificationState(out var state))
            return false; // fail-open for DND
        // QUNS_BUSY / QUNS_RUNNING_D3D_FULL_SCREEN / QUNS_PRESENTATION_MODE ≈ DND
        return state is 2 or 3 or 4;
    }

    public bool TryGetNotificationsAllowed(out bool allowed)
    {
        if (!TryQueryUserNotificationState(out var state))
        {
            allowed = true; // fail-open
            return false;
        }
        // QUNS_ACCEPTS_NOTIFICATIONS = 5; QUNS_QUIET_TIME = 6 still allows app toasts sometimes
        allowed = state is 5 or 6 or 1 /* QUNS_NOT_PRESENT still show */;
        // Actually QUNS_NOT_PRESENT=1 means no user present — still fire is OK for fail-open.
        // Suppress only BUSY / FULLSCREEN / PRESENTATION.
        allowed = state is not (2 or 3 or 4);
        return true;
    }

    private static bool TryQueryUserNotificationState(out int state)
    {
        state = 0;
        try
        {
            var hr = Native.SHQueryUserNotificationState(out state);
            return hr == 0;
        }
        catch
        {
            return false;
        }
    }

    private static class Native
    {
        public delegate bool EnumWindowsProc(nint hWnd, nint lParam);

        [DllImport("user32.dll")]
        public static extern nint GetForegroundWindow();

        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool IsWindow(nint hWnd);

        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool SetForegroundWindow(nint hWnd);

        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool AllowSetForegroundWindow(int dwProcessId);

        [DllImport("user32.dll")]
        public static extern uint GetWindowThreadProcessId(nint hWnd, out uint lpdwProcessId);

        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool AttachThreadInput(uint idAttach, uint idAttachTo, bool fAttach);

        [DllImport("user32.dll", EntryPoint = "GetWindowLongPtrW", SetLastError = true)]
        public static extern nint GetWindowLongPtr(nint hWnd, int nIndex);

        [DllImport("user32.dll", EntryPoint = "SetWindowLongPtrW", SetLastError = true)]
        public static extern nint SetWindowLongPtr(nint hWnd, int nIndex, nint dwNewLong);

        [DllImport("user32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool SetWindowPos(
            nint hWnd, nint hWndInsertAfter, int X, int Y, int cx, int cy, uint uFlags);

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        public static extern int GetWindowText(nint hWnd, StringBuilder lpString, int nMaxCount);

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        public static extern int GetWindowTextLength(nint hWnd);

        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, nint lParam);

        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool IsWindowVisible(nint hWnd);

        [DllImport("user32.dll", SetLastError = true)]
        public static extern nint OpenInputDesktop(uint dwFlags, bool fInherit, uint dwDesiredAccess);

        [DllImport("user32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool CloseDesktop(nint hDesktop);

        [DllImport("shell32.dll")]
        public static extern int SHQueryUserNotificationState(out int pquns);
    }
}
