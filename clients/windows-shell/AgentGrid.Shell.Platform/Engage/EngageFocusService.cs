using AgentGrid.Shell.Platform.Interop;

namespace AgentGrid.Shell.Platform.Engage;

public enum EngageRestoreOutcome
{
    RestoreLive,
    RestoreDenied,
    TargetDestroyed,
}

public sealed record EngageRestoreResult(EngageRestoreOutcome Outcome, string? Detail = null);

/// <summary>
/// Engage focus capture/restore (contract-engage-focus-return-mechanism, FR-EF-01/02).
/// Records pre-engage foreground HWND; on confirm/cancel attempts restore.
/// MUST NOT force Shell's own window as substitute when target is destroyed.
/// </summary>
public sealed class EngageFocusService
{
    private readonly IWin32InteropService _win32;
    private nint _preEngageHwnd;
    private bool _armed;

    public EngageFocusService(IWin32InteropService win32) =>
        _win32 = win32 ?? throw new ArgumentNullException(nameof(win32));

    public nint PreEngageHwnd => _preEngageHwnd;
    public bool IsArmed => _armed;

    /// <summary>Call before removing WS_EX_NOACTIVATE / taking keyboard focus.</summary>
    public void EnterEngage()
    {
        _preEngageHwnd = _win32.GetForegroundWindow();
        _armed = true;
    }

    /// <summary>
    /// Attempt restore on confirm/cancel. Does not activate Shell as fallback (FR-EF-02).
    /// </summary>
    public EngageRestoreResult ExitEngage(nint shellHwnd = 0)
    {
        if (!_armed)
            return new EngageRestoreResult(EngageRestoreOutcome.TargetDestroyed, "not armed");

        _armed = false;
        var target = _preEngageHwnd;
        _preEngageHwnd = 0;

        if (target == 0 || !_win32.IsWindow(target))
            return new EngageRestoreResult(EngageRestoreOutcome.TargetDestroyed, "target destroyed");

        // Never substitute Shell's own HWND.
        if (shellHwnd != 0 && target == shellHwnd)
            return new EngageRestoreResult(EngageRestoreOutcome.TargetDestroyed, "refusing self-restore");

        if (_win32.SetForegroundWindow(target))
            return new EngageRestoreResult(EngageRestoreOutcome.RestoreLive);

        return new EngageRestoreResult(EngageRestoreOutcome.RestoreDenied, "SetForegroundWindow denied");
    }
}
