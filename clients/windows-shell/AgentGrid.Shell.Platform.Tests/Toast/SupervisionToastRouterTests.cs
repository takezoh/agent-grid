using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Platform.Interop;
using AgentGrid.Shell.Platform.Toast;

namespace AgentGrid.Shell.Platform.Tests.Toast;

public class SupervisionToastRouterTests
{
    [Fact]
    public async Task Fires_once_per_new_approval_when_unwatched()
    {
        var notifier = new RecordingNotifier();
        var decision = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var router = new SupervisionToastRouter(decision, notifier, () => false, () => 0);

        var snap = SupervisionSnapshot.Empty with
        {
            Approvals = new[] { new ApprovalItem("ap1", "s1", "echo") },
        };
        await router.OnSnapshotAsync(snap);
        await router.OnSnapshotAsync(snap); // same id — no second toast
        Assert.Single(notifier.Approvals);

        var snap2 = snap with
        {
            Approvals = new[]
            {
                new ApprovalItem("ap1", "s1", "echo"),
                new ApprovalItem("ap2", "s1", "rm"),
            },
        };
        await router.OnSnapshotAsync(snap2);
        Assert.Equal(2, notifier.Approvals.Count);
    }

    [Fact]
    public async Task Suppresses_when_panel_watched()
    {
        var notifier = new RecordingNotifier();
        var decision = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var router = new SupervisionToastRouter(decision, notifier, () => true, () => 0);
        var snap = SupervisionSnapshot.Empty with
        {
            Approvals = new[] { new ApprovalItem("ap1", "s1", "echo") },
        };
        await router.OnSnapshotAsync(snap);
        Assert.Empty(notifier.Approvals);
    }

    // witness-unwatched-toast-normal / witness-frame-flood-adversarial:
    // session-only churn (continuous v) must not re-fire a toast for a seen id.
    [Fact]
    public async Task Session_churn_snapshots_do_not_retoast_seen_approval()
    {
        var notifier = new RecordingNotifier();
        var decision = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var router = new SupervisionToastRouter(decision, notifier, () => false, () => 0);

        var snap = SupervisionSnapshot.Empty with
        {
            Approvals = new[] { new ApprovalItem("ap1", "s1", "echo") },
            Sessions = new[] { new SessionSummary("s1", SessionPhase.Waiting, "T1") },
        };
        await router.OnSnapshotAsync(snap);
        Assert.Single(notifier.Approvals);

        // Continuous v-style session updates while the same approval remains.
        for (var i = 0; i < 5; i++)
        {
            var churn = snap with
            {
                Sessions = new[] { new SessionSummary("s1", SessionPhase.Running, $"T{i}") },
            };
            await router.OnSnapshotAsync(churn);
        }
        Assert.Single(notifier.Approvals);
    }

    // witness-frame-flood-adversarial: flyout open/close interleave after first fire.
    [Fact]
    public async Task Flyout_open_close_after_fire_does_not_retoast()
    {
        var flyoutOpen = false;
        var notifier = new RecordingNotifier();
        var decision = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var router = new SupervisionToastRouter(decision, notifier, () => flyoutOpen, () => 0);

        var snap = SupervisionSnapshot.Empty with
        {
            Approvals = new[] { new ApprovalItem("ap1", "s1", "echo") },
        };
        await router.OnSnapshotAsync(snap);
        Assert.Single(notifier.Approvals);

        flyoutOpen = true;
        await router.OnSnapshotAsync(snap);
        flyoutOpen = false;
        await router.OnSnapshotAsync(snap);
        await router.OnSnapshotAsync(snap);
        Assert.Single(notifier.Approvals); // still exactly once
    }

    // witness-frame-flood-adversarial: watched arrival suppresses fire; later unwatched
    // re-apply of the same id must not spam a late toast (seen-at-first-observation).
    [Fact]
    public async Task Watched_arrival_then_unwatched_reapply_does_not_fire()
    {
        var flyoutOpen = true;
        var notifier = new RecordingNotifier();
        var decision = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var router = new SupervisionToastRouter(decision, notifier, () => flyoutOpen, () => 0);

        var snap = SupervisionSnapshot.Empty with
        {
            Approvals = new[] { new ApprovalItem("ap1", "s1", "echo") },
        };
        await router.OnSnapshotAsync(snap);
        Assert.Empty(notifier.Approvals);

        flyoutOpen = false;
        await router.OnSnapshotAsync(snap);
        Assert.Empty(notifier.Approvals);
    }

