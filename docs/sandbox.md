# Sandbox Backends

User-facing setup (devcontainer.json, `[sandbox.*]` settings, credential proxy provider config) lives in [sandbox-setup.md](sandbox-setup.md). This document covers the architecture, security model, and lifecycle.

## Purpose

Sandbox backends isolate agent processes per project — each project directory runs inside its own container with scoped filesystem, network, and capability restrictions.

The state layer knows only `LaunchPlan.Project` (the project path); it has no awareness of which backend is active. Backend selection and command wrapping live in the runtime layer; container lifecycle lives in the `sandbox/` package.

Roost does not build images. The image name is declared by the user in `devcontainer.json` (`image:` or `build.name`). Container lifecycle (create / start / exec / remove) uses docker directly.

## Layer Responsibilities

| Layer | Sandbox role |
|---|---|
| `state/` | Holds `LaunchPlan.Project`. Backend-agnostic |
| `runtime/` | `AgentLauncher` wraps `LaunchPlan` into `WrappedLaunch{Command, Env, Mounts, ContainerSockDir, Cleanup}`. `SandboxDispatcher` resolves which launcher (direct / devcontainer) to use per project via `config.SandboxResolver` |
| `sandbox/` | `Manager[I any]` interface + backend implementations. Owns container lifecycle only; does not import driver / lib / runtime / tui |
| `credproxy` library (`providers/<name>/`) | AWS SSO / gcloud / ssh-agent providers. Tool-specific env var names (`AWS_*`, `GOOGLE_*`, `SSH_AUTH_SOCK`) live exclusively here |
| `hostexec/` | Host-exec broker — runs allowlisted host binaries on behalf of container processes via SCM_RIGHTS stdio forwarding |

`sandbox/` is tool-agnostic. It does not contain knowledge of any specific tool (e.g. Claude). Tool-specific host paths are declared by the user in `devcontainer.json` or `~/.roost/settings.toml`; they are never hardcoded in Go source.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Backend abstraction | `sandbox.Manager[I any]` + typed `Instance[I]` | Eliminates `any` and forced type-asserts. Backend-specific state (e.g. `*devcontainer.ContainerState`) is carried as the type parameter |
| Instance scope | Per-project; frames share via ref-count | Multiple frames in the same project share one container. Image is shared across projects at build time but containers remain per-project. `AcquireFrame` / `ReleaseFrame` manage the ref-count; `DestroyInstance` is called when the count reaches zero |
| Config resolution | User scope + project scope merged by `config.SandboxResolver` with mtime-based caching | Default direct mode; individual repos opt into devcontainer without daemon restart |
| Lifecycle and effect | detach → `EffDetachClient` only (container survives); shutdown → `EffReleaseFrameSandboxes` then `EffKillSession` (container destroyed); SIGINT (ctx cancel) → same as detach | Container lifetime decisions are expressed as state-layer effects, ordered in the event loop rather than a defer stack |
| Crash recovery | `PruneOrphans` at daemon startup enumerates containers labelled `roost-managed=1` and kills any whose project is not in sessions.json | Covers SIGKILL / panic paths where defer and effects never run. sessions.json is the ground truth |
| Image resolution | `LoadSpec` reads `image:` (top-level) then `build.name` from devcontainer.json | Roost does not build images. The user builds externally and declares the image name in devcontainer.json |

## Frame Lifecycle Interaction

**New frame**
`AgentLauncher.WrapLaunch` → `EnsureInstance` (singleflight-serialized per project) → `AcquireFrame` → the resulting `WrappedLaunch` is embedded in `EffSpawnTmuxWindow`

**Warm start**
`AdoptFrame` reclaims the still-running container and increments the ref-count for each restored frame; `RecoverSandboxFrames` replays per-frame bearer tokens from `<dataDir>/warm/`

**Frame exit / shutdown**
`Cleanup` callback → `ReleaseFrame` → if count reaches zero → `DestroyInstance`

**Daemon startup**
`PruneOrphans` kills containers outside the known project set; `<dataDir>/warm/` is wiped at cold start

## Devcontainer Backend

### Image Resolution

At session start, `LoadSpec` reads the image name from `.devcontainer/devcontainer.json`:

1. If `image:` is set → use it.
2. Else if `build.name` is set → use it.
3. Neither found → error: `devcontainer.json: image or build.name is required`.

