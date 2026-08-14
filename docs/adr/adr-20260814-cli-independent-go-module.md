---
id: adr-20260814-cli-independent-go-module
kind: adr
title: Ship the terminal CLI as an independent Go module with no services dependency
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- native-clients
- cli
- go-module
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: adr-20260727-independent-web-host-module}
- {type: references, target: adr-20260727-root-go-module-flag-day}
source_paths:
- clients/cli
- Makefile
- test-harness/profiles.json
consequences:
  positive:
  - The boundary is a compile-time property rather than a review rule; a services
    import cannot build without a require the module does not have.
  - The CLI joins build, test, vet, lint, verification profile and protocol-version
    projection at introduction, so it is never a silently ungated tree.
  negative:
  - A third Go module means a third set of gate entries and a third dependency surface
    to keep current.
  - Anything genuinely shared with the web host must be extracted deliberately rather
    than imported opportunistically.
  neutral:
  - The nested-module mechanism, GOWORK=off and the absence of a root module are unchanged;
    this decision reuses them rather than introducing a pattern.
confirmation: '`cd clients/cli && GOWORK=off go list -deps ./... | grep -q ''takezoh/agent-grid/services''`
  must find nothing, and a gorules test asserts the module appears in the build and
  verification entry points.'
summary: Place clients/cli in its own nested Go module so the prohibition on depending
  on services is enforced by the module graph, and register it in every repository
  gate at introduction.
updated: '2026-08-14'
---

# Ship the terminal CLI as an independent Go module with no services dependency

## Context

The terminal CLI is a client of the **public wire contract**. The source requirement states the boundary directly: no `services/*` internal Go API, no PTY ownership, no session persistence, no output fan-out. A boundary of this kind stated only in prose survives exactly as long as the reviewer who remembers it.

The repository already has a precedent and a mechanism. There is no `go.mod` and no `go.work` at the repository root, and every Go command runs with `GOWORK=off`. A nested module that does not `require` the services module therefore **cannot compile** an import of it. `adr-20260727-independent-web-host-module` named this mechanism for `clients/web-host`; `adr-20260727-root-go-module-flag-day` rejected the root-module alternative.

There is also a measured hole to avoid reproducing: `make lint` runs `cd services && golangci-lint run ./...` only, so `clients/web-host` has no lint gate. A new module added without gate registration inherits that hole by default.

## Decision

`clients/cli` is an independent Go module at `clients/cli/go.mod` with no `require` and no `replace` on `github.com/takezoh/agent-grid/services`. No `go.mod` or `go.work` is created at the repository root.

Registration into the repository's build, test, vet, lint, verification-profile and protocol-version-projection entry points happens in the same unit that creates the module, not later.

## Consequences

{% consequence kind="positive" %}
The boundary becomes a compile-time property: without a `require`, a services import does not build. The dependency check is the same shape the repository already uses for the bridge (`go list -deps` filtered against a forbidden set). Registering the gates at introduction keeps the module from starting life unlinted.
{% /consequence %}

{% consequence kind="negative" %}
A third Go module is a third set of gate entries, a third `go.mod` to keep current, and a third place a dependency bump has to land. Code genuinely shared with the web host has to be extracted deliberately — which is what `adr-20260814-shared-go-client-config-module` does for the configuration loader — instead of being imported opportunistically.
{% /consequence %}

{% consequence kind="neutral" %}
The nested-module mechanism, `GOWORK=off`, and the absence of a root module are pre-existing and unchanged. This decision consumes them rather than introducing a pattern.
{% /consequence %}

## Alternatives

- **Place the CLI inside the services module's `cmd/`** — rejected: it contradicts the source requirement's boundary rule and the ownership structure of `adr-20260727-services-go-module-flag-day`, and it makes every services package reachable by construction.
- **Ride inside the `clients/web-host` module and reuse its `internal/` packages** — rejected: it pollutes a shipped client host's module for the CLI's convenience. The part actually worth sharing is extracted properly instead.
- **Create a repository-root module** — rejected: already decided against in `adr-20260727-root-go-module-flag-day`.

Confirmation: `cd clients/cli && GOWORK=off go list -deps ./... | grep -q 'takezoh/agent-grid/services'` must find nothing (`ver-no-services-module-edge`), and `TestCLIModuleRegisteredInBuildAndVerificationEntryPoints` in `services/gorules` asserts the module is enumerated by the build and verification entry points (`ver-cli-registered-in-gates`).

## Related

- requirements: `NFR-002`, `NFR-005`
- contracts: `contract-module-boundary`
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-cli-independent-go-module`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
