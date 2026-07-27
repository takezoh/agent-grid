---
id: adr-20260727-independent-web-host-module
kind: adr
title: Independent Web host nested Go module
status: accepted
created: '2026-07-27'
decision_makers:
- unknown
tags:
- web-host
- go-module
- dependency-boundary
owners: []
relations:
- {type: originatedFrom, target: change-20260727-web-host-client-boundary}
source_paths:
- clients/web-host
- Makefile
- scripts/run-dev.sh
summary: Place Web host in an independently buildable nested module with no parent-module
  edge.
consequences:
  positive:
  - Web host is a real client host boundary with independent build and tests.
  negative:
  - Its small TLS serving responsibility is duplicated from the root server module.
  neutral:
  - Root and nested Go gates must run separately because ./... does not cross a nested
    module.
confirmation: GOWORK=off module and import audits show no root module edge.
---

## Context

Current `src/cmd/uihost` imports root `platform/lib/tlsdev`. A move that retains this edge would only rename the directory and would violate the required client/server boundary.

## Decision

Create module `github.com/takezoh/agent-grid/clients/web-host`. Move the command, host handlers, embedded assets and tests into it. The module must not require, replace, import, or use an internal package from the repository-root module.

The Web host owns its cohesive TLS listener behavior in a private package using the standard library. Do not create a generic backend/shared module merely to move this small responsibility.

## Consequences

{% consequence kind="positive" %}
Independent module resolution mechanically enforces the host boundary.
{% /consequence %}

{% consequence kind="negative" %}
TLS certificate/listener tests exist in both root server and Web host ownership boundaries.
{% /consequence %}

{% consequence kind="neutral" %}
Web host continues embedding clients/ui output, now under clients/web-host/dist.
{% /consequence %}

## Alternatives

- Root-module `require`/`replace`: rejected by the explicit isolation requirement.
- Shared utility module: rejected as an ambiguous new box without an independently evolving public capability.
- Remove TLS dev behavior: rejected because relocation must preserve host behavior.

## Confirmation

Run `cd clients/web-host && GOWORK=off go test ./...`.
