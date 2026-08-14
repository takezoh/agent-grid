# Client host configuration

This directory is the **canonical client configuration contract**. It is not
owned by any one host: the web host, the terminal CLI, Windows Shell and
Workspace are all consumers of it, and each is bound to it in exactly the same
way. `servers.json` (version 2) is shared by all four; desktop hosts
additionally read appearance and process settings.

Each language holds exactly **one** loader of `servers.json`, and a repository
check fails when a second one appears
(`cd services && go test ./gorules/ -run TestClientConfigHasExactlyOneLoaderPerLanguage`):

| Language | Loader | Hosts |
|---|---|---|
| Go | `clients/goclient/clientconfig` | web host, terminal CLI |
| TypeScript | `clients/workspace/src/main/desktop-config.ts` | Workspace |
| C# | `clients/windows-shell/AgentGrid.Shell.Core/Configuration/DesktopConfig.cs` | Windows Shell |

`servers.schema.json` is **documentation-only for the loaders** — none of them
validates against it, because wire and persistence types stay stdlib-only in
every language. What keeps the three honest instead:

- `servers.schema.json` `x-contract-rules` — the rule registry, read by a test.
- `fixtures/servers-corpus.json` — the expected-outcome corpus. Every case
  records the accept-or-reject decision and, when accepted, the admission
  state, canonical URL and derived identifier of each entry. All three loaders
  assert against that record; asserting only that each one throws is what this
  replaces. Adding a rule means adding a fixture, not editing three test bodies.
- `fixtures/server-id-vectors.json` — cross-language golden vectors for the
  derived identifier.

Run all three readers with `scripts/check-client-config-parity.sh`
(`./scripts/run-verification-profile.sh pr schema`). See
[ADR: shared Go client config module](../../docs/adr/adr-20260814-shared-go-client-config-module.md).

- `servers.json` (schema version 2): enabled server connections, display names,
  explicit `auth` values, and the optional `default_execution` marker used by
  New Session. Exactly one enabled entry is selected in Settings; legacy files
  without the marker temporarily use their first enabled entry until saved.
- `appearance.json`: theme, density, and UI font scale
- `shell.json`: Shell process settings, including the optional Workspace-selected
  physical `display_anchor` used to place the Shell panel. Workspace owns the
  display picker; Shell only consumes the saved anchor at startup.
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
Configuration is loaded once at process start.

**Admission is per entry.** A malformed entry costs that entry and the entries
it collides with — not the fleet. A typo in a server the operator is not using
must not take away every other server, on every host, at once. Each entry ends
in one of three states:

| State | Meaning |
|---|---|
| `usable` | Valid and enabled; connected to. |
| `disabled` | Valid, not enabled. Never connected to, and not an error. |
| `configuration_invalid` | Broke a rule, or collides with an entry that did. Never probed, never connected to, and always reported with its position, its display name as written, and the rule it violates. |

A loader never renames, normalises, suffixes, disables, deduplicates or drops
an entry to resolve a violation. When two entries collide — on a display name
or on a canonical URL — **both** are invalid: blaming one member requires
choosing which, and every choice rule silently redirects an existing
`--server <name>` to a different machine.

Only three things stop a host, and they are exactly the cases where no entry
can be named: unreadable bytes or content that is not JSON, a `schema_version`
that is not 2, and a `servers` key that is absent or is not an array. An empty
`servers` array — or one in which nothing is usable — is an empty fleet, which
every host must survive and present.

**Display names.** Among enabled entries, display names are unique by exact
code-point equality after trimming (no case folding, no Unicode normalisation).
After trimming a name is non-empty, contains no ASCII control character, and
does not begin with `-`, because it is what `--server` takes and a command line
reads a leading dash as a flag.

The complete rule list is `servers.schema.json` `x-contract-rules`; every rule
is exercised by a fixture in `fixtures/servers-corpus.json`, and a test fails
when one is not. See
[ADR: server display name naming contract](../../docs/adr/adr-20260814-server-display-name-naming-contract.md).

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
canonical gateway URL. Canonicalization lower-cases scheme and host, drops a
default port, and removes trailing path slashes. Credentials, a query and a
fragment are **rejected, not stripped**: all three are things an operator wrote
on purpose, and discarding them silently would leave the operator believing the
client is doing something it is not — most sharply for a credential, where the
belief is that authentication is configured. The identifier is not
user-configurable or persisted, so every conforming client given the same
gateway URL uses the same identifier. `display_name` remains
editable per server entry. Desktop session identity is `{serverId, sessionId}`.
Once routed to a server connection, only `sessionId` is sent to that server.

Workspace loads the UI bundled in the application. Server entries therefore
contain only the backend `url`; they do not contain a UI origin.

Workspace also persists its own last-used display in `workspace-state.json`, not
in shared configuration. This runtime state is independent from the explicit
Shell display selection in `shell.json`.