The image must already exist locally (or be pullable by docker on first `docker create`). roost never invokes a build.

### Container Identity

One long-lived container per project idles between frame activations. Frames join via `docker exec -it` rather than spawning a new container per frame.

- Container name: `roost-<sha256[:6] of project path>`
- Labels: `roost-managed=1`, `roost-project=<abs-path>`

Multiple projects can share the same image name; each project still gets its own container.

### Workspace Mount

The project directory is bind-mounted into the container. By default the container-side path mirrors the host path exactly (e.g. host `/home/u/proj` → container `/home/u/proj`), so editor path resolution and CLI commands work without translation.

`workspaceMount` in devcontainer.json overrides the bind-mount entirely. `host_path_mount_prefix` (roost setting) prepends a fixed prefix (e.g. `/mnt`) when devcontainer.json does not pin the workspace path. Working directory is set via `docker create -w` — no `WORKDIR` is needed in the Dockerfile.

### devcontainer.json Keys Honored by roost

| Key | Effect |
|---|---|
| `image` | Image name to use (standard devcontainer.json field) |
| `build.name` | Image name to use when `build:` is present (roost extension; `--image-name` equivalent) |
| `mounts` | Extra bind-mounts passed as `--mount` / `-v` to `docker create` |
| `containerEnv` | Environment variables injected via `-e` |
| `containerUser` | User for `docker create -u` |
| `remoteUser` | User for `docker exec -u` (takes precedence over `containerUser`) |
| `workspaceFolder` | Container-side workspace path (default: mirror of host path) |
| `workspaceMount` | Replaces the default workspace bind-mount |
| `runArgs` | Extra args appended to `docker create` (resource limits, capabilities, etc.) |
| `postCreateCommand` | Command (string or array) run once after the container is created |
| `preExecCommand` | Shell string (roost extension) run inside the container before each `docker exec` launch |

Variable substitution in string values: `${localWorkspaceFolder}`, `${localWorkspaceFolderBasename}`, `${containerWorkspaceFolder}`, `${localEnv:VAR}`. All other devcontainer.json keys (`features`, `customizations`, …) are ignored.

### Crash Recovery

`PruneOrphans` runs at daemon startup. It lists containers with label `roost-managed=1` and removes any whose `roost-project` label value is absent from sessions.json.

## Container IPC Endpoint

Each sandboxed project gets a dedicated Unix socket at `<dataDir>/run/<project-hash>/roost.sock` on the host. It is bind-mounted read-write into the container at `/opt/roost/run/roost.sock` (via the per-project run dir mount that already exists for credential helper files). The container agent reads `ROOST_SOCKET` (set to `/opt/roost/run/roost.sock`) to locate it.

**API surface**: only `hook-event` is implemented. Commands such as `event`, `surface.send_text`, `peer.send`, `shutdown`, and all others are structurally absent — no handler is registered, so they receive a protocol error without touching state.

**Authentication**: at frame spawn time, a 32-byte `crypto/rand` token is generated and injected into the container via `ROOST_SOCKET_TOKEN`. Every `hook-event` message carries the token; server-side Lookup resolves it to the owning frame ID. No client-supplied frame ID is trusted.

**Warm-start recovery**: tokens are persisted to `<dataDir>/warm/<frameID>.json` (atomic write, `0o600`). On daemon warm restart (containers survive, daemon replaces), `RecoverSandboxFrames` reads `warm/*.json` and re-registers each token for live frames so hook events continue to work immediately. The `warm/` directory is never bind-mounted into containers — a container process cannot read other frames' tokens.

**Cold start**: `warm/` is wiped before `LoadSnapshot` runs, ensuring stale tokens from a previous run do not survive a session-destroying restart.

## Container↔Host Path Translation

Sandboxed agents emit hook payloads containing container-absolute paths (e.g. `/workspaces/proj/.../session.jsonl`), but the daemon, drivers, and TUI operate on host-absolute paths. `lib/pathmap` translates at the IPC boundary using the bind-mount table captured in `WrappedLaunch.Mounts`. State, runtime above the launcher, proto, and TUI never see container paths.

## Host Mounts

Bind-mounts are declared in devcontainer.json `mounts`. `sandbox/` does not have a global host-mounts config for arbitrary paths — tool-specific paths belong in project or user devcontainer.json, keeping the sandbox layer tool-agnostic.

