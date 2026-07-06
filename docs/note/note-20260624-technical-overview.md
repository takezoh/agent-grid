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
- {type: referencedBy, target: note-20260624-docs-overview}
- {type: references, target: component-20260624-client-interfaces}
- {type: references, target: component-20260624-client-ipc}
- {type: references, target: component-20260624-client-overview}
- {type: references, target: component-20260624-client-process-model}
- {type: references, target: component-20260624-client-state-monitoring}
- {type: references, target: component-20260624-orchestrator-overview}
- {type: references, target: component-20260624-orchestrator-symphony-conformance}
- {type: references, target: component-20260624-platform-agent-protocol}
- {type: references, target: component-20260624-platform-brokers}
- {type: references, target: component-20260624-platform-overview}
- {type: references, target: component-20260624-platform-sandbox}
- {type: references, target: component-20260624-platform-spawn-and-launch}
- {type: references, target: note-20260624-technical-code-enforcement}
- {type: references, target: note-20260624-technical-guardrails}
- {type: references, target: note-20260624-technical-harness-engineering-assessment}
- {type: referencedBy, target: note-20260624-user-overview}
source_paths:
- ARCHITECTURE.md
- src/.golangci.yml
topic: technical
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

- **[platform/](../component/component-20260624-platform-overview.md)** — shared infrastructure
  - [Spawn & launch](../component/component-20260624-platform-spawn-and-launch.md) — `agentlaunch`/`procgroup`/`pathmap`: the command-string → process launch layer
  - [Brokers](../component/component-20260624-platform-brokers.md) — `hostexec`/`mcpproxy`/`credproxy`: host mediation and policy enforcement
  - [Agent protocol](../component/component-20260624-platform-agent-protocol.md) — `codexclient`/`codexschema`/`lib`: the Codex app-server stdio protocol
  - [Sandbox backends](../component/component-20260624-platform-sandbox.md) — per-project devcontainer isolation, image resolution, credential proxy
- **[client/](../component/component-20260624-client-overview.md)** — the client session lifecycle manager
  - [Process model](../component/component-20260624-client-process-model.md) — daemon process, pty multiplexer, rendering boundary
  - [IPC and tool system](../component/component-20260624-client-ipc.md) — message format, command surface, concurrency model
  - [State monitoring](../component/component-20260624-client-state-monitoring.md) — driver plugins, the polling pipeline, hook routing
  - [Interfaces](../component/component-20260624-client-interfaces.md) — Go type definitions, data files, source tree
- **[orchestrator/](../component/component-20260624-orchestrator-overview.md)** — the autonomous Symphony pipeline
  - [Symphony conformance](../component/component-20260624-orchestrator-symphony-conformance.md) — SPEC §17 ↔ test table and documented posture

## Cross-cutting

- **[Guardrails](../note/note-20260624-technical-guardrails.md)** — controlling the autonomous agents the orchestrator dispatches: admission (eligibility / blockers / claim), concurrency caps, capability sandboxing (devcontainer / hostexec / mcpproxy / credproxy), autonomy policy (approval & sandbox, requestUserInput hard-fail), and liveness bounds (timeouts, retry/backoff).
- **[Code & architecture enforcement](../note/note-20260624-technical-code-enforcement.md)** — keeping the codebase true to its architecture: import boundaries (10 depguard rules), no mutexes in `state/`, function/file length, feature-flag mechanics, and the wire-format convention.
- **[Harness engineering assessment](../note/note-20260624-technical-harness-engineering-assessment.md)** — a dated evaluation of how well agent-grid (the outer harness) drives Claude/Codex (the inner harness), graded across design, implementation, test, documentation, and CI, with prioritized recommendations.
