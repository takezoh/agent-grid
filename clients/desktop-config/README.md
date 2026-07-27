# Desktop configuration

Windows Shell and Workspace read the same four JSON files from
`%APPDATA%\agent-grid\config`:

- `servers.json` (schema version 2): enabled server connections, display names,
  and explicit `auth` values
- `appearance.json`: theme, density, and UI font scale
- `shell.json`: Shell process settings
- `workspace.json`: Workspace process settings

Pass `--config-dir <path>` to either executable to replace the whole directory.
Shell forwards the same argument when it starts Workspace. Tests must use a
temporary directory through this argument and must not read or write the user
configuration directory.

Missing files are generated with their current schema version (`servers.json`
uses 2; the other documents use 1). Existing files are never overwritten.
Configuration is loaded once at process start; malformed files,
unsupported versions, duplicate server URLs, and invalid values stop startup
with a file-specific error.

Each server has exactly one authentication variant:

```json
{"auth":{"mode":"none"}}
{"auth":{"mode":"bearer","token":"the-server-token"}}
```

Bearer values are stored directly in `servers.json`; clients do not read token
files. Configuration writers restrict the file to the current user (mode 0600
on Unix; the user's profile ACL on Windows), and settings UIs mask the value.
`none` is valid only for a server started with `-no-auth`. Schema version 1 and
`token_path` require manual editing and are never migrated or rewritten
automatically.

Clients derive `serverId` as `srv_` plus the lowercase SHA-256 hex digest of the
canonical gateway URL. Canonicalization removes query, fragment, credentials,
and trailing path slashes; URL parsing normalizes scheme, host, and default
ports. It is not user-configurable or persisted, so every conforming client
given the same gateway URL uses the same identifier. `display_name` remains
editable per server entry. Desktop session identity is `{serverId, sessionId}`.
Once routed to a server connection, only `sessionId` is sent to that server.

Workspace loads the UI bundled in the application. Server entries therefore
contain only the backend `url`; they do not contain a UI origin.
