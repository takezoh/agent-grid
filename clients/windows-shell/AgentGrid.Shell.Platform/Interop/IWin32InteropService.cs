namespace AgentGrid.Shell.Platform.Interop;

/// <summary>
/// All Win32 interop behind an interface so pure logic is unit-testable with fakes.
/// Owns the cross-flow focus-transfer invariant (FR-FOCUS-INV,
/// contract-cross-flow-focus-invariant).
/// </summary>
public interface IWin32InteropService
{
    nint GetForegroundWindow();

    bool IsWindow(nint hwnd);

    bool SetForegroundWindow(nint hwnd);

    /// <summary>
    /// Allow the calling process to set foreground. Used before jump-back / engage restore.
    /// </summary>
    bool AllowSetForegroundWindow(int processId);

    /// <summary>GetWindowThreadProcessId — returns thread id (0 on failure).</summary>
    uint GetWindowThreadProcessId(nint hwnd, out uint processId);

    /// <summary>
    /// AttachThreadInput for engage focus-return
    /// (adr-20260724-engage-focus-restore-mechanism).
    /// </summary>
    bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach);

    /// <summary>
    /// Apply or clear WS_EX_NOACTIVATE on a top-level HWND (panel glance).
    /// </summary>
    bool SetNoActivate(nint hwnd, bool noActivate);

    string? GetWindowProcessName(nint hwnd);

    string? GetWindowTitle(nint hwnd);

    IReadOnlyList<WindowInfo> EnumerateWindows();

    bool IsSessionLocked();

    bool IsDoNotDisturb();

    /// <summary>
    /// QUERY_USER_NOTIFICATION_STATE-style query. Fail-open: when query fails, treat as
    /// notifications allowed (adr-20260724-toast-panel-watched-fail-open).
    /// </summary>
    bool TryGetNotificationsAllowed(out bool allowed);
}

public sealed record WindowInfo(
    nint Hwnd,
    string? ProcessName,
    string? Title,
    int ProcessId);
