using System.Xml.Linq;
using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Platform.Toast;
using AgentGrid.Shell.WinUI.Panel;
using Microsoft.Windows.AppNotifications;
using Microsoft.Windows.AppNotifications.Builder;

namespace AgentGrid.Shell.WinUI.Toast;

/// <summary>
/// AppNotificationManager-backed toast (FR-TOAST-01).
/// Unpackaged COM background activation is registered when possible; on failure
/// S3 gate falls back to panel-expand-only actions (docs/s3-prototypes-gate.md).
/// </summary>
public sealed class AppNotificationToastService : IToastNotifier, IDisposable
{
    public const string ActionApprove = "approve";
    public const string ActionDeny = "deny";
    public const string ActionExpand = "expand";
    public const string ActionAnswer = "answer";

    private readonly ShellCompositionRoot _root;
    private readonly Func<PanelWindow?> _panel;
    private readonly SupervisionToastRouter _router;
    private bool _registered;
    private bool _comActivationOk;

    public AppNotificationToastService(ShellCompositionRoot root, Func<PanelWindow?> panel)
    {
        _root = root;
        _panel = panel;
        _router = new SupervisionToastRouter(
            root.ToastDecisions,
            this,
            panelFlyoutOpen: () => panel()?.IsFlyoutOpen ?? false,
            panelHwnd: () => panel()?.Hwnd ?? 0);

        root.Supervision.SnapshotChanged += snap =>
            _ = _router.OnSnapshotAsync(snap);
    }

    public bool ComActivationRegistered => _comActivationOk;

    public void Register()
    {
        if (_registered) return;
        try
        {
            // Unpackaged apps: Register() associates process with toast COM callback.
            AppNotificationManager.Default.NotificationInvoked += OnNotificationInvoked;
            AppNotificationManager.Default.Register();
            _comActivationOk = true;
            _registered = true;
        }
        catch (Exception ex)
        {
            // S3 fail path: toast may still show while process is running, but
            // cold-start COM activation is unavailable → treat as panel-expand only.
            System.Diagnostics.Debug.WriteLine($"AppNotification Register failed: {ex.Message}");
            _comActivationOk = false;
            _registered = true;
            try
            {
                AppNotificationManager.Default.NotificationInvoked += OnNotificationInvoked;
            }
            catch
            {
                /* fully unavailable */
            }
        }
    }

    public async Task ShowApprovalAsync(ApprovalItem item, CancellationToken ct = default)
    {
        var builder = new AppNotificationBuilder()
            .AddText("Approval required")
            .AddText(Truncate(item.Summary, 120))
            .SetTag(item.ApprovalId)
            .SetGroup("agent-grid-supervision");

        if (_comActivationOk)
        {
            builder
                .AddButton(new AppNotificationButton("Approve")
                    .AddArgument("action", ActionApprove)
                    .AddArgument("id", item.ApprovalId)
                    .AddArgument("session", item.SessionId)
                    .AddArgument("summary", Truncate(item.Summary, 80)))
                .AddButton(new AppNotificationButton("Deny")
                    .AddArgument("action", ActionDeny)
                    .AddArgument("id", item.ApprovalId)
                    .AddArgument("session", item.SessionId)
                    .AddArgument("summary", Truncate(item.Summary, 80)));
        }
        else
        {
            // Fail-soft: expand panel only (S3 gate fallback).
            builder.AddButton(new AppNotificationButton("Open panel")
                .AddArgument("action", ActionExpand)
                .AddArgument("id", item.ApprovalId));
        }

        Show(builder.BuildNotification());
        await Task.CompletedTask;
    }

    public async Task ShowQuestionAsync(QuestionItem item, CancellationToken ct = default)
    {
        var builder = new AppNotificationBuilder()
            .AddText("Question")
            .AddText(Truncate(item.Prompt, 120))
            .SetTag(item.QuestionId)
            .SetGroup("agent-grid-supervision");

        // Inline textbox IME is S3 prototype assumption; default to expand-panel.
        // When COM ok we still offer Expand; IME textbox can be enabled after gate PASS.
        builder.AddButton(new AppNotificationButton("Answer in panel")
            .AddArgument("action", ActionExpand)
            .AddArgument("id", item.QuestionId)
            .AddArgument("kind", "question"));

        Show(builder.BuildNotification());
        await Task.CompletedTask;
    }

    public async Task DismissAsync(string itemId, CancellationToken ct = default)
    {
        try
        {
            AppNotificationManager.Default.RemoveByTagAsync(itemId).AsTask().GetAwaiter().GetResult();
        }
        catch
        {
            /* best-effort */
        }
        await Task.CompletedTask;
    }

    public void Dispose()
    {
        try
        {
            if (_registered)
                AppNotificationManager.Default.Unregister();
        }
        catch
        {
            /* ignore */
        }
    }

    private void Show(AppNotification notification)
    {
        try
        {
            AppNotificationManager.Default.Show(notification);
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"toast show failed: {ex.Message}");
            _panel()?.ShowGlance();
        }
    }

    private void OnNotificationInvoked(
        AppNotificationManager sender,
        AppNotificationActivatedEventArgs args)
    {
        var action = GetArg(args, "action");
        var id = GetArg(args, "id");
        var session = GetArg(args, "session");
        var summary = GetArg(args, "summary") ?? "";

        switch (action)
        {
            case ActionApprove when id is not null && session is not null:
                _ = _root.Supervision.SubmitApprovalAsync(id, session, "accept", summary);
                break;
            case ActionDeny when id is not null && session is not null:
                _ = _root.Supervision.SubmitApprovalAsync(id, session, "deny", summary);
                break;
            case ActionExpand:
            default:
                _panel()?.ShowGlance();
                break;
        }
    }

    private static string? GetArg(AppNotificationActivatedEventArgs args, string key) =>
        args.Arguments.TryGetValue(key, out var v) ? v : null;

    private static string Truncate(string s, int max) =>
        s.Length <= max ? s : s[..(max - 1)] + "…";
}

/// <summary>
/// Pure XML builder used by unit tests without WinRT (wire-format contract).
/// </summary>
public static class ToastXml
{
    public static string ApprovalToastXml(string id, string session, string summary, bool withButtons)
    {
        var text = new XElement("text", summary);
        var visual = new XElement("visual", new XElement("binding", new XAttribute("template", "ToastGeneric"), text));
        var toast = new XElement("toast", visual);
        if (withButtons)
        {
            toast.Add(new XElement("actions",
                new XElement("action",
                    new XAttribute("content", "Approve"),
                    new XAttribute("arguments", $"action={AppNotificationToastService.ActionApprove};id={id};session={session}")),
                new XElement("action",
                    new XAttribute("content", "Deny"),
                    new XAttribute("arguments", $"action={AppNotificationToastService.ActionDeny};id={id};session={session}"))));
        }
        else
        {
            toast.Add(new XElement("actions",
                new XElement("action",
                    new XAttribute("content", "Open panel"),
                    new XAttribute("arguments", $"action={AppNotificationToastService.ActionExpand};id={id}"))));
        }
        return toast.ToString(SaveOptions.DisableFormatting);
    }
}