    // CONTRACT DEVIATION PIN (as-is behaviour, 2026-07-26).
    //
    // design-plan.json declares the invariant 「同一 pending のトーストは高々 1 回」
    // (contract-island-attention-observability; witness-unwatched-toast-normal forbids
    // "2 通目のトースト"). That invariant does NOT hold on the offline-rollback path:
    //
    //   SupervisionToastRouter.OnSnapshotAsync ends with
    //     _seen.RemoveWhere(id => !live.Contains(id))
    //   so an id that leaves the queue is forgotten. ApprovalSubmissionService
    //   .SubmitApprovalAsync applies TWO events on a network failure —
    //   EvtApprovalSubmitRequested (optimistic removal) then EvtApprovalSubmitFailed
    //   (rollback restore) — and each raises SnapshotChanged, which
    //   AppNotificationToastService forwards to this router. With the panel unwatched
    //   (e.g. the operator answered via the toast button, UAC-031's own path) the same
    //   ApprovalId is dropped from _seen and re-added, so a SECOND toast fires.
    //
    // The other toast tests all keep the id continuously present, so they only pin
    // orderings where _seen is never cleared. This test pins the real behaviour so the
    // deviation is visible and any future change to it is deliberate.
    // adr-20260725-vibe-island-asis-contract-pin forbids fixing this here (it needs a
    // production change: keep _seen entries until the id is *resolved*, not merely
    // absent). Recorded in verification.md as a contract↔implementation gap.
    [Fact]
    public async Task Rollback_restore_while_unwatched_refires_toast_as_is()
    {
        var notifier = new RecordingNotifier();
        var decision = new ToastDecisionService(new PanelWatchedPredicate(new OpenUnlocked()));
        var router = new SupervisionToastRouter(decision, notifier, () => false, () => 0);

        var withPending = SupervisionSnapshot.Empty with
        {
            Approvals = new[] { new ApprovalItem("ap1", "s1", "echo") },
        };

        // Arrival while unwatched → first toast.
        await router.OnSnapshotAsync(withPending);
        Assert.Single(notifier.Approvals);

        // Optimistic removal (EvtApprovalSubmitRequested) → id leaves the queue.
        await router.OnSnapshotAsync(SupervisionSnapshot.Empty);
        Assert.Single(notifier.Approvals);

        // Rollback restore (EvtApprovalSubmitFailed) → same id returns.
        await router.OnSnapshotAsync(withPending);

        // As-is: a second toast for one pending. If a future change makes this 1,
        // that is the invariant being honoured — update this test and the
        // verification.md deviation note together.
        Assert.Equal(2, notifier.Approvals.Count);
        Assert.All(notifier.Approvals, a => Assert.Equal("ap1", a.ApprovalId));
    }

    private sealed class RecordingNotifier : IToastNotifier
    {
        public List<ApprovalItem> Approvals { get; } = new();
        public Task ShowApprovalAsync(ApprovalItem item, CancellationToken ct = default)
        {
            Approvals.Add(item);
            return Task.CompletedTask;
        }
        public Task ShowQuestionAsync(QuestionItem item, CancellationToken ct = default) =>
            Task.CompletedTask;
        public Task DismissAsync(string itemId, CancellationToken ct = default) =>
            Task.CompletedTask;
    }

    private sealed class OpenUnlocked : IWin32InteropService
    {
        public nint GetForegroundWindow() => 0;
        public bool IsWindow(nint hwnd) => true;
        public bool SetForegroundWindow(nint hwnd) => true;
        public bool AllowSetForegroundWindow(int processId) => true;
        public uint GetWindowThreadProcessId(nint hwnd, out uint processId)
        {
            processId = 1;
            return 1;
        }
        public bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach) => true;
        public bool SetNoActivate(nint hwnd, bool noActivate) => true;
        public string? GetWindowProcessName(nint hwnd) => null;
        public string? GetWindowTitle(nint hwnd) => null;
        public IReadOnlyList<WindowInfo> EnumerateWindows() => Array.Empty<WindowInfo>();
        public bool IsSessionLocked() => false;
        public bool IsDoNotDisturb() => false;
        public bool TryGetNotificationsAllowed(out bool allowed)
        {
            allowed = true;
            return true;
        }
    }
}
