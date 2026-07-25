---
change: change-20260725-claude-launch-mcp-boundary
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- **FR-001 (ubiquitous)**: A host process launched with
  `ManagedFrameMessaging=true` shall retain the caller-provided `HOME` value.
- **FR-002 (event-driven)**: When a host managed-frame-messaging launch is
  normalized for `frame-exec`, agent-grid shall append a Claude
  `--mcp-config` argument containing the frame-scoped `agent_frames` stdio
  server command and socket path.
- **FR-003 (state-driven)**: The managed MCP flag/value pair shall be the final
  two elements of Claude's argv because the CLI option accepts multiple
  trailing configuration values.
- **FR-004 (unwanted)**: The host launch path shall not create a synthetic
  home directory, symlink user Claude state, add real/overlay HOME marker
  variables, or rewrite hook transcript paths.
- **FR-005 (ubiquitous)**: Frame source identity and authorization shall remain
  daemon-bound through `AG_SOCKET_TOKEN`; the MCP configuration shall not
  carry caller-supplied source identity.

## Compatibility and scope

- Existing user, project, and organization MCP sources remain enabled because
  agent-grid does not pass `--strict-mcp-config`.
- Container workspace `.mcp.json` overlays and Codex `dynamicTools` are outside
  this change.
- Missing `SelfBin` or socket configuration retains the previous no-injection
  behavior.

## Acceptance

1. Given a caller HOME and a managed host launch, the spawned environment
   contains the same HOME and no overlay cleanup.
2. The encoded `AG_FRAME_SPEC.MainCommand` retains the original Claude argv
   and ends in a valid `--mcp-config` JSON pair for `agent_frames`.
3. New-session and cold-start host paths satisfy the same token, HOME, and argv
   contract.
4. A real supported Claude CLI accepts the generated argv without a model
   request.
