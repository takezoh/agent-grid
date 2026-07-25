---
id: adr-20260725-managed-claude-mcp-launch-scope
kind: adr
title: Managed Claude MCP exposure is launch-scoped and must preserve HOME
status: accepted
created: '2026-07-25'
decision_makers:
- unknown
tags: []
owners: []
consequences:
  positive:
  - Claude and every child process observe the caller's real HOME.
  - agent_frames exposure is scoped to the managed Claude invocation without modifying
    user configuration.
  negative:
  - Supported Claude versions must accept the --mcp-config CLI contract.
  neutral:
  - Container Claude and Codex retain their existing exposure mechanisms.
confirmation: T0/T1 launch-contract tests plus TestFakeVsRealManagedClaudeMCPConfig
relations:
- {type: originatedFrom, target: change-20260725-claude-launch-mcp-boundary}
- {type: supersedes, target: adr-20260706-frame-messaging-managed-tool-exposure}
source_paths:
- src/platform/agentlaunch/
- src/host/runtime/
- src/host/event/
summary: Host Claude sessions preserve HOME and receive agent_frames through explicit
  launch-scoped MCP configuration owned by agentlaunch.
updated: '2026-07-25'
---

# Managed Claude MCP exposure is launch-scoped and must preserve HOME

## Context

{% context %}
Host Claude sessions need a frame-scoped `agent_frames` MCP server whose source
authority is bound by the daemon. The Phase 1 decision used an agent-grid-owned
temporary HOME so that an augmented `~/.claude/settings.json` could be selected
without modifying user configuration.

Changing HOME is broader than the intended MCP concern: it changes shell `~`,
Git, SSH, SDK, credential, cache, and arbitrary child-process configuration
lookup. It also changed Claude transcript paths and forced a compensating event
rewrite. Claude Code now exposes a documented, invocation-scoped
`--mcp-config` JSON/file contract, and the supported real CLI accepts inline
JSON during `--init-only`.
{% /context %}

## Decision

{% decision %}
For host Claude launches with `ManagedFrameMessaging=true`,
`platform/agentlaunch` shall preserve the caller environment and append one
inline `--mcp-config` flag/value pair to the normalized Claude argv. The value
shall define only the daemon-bound `agent_frames` stdio server. The pair shall
be appended last because `--mcp-config` consumes multiple trailing values.

One pure helper in `platform/agentlaunch` owns serialization and placement.
Host launch adapters consume that helper; event delivery shall not compensate
for launch-time filesystem indirection.

agent-grid shall not pass `--strict-mcp-config`, mutate user/project settings,
or encode source frame identity in MCP input. Container `.mcp.json` overlays
and Codex `dynamicTools` remain unchanged.

This decision supersedes only the Claude host-exposure portion of
`adr-20260706-frame-messaging-managed-tool-exposure`; its daemon authority,
container exposure, and Codex decisions remain valid.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
Claude and its child tools observe the real caller HOME, while agent_frames is
available only to the managed invocation and user configuration remains
unmodified.
{% /consequence %}

{% consequence kind="negative" %}
The supported Claude CLI compatibility floor now includes acceptance of inline
`--mcp-config`; a CLI that removes or changes that contract will fail launch
and the opt-in real-CLI fidelity test.
{% /consequence %}

{% consequence kind="neutral" %}
The daemon remains the authority for source identity and authorization.
Container Claude and Codex continue using their existing exposure mechanisms.
{% /consequence %}

## Alternatives

- **Retain the temporary HOME overlay** — rejected because it changes unrelated
  process behavior and requires symlink lifecycle and transcript compensation.
- **Use `CLAUDE_CONFIG_DIR`** — rejected because it is not the documented
  per-invocation MCP contract and still redirects a wider Claude state
  namespace.
- **Write user/project MCP settings** — rejected because agent-grid must not
  mutate user-owned configuration.
- **Pass `--strict-mcp-config`** — rejected because it would discard user,
  project, and organization MCP sources.

## Confirmation

T0 serialization/placement tests, both T1 host launch paths, runtime
new-session/cold-start matrix tests, horizontal absence of managed HOME
markers, and `TestFakeVsRealManagedClaudeMCPConfig`.


{% transition from="proposed" to="accepted" date="2026-07-25" %}
User approved the structural boundary correction and requested ADR fixation on 2026-07-25.
{% /transition %}
