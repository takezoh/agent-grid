using AgentGrid.Shell.Platform.Interop;

namespace AgentGrid.Shell.Platform.Toast;

/// <summary>
/// Toast fires iff panel is unwatched (contract-toast-panel-watched-detection).
/// Fail-open on notification-state query errors (adr-20260724-toast-panel-watched-fail-open).
/// </summary>
public sealed class PanelWatchedPredicate
{
    private readonly IWin32InteropService _win32;

    public PanelWatchedPredicate(IWin32InteropService win32) =>
        _win32 = win32 ?? throw new ArgumentNullException(nameof(win32));

    /// <summary>
    /// Returns true when a toast SHOULD fire for a new supervision item.
    /// </summary>
    public bool ShouldFireToast(bool panelFlyoutOpen, nint panelHwnd)
    {
        // Panel is watched when flyout is open and not obscured by lock/DND-suppressed states.
        if (panelFlyoutOpen)
        {
            // Still suppress if session locked — user cannot act on toast either.
            if (_win32.IsSessionLocked())
                return false;
            return false; // watched: queue update alone, no toast
        }

        if (_win32.IsSessionLocked())
            return false;

        if (_win32.IsDoNotDisturb())
            return false;

        // Fail-open: if notification query fails, allow toast.
        if (_win32.TryGetNotificationsAllowed(out var allowed))
        {
            if (!allowed)
                return false;
        }

        // Panel not open / not watched → toast.
        // Do not use panel HWND foreground as a silent activation path (FR-FOCUS-INV).
        _ = panelHwnd;
        return true;
    }
}
