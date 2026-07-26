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

    [Fact]
    public void Session_label_excludes_server_id()
    {
        var session = new SessionSummary(
            "123456789abcdef", SessionPhase.Running, Title: "server title", ServerId: "remote");

        Assert.Equal("server title", PanelPresentation.SessionLabel(session));
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

    [Theory]
    [InlineData(SessionPhase.Running, "running")]
    [InlineData(SessionPhase.Waiting, "waiting")]
    [InlineData(SessionPhase.Failed, "stopped")]
    [InlineData(SessionPhase.Done, "idle")]
    public void Phase_maps_to_workspace_status_icon_kind(SessionPhase phase, string visualKind)
    {
        Assert.Equal(visualKind, PanelPresentation.PhaseVisualKind(phase));
    }

    [Fact]
    public void Composite_session_precedence_matches_shared_status_matrix()
    {
        Assert.Equal("action", PanelPresentation.SessionVisualKind(
            new SessionSummary("a", SessionPhase.Running, HumanInput: true)));
        Assert.Equal("failed", PanelPresentation.SessionVisualKind(
            new SessionSummary("f", SessionPhase.Failed, HumanInput: true)));
        Assert.Equal("stopped", PanelPresentation.SessionVisualKind(
            new SessionSummary("s", SessionPhase.Done, Lifecycle: "stopped")));
        Assert.Equal("idle", PanelPresentation.SessionVisualKind(
            new SessionSummary("i", SessionPhase.Done, Lifecycle: "idle")));
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

    private static PanelGlanceView ViewWith(
        bool offline = false,
        int approvals = 0,
        int questions = 0,
        int notices = 0,
        int sessions = 0)
    {
        var snap = new SupervisionSnapshot(
            Enumerable.Range(0, sessions)
                .Select(i => new SessionSummary($"s{i}", SessionPhase.Running)).ToList(),
            Enumerable.Range(0, approvals)
                .Select(i => new ApprovalItem($"a{i}", "s0", $"cmd {i}")).ToList(),
            Enumerable.Range(0, questions)
                .Select(i => new QuestionItem($"q{i}", "s0", $"ask {i}")).ToList(),
            Enumerable.Range(0, notices)
                .Select(i => new AlreadyHandledNotice($"n{i}", "s0", "approval", "handled")).ToList(),
            ConnectionFailed: offline,
            ConnectionFailureReason: offline ? "boom" : null);
        return PanelGlanceView.From(snap);
    }

    [Fact]
    public void Sections_render_nothing_when_there_is_nothing()
    {
        // Vibe Island: no content → no sections. No empty-state filler,
        // no headers, no healthy-state connection badge.
        var s = PanelPresentation.Sections(ViewWith());
        Assert.Equal(new PanelSections(false, false, false, false, false), s);
    }

    // witness-quiet-normal, presentation-model half: the section flags flip from
    // true (pending present) to false (pending drained).
    //
    // Scope note: this does NOT reject the counterexample "empty ListView kept
    // collapsed". PanelWindow consumes these flags as
    // `PendingList.Visibility = sections.ShowPending ? Visible : Collapsed`, so a
    // collapsed-but-present element satisfies these assertions. The invariant
    // 「空セクションは可視化制御ではなく要素として生成されない」 is discriminated only by
    // PanelWindowUiE2ETests.Empty_sections_do_not_exist_in_the_tree (T3, not
    // always-on) — recorded as such in verification.md.
    [Fact]
    public void Pending_drain_hides_approval_and_reply_sections()
    {
        var withPending = PanelPresentation.Sections(ViewWith(approvals: 1, questions: 1));
        Assert.True(withPending.ShowPending);
        Assert.True(withPending.ShowReply);

        var drained = PanelPresentation.Sections(ViewWith());
        Assert.False(drained.ShowPending);
        Assert.False(drained.ShowReply);
        Assert.False(drained.ShowNotice);
    }

    [Fact]
    public void Sections_show_only_what_has_content()
    {
        Assert.Equal(new PanelSections(true, false, false, false, false),
            PanelPresentation.Sections(ViewWith(sessions: 2)));
        Assert.Equal(new PanelSections(false, true, false, false, false),
            PanelPresentation.Sections(ViewWith(notices: 1)));
        Assert.Equal(new PanelSections(false, false, false, false, true),
            PanelPresentation.Sections(ViewWith(offline: true)));
    }

    [Fact]
    public void Approval_shows_pending_without_reply_entry()
    {
        var s = PanelPresentation.Sections(ViewWith(sessions: 1, approvals: 1));
        Assert.True(s.ShowPending);
        Assert.False(s.ShowReply);
    }

    [Fact]
    public void Question_shows_pending_with_reply_entry()
    {
        var s = PanelPresentation.Sections(ViewWith(sessions: 1, questions: 1));
        Assert.True(s.ShowPending);
        Assert.True(s.ShowReply);
    }

    [Fact]
    public void Session_label_matches_workspace_title_slot()
    {
        Assert.Equal("build fix", PanelPresentation.SessionLabel(
            new SessionSummary("123456789abcdef", SessionPhase.Running, "build fix")));
        Assert.Equal("", PanelPresentation.SessionLabel(
            new SessionSummary("123456789abcdef", SessionPhase.Running)));
        Assert.Equal("  ", PanelPresentation.SessionLabel(
            new SessionSummary("123456789abcdef", SessionPhase.Running, "  ")));
    }

    [Fact]
    public void Session_label_preserves_workspace_title_content()
    {
        Assert.Equal("/home/dev/dev/dev-skills", PanelPresentation.SessionLabel(
            new SessionSummary("123456789abcdef", SessionPhase.Running, "/home/dev/dev/dev-skills")));
    }
}
