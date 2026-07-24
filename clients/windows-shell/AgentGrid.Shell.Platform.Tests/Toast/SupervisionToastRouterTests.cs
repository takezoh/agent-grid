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
