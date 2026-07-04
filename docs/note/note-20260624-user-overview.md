---
id: note-20260624-user-overview
kind: note
title: User Guide
status: published
created: '2026-06-24'
updated: '2026-07-04'
tags:
- user
- legacy-import
owners: []
relations:
- {type: referencedBy, target: note-20260624-docs-overview}
- {type: references, target: note-20260624-technical-overview}
- {type: references, target: note-20260624-user-getting-started}
- {type: references, target: note-20260624-user-orchestrator}
- {type: references, target: note-20260624-user-sandbox}
- {type: references, target: note-20260624-user-systemd}
- {type: references, target: note-20260624-user-web-server}
source_paths: []
topic: user
---

<!-- migrated_from: docs/user/README.md -->

# User Guide

Documentation for running Agent Reactor. Start here if you want to launch agents and watch their status, or run an unattended pipeline.

Agent Reactor ships three binaries that map onto the [three-layer architecture](../../ARCHITECTURE.md):

| Binary | Layer | What it is for |
|---|---|---|
| `server` | client | Single-process backend — session daemon + HTTP/WS gateway in one binary (xterm.js front-end runs through the embedded gateway) |
| `web` | client | Browser UI host — serves the React/xterm.js bundle and reverse-proxies REST/WS to `server` |
| `orchestrator` | orchestrator | Unattended scheduler that reads a `WORKFLOW.md` and drives agents against a tracker |
| `claude-app-server` | platform / orchestrator | Codex app-server shim that lets the orchestrator drive a Claude agent |

## Pages

- [Getting started](../note/note-20260624-user-getting-started.md) — requirements, `make install`, first run, choosing a binary, agent setup
- [web stack (ad-hoc launch)](../note/note-20260624-user-web-server.md) — the browser-facing `server` + `web` processes for local/dev use
- [run as a systemd service](../note/note-20260624-user-systemd.md) — production stack (`server` + `web`) as per-user systemd units, with token persistence and boot-time autostart
- [orchestrator](../note/note-20260624-user-orchestrator.md) — the `orchestrator` layer for end users: running a `WORKFLOW.md` pipeline, agent selection, observability HTTP
- [sandbox setup](../note/note-20260624-user-sandbox.md) — the `platform` layer for end users: per-project devcontainer isolation and credential proxy

For internals, see the [technical docs](../note/note-20260624-technical-overview.md).
