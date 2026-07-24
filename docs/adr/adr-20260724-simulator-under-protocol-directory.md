---
id: adr-20260724-simulator-under-protocol-directory
kind: adr
title: Simulator lives under protocol/simulator/ as a governed extension of plan-20260723-repo-structure.md
status: accepted
created: 2026-07-24
summary: Simulator lives under protocol/simulator/ as a governed extension of plan-20260723-repo-structure.md
decision_makers:
  - agent-grid-maintainers
  - product-owner
consulted:
  - runtime-maintainers
  - server-api-maintainers
  - contract-layer-maintainers
informed:
  - agent-grid-users
tags:
  - native-clients
  - phase0-1
owners:
  - agent-grid-maintainers
relations:
  - type: originatedFrom
    target: change-20260723-native-clients-phase01
source_paths:
  []
consequences:
  positive:
    - The simulator sits adjacent to the schemas it serves; readers do not need to cross-reference between top-level trees.
    - 'kind=planned in the component''s repo_grounding no longer contradicts an absent planned-file entry.'
  negative:
    - protocol/ grows beyond a pure schema directory; the maintainer set must own the sim server as production-adjacent code.
    - 'clients/ tree''s sdk consumers must reach across to protocol/simulator/ for their e2e suites.'
  neutral:
    - 'The recording format follows src/platform/agent/fakecodex/''s .jsonl precedent; no new recording format is invented.'
---


# Simulator lives under protocol/simulator/ as a governed extension of plan-20260723-repo-structure.md

## Context

{% context %}
The simulator (fixture + recorded stream + sim server) needs a home. protocol/README.md already lists a planned-file table that does not include a simulator/ subdirectory (issue-d2-simulator-grounding-contradicts-scaffold); silently inventing one contradicts the scaffold ownership plan-20260723-repo-structure.md holds. Placing the simulator under contracts/ or a top-level tools/ directory would separate it from the schemas it serves and break the 'sim server serves protocol/'s surface' contract.
{% /context %}

## Decision

{% decision %}
The simulator lives under protocol/simulator/ (fixtures/, recordings/, server/) as a governed extension of plan-20260723-repo-structure.md's planned-file table. This ADR is the explicit disposition: protocol/simulator/ is a named sub-scaffold owned by contract-layer maintainers, referenced from protocol/README.md's planned-file list, and the plan-20260723-repo-structure.md M3 landing step records this ADR as its authority for the addition.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- The simulator sits adjacent to the schemas it serves; readers do not need to cross-reference between top-level trees.
- kind=planned in the component's repo_grounding no longer contradicts an absent planned-file entry.
{% /consequence %}

{% consequence kind="negative" %}
- protocol/ grows beyond a pure schema directory; the maintainer set must own the sim server as production-adjacent code.
- clients/ tree's sdk consumers must reach across to protocol/simulator/ for their e2e suites.
{% /consequence %}

{% consequence kind="neutral" %}
- The recording format follows src/platform/agent/fakecodex/'s .jsonl precedent; no new recording format is invented.
{% /consequence %}

## Alternatives

- **Top-level tools/simulator/** — Separates the sim server from the schemas it serves; readers cross two trees to trace a fixture to its schema.
- **contracts/simulator/** — contracts/ holds behavior contracts, not executable code; mixing them dilutes both.
- **protocol/simulator/ without an ADR-tracked extension** — Silently inventing a subdirectory contradicts the scaffold decision plan-20260723-repo-structure.md owns.

## Related

- decision inputs: `decision-input-simulator-scaffold-placement`, `decision-input-simulator-shape`
- requirements: `FR-P1-08`
- contracts: `contract-simulator-recorded-scenario-replay`
