using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class PanelPresentationTests
{
    [Theory]
    [InlineData("approval", "Permission request", "warning")]
    [InlineData("question", "Question", "info")]
    [InlineData("already-handled", "Handled elsewhere", "muted")]
    public void Kind_maps_to_label_and_accent(string kind, string label, string accent)
    {
        Assert.Equal(label, PanelPresentation.KindLabel(kind));
        Assert.Equal(accent, PanelPresentation.KindAccent(kind));
    }

    [Fact]
    public void Unknown_kind_falls_back_to_raw_kind_and_muted()
    {
        Assert.Equal("mystery", PanelPresentation.KindLabel("mystery"));
        Assert.Equal("muted", PanelPresentation.KindAccent("mystery"));
    }

    [Theory]
    [InlineData("abc", "abc")]
    [InlineData("12345678", "12345678")]
    [InlineData("123456789abcdef", "12345678")]
    public void Session_id_is_truncated_to_eight_chars(string id, string expected)
    {
        Assert.Equal(expected, PanelPresentation.ShortSessionId(id));
    }

    [Theory]
    [InlineData(SessionPhase.Running, "Running", "success")]
    [InlineData(SessionPhase.Waiting, "Waiting", "warning")]
    [InlineData(SessionPhase.Failed, "Failed", "danger")]
    [InlineData(SessionPhase.Done, "Done", "muted")]
    public void Phase_maps_to_label_and_accent(SessionPhase phase, string label, string accent)
    {
        Assert.Equal(label, PanelPresentation.PhaseLabel(phase));
        Assert.Equal(accent, PanelPresentation.PhaseAccent(phase));
    }

    private static PanelGlanceView View(
        bool offline = false,
        int pending = 0,
        params SessionPhase[] phases)
    {
        var snap = new SupervisionSnapshot(
            phases.Select((p, i) => new SessionSummary($"s{i}", p)).ToList(),
            Enumerable.Range(0, pending)
                .Select(i => new ApprovalItem($"a{i}", "s0", $"cmd {i}")).ToList(),
            Array.Empty<QuestionItem>(),
            Array.Empty<AlreadyHandledNotice>(),
            ConnectionFailed: offline,
            ConnectionFailureReason: offline ? "boom" : null);
        return PanelGlanceView.From(snap);
    }

    [Fact]
    public void Compact_summary_prioritizes_offline_over_everything()
    {
        var s = PanelPresentation.Compact(
            View(offline: true, pending: 2, phases: new[] { SessionPhase.Running }));
        Assert.Equal(("offline", "danger"), (s.Text, s.AccentToken));
    }

    [Fact]
    public void Compact_summary_shows_pending_before_session_phases()
    {
        var s = PanelPresentation.Compact(
            View(pending: 2, phases: new[] { SessionPhase.Running }));
        Assert.Equal(("2 waiting", "warning"), (s.Text, s.AccentToken));
    }

    [Fact]
    public void Compact_summary_ranks_failed_then_working_then_done_then_idle()
    {
        Assert.Equal(("1 failed", "danger"),
            Summary(View(phases: new[] { SessionPhase.Failed, SessionPhase.Running })));
        Assert.Equal(("2 working", "success"),
            Summary(View(phases: new[] { SessionPhase.Running, SessionPhase.Running, SessionPhase.Done })));
        Assert.Equal(("done", "success"),
            Summary(View(phases: new[] { SessionPhase.Done, SessionPhase.Done })));
        Assert.Equal(("2 session(s)", "muted"),
            Summary(View(phases: new[] { SessionPhase.Waiting, SessionPhase.Done })));
        Assert.Equal(("idle", "muted"), Summary(View()));

        static (string, string) Summary(PanelGlanceView v)
        {
            var s = PanelPresentation.Compact(v);
            return (s.Text, s.AccentToken);
        }
    }

    [Fact]
    public void Session_label_prefers_title_and_falls_back_to_short_id()
    {
        Assert.Equal("build fix", PanelPresentation.SessionLabel(
            new SessionSummary("123456789abcdef", SessionPhase.Running, "build fix")));
        Assert.Equal("12345678", PanelPresentation.SessionLabel(
            new SessionSummary("123456789abcdef", SessionPhase.Running)));
        Assert.Equal("12345678", PanelPresentation.SessionLabel(
            new SessionSummary("123456789abcdef", SessionPhase.Running, "  ")));
    }
}
