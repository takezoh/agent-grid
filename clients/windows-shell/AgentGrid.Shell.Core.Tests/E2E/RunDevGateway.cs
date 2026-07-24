namespace AgentGrid.Shell.Core.Tests.E2E;

/// <summary>
/// Opt-in T3 fidelity against the repo-standard WSL stack
/// (<c>make run-dev</c> / <c>scripts/run-dev.sh</c>, loopback -no-auth).
///
/// Enable with <c>AG_E2E_RUN_DEV=1</c>. Gateway URL defaults to
/// <c>http://127.0.0.1:8443</c> (<c>AG_E2E_GATEWAY_URL</c> override).
/// </summary>
public static class RunDevGateway
{
    public const string EnableEnv = "AG_E2E_RUN_DEV";
    public const string UrlEnv = "AG_E2E_GATEWAY_URL";
    public const string DefaultUrl = "http://127.0.0.1:8443";

    public static bool Enabled
    {
        get
        {
            var v = Environment.GetEnvironmentVariable(EnableEnv);
            return v is "1" or "true" or "TRUE" or "yes" or "YES";
        }
    }

    public static Uri BaseUri =>
        new(Environment.GetEnvironmentVariable(UrlEnv) ?? DefaultUrl);

    /// <summary>Skip message for xUnit when the e2e fixture is not requested.</summary>
    public static string SkipUnlessEnabled()
    {
        if (Enabled)
            return string.Empty;
        return $"set {EnableEnv}=1 and run `make run-dev` (scripts/run-dev.sh) first";
    }

    public static async Task EnsureReachableAsync(CancellationToken ct = default)
    {
        using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(3) };
        using var res = await http.GetAsync(new Uri(BaseUri, "/api/sessions"), ct)
            .ConfigureAwait(false);
        if (!res.IsSuccessStatusCode)
        {
            throw new InvalidOperationException(
                $"run-dev gateway not ready at {BaseUri} (status {(int)res.StatusCode}). " +
                "Start with: make run-dev");
        }
    }
}
