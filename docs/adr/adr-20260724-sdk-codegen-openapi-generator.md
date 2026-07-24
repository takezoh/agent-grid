---
id: adr-20260724-sdk-codegen-openapi-generator
kind: adr
title: SDK codegen adopts OpenAPI Generator with pinned templates for C#/Swift/Kotlin/TypeScript
status: rejected
created: '2026-07-24'
summary: SDK codegen adopts OpenAPI Generator with pinned templates for C#/Swift/Kotlin/TypeScript
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
- {type: originatedFrom, target: change-20260723-native-clients-phase01}
source_paths: []
consequences:
  positive:
  - Single generator covers both REST and event/command surfaces without stitching
    two tools.
  - Multi-language coverage matches the four targets without additional per-language
    plumbing.
  negative:
  - Template quality varies per language; the pinned template set must be reviewed
    once and updated only under the compatibility profile.
  - OpenAPI Generator's Java runtime is added to the SDK generation environment (not
    to the shipped clients).
  neutral:
  - quicktype and hand-written codegen remain viable fallbacks if OpenAPI Generator's
    output diverges from the schema in practice; the trade-off record is left in this
    ADR for a future revisit.
updated: '2026-07-24'
---


# SDK codegen adopts OpenAPI Generator with pinned templates for C#/Swift/Kotlin/TypeScript

> **Rejected 2026-07-24.** Superseded by `adr-20260724-sdk-codegen-quicktype-typegen` after `adr-20260724-protocol-message-schema-sot-rest-binding` re-layered the contract (message schemas = SoT, REST = binding annex), which removed the single-generator-input premise this decision rested on and with it the justification for a Java-based toolchain.

## Context

{% context %}
The tool choice for the four target SDKs must produce byte-identical output for identical schema input, cover REST (openapi.yaml) and event/command JSON Schema shapes (events.schema.json etc.), and stay maintainable for a small team. Named candidates: OpenAPI Generator (multi-language, mature REST support), quicktype (JSON-Schema-first, weaker REST), hand-written per-language codegen scripts (extends ADR-0021's pattern).
{% /context %}

## Decision

{% decision %}
OpenAPI Generator is adopted, pinned by version in the compatibility profile (scripts/run-verification-profile.sh pr compatibility) with a pinned template set. openapi.yaml drives REST generation; JSON Schema fragments are inlined into openapi.yaml components/schemas so a single generator invocation covers both. Status is proposed (not accepted) because the tool version pin is set at first landing chunk; the ADR moves to accepted at that landing.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Single generator covers both REST and event/command surfaces without stitching two tools.
- Multi-language coverage matches the four targets without additional per-language plumbing.
{% /consequence %}

{% consequence kind="negative" %}
- Template quality varies per language; the pinned template set must be reviewed once and updated only under the compatibility profile.
- OpenAPI Generator's Java runtime is added to the SDK generation environment (not to the shipped clients).
{% /consequence %}

{% consequence kind="neutral" %}
- quicktype and hand-written codegen remain viable fallbacks if OpenAPI Generator's output diverges from the schema in practice; the trade-off record is left in this ADR for a future revisit.
{% /consequence %}

## Alternatives

- **quicktype** — Strong for events/commands.schema.json but weaker for REST (openapi.yaml); would require stitching a second generator.
- **Hand-written per-language codegen scripts** — Extends ADR-0021's own revisit trigger; multiplies maintenance for four languages.

## Related

- decision inputs: `decision-input-sdk-generator-toolchain`
- requirements: `FR-P1-02`, `NFR-01`
- contracts: `contract-sdk-generation-determinism`
