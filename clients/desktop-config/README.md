# Desktop configuration

Windows Shell and Workspace read the same four JSON files from
`%APPDATA%\agent-grid\config`:

- `servers.json`: enabled server connections and local launch policy
- `appearance.json`: theme, density, and UI font scale
- `shell.json`: Shell process settings
- `workspace.json`: Workspace process settings

Pass `--config-dir <path>` to either executable to replace the whole directory.
Shell forwards the same argument when it starts Workspace. Tests must use a
temporary directory through this argument and must not read or write the user
configuration directory.

Missing files are generated with `schema_version: 1`. Existing files are never
overwritten. Configuration is loaded once at process start; malformed files,
unsupported versions, duplicate server IDs, and invalid values stop startup
with a file-specific error.

`server.id` is a stable client-local routing key. Desktop session identity is
`{serverId, sessionId}`. Once a request has been routed to a server connection,
only `sessionId` is sent to that server.
