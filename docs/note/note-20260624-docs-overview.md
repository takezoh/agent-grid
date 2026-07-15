---
id: note-20260624-docs-overview
kind: note
title: Documentation
status: published
created: '2026-06-24'
updated: '2026-07-15'
tags:
- docs
- legacy-import
owners: []
relations:
- {type: references, target: component-20260624-client-overview}
- {type: references, target: component-20260624-orchestrator-overview}
- {type: references, target: component-20260624-platform-overview}
- {type: references, target: note-20260624-agent-contributing}
- {type: references, target: note-20260624-agent-overview}
- {type: references, target: note-20260624-agent-testing}
- {type: references, target: note-20260624-agent-workflow-authoring}
- {type: references, target: note-20260624-technical-overview}
- {type: references, target: note-20260624-user-getting-started}
- {type: references, target: note-20260624-user-orchestrator}
- {type: references, target: note-20260624-user-overview}
- {type: references, target: note-20260624-user-sandbox}
- {type: references, target: note-20260624-user-systemd}
- {type: references, target: note-20260624-user-web-server}
- {type: references, target: note-20260715-user-codex-remote-control}
source_paths:
- ARCHITECTURE.md
- WORKFLOW.md
topic: docs
summary: Agent Grid is one Go module that ships three binaries built on a three-layer
  architecture. This documentation is organized along two axes. The files are now
  stored as docs-skill structured records (adr, spec, plan, ux,
---

<!-- migrated_from: docs/README.md -->

# Documentation

Agent Grid is one Go module that ships **three binaries** built on a **three-layer architecture**. This documentation is organized along two axes. The files are now stored as docs-skill structured records (`adr`, `spec`, `plan`, `ux`, `note`, and `component`):

- **Audience** — who is reading: an end **user** running the tools, an **agent** (AI agent or contributor) doing work in the repo, or a developer who needs the **technical** internals.
- **Architecture layer** — which part of the system: `platform/` (shared infrastructure), `client/` (the session daemon + web frontend), or `orchestrator/` (the autonomous Symphony pipeline).

See [ARCHITECTURE.md](../../ARCHITECTURE.md) for the canonical definition of the three layers and the import boundaries enforced by `depguard`.

## Audience × Layer map

| Audience \ Layer | platform | client (server) | orchestrator | Cross-cutting |
|---|---|---|---|---|
| **User** | [sandbox setup](../note/note-20260624-user-sandbox.md) | [web stack](../note/note-20260624-user-web-server.md) · [systemd service](../note/note-20260624-user-systemd.md) | [orchestrator](../note/note-20260624-user-orchestrator.md) | [getting started](../note/note-20260624-user-getting-started.md) · [Codex Remote Control](../note/note-20260715-user-codex-remote-control.md) |
| **Agent** | — | — | [WORKFLOW.md authoring](../note/note-20260624-agent-workflow-authoring.md) | [contributing](../note/note-20260624-agent-contributing.md), [testing](../note/note-20260624-agent-testing.md) |
| **Technical** | [platform/](../component/component-20260624-platform-overview.md) | [client/](../component/component-20260624-client-overview.md) | [orchestrator/](../component/component-20260624-orchestrator-overview.md) | [ARCHITECTURE.md](../../ARCHITECTURE.md) |

## By audience

### [User](../note/note-20260624-user-overview.md) — running the tools

You want to launch agents, watch their status, and (optionally) run an unattended pipeline.

- [Getting started](../note/note-20260624-user-getting-started.md) — requirements, install, first run, choosing a binary, agent setup
- [web stack (ad-hoc launch)](../note/note-20260624-user-web-server.md) — running `server` (daemon + HTTP/WS gateway) + `web` for browser-driven session management
- [run as a systemd service](../note/note-20260624-user-systemd.md) — production deployment of the three-process stack with token persistence
- [Codex Remote Control](../note/note-20260715-user-codex-remote-control.md) — host-scoped daemon setup, mobile pairing lifecycle, and host/devcontainer session behavior
- [orchestrator](../note/note-20260624-user-orchestrator.md) — running an unattended pipeline from a `WORKFLOW.md`, agent selection, observability HTTP
- [sandbox setup](../note/note-20260624-user-sandbox.md) — per-project devcontainer isolation and credential proxy configuration

### [Agent](../note/note-20260624-agent-overview.md) — doing work in the repo

You are an AI agent or a contributor changing the code, or authoring the workflow that drives the autonomous agent.

- [Contributing](../note/note-20260624-agent-contributing.md) — build/test/vet/lint, coding rules, library selection
- [WORKFLOW.md authoring](../note/note-20260624-agent-workflow-authoring.md) — front matter, the prompt template, the issue state flow, the `linear_graphql` tool
- [Testing](../note/note-20260624-agent-testing.md) — testability as a design constraint, Tier-based coverage targets

### [Technical](../note/note-20260624-technical-overview.md) — internals by layer

You need to understand how a layer is built.

- [platform/](../component/component-20260624-platform-overview.md) — shared infrastructure: sandbox, brokers, credential proxy, logger, trackers, tool wrappers
- [client/](../component/component-20260624-client-overview.md) — agent-grid's session daemon: Functional Core / Imperative Shell, the state machine, drivers, subsystems, IPC, web frontend
- [orchestrator/](../component/component-20260624-orchestrator-overview.md) — the poll / dispatch / reconcile pipeline and Symphony SPEC conformance
