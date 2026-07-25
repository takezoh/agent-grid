using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class IslandMetricsTests
{
    [Fact]
    public void Compact_notch_is_wide_and_thin()
    {
        Assert.Equal(new IslandSize(312, 32), IslandMetrics.Compact(1.0));
    }

    [Fact]
    public void Expanded_size_tracks_measured_content()
    {
        // Vibe Island contract: the expanded island hugs its content instead
        // of jumping to a fixed panel size.
        Assert.Equal(new IslandSize(500, 300), IslandMetrics.ExpandedFor(500, 300, 1.0));
        Assert.Equal(new IslandSize(420, 260), IslandMetrics.ExpandedFor(420, 260, 1.0));
    }

    [Fact]
    public void Expanded_size_clamps_into_bounds()
    {
        // Tiny content still yields a usable island; huge content stops at the
        // max bounds (the surplus scrolls inside the panel instead). The min
        // bounds are a usability floor small enough that an island holding
        // only the header strip hugs down to it (no filler space).
        Assert.Equal(new IslandSize(320, 32), IslandMetrics.ExpandedFor(10, 5, 1.0));
        Assert.Equal(new IslandSize(680, 600), IslandMetrics.ExpandedFor(2000, 3000, 1.0));
    }

    [Fact]
    public void Expansion_flares_horizontally_from_the_compact_pill()
    {
        // Even the smallest expanded island is wider than the compact bar, so
        // the morph always reads as two-dimensional.
        var compact = IslandMetrics.Compact(1.0);
        var minExpanded = IslandMetrics.ExpandedFor(0, 0, 1.0);
        Assert.True(minExpanded.Width > compact.Width);
        Assert.True(minExpanded.Height >= compact.Height);
        Assert.True(IslandMetrics.ExpandedMaxWidthDips >= IslandMetrics.CompactWidthDips * 2);
    }

    [Theory]
    [InlineData(1.25)]
    [InlineData(1.5)]
    [InlineData(2.0)]
    public void High_dpi_scales_physical_size_with_rounding(double scale)
    {
        var compact = IslandMetrics.Compact(scale);
        Assert.Equal((int)Math.Round(IslandMetrics.CompactWidthDips * scale), compact.Width);
        Assert.Equal((int)Math.Round(IslandMetrics.CompactHeightDips * scale), compact.Height);

        var expanded = IslandMetrics.ExpandedFor(500, 300, scale);
        Assert.Equal((int)Math.Round(500 * scale), expanded.Width);
        Assert.Equal((int)Math.Round(300 * scale), expanded.Height);
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-1.0)]
    public void Invalid_scale_falls_back_to_100_percent(double scale)
    {
        // Pre-activation (no XamlRoot yet) the scale is unknown; sizing must
        // still produce a sane window instead of a zero/negative rect.
        Assert.Equal(IslandMetrics.Compact(1.0), IslandMetrics.Compact(scale));
        Assert.Equal(IslandMetrics.ExpandedFor(500, 300, 1.0),
            IslandMetrics.ExpandedFor(500, 300, scale));
    }

    [Fact]
    public void Header_only_content_hugs_close_to_the_compact_pill()
    {
        // Vibe Island: with every section empty the expanded island is just
        // the header strip — the floor must not force a big blank panel.
        var minExpanded = IslandMetrics.ExpandedFor(0, 0, 1.0);
        Assert.Equal(IslandMetrics.CompactHeightDips, minExpanded.Height);
    }
}
