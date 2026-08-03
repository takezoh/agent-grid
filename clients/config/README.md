# Client host configuration

Web host, Windows Shell, and Workspace share the `servers.json` version 2
contract. Desktop hosts additionally read appearance and process settings.

- `servers.json` (schema version 2): enabled server connections, display names,
  explicit `auth` values, and the optional `default_execution` marker used by
  New Session. Exactly one enabled entry is selected in Settings; legacy files
  without the marker temporarily use their first enabled entry until saved.
- `appearance.json`: theme, density, and UI font scale
- `shell.json`: Shell process settings
- `workspace.json`: Workspace process settings
- `github.schema.json` / `fixtures/github-oauth-valid.json`: release defaults for
  the public GitHub OAuth App device-flow registration (`app_id`, `client_id`,
  `read:user` + `repo` scopes). Workspace copies these defaults to its own
  `github.json`; App ID and Client ID are editable from Settings > GitHub.
  No `client_secret`, bearer token, or refresh material is stored here.
  Registration ownership, release validation, rotation, and the current
  production limitation are documented in
  [GitHub OAuth App operations](../../docs/note/note-20260729-github-oauth-app-operations.md).
- `fixtures/github-projection-credential-free.json`: sample SPA-facing account
  and catalog projection (status, repositories only)

Client presentation settings belong here rather than in the server's
`settings.toml`. In particular, `appearance.json.theme` is the canonical theme
setting; the retired server-side `theme`, `monitor.fast_poll_interval_ms`, and
`session.auto_name` keys are not server configuration. The former server-side
`editor` section and `open-editor` palette action are retired rather than moved
to a client.

Pass `--config-dir <path>` to a client host to replace the whole directory.
Shell forwards the same argument when it starts Workspace. Web host loads
`servers.json` only; desktop hosts additionally load `github.json`. Tests must use a
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
