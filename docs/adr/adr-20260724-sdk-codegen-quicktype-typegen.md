---
id: adr-20260724-sdk-codegen-quicktype-typegen
kind: adr
title: SDK codegen adopts quicktype type generation from JSON Schema; transport is
  hand-written per language
status: accepted
created: '2026-07-24'
summary: quicktype, pinned via the npm lockfile, generates typed models + serialization
  for C#/Swift/Kotlin/TS from protocol/*.schema.json; the thin transport (REST calls,
  WS framing, reconnect) is hand-written per SDK — no Java toolchain
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
- {type: supersedes, target: adr-20260724-sdk-codegen-openapi-generator}
source_paths: []
consequences:
  positive:
  - No new toolchain — quicktype runs on Node, which the repo already carries for
    clients/ui (npm lockfile pins the exact version); the Java runtime requirement
    disappears.
  - Generated surface is small (models + serializers only), so per-language output
    is reviewable and the template-quality variance that plagued the OpenAPI Generator
    option does not apply.
  - Generation targets exactly what codegen is good at here — typed messages + validation;
    the transport/reconnect/replay logic was hand-written under every candidate, so
    no generation value is lost.
  negative:
  - REST client calls (a handful of endpoints) are hand-written per language against
    openapi.yaml instead of generated — four small transport layers to maintain.
  - quicktype's per-language serializer idioms (Codable / kotlinx.serialization /
    System.Text.Json) must be spot-checked once per target for round-trip fidelity
    against the wire fixtures.
  neutral:
  - 'Accepted 2026-07-24: the landing gate is satisfied — quicktype pinned at 23.0.171
    in clients/sdk/package-lock.json, per-language emit options checked in at clients/sdk/quicktype-emit.json
    (decision: default emit idioms; strict fail-on-unknown-field decoders rejected because
    NFR-05''s additive-only schema evolution requires unknown-field tolerance in old clients).'
updated: '2026-07-24'
---


# SDK codegen adopts quicktype type generation from JSON Schema; transport is hand-written per language

## Context

{% context %}
adr-20260724-sdk-codegen-openapi-generator (now rejected) picked OpenAPI Generator on the premise that a single tool should cover REST client + model generation from an openapi.yaml that inlined every JSON Schema. adr-20260724-protocol-message-schema-sot-rest-binding has since re-layered the contract: protocol/*.schema.json is the SoT and REST is a small binding annex. Under that layering the generator's job shrinks to typed models + serialization for C#/Swift/Kotlin/TS — exactly quicktype's home ground — while the REST surface is a handful of endpoints whose per-language thin clients are cheaper to hand-write than to configure a Java-based generator for. The WS transport, reconnect, and replay logic was hand-written under every candidate. The determinism requirement (byte-identical output for identical schema input, FR-P1-02/NFR-01) is tool-agnostic.
{% /context %}

## Decision

{% decision %}
quicktype, pinned via the repo npm lockfile and recorded in the compatibility profile, generates typed models + serialization code for C#, Swift, Kotlin, and TypeScript from protocol/*.schema.json. Per-language emit options are fixed in a checked-in config; generation runs only under the pinned wrapper script (scripts/generate-sdks.sh) and the compatibility profile. The thin transport per SDK — REST calls declared by openapi.yaml, WS framing, reconnect per the reconnect contract — is hand-written, matching the SDK scope 'transport, typed messages, validation, version negotiation; no presentation behavior'. Go remains hand-written stdlib-only (ADR-0021's surviving rule). The version pin landed as quicktype 23.0.171 (clients/sdk/package-lock.json), satisfying the acceptance gate.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- No new toolchain — quicktype runs on Node, already in the repo for clients/ui; the Java runtime requirement disappears.
- Generated surface is small (models + serializers only), reviewable per language; OpenAPI Generator's template-quality variance does not apply.
- Generation targets exactly what codegen is good at here; no generation value is lost since transport was hand-written under every candidate.
{% /consequence %}

{% consequence kind="negative" %}
- REST client calls are hand-written per language against openapi.yaml — four small transport layers to maintain.
- quicktype's per-language serializer idioms must be spot-checked once per target for round-trip fidelity against the wire fixtures.
{% /consequence %}

{% consequence kind="neutral" %}
- Accepted 2026-07-24: the landing gate is satisfied — quicktype pinned at 23.0.171 in clients/sdk/package-lock.json, per-language emit options checked in at clients/sdk/quicktype-emit.json (decision: default emit idioms; strict fail-on-unknown-field decoders rejected because NFR-05's additive-only schema evolution requires unknown-field tolerance in old clients).
{% /consequence %}

## Alternatives

- **OpenAPI Generator (superseded decision)** — Single tool for REST + models, but requires a Java runtime, generates large per-language scaffolding of uneven quality, and its main value (REST client generation) covers only the contract's annex. Rejected with the SoT re-layering.
- **Per-language native generators (Apple swift-openapi-generator, Kiota, openapi-typescript)** — Most idiomatic output, but four independent toolchains and pin/update cycles — the plan's named 'excessive parallel scope' risk.
- **Hand-written codegen scripts for all four languages** — Full control and no external tool trust, but owning a four-target code generator is a standing project; quicktype provides the same model-generation output at near-zero maintenance.

## Related

- decision inputs: `decision-input-sdk-generator-toolchain`
- requirements: `FR-P1-02`, `NFR-01`
- contracts: `contract-sdk-generation-determinism`
