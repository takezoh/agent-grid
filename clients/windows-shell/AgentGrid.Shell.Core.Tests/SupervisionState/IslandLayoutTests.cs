using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class IslandLayoutTests
{
    [Fact]
    public void Centered_island_keeps_its_center()
    {
        // work [0,1920), width 400, center 960 → left 760
        Assert.Equal(760, IslandLayout.ClampedX(960, 400, 0, 1920));
    }

    [Fact]
    public void Left_overflow_clamps_to_work_left_edge()
    {
        Assert.Equal(0, IslandLayout.ClampedX(100, 400, 0, 1920));
        Assert.Equal(50, IslandLayout.ClampedX(100, 400, 50, 1920));
    }

    [Fact]
    public void Right_overflow_clamps_to_work_right_edge()
    {
        Assert.Equal(1520, IslandLayout.ClampedX(1900, 400, 0, 1920));
    }

    [Fact]
    public void Island_wider_than_work_area_pins_to_left_edge()
    {
        Assert.Equal(0, IslandLayout.ClampedX(960, 2200, 0, 1920));
    }

    [Fact]
    public void Morph_around_edge_anchor_stays_on_screen()
    {
        // Anchor dragged to the far right with the compact bar (340 wide),
        // then auto-expand to 460: the expanded panel must not overflow.
        var compactLeft = IslandLayout.ClampedX(1900, 340, 0, 1920);
        Assert.Equal(1580, compactLeft);
        var expandedLeft = IslandLayout.ClampedX(1900, 460, 0, 1920);
        Assert.Equal(1460, expandedLeft);
    }

    [Theory]
    [InlineData(-500, 0)]
    [InlineData(900, 900)]
    [InlineData(5000, 1520)]
    public void ClampLeft_bounds_raw_drag_positions(int x, int expected)
    {
        Assert.Equal(expected, IslandLayout.ClampLeft(x, 400, 0, 1920));
    }

    [Fact]
    public void Fit_caps_oversized_expansion_to_the_work_area()
    {
        // 640-DIP expansion at 200% scale = 1280 physical; on a 1024-wide
        // work area the island must shrink to fit, not overflow the right edge.
        Assert.Equal(new IslandSize(1024, 540),
            IslandLayout.Fit(new IslandSize(1280, 540), 1024, 728));
        Assert.Equal(new IslandSize(700, 540),
            IslandLayout.Fit(new IslandSize(700, 540), 1920, 1040));
    }

    [Fact]
    public void Edge_anchored_expansion_stays_inside_the_work_area()
    {
        // Compact pill dragged flush right, then auto-expand to the largest
        // content-driven size: fit + clamp must keep both edges on screen.
        var compact = IslandMetrics.Compact(1.0);
        var expanded = IslandMetrics.ExpandedFor(
            IslandMetrics.ExpandedMaxWidthDips, IslandMetrics.ExpandedMaxHeightDips, 1.0);
        var compactLeft = IslandLayout.ClampLeft(1920, compact.Width, 0, 1920);
        var anchor = compactLeft + compact.Width / 2;

        var fitted = IslandLayout.Fit(expanded, 1920, 1040);
        var left = IslandLayout.ClampedX(anchor, fitted.Width, 0, 1920);
        Assert.True(left >= 0);
        Assert.True(left + fitted.Width <= 1920);
    }
}
