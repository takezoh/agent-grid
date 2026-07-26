namespace AgentGrid.Shell.Menu;

/// <summary>
/// Application-level shell menu handlers.
/// </summary>
public sealed class ShellMenuHandlers
{
    private readonly Action _quitApplication;

    public ShellMenuHandlers(Action quitApplication)
    {
        _quitApplication = quitApplication ?? throw new ArgumentNullException(nameof(quitApplication));
    }

    /// <summary>
    /// Quits the client. Server lifecycle is outside the client's responsibility.
    /// </summary>
    public void OnQuit()
    {
        _quitApplication();
    }
}
