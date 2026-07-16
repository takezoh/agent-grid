---
id: note-20260624-technical-overview
kind: note
title: Technical Documentation
status: published
created: '2026-06-24'
updated: '2026-07-04'
tags:
- technical
- legacy-import
owners: []
relations:
- {type: references, target: design-client}
- {type: references, target: design-orchestrator}
- {type: references, target: design-platform}
- {type: references, target: note-20260624-technical-code-enforcement}
- {type: references, target: note-20260624-technical-guardrails}
- {type: references, target: note-20260624-technical-harness-engineering-assessment}
source_paths:
- ARCHITECTURE.md
- src/.golangci.yml
topic: technical
summary: Internals organized by the three architecture layers. The canonical overview
  — scope, design principles, the layer trees, and import boundaries — is in ARCHITECTURE.md.
  This directory holds the per-layer deep dives.
---

<!-- migrated_from: docs/technical/README.md -->

# Technical Documentation

Internals organized by the three architecture layers. The canonical overview — scope, design principles, the layer trees, and import boundaries — is in [ARCHITECTURE.md](../../ARCHITECTURE.md). This directory holds the per-layer deep dives.

## The three layers

```
platform/      Shared infrastructure — the client and orchestrator both depend on this
client/        client-specific code — state machine, runtime, drivers, IPC, web frontend
orchestrator/  Symphony SPEC implementation — poll/dispatch/reconcile + observability HTTP
cmd/           Binary entry points
```

Import direction (enforced by `depguard`, `src/.golangci.yml`): `cmd/* → client/* + orchestrator/* + platform/*` with no reverse. `platform/*` imports neither `client/*` nor `orchestrator/*`; `client/*` does not import `orchestrator/*`; `orchestrator/*` does not import `client/*`.

## Per-layer deep dives

- **[platform/](../design/design-platform.md)** — shared infrastructure
  - [Spawn & launch](../design/design-platform.md#legacy-source-component-20260624-platform-spawn-and-launch) — `agentlaunch`/`procgroup`/`pathmap`: the command-string → process launch layer
  - [Brokers](../design/design-platform.md#legacy-source-component-20260624-platform-brokers) — `hostexec`/`mcpproxy`/`credproxy`: host mediation and policy enforcement
  - [Agent protocol](../design/design-platform.md#legacy-source-component-20260624-platform-agent-protocol) — `codexclient`/`codexschema`/`lib`: the Codex app-server stdio protocol
  - [Sandbox backends](../design/design-platform.md#legacy-source-component-20260624-platform-sandbox) — per-project devcontainer isolation, image resolution, credential proxy
- **[client/](../design/design-client.md)** — the client session lifecycle manager
  - [Process model](../design/design-client.md#legacy-source-component-20260624-client-process-model) — daemon process, pty multiplexer, rendering boundary
  - [IPC and tool system](../design/design-client.md#legacy-source-component-20260624-client-ipc) — message format, command surface, concurrency model
  - [State monitoring](../design/design-client.md#legacy-source-component-20260624-client-state-monitoring) — driver plugins, the polling pipeline, hook routing
  - [Interfaces](../design/design-client.md#legacy-source-component-20260624-client-interfaces) — Go type definitions, data files, source tree
- **[orchestrator/](../design/design-orchestrator.md)** — the autonomous Symphony pipeline
  - [Symphony conformance](../design/design-orchestrator.md#legacy-source-component-20260624-orchestrator-symphony-conformance) — SPEC §17 ↔ test table and documented posture

## Cross-cutting

- **[Guardrails](note-20260624-technical-guardrails.md)** — controlling the autonomous agents the orchestrator dispatches: admission (eligibility / blockers / claim), concurrency caps, capability sandboxing (devcontainer / hostexec / mcpproxy / credproxy), autonomy policy (approval & sandbox, requestUserInput hard-fail), and liveness bounds (timeouts, retry/backoff).
- **[Code & architecture enforcement](note-20260624-technical-code-enforcement.md)** — keeping the codebase true to its architecture: import boundaries (10 depguard rules), no mutexes in `state/`, function/file length, feature-flag mechanics, and the wire-format convention.
- **[Harness engineering assessment](note-20260624-technical-harness-engineering-assessment.md)** — a dated evaluation of how well agent-grid (the outer harness) drives Claude/Codex (the inner harness), graded across design, implementation, test, documentation, and CI, with prioritized recommendations.
