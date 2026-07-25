namespace AgentGrid.Shell.Core.SupervisionState;

/// <summary>Island window size in physical pixels.</summary>
public readonly record struct IslandSize(int Width, int Height);

/// <summary>
/// Island sizing contract, authored in logical DIPs and converted to physical
/// pixels at the display scale. Vibe Island UX: the compact pill is a fixed
/// narrow bar, while the expanded island hugs its content — the window is
/// sized from the measured content and merely clamped into bounds, so the
/// island grows sideways and downward as sessions/approvals accumulate and
/// shrinks back when the queue drains. Past the max bounds the content
/// scrolls inside instead of growing the window further.
/// </summary>
public static class IslandMetrics
{
    // Widths include the two 12-DIP concave wings painted by the window.
    // Min bounds are a usability floor, not a filler-space reserve: with all
    // sections empty the expanded island is just the header strip, and the
    // window hugs down to it instead of framing blank space.
    public const double CompactWidthDips = 312;
    public const double CompactHeightDips = 32;
    public const double ExpandedMinWidthDips = 320;
    public const double ExpandedMaxWidthDips = 680;
    public const double ExpandedMinHeightDips = CompactHeightDips;
    public const double ExpandedMaxHeightDips = 600;

    public static IslandSize Compact(double scale)
    {
        scale = Sane(scale);
        return new IslandSize(Physical(CompactWidthDips, scale), Physical(CompactHeightDips, scale));
    }

    /// <summary>
    /// Expanded size for measured content (DIPs), clamped into the expanded
    /// bounds. Content-driven: callers re-measure on every snapshot render.
    /// </summary>
    public static IslandSize ExpandedFor(double contentWidthDips, double contentHeightDips, double scale)
    {
        scale = Sane(scale);
        var w = Math.Clamp(contentWidthDips, ExpandedMinWidthDips, ExpandedMaxWidthDips);
        var h = Math.Clamp(contentHeightDips, ExpandedMinHeightDips, ExpandedMaxHeightDips);
        return new IslandSize(Physical(w, scale), Physical(h, scale));
    }

    private static double Sane(double scale) => scale <= 0 ? 1.0 : scale;

    private static int Physical(double dips, double scale) => (int)Math.Round(dips * scale);
}
