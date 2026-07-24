using System.IO.Pipes;
using System.Net.Sockets;
using System.Text;

namespace AgentGrid.Shell.Core.WorkspaceLauncher;

/// <summary>
/// Boundary-1 client: JSON Lines over named pipe (Windows) or Unix domain socket (WSL/Linux tests).
/// Path defaults: \\.\pipe\agent-grid-workspace / AG_WORKSPACE_CONTROL_PATH.
/// </summary>
public sealed class NamedPipeWorkspaceControlClient : IWorkspaceControlClient, IAsyncDisposable
{
    private readonly string _path;
    private readonly TimeSpan _connectTimeout;

    public NamedPipeWorkspaceControlClient(string? path = null, TimeSpan? connectTimeout = null)
    {
        _path = path ?? DefaultControlPath();
        _connectTimeout = connectTimeout ?? TimeSpan.FromSeconds(2);
    }

    public string Path => _path;

    public static string DefaultControlPath()
    {
        if (OperatingSystem.IsWindows())
            return @"\\.\pipe\agent-grid-workspace";
        return Environment.GetEnvironmentVariable("AG_WORKSPACE_CONTROL_PATH")
               ?? "/tmp/agent-grid-workspace.sock";
    }

    public async Task<ControlReply> SendAsync(ControlEnvelope envelope, CancellationToken ct = default)
    {
        await using var stream = await ConnectAsync(ct).ConfigureAwait(false);
        var line = envelope.ToJsonLine() + "\n";
        var bytes = Encoding.UTF8.GetBytes(line);
        await stream.WriteAsync(bytes, ct).ConfigureAwait(false);
        await stream.FlushAsync(ct).ConfigureAwait(false);

        using var reader = new StreamReader(stream, Encoding.UTF8, detectEncodingFromByteOrderMarks: false, leaveOpen: true);
        var replyLine = await reader.ReadLineAsync(ct).ConfigureAwait(false);
        if (string.IsNullOrEmpty(replyLine))
            return ControlReply.Fail("empty reply");
        return ParseReply(replyLine);
    }

    public ValueTask DisposeAsync() => ValueTask.CompletedTask;

    private async Task<Stream> ConnectAsync(CancellationToken ct)
    {
        if (OperatingSystem.IsWindows() && _path.StartsWith(@"\\.\pipe\", StringComparison.OrdinalIgnoreCase))
        {
            var pipeName = _path[@"\\.\pipe\".Length..];
            var client = new NamedPipeClientStream(
                ".",
                pipeName,
                PipeDirection.InOut,
                PipeOptions.Asynchronous);
            using var linked = CancellationTokenSource.CreateLinkedTokenSource(ct);
            linked.CancelAfter(_connectTimeout);
            await client.ConnectAsync(linked.Token).ConfigureAwait(false);
            return client;
        }

        // Unix domain socket (WSL / Linux CI / tests).
        var socket = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
        try
        {
            using var linked = CancellationTokenSource.CreateLinkedTokenSource(ct);
            linked.CancelAfter(_connectTimeout);
            var ep = new UnixDomainSocketEndPoint(_path);
            await socket.ConnectAsync(ep, linked.Token).ConfigureAwait(false);
            return new NetworkStream(socket, ownsSocket: true);
        }
        catch
        {
            socket.Dispose();
            throw;
        }
    }

    public static ControlReply ParseReply(string line)
    {
        try
        {
            using var doc = System.Text.Json.JsonDocument.Parse(line);
            var root = doc.RootElement;
            if (root.TryGetProperty("ok", out var okEl) && okEl.ValueKind == System.Text.Json.JsonValueKind.True)
                return ControlReply.Success();
            var err = root.TryGetProperty("error", out var e) && e.ValueKind == System.Text.Json.JsonValueKind.String
                ? e.GetString() ?? "error"
                : "error";
            return ControlReply.Fail(err);
        }
        catch (System.Text.Json.JsonException ex)
        {
            return ControlReply.Fail($"malformed reply: {ex.Message}");
        }
    }
}

/// <summary>
/// Spawns the Workspace executable when the control pipe is missing
/// (contract-b1-b2-launch-ordering).
/// </summary>
public sealed class ProcessWorkspaceLauncher : IWorkspaceProcessLauncher
{
    private readonly string _exePath;
    private readonly string[] _args;
    private readonly Func<string, string[], Task> _start;

    public ProcessWorkspaceLauncher(
        string exePath,
        string[]? args = null,
        Func<string, string[], Task>? start = null)
    {
        _exePath = exePath;
        _args = args ?? Array.Empty<string>();
        _start = start ?? DefaultStartAsync;
    }

    public Task SpawnAsync(CancellationToken ct = default) =>
        _start(_exePath, _args).WaitAsync(ct);

    private static Task DefaultStartAsync(string fileName, string[] args)
    {
        var psi = new System.Diagnostics.ProcessStartInfo
        {
            FileName = fileName,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        foreach (var a in args)
            psi.ArgumentList.Add(a);
        System.Diagnostics.Process.Start(psi);
        return Task.CompletedTask;
    }
}