## Credential Proxy

In devcontainer mode roost always runs an in-process HTTP server backed by the `credproxy` library. The server listens on `<dataDir>/run/credproxy.sock` on the host and is bind-mounted per project into each container at `/opt/roost/run/credproxy.sock`. Its lifetime is tied to the roost process — no external daemon is needed. Each provider self-gates on its own configuration and contributes nothing to the container when its settings are empty.

Providers come from two sources: the external `credproxy` library's `providers/<name>/` packages (AWS SSO, gcloud, ssh-agent) and the local `hostexec/` package (host-exec broker — not a credential, but uses the same `container.Provider` interface). Each provider contributes to the runtime by:

1. Building a `container.Spec` (env vars to inject, files to materialize under the per-project run dir, optional bind-mounts).
2. Optionally registering an HTTP route on the proxy server (AWS SSO uses this; others rely on bind-mounts only).

Generic layers (`runtime/`, `sandbox/`, `state/`, `tui/`, `proto/`) never reference tool-specific env var names (`AWS_*`, `GOOGLE_*`, `SSH_AUTH_SOCK`). Those names appear only inside the corresponding provider package.

### AWS SSO, gcloud CLI, SSH Agent

Behavior of each provider (credential fetch flow, security model, container env vars) is documented in the credproxy repository:

- [providers/awssso](https://github.com/takezoh/credproxy/tree/main/providers/awssso) — `credential_process` via proxy HTTP route; `~/.aws/sso/cache` never bind-mounted
- [providers/gcloudcli](https://github.com/takezoh/credproxy/tree/main/providers/gcloudcli) — SA impersonation + synthetic `CLOUDSDK_CONFIG`; `~/.config/gcloud` never bind-mounted
- [providers/sshagent](https://github.com/takezoh/credproxy/tree/main/providers/sshagent) — per-project ephemeral agent; container receives socket only

`SSH_AUTH_SOCK` is injected at both container-creation time (`docker create -e`) and at each frame launch (`docker exec -e`), so updating the key list takes effect on the next launch without recreating the container.

### Host-exec broker

The `hostexec` provider lets container processes invoke host binaries (e.g. `gh`, `aws`, `kubectl`) without receiving any credentials or tokens. The host executes the binary; the container only sees stdio.

**Mechanism:**

1. The host starts a per-project Unix socket broker (`<dataDir>/run/<project-hash>/hostexec.sock`) bind-mounted at `/opt/roost/run/hostexec.sock` inside the container.
2. Shell shim scripts are written to `/opt/roost/run/hostexec-shims/<name>` and prepended to `PATH`. Each shim calls `roost host-exec <name> "$@"`.
3. The shim sends the request (binary name, args, cwd) plus the three stdio fds via SCM_RIGHTS over the socket.
4. The broker policy-checks the command, then exec's the host binary with the transferred fds as its stdin/stdout/stderr. The exit code is returned to the shim.

**Policy (deny-first, default-deny):**

Allow and deny patterns follow Claude Code's Bash permission semantics: argv is reconstructed into a shell string with single-quoting, and patterns use `*` as a wildcard matching any substring including spaces.

```
deny?  → reject
allow? → permit
else   → reject
```

Leading `KEY=VALUE` env assignments in patterns are stripped before matching, so `"GH_TOKEN=x gh pr *"` is equivalent to `"gh pr *"` for both matching and binary name extraction.

Binary names must match `[a-zA-Z0-9][a-zA-Z0-9._-]*`; patterns whose first non-env token fails this check are rejected at config load time.

User-scope and project-scope `allow`/`deny` lists are concatenated; project patterns cannot remove user-level deny rules.

### Subscription credentials (interactive auth)

Some tools (Claude Code, etc.) authenticate via OAuth flows that store refresh tokens in user-config files. The credential proxy cannot synthesise these — they require a real interactive login. The user opts in by declaring a bind-mount in devcontainer.json for the credential file/directory. This exposes the OAuth refresh token to the container; the trade-off is the user's call.

**Container reuse**: `/opt/roost/run` is bind-mounted at container-creation time. If an existing container lacks this mount (created with an older roost version), remove it with `docker rm -f roost-<hash>` and relaunch.
