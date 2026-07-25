---
id: adr-20260725-managed-mcp-exposure-materialization
kind: adr
title: Managed MCP exposure uses one endpoint catalog and lifecycle-specific materializers
status: accepted
created: '2026-07-25'
decision_makers:
- unknown
tags: []
owners: []
consequences:
  positive:
  - Host and container MCP exposure share one endpoint descriptor without changing
    HOME or user config.
  negative:
  - Host ManagedMCPTools remains a closed-world Claude-compatible placement contract.
  neutral:
  - Codex dynamicTools remains a separate protocol implementation.
confirmation: Contract tests for validation ordering, host/container descriptor equivalence,
  overlay ownership, and production declaration scan.
relations:
- {type: originatedFrom, target: change-20260725-managed-tool-exposure}
- {type: supersedes, target: adr-20260725-managed-claude-mcp-launch-scope}
- {type: references, target: adr-20260706-frame-messaging-managed-tool-exposure}
source_paths:
- src/platform/agentlaunch/
- src/platform/mcpproxy/
- src/host/runtime/
- src/host/driver/
summary: Defines the shared managed MCP endpoint catalog, Claude-compatible host placement
  contract, container overlay composition, and fail-closed authority ordering.
updated: '2026-07-25'
---

# Managed MCP exposure uses one endpoint catalog and lifecycle-specific materializers

## Context

{% context %}
Host Claude currently requests frame messaging through a Claude-named boolean/helper,
while container provisioning independently injects the same `agent_frames` MCP
endpoint through `mcpproxy` and an overlay composer. This duplicates endpoint
ownership and confuses product availability, client placement, authority, and
lifecycle. Container artifacts are provisioned before individual frame launches and
therefore cannot be derived from a frame LaunchPlan.
{% /context %}

## Decision

{% decision %}
`platform/agentlaunch` shall own a validated `ManagedMCPTools` set and one MCP
endpoint catalog containing alias, stdio type, subcommand, collision posture, and
daemon-authority property. Host and container materializers shall consume this
catalog and resolve only environment-specific command/socket values.

Host `LaunchPlan.ManagedMCPTools` is explicitly limited to client launches compatible
with a trailing inline `--mcp-config` pair. Claude is the only production declarer.
Another driver may declare it only after adding and proving compatible placement;
the direct adapter shall not infer compatibility merely from a product capability.

Host ordering is validate tools/endpoint, determine authority, mint/register token,
render argv, then spawn. Unknown tools and missing requested endpoint values fail
closed; renderer/spawn failure must not retain token registration. Empty tools are
a non-mutating no-op and clear ambient host token.

`BuildContainerOverlay` shall independently materialize an explicit container-scope
default set (currently `agent_frames`) for each absolute deduplicated workspace.
It shall not inspect per-frame plans. `mcpproxy` shall emit configured external
aliases only; agentlaunch shall compose its mount as a base and produce one final
read-only `.mcp.json` mount per target.

Codex `dynamicTools` remains outside this change. It expresses related product
semantics through a different protocol, schema, and thread lifecycle; this ADR does
not claim a shared schema catalog.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
Host and container endpoints share one precise contract while preserving HOME and
user-owned configuration.
{% /consequence %}

{% consequence kind="negative" %}
The host field intentionally remains closed-world: new MCP clients require an
explicit placement-compatibility change and contract test.
{% /consequence %}

{% consequence kind="neutral" %}
Container authority policy and Codex dynamicTools behavior remain unchanged.
{% /consequence %}

## Alternatives

- A universal product capability interpreted by adapters was rejected because it
  would silently apply Claude CLI syntax to future clients.
- A placement registry/enum was rejected as unnecessary while only one compatible
  host client exists; the closed-world contract is smaller and testable.
- Deriving container overlay from frame plans was rejected because provisioning
  precedes frames and shared containers make the result launch-order dependent.
- Moving all rendering to mcpproxy was rejected because direct host launch does not
  require a proxy and proxy aliases are a separate responsibility.
- Converting Codex dynamicTools to MCP was rejected as unrelated protocol expansion.

## Confirmation

Invariant-named tests validate unknown-bit and endpoint failure before authority,
Claude-only production declarations, host/container descriptor equivalence, one
container mount per target, mcpproxy absence of managed aliases, input non-mutation,
and unchanged Codex dynamicTools behavior.


{% transition from="proposed" to="accepted" date="2026-07-25" %}
User approved this direction and requested implementation and ADR fixation on 2026-07-25.
{% /transition %}
