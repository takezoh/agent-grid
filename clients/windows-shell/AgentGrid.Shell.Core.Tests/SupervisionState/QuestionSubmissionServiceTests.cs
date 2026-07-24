using System.Net;
using System.Text;
using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class QuestionSubmissionServiceTests
{
    [Fact]
    public async Task Submit_question_success_removes_item()
    {
        var http = new Scripted(req =>
        {
            if (req.RequestUri!.AbsolutePath.Contains("/questions/", StringComparison.Ordinal))
                return new HttpResponseMessage(HttpStatusCode.OK)
                {
                    Content = new StringContent("{}", Encoding.UTF8, "application/json"),
                };
            throw new InvalidOperationException(req.RequestUri.ToString());
        });
        using var clientHttp = new HttpClient(http) { BaseAddress = new Uri("http://127.0.0.1:9") };
        var gateway = new ShellGatewayClient(new Uri("http://127.0.0.1:9"), new StaticToken("t"), clientHttp);
        var svc = new ApprovalSubmissionService(gateway);
        svc.Apply(new EvtQuestionRequested("q1", "s1", "Continue?"));
        Assert.Single(svc.Snapshot.Questions);

        await svc.SubmitQuestionAsync("q1", "s1", "yes", "Continue?");
        Assert.Empty(svc.Snapshot.Questions);
    }

    [Fact]
    public async Task Submit_question_network_error_restores_item()
    {
        var http = new Scripted(_ => throw new HttpRequestException("down"));
        using var clientHttp = new HttpClient(http) { BaseAddress = new Uri("http://127.0.0.1:9") };
        var gateway = new ShellGatewayClient(new Uri("http://127.0.0.1:9"), new StaticToken("t"), clientHttp);
        var svc = new ApprovalSubmissionService(gateway);
        svc.Apply(new EvtQuestionRequested("q1", "s1", "Continue?"));

        await svc.SubmitQuestionAsync("q1", "s1", "yes", "Continue?");
        Assert.Single(svc.Snapshot.Questions);
        Assert.Equal("Continue?", svc.Snapshot.Questions[0].Prompt);
    }

    private sealed class StaticToken : ITokenSource
    {
        private readonly string _t;
        public StaticToken(string t) => _t = t;
        public Task<string> ReadFreshAsync(CancellationToken ct = default) => Task.FromResult(_t);
    }

    private sealed class Scripted : HttpMessageHandler
    {
        private readonly Func<HttpRequestMessage, HttpResponseMessage> _fn;
        public Scripted(Func<HttpRequestMessage, HttpResponseMessage> fn) => _fn = fn;
        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken) =>
            Task.FromResult(_fn(request));
    }
}
