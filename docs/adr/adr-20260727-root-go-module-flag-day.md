---
id: adr-20260727-root-go-module-flag-day
kind: adr
title: Repository-root Go module flag day
status: rejected
created: '2026-07-27'
decision_makers:
- unknown
tags:
- go-module
- repository-layout
- migration
owners: []
relations:
- {type: originatedFrom, target: change-20260727-web-host-client-boundary}
source_paths:
- go.mod
- cmd
- host
- server
- platform
- orchestrator
- internal
- gorules
- protocolsim
- .golangci.yml
- Makefile
- .github
- scripts
summary: Move the Go module and named source roots together and relocate generated
  binaries to bin/.
consequences:
  positive:
  - Go source ownership and commands use the repository root directly.
  negative:
  - The migration has broad atomic path churn and cannot be safely split.
  neutral:
  - Completed and abandoned change packages continue recording their historical src
    paths.
confirmation: Root/nested gates and active-reference audit pass while terminal history
  hashes remain unchanged.
updated: '2026-07-27'
---

## Context

The user selected a flag-day move of module metadata and named source roots. At repository root, source directories `server/` and `orchestrator/` collide with existing generated binary names, so leaving artifacts in place is impossible. Dual roots, aliases or symlinks would retain ambiguous ownership.

## Decision

Move tracked `src/go.mod`, `src/go.sum`, `src/{cmd,host,server,platform,orchestrator,internal,gorules,protocolsim,.golangci.yml}` to repository root in one change. Generated executables move to `bin/`.

Update all active consumers in the same change: source/tests, Makefile, workflows, scripts, AGENTS.md, ARCHITECTURE.md, `docs/design/`, current guides/notes, and draft/ready/active/closing change packages. Done/abandoned change packages remain immutable history.

Tracked empty `src/.claude/settings*.json` placeholders are deleted, not copied over repository-local `.claude`. After cutover, `src/` and compatibility forwarders must not exist.

## Consequences

{% consequence kind="positive" %}
Root Go commands and one source layout become canonical.
{% /consequence %}

{% consequence kind="negative" %}
Every active path consumer and build artifact location changes atomically.
{% /consequence %}

{% consequence kind="neutral" %}
Historical change prose can still mention the old layout as a past fact.
{% /consequence %}

## Alternatives

- Incremental dual layout/symlink: rejected because it keeps two active ownership paths.
- Keep binaries at root: rejected because file names collide with source directories.
- Rewrite every historical document: rejected because completed change packages are immutable evidence.

## Confirmation

Run `GOWORK=off go test ./...` plus the nested-module, active-reference, docs-conformance and terminal-history hash gates.
