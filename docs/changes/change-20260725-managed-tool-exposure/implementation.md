---
change: change-20260725-managed-tool-exposure
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Design

Use one `platform/agentlaunch` type and catalog for managed MCP endpoints. Host
`LaunchPlan.ManagedMCPTools` is a narrow placement contract. The direct launch
adapters validate it, derive daemon authority, and invoke the inline CLI
materializer. `BuildContainerOverlay` separately passes
`defaultContainerManagedMCPTools` to the file materializer.

`mcpproxy` becomes owner only of configured external proxy aliases. Its generated
mount, when present, is an input to agentlaunch composition; after composition
each workspace target has exactly one final `.mcp.json` mount.

## Dependency-ordered chunks

1. Add RED catalog, validation, immutability, endpoint, and renderer contracts.
2. Introduce typed set/catalog and replace duplicated LaunchPlan boolean.
3. Propagate Claude-only host declaration; enforce validation → authority → render → spawn.
4. Move container managed alias composition from mcpproxy to agentlaunch.
5. Update durable design/ADR and run horizontal/conformance verification.

## Failure responsibility

- Unknown tool or missing requested endpoint: agentlaunch validation error.
- Unsupported host placement: driver contract violation, prevented by assignment scan and tests.
- Overlay parse/write failure: container provisioning failure; user file remains unchanged.
- Renderer failure after validation: launch aborts without spawn; token registration must not survive.
