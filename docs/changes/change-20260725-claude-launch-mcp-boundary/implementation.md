---
change: change-20260725-claude-launch-mcp-boundary
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Boundary ownership

`platform/agentlaunch.AppendManagedClaudeMCPConfig` is the sole producer of the
host Claude launch overlay. It accepts already-tokenized argv and the resolved
server executable/socket endpoint, returns a new argv, and performs no
filesystem or environment mutation.

Both host launch adapters normalize their plan first and then call this helper:

- `platform/agentlaunch.DirectDispatcher`
- `host/runtime.DirectLauncher`

The helper appends the flag last, serializes configuration using `encoding/json`,
and reuses the package-wide managed `agent_frames` alias.

## Removed responsibilities

- `PrepareManagedClaudeHome` and its temporary directory/symlink lifecycle
- `DirectDispatcher.DataDir` and `DirectLauncher.DataDir`
- `AG_CLAUDE_REAL_HOME` and `AG_CLAUDE_OVERLAY_HOME`
- hook payload transcript-path rewriting

`platform/mcpoverlay` remains the owner of container/workspace JSON overlay
merging; it no longer has a host HOME-overlay consumer.

## Failure behavior

JSON serialization failure aborts launch preparation. An unsupported Claude
version rejects `--mcp-config` at process startup; the opt-in real CLI test
detects this compatibility break. Missing executable/socket inputs preserve the
prior no-injection behavior.
