using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Boundary-2 supervision surface: mint ticket → WS connect → parse frames →
/// feed ApprovalSubmissionService, with full-jitter reconnect backoff
/// (contract-b3-restart-continuity, ADR-0012 pattern).
/// </summary>
public sealed class SupervisionWsSession : ISurfaceResubscriber, IAsyncDisposable
{
    private readonly ShellGatewayClient _gateway;
    private readonly ApprovalSubmissionService _supervision;
    private readonly IWebSocketTransportFactory _wsFactory;
    private readonly TimeSpan _maxBackoff;
    private readonly Func<int, TimeSpan> _backoff;
    private CancellationTokenSource? _loopCts;
    private Task? _loopTask;
    private int _attempt;

    public SupervisionWsSession(
        ShellGatewayClient gateway,
        ApprovalSubmissionService supervision,
        IWebSocketTransportFactory? wsFactory = null,
        TimeSpan? maxBackoff = null,
        Func<int, TimeSpan>? backoff = null)
    {
        _gateway = gateway;
        _supervision = supervision;
        _wsFactory = wsFactory ?? new ClientWebSocketTransportFactory();
        _maxBackoff = maxBackoff ?? TimeSpan.FromSeconds(30);
        _backoff = backoff ?? DefaultFullJitter;
    }

    public ApprovalSubmissionService Supervision => _supervision;

    /// <summary>Start the receive/reconnect loop (non-blocking).</summary>
    public void Start()
    {
        if (_loopTask is not null)
            return;
        _loopCts = new CancellationTokenSource();
        _loopTask = Task.Run(() => RunLoopAsync(_loopCts.Token));
    }

    /// <summary>contract-b3-restart-continuity: re-mint ticket and reconnect all surfaces.</summary>
    public Task ResubscribeAllAsync(CancellationToken ct = default)
    {
        // Force the loop to drop and reconnect by cancelling current connect.
        // A dedicated restart signal is simpler: cancel + restart loop.
        StopLoop();
        Start();
        return Task.CompletedTask;
    }

    public async Task StopAsync()
    {
        StopLoop();
        if (_loopTask is not null)
        {
            try
            {
                await _loopTask.ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
                /* expected */
            }
            _loopTask = null;
        }
    }

    public async ValueTask DisposeAsync() => await StopAsync().ConfigureAwait(false);

    /// <summary>
    /// Single connect→receive session (testable without background loop).
    /// Returns after close or cancel. Applies connection failed/restored events.
    /// </summary>
    public async Task RunOnceAsync(CancellationToken ct = default)
    {
        IWebSocketTransport? ws = null;
        try
        {
            var (ticket, _) = await _gateway.MintWsTicketAsync(ct).ConfigureAwait(false);
            ws = _wsFactory.Create();
            await ws.ConnectAsync(_gateway.WebSocketUri(ticket), ct).ConfigureAwait(false);
            _supervision.Apply(new EvtConnectionRestored());
            _attempt = 0;

            while (!ct.IsCancellationRequested)
            {
                var text = await ws.ReceiveTextAsync(ct).ConfigureAwait(false);
                if (text is null)
                    break;
                var ev = WsFrameParser.TryParse(text);
                if (ev is not null)
                    _supervision.Apply(ev);
            }
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            _supervision.Apply(new EvtConnectionFailed(ex.Message));
            throw;
        }
        finally
        {
            if (ws is not null)
                await ws.DisposeAsync().ConfigureAwait(false);
        }
    }

    private async Task RunLoopAsync(CancellationToken ct)
    {
        while (!ct.IsCancellationRequested)
        {
            try
            {
                await RunOnceAsync(ct).ConfigureAwait(false);
                // Clean close → still reconnect (daemon may have restarted).
                _supervision.Apply(new EvtConnectionFailed("websocket closed"));
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                return;
            }
            catch
            {
                // Failure already recorded in RunOnceAsync.
            }

            _attempt++;
            var delay = _backoff(_attempt);
            if (delay > _maxBackoff)
                delay = _maxBackoff;
            try
            {
                await Task.Delay(delay, ct).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                return;
            }
        }
    }

    private void StopLoop()
    {
        try
        {
            _loopCts?.Cancel();
        }
        catch
        {
            /* ignore */
        }
        _loopCts?.Dispose();
        _loopCts = null;
    }

    /// <summary>Full-jitter exponential: min(cap, random * 2^attempt * 100ms).</summary>
    public static TimeSpan DefaultFullJitter(int attempt)
    {
        var exp = Math.Min(attempt, 8);
        var ceilingMs = 100 * Math.Pow(2, exp);
        var ms = Random.Shared.NextDouble() * ceilingMs;
        return TimeSpan.FromMilliseconds(Math.Max(50, ms));
    }
}
