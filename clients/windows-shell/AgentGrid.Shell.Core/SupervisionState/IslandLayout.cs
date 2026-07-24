namespace AgentGrid.Shell.Core.SupervisionState;

/// <summary>
/// Pure geometry for the island window: horizontal anchoring along the top
/// screen edge with clamping so the panel never leaves the work area.
/// All values are physical pixels.
/// </summary>
public static class IslandLayout
{
    /// <summary>Clamp a window left edge into [workX, workX + workWidth - width].</summary>
    public static int ClampLeft(int x, int width, int workX, int workWidth)
    {
        var min = workX;
        var max = workX + workWidth - width;
        if (max < min)
            max = min; // island wider than the work area: pin to the left edge
        return Math.Clamp(x, min, max);
    }

    /// <summary>
    /// Left edge for a window that should keep its center at centerX (the
    /// user-dragged anchor), clamped into the work area. Morphs between
    /// compact/expanded sizes stay anchored around the same center.
    /// </summary>
    public static int ClampedX(int centerX, int width, int workX, int workWidth) =>
        ClampLeft(centerX - width / 2, width, workX, workWidth);
}
