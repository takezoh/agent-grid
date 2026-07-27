---
id: adr-20260727-services-go-module-flag-day
kind: adr
title: Independent services Go module flag day
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags:
- services
- go-module
- repository-layout
- migration
owners: []
relations:
- {type: originatedFrom, target: change-20260727-web-host-client-boundary}
source_paths:
- services
- clients/config
- clients/web-host
- protocol
- contracts
- docs
- deploy
- scripts
- test-harness
summary: Move nonclient deployable services and dedicated shared infrastructure into
  an independent services Go module.
consequences:
  positive:
  - Services, client hosts, protocols, contracts and deployment assets have explicit
    top-level owners.
  negative:
  - The migration has broad atomic path churn and cannot be safely split.
  neutral:
  - Completed and abandoned change packages continue recording historical src and
    clients/desktop-config paths.
confirmation: Services/Web-host module gates and top-level ownership audit pass while
  terminal history hashes remain unchanged.
---

## Context

The repository needs a named umbrella for nonclient deployable services and their dedicated shared infrastructure. `backend`, `runtime`, and `host` are ambiguous umbrellas, while a repository-root Go module would flatten the ownership distinction and mix service infrastructure with Go client hosts.

## Decision

Create independent module `github.com/takezoh/agent-grid/services`. In one flag day move `src/go.mod`, `src/go.sum`, `src/cmd`, `src/host`, `src/server`, `src/platform`, `src/orchestrator`, `src/internal`, `src/gorules`, `src/protocolsim`, and `src/.golangci.yml` under `services/`.

`services/` contains nonclient deployable services and shared infrastructure dedicated to those services. Go client hosts remain under `clients/`, including independent `clients/web-host`.

Rename `clients/desktop-config` to `clients/config`; it owns language-neutral versioned schema and fixtures shared by every client host.

The allowed top-level ownership structure is `services/`, `clients/`, `protocol/`, `contracts/`, `docs/`, `deploy/`, `scripts/`, and `test-harness/`. Do not create a repository-root Go module, flat root Go packages, or an `src` wrapper.

Active build, CI, scripts, current design/guides and nonterminal changes move atomically. Done/abandoned change packages remain history. Tracked empty `src/.claude` placeholders are deleted without touching repository-local `.claude`.

## Consequences

{% consequence kind="positive" %}
Service and client-host dependencies are made explicit by independent module boundaries.
{% /consequence %}

{% consequence kind="negative" %}
Every active Go path and desktop-config consumer changes atomically.
{% /consequence %}

{% consequence kind="neutral" %}
Historical terminal change prose may retain old paths as past facts.
{% /consequence %}

## Alternatives

- Repository-root Go module and flat packages: rejected by the user because they erase the services ownership boundary.
- `backend`, `runtime`, or `host` umbrella: rejected because each is ambiguous about deployability and client ownership.
- Incremental dual roots or `src` wrapper: rejected because it creates two active ownership paths.

## Confirmation

Run `cd services && GOWORK=off go test ./...`, the independent `clients/web-host` gate, module-path/import audits, top-level ownership audit, active-reference audit, docs conformance, and terminal-history hash comparison.
