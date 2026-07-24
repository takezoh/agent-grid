using System.Net.Sockets;
using System.Text;
using AgentGrid.Shell.Core.WorkspaceLauncher;

namespace AgentGrid.Shell.Core.Tests.WorkspaceLauncher;

public class NamedPipeClientTests
{
    [Fact]
    public async Task Round_trip_openSession_over_unix_socket()
    {
        if (OperatingSystem.IsWindows())
        {
            // Named-pipe server side lives in Electron; this test exercises the Unix path
            // used when developing under WSL. Windows path is covered by Process spawn wiring.
        }

        var sockPath = Path.Combine(Path.GetTempPath(), $"ag-ctrl-{Guid.NewGuid():N}.sock");
        if (File.Exists(sockPath))
            File.Delete(sockPath);

        await using var server = await StartEchoServerAsync(sockPath);
        try
        {
            var client = new NamedPipeWorkspaceControlClient(sockPath, TimeSpan.FromSeconds(3));
            var reply = await client.SendAsync(new ControlEnvelope { Op = "openSession", Id = "sess-1" });
            Assert.True(reply.Ok);
        }
        finally
        {
            try { File.Delete(sockPath); } catch { /* ignore */ }
        }
    }

    [Fact]
    public void ParseReply_shapes()
    {
        var ok = NamedPipeWorkspaceControlClient.ParseReply("""{"ok":true}""");
        Assert.True(ok.Ok);
        var bad = NamedPipeWorkspaceControlClient.ParseReply("""{"ok":false,"error":"nope"}""");
        Assert.False(bad.Ok);
        Assert.Equal("nope", bad.Error);
    }

    [Fact]
    public async Task WorkspaceLauncher_retries_after_spawn()
    {
        var attempts = 0;
        var client = new ScriptedControlClient(() =>
        {
            attempts++;
            if (attempts < 2)
                throw new IOException("pipe missing");
            return ControlReply.Success();
        });
        var spawned = 0;
        var launcher = new WorkspaceLauncherService(
            client,
            new ScriptedProcess(() => { spawned++; return Task.CompletedTask; }),
            maxAttempts: 3,
            initialBackoff: TimeSpan.FromMilliseconds(1));

        var reply = await launcher.OpenSessionAsync("s1");
        Assert.True(reply.Ok);
        Assert.Equal(1, spawned);
        Assert.Equal(2, attempts);
    }

    private static Task<IAsyncDisposable> StartEchoServerAsync(string path)
    {
        var socket = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
        socket.Bind(new UnixDomainSocketEndPoint(path));
        socket.Listen(4);
        var cts = new CancellationTokenSource();
        var task = Task.Run(async () =>
        {
            while (!cts.IsCancellationRequested)
            {
                Socket accepted;
                try
                {
                    accepted = await socket.AcceptAsync(cts.Token).ConfigureAwait(false);
                }
                catch (OperationCanceledException)
                {
                    break;
                }
                _ = Task.Run(async () =>
                {
                    await using var ns = new NetworkStream(accepted, ownsSocket: true);
                    using var reader = new StreamReader(ns, Encoding.UTF8, leaveOpen: true);
                    _ = await reader.ReadLineAsync().ConfigureAwait(false);
                    var reply = Encoding.UTF8.GetBytes("""{"ok":true}""" + "\n");
                    await ns.WriteAsync(reply).ConfigureAwait(false);
                    await ns.FlushAsync().ConfigureAwait(false);
                });
            }
        }, cts.Token);

        IAsyncDisposable d = new AsyncDisposeAction(async () =>
        {
            cts.Cancel();
            socket.Dispose();
            try { await task.ConfigureAwait(false); } catch { /* ignore */ }
            cts.Dispose();
        });
        return Task.FromResult(d);
    }

    private sealed class AsyncDisposeAction : IAsyncDisposable
    {
        private readonly Func<Task> _fn;
        public AsyncDisposeAction(Func<Task> fn) => _fn = fn;
        public ValueTask DisposeAsync() => new(_fn());
    }

    private sealed class ScriptedControlClient : IWorkspaceControlClient
    {
        private readonly Func<ControlReply> _fn;
        public ScriptedControlClient(Func<ControlReply> fn) => _fn = fn;
        public Task<ControlReply> SendAsync(ControlEnvelope envelope, CancellationToken ct = default) =>
            Task.FromResult(_fn());
    }

    private sealed class ScriptedProcess : IWorkspaceProcessLauncher
    {
        private readonly Func<Task> _fn;
        public ScriptedProcess(Func<Task> fn) => _fn = fn;
        public Task SpawnAsync(CancellationToken ct = default) => _fn();
    }
}
