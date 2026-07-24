namespace AgentGrid.Shell.Core.SessionIdentity;

/// <summary>
/// Desktop-side session identity. ServerId selects a configured connection;
/// only SessionId crosses that server's HTTP/WebSocket boundary.
/// </summary>
public readonly record struct ServerSessionId(string ServerId, string SessionId)
{
    public void Validate()
    {
        if (string.IsNullOrWhiteSpace(ServerId))
            throw new ArgumentException("server id is required", nameof(ServerId));
        if (string.IsNullOrWhiteSpace(SessionId))
            throw new ArgumentException("session id is required", nameof(SessionId));
    }

    public override string ToString() => $"{ServerId}:{SessionId}";
}
