namespace AgentGrid.Shell.Core.Tests.E2E;

/// <summary>
/// xUnit Fact that is skipped unless <c>AG_E2E_RUN_DEV=1</c>.
/// </summary>
[AttributeUsage(AttributeTargets.Method, AllowMultiple = false)]
public sealed class RunDevFactAttribute : FactAttribute
{
    public RunDevFactAttribute()
    {
        if (!RunDevGateway.Enabled)
            Skip = RunDevGateway.SkipUnlessEnabled();
    }
}
