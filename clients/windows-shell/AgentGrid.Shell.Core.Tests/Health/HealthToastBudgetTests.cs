using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.Health;

namespace AgentGrid.Shell.Core.Tests.Health;

public class HealthToastBudgetTests
{
    [Theory]
    [InlineData(DaemonState.Healthy, DaemonState.Degraded)]
    [InlineData(DaemonState.Degraded, DaemonState.Healthy)]
    [InlineData(DaemonState.NotRunning, DaemonState.Spawning)]
    public void Health_transitions_would_violate_if_toasted(DaemonState from, DaemonState to)
    {
        Assert.True(HealthToastBudget.WouldViolateBudget(from, to));
    }

    [Theory]
    [InlineData("approval", true)]
    [InlineData("question", true)]
    [InlineData("health", false)]
    [InlineData("daemon", false)]
    public void Only_supervision_sources_allowed(string source, bool ok)
    {
        Assert.Equal(ok, HealthToastBudget.IsSupervisionToastSource(source));
    }
}
