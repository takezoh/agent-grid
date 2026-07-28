---
id: adr-20260728-repository-devcontainer-scope
kind: adr
title: Repository-scoped devcontainer over session-specific clones
status: accepted
created: '2026-07-28'
decision_makers:
- pending user consultation
consulted:
- design integration critique
informed:
- platform sandbox and host runtime implementers
tags:
- github
- devcontainer
- sandbox
owners: []
relations:
- {type: originatedFrom, target: change-20260728-github-repository-sessions}
- {type: references, target: adr-20260705-fakedocker-path-injection}
- {type: references, target: adr-20260727-typed-launch-profile-schema}
- {type: references, target: adr-20260705-test-tier-taxonomy}
source_paths:
- services/platform/agentlaunch/
- services/platform/sandbox/
- services/host/runtime/
summary: Separate durable repository environment identity from isolated session clone
  workspaces pending consultation.
consequences:
  positive:
  - Independent session clones reuse exactly one matching per-project repository environment.
  negative:
  - Environment identity/config generation requires a new durable record and explicit
    stale recreation.
  neutral:
  - Existing platform launch remains canonical and only its typed descriptor is extended.
confirmation: RepositoryDevcontainerContract and FakeVsRealRepositoryDevcontainer
  prove same-repository reuse, cross-repository isolation, restart identity, and no
  isolation fallback.
updated: '2026-07-28'
---

## Status

Proposed. This decision is intentionally pending user consultation and does not authorize implementation until accepted or rejected.

## Context

Current project path participates in both container key and workspace path, but approved UX requires one repository environment and independent session clones.

## Decision

Pending consultation, persist an execution-server-qualified repository environment record binding GitHub repository ID, container identity, pinned devcontainer digest/generation, and readiness. Mount the repository sessions root, use each session clone as frame cwd, and permit per-project isolation only.

## Consequences

{% consequence kind="positive" %}
Independent session clones reuse exactly one matching per-project repository environment.
{% /consequence %}

{% consequence kind="negative" %}
Environment identity/config generation requires a new durable record and explicit stale recreation.
{% /consequence %}

{% consequence kind="neutral" %}
Existing platform launch remains canonical and only its typed descriptor is extended.
{% /consequence %}

## Alternatives

- Use clone path as container key: Silently creates one environment per session.
- One shared environment across repositories: Breaks repository capability isolation.
- Per-session devcontainer: Contradicts the confirmed repository-scoped observation.

## Confirmation

RepositoryDevcontainerContract and FakeVsRealRepositoryDevcontainer prove same-repository reuse, cross-repository isolation, restart identity, and no isolation fallback.
