---
change: change-20260725-managed-tool-exposure
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- FR-001 (ubiquitous): System shall represent managed MCP endpoints as a validated typed set; the first entry shall be `agent_frames`.
- FR-002 (event): When Claude prepares a compatible host launch, it shall declare `agent_frames` without naming executable or socket paths.
- FR-003 (state): While a host plan declares managed MCP tools, the adapter shall validate the set and endpoint before authority issuance, then append exactly one trailing inline `--mcp-config` pair.
- FR-004 (negative): While a host plan declares no managed MCP tools, no MCP arguments shall be added and ambient `AG_SOCKET_TOKEN` shall be cleared.
- FR-005 (failure): If a requested set contains unknown tools or its endpoint lacks command/socket, launch shall fail before token registration or process spawn.
- FR-006 (container): When container overlay is built, the explicit container default set shall be rendered for every deduplicated absolute workspace target.
- FR-007 (composition): When credproxy supplies `.mcp.json`, its external aliases shall form the base and managed aliases shall be composed exactly once.
- FR-008 (compatibility): User aliases shall retain current collision policy: external proxy aliases override, managed `agent_frames` does not.
- FR-009 (endpoint): Host shall resolve self binary/host socket; container shall resolve container binary/container socket. Alias, stdio type, and subcommand shall remain identical.
- FR-010 (safety): System shall not modify HOME or user-owned project/config files; generated overlays shall remain managed and read-only.
- FR-011 (lifecycle): Per-frame host launch shall not create container overlays, and container provisioning shall not inspect a frame LaunchPlan.
- FR-012 (Codex): Existing Codex `dynamicTools` schema, count, lifecycle, and dispatch behavior shall remain unchanged.

## Invariants

- `ManagedMCPTools` on host LaunchPlan means compatibility with Claude-style trailing inline `--mcp-config`; it is not a universal MCP-client marker.
- Only Claude production drivers may assign the host field until another client proves placement compatibility.
- Validation precedes authority decision; renderer failure cannot leave a registered token or spawned process.
- Container token policy remains derived from container execution, not workspace MCP availability.
- MCP endpoint descriptor is the SSOT; Codex dynamic tool schema is outside this catalog.
