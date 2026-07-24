using AgentGrid.Shell.Platform.Interop;

namespace AgentGrid.Shell.Platform.JumpBack;

public enum JumpBackOutcome
{
    Activated,
    NotFound,
    Conflicting,
}

public sealed record JumpBackResult(JumpBackOutcome Outcome, string? Detail = null);

public sealed record JumpBackTarget(
    nint? CachedHwnd,
    string? ProcessName,
    string? TitleContains,
    string? CwdHint);

/// <summary>
/// Staged best-effort resolution (contract-jump-back-staged-resolution, FR-JB-01..03):
/// (1) cached HWND identity-verified → SetForegroundWindow
/// (2) process-name + title matching
/// (3) explicit not-found — NEVER activate an arbitrary fallback window.
/// </summary>
public sealed class JumpBackService
{
    private readonly IWin32InteropService _win32;

    public JumpBackService(IWin32InteropService win32) =>
        _win32 = win32 ?? throw new ArgumentNullException(nameof(win32));

    public JumpBackResult Jump(JumpBackTarget target)
    {
        // Stage 1: cached HWND with identity verification.
        if (target.CachedHwnd is { } hwnd && hwnd != 0 && _win32.IsWindow(hwnd))
        {
            if (IdentityMatches(hwnd, target))
            {
                if (_win32.SetForegroundWindow(hwnd))
                    return new JumpBackResult(JumpBackOutcome.Activated, "cached-hwnd");
                return new JumpBackResult(JumpBackOutcome.Conflicting, "SetForegroundWindow denied");
            }
        }

        // Stage 2: process-name + title matching.
        var matches = _win32.EnumerateWindows()
            .Where(w => ProcessMatches(w, target) && TitleMatches(w, target))
            .ToList();

        if (matches.Count == 1)
        {
            if (_win32.SetForegroundWindow(matches[0].Hwnd))
                return new JumpBackResult(JumpBackOutcome.Activated, "process-title");
            return new JumpBackResult(JumpBackOutcome.Conflicting, "SetForegroundWindow denied");
        }

        if (matches.Count > 1)
            return new JumpBackResult(JumpBackOutcome.Conflicting, $"ambiguous: {matches.Count} windows");

        // Stage 3: explicit failure — do not activate any arbitrary window (FR-JB-03).
        return new JumpBackResult(JumpBackOutcome.NotFound, "target not found");
    }

    private bool IdentityMatches(nint hwnd, JumpBackTarget target)
    {
        if (target.ProcessName is null && target.TitleContains is null)
            return true; // HWND alone accepted when no identity constraints given.
        var name = _win32.GetWindowProcessName(hwnd);
        var title = _win32.GetWindowTitle(hwnd);
        return ProcessMatches(new WindowInfo(hwnd, name, title, 0), target)
               && TitleMatches(new WindowInfo(hwnd, name, title, 0), target);
    }

    private static bool ProcessMatches(WindowInfo w, JumpBackTarget target)
    {
        if (target.ProcessName is null) return true;
        return string.Equals(w.ProcessName, target.ProcessName, StringComparison.OrdinalIgnoreCase);
    }

    private static bool TitleMatches(WindowInfo w, JumpBackTarget target)
    {
        if (target.TitleContains is null) return true;
        return w.Title is not null &&
               w.Title.Contains(target.TitleContains, StringComparison.OrdinalIgnoreCase);
    }
}
