namespace AgentGrid.Shell.Core.SupervisionState;

public enum IslandMode
{
    Compact,
    Expanded,
}

/// <summary>Why the island is currently expanded — decides auto-collapse.</summary>
public enum IslandExpandReason
{
    None,
    User,
    AutoPending,
}

/// <summary>
/// Dynamic-island mode state (Vibe Island UX): the panel lives as a compact
/// notch bar, auto-expands when an approval/question arrives, and auto-collapses
/// when the pending queue drains — unless the user expanded it themselves.
/// </summary>
public sealed record IslandState(
    IslandMode Mode,
    IslandExpandReason Reason,
    int PendingCount)
{
    /// <summary>
    /// First launch shows the full panel once (discoverability; also the UI e2e
    /// AutomationId contract asserts against the expanded tree at startup).
    /// The morph loop starts after the first user collapse.
    /// </summary>
    public static IslandState Initial { get; } = new(
        IslandMode.Expanded, IslandExpandReason.None, 0);
}

/// <summary>Pure transitions; UI applies Mode changes (resize + visibility).</summary>
public static class IslandStateMachine
{
    public static IslandState OnSnapshot(IslandState state, int pendingCount)
    {
        // New pending work pops the notch open (each new request, not only 0→1,
        // so a user who collapsed mid-queue still sees fresh arrivals).
        if (state.Mode == IslandMode.Compact && pendingCount > state.PendingCount)
            return new(IslandMode.Expanded, IslandExpandReason.AutoPending, pendingCount);

        // Queue drained: only auto-opened panels close themselves.
        if (state.Mode == IslandMode.Expanded
            && state.Reason == IslandExpandReason.AutoPending
            && pendingCount == 0
            && state.PendingCount > 0)
        {
            return new(IslandMode.Compact, IslandExpandReason.None, 0);
        }

        return state with { PendingCount = pendingCount };
    }

    public static IslandState OnUserExpand(IslandState state) =>
        state with { Mode = IslandMode.Expanded, Reason = IslandExpandReason.User };

    public static IslandState OnUserCollapse(IslandState state) =>
        state with { Mode = IslandMode.Compact, Reason = IslandExpandReason.None };

    public static IslandState OnUserToggle(IslandState state) =>
        state.Mode == IslandMode.Expanded ? OnUserCollapse(state) : OnUserExpand(state);
}
