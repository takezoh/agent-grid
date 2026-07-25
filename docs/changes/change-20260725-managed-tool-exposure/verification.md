---
change: change-20260725-managed-tool-exposure
role: verification
evidence_refs:
- {type: test, ref: src/platform/agentlaunch/managed_claude_mcp_test.go}
- {type: contract, ref: src/platform/agentlaunch/frame_messaging_mcp_overlay_test.go}
- {type: test, ref: src/platform/mcpproxy/provider_test.go}
- {type: test, ref: src/host/runtime/frame_launch_matrix_test.go}
- {type: command, ref: make lint}
- {type: command, ref: cd src && go test ./...}
---

<!-- lifecycle is owned by change.md -->

# Verification

## Required evidence

- T0: typed-set unknown-bit rejection, empty no-op, endpoint validation, exact alias/type/subcommand, deterministic and input-preserving render.
- T1 host: Claude new-session and cold-start produce inline config plus frame token; empty/non-Claude plans produce neither; missing endpoint causes no spawn/token.
- T1 container: no-proxy, proxy, shared-target, non-absolute-target, and user-collision cases yield one final read-only mount per target.
- Ownership: mcpproxy output contains no managed alias; composed agentlaunch output contains exactly one.
- Contract: host/container render the same alias, `stdio`, and subcommand with only command/socket differences.
- Regression: existing Codex dynamicTools schema/count/dispatch tests and real Claude MCP fidelity remain green.
- Horizontal search: zero production references to `ManagedFrameMessaging`, `AppendManagedClaudeMCPConfig`, or independent mcpproxy `agent_frames` injection.

## Commands

`go test ./platform/agentlaunch ./platform/mcpproxy ./host/runtime ./host/driver ./host/state`

`go test ./orchestrator/... ./platform/tracker/... ./cmd/orchestrator/... ./cmd/claude-app-server/...`

`go test ./...`

`make lint`
