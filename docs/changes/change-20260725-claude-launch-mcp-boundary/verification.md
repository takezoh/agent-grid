---
change: change-20260725-claude-launch-mcp-boundary
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## RED evidence

Before production changes:

- `TestDirectDispatcher_WrapManagedFrameMessagingPreservesHomeAndInjectsMCPConfig`
  failed because HOME pointed to `/tmp/managed-claude-home/...`.
- `TestDirectLauncher_keepsManagedFrameMessagingToken` failed for the same
  reason.

## Tiers

- **T0 contract**:
  `TestManagedClaudeMCPLaunchContract_AppendsConfigLastWithoutMutatingInput`
  validates input immutability, terminal flag placement, and JSON schema.
- **T1 wired**:
  Direct dispatcher/launcher tests and the new-session/cold-start runtime matrix
  verify preserved HOME, token continuity, encoded frame-exec argv, and absence
  of managed HOME artifacts.
- **T3 fidelity**:
  `TestFakeVsRealManagedClaudeMCPConfig` runs the generated argv against
  `AG_E2E_CLAUDE_BIN` with `--init-only`, requiring no model request.

## Commands

```sh
cd src && go test ./platform/agentlaunch ./platform/mcpoverlay ./host/event ./host/runtime
cd src && AG_E2E_CLAUDE_BIN=claude go test -tags e2e -run TestFakeVsRealManagedClaudeMCPConfig ./platform/agentlaunch
cd src && go test ./...
make vet
make lint
python .../docs_cli.py lint --conformance
```

## Results

- Targeted T0/T1 packages: pass.
- Real Claude 2.1.218 managed MCP contract: pass.
- `go test ./...`: pass.
- `make vet`: pass.
- `make lint`: pass, zero issues.
- docs lint/conformance: pass, zero warnings.
- Full `make test-e2e` with `AG_E2E_CLAUDE_BIN=claude`: the new
  `TestFakeVsRealManagedClaudeMCPConfig` passes; two pre-existing
  `host/lib/agenthook` real-Claude hook payload tests fail because that CLI
  exits status 1 without stderr. Those failures are outside this launch-boundary
  change and do not invalidate the narrower no-model-request CLI contract.
