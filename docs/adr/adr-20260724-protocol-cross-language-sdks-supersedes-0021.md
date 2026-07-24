---
id: adr-20260724-protocol-cross-language-sdks-supersedes-0021
kind: adr
title: 'Cross-language generated SDKs supersede ADR-0021''s hand-written scope; Go stays stdlib-only'
status: accepted
created: 2026-07-24
summary: 'Cross-language generated SDKs supersede ADR-0021''s hand-written scope; Go stays stdlib-only'
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
    - Contract SoT is a single tool-consumable schema; new languages come at generation cost, not hand-write cost.
    - Go-side stdlib-only property is preserved by the same lint rule already enforcing it.
  negative:
    - Adds a JSON Schema (2020-12) authoring skill to the maintainer set.
    - Generated code review becomes a diff-review discipline; tool version drift can hide meaningful contract change (mitigated by NFR-01 determinism gate).
  neutral:
    - ADR-0021 is marked superseded (status=superseded) with this ADR as supersededBy target; hand-written TS is a live migration source, not deleted immediately.
---


# Cross-language generated SDKs supersede ADR-0021's hand-written scope; Go stays stdlib-only

## Context

{% context %}
ADR-0021 accepted hand-written wire types for the browser TS client on the grounds that 4 languages was speculative. Phase 1 makes them concrete (C#/Swift/Kotlin/TS plus the browser TS client) and adds a schema SoT under protocol/. The Go-side stdlib-only rule (AGENTS.md) must not be silently dropped by the pipeline change.
{% /context %}

## Decision

{% decision %}
A new accepted ADR supersedes ADR-0021 for cross-language generated clients only. Wire schema SoT is protocol/*.schema.json, with openapi.yaml declaring the REST binding of those types (adr-20260724-protocol-message-schema-sot-rest-binding); C#/Swift/Kotlin/TS typed models are generated from the schemas. The Go-side rule (src/host/proto and any Go-side generated helper depend only on the Go standard library) survives verbatim under this ADR and is enforced by the existing stdlib-only depguard/build gate. The browser TS client's migration from hand-written wire (clients/ui/src/wire/*) to the generated TS SDK is incremental and outside this ADR's cutover requirement.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Contract SoT is a single tool-consumable schema; new languages come at generation cost, not hand-write cost.
- Go-side stdlib-only property is preserved by the same lint rule already enforcing it.
{% /consequence %}

{% consequence kind="negative" %}
- Adds a JSON Schema (2020-12) authoring skill to the maintainer set.
- Generated code review becomes a diff-review discipline; tool version drift can hide meaningful contract change (mitigated by NFR-01 determinism gate).
{% /consequence %}

{% consequence kind="neutral" %}
- ADR-0021 is marked superseded (status=superseded) with this ADR as supersededBy target; hand-written TS is a live migration source, not deleted immediately.
{% /consequence %}

## Alternatives

- **Extend ADR-0021 in place to cover 4 languages** — Loses the audit signal that this is a different kind of contract; ADR-0021 explicitly named 'revisit if 4 languages become concrete' as its trigger.
- **Hand-write all 4 languages** — Multiplies drift risk by 4; ADR-0021's own consequence table already predicted a codegen trigger.

## Related

- decision inputs: `decision-input-adr0021-supersede-scope`, `decision-input-protocol-schema-tooling`, `decision-input-sdk-generator-toolchain`
- requirements: `FR-P1-01`, `FR-P1-10`
- contracts: `contract-sdk-generation-determinism`, `contract-adr0021-supersede-stdlib-preservation`
