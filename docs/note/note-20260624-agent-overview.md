---
id: note-20260624-agent-overview
kind: note
title: Agent Guide
status: published
created: '2026-06-24'
updated: '2026-07-05'
tags:
- agent
- legacy-import
owners: []
relations:
- {type: references, target: component-20260624-client-overview}
- {type: references, target: component-20260624-orchestrator-overview}
- {type: references, target: component-20260624-platform-overview}
- {type: references, target: note-20260624-agent-contributing}
- {type: references, target: note-20260624-agent-testing}
- {type: references, target: note-20260624-agent-workflow-authoring}
- {type: referencedBy, target: note-20260624-docs-overview}
source_paths:
- WORKFLOW.md
- AGENTS.md
- ARCHITECTURE.md
topic: agent
summary: Documentation for agents — AI agents and human contributors doing work in
  this repository. If you are changing code, read contributing. If you are authoring
  the workflow that drives the autonomous orchestrator, read
---

<!-- migrated_from: docs/agent/README.md -->

# Agent Guide

Documentation for **agents** — AI agents and human contributors doing work in this repository. If you are changing code, read [contributing](../note/note-20260624-agent-contributing.md). If you are authoring the workflow that drives the autonomous orchestrator, read [WORKFLOW.md authoring](../note/note-20260624-agent-workflow-authoring.md).

The repo's canonical build/test/rules summary lives in [AGENTS.md](../../AGENTS.md) at the root (it is loaded automatically by Claude/Gemini/Codex via `@AGENTS.md`). These pages expand on it.

## Pages

- [Contributing](../note/note-20260624-agent-contributing.md) — build/test/vet/lint commands, coding rules (file/function limits, the reducer exemption, mandatory tests), and the library-selection process
- [WORKFLOW.md authoring](../note/note-20260624-agent-workflow-authoring.md) — how the orchestrator's driving prompt is structured: the issue state flow, idempotency invariants, and the `linear_graphql` tool
- [Testing](../note/note-20260624-agent-testing.md) — testability as a design constraint and the Tier-based coverage targets

## Architecture context

All three layers and their import boundaries are defined in [ARCHITECTURE.md](../../ARCHITECTURE.md). Before adding code, know which layer you are in:

- `platform/` — shared infrastructure; must not import `client/` or `orchestrator/`
- `client/` — agent-grid's client; must not import `orchestrator/`
- `orchestrator/` — Symphony pipeline; must not import `client/`

Layer internals: [platform/](../component/component-20260624-platform-overview.md) · [client/](../component/component-20260624-client-overview.md) · [orchestrator/](../component/component-20260624-orchestrator-overview.md).
