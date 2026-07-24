---
id: adr-20260724-capability-negotiation-bundled-remote-two-axis
kind: adr
title: Capability negotiation uses a bundled/remote two-axis policy; bundled skips per-capability negotiation
status: accepted
created: 2026-07-24
summary: Capability negotiation uses a bundled/remote two-axis policy; bundled skips per-capability negotiation
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
    - Bundled clients pay no negotiation cost (NFR-04).
    - Remote clients cannot silently invoke unavailable capabilities.
    - Version-skew scenarios are recorded contract-evolution profile territory, not ad-hoc runtime behavior.
  negative:
    - Two branches in the handshake increase the negotiation code path count; both branches must be covered by tests.
    - Missing-version-field peers cause a strict downgrade that may confuse pre-Phase-1 peers; the fail-closed default is intentional.
  neutral:
    - capabilities.schema.json is authored under protocol/; per-capability entries are additive-only per NFR-05.
---


# Capability negotiation uses a bundled/remote two-axis policy; bundled skips per-capability negotiation

## Context

{% context %}
The plan distinguishes a bundled daemon (same build as the shell) from a remote/version-skewed daemon. Paying per-capability negotiation cost on the bundled axis contradicts NFR-04. A single-axis semver-only policy would contradict plan §Strategic decisions #5.
{% /context %}

## Decision

{% decision %}
Two-axis compatibility policy: (a) bundled — client and daemon share build; handshake performs a single version-match check and assumes full capability compatibility (no extra round-trip); (b) remote/version-skew — daemon returns its capabilities.schema.json-declared feature set and the client degrades any capability the daemon does not declare to a documented disabled/hidden state, never invoking it speculatively. A peer that omits the version field entirely is treated as lowest-capability (fail-closed on unknown).
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Bundled clients pay no negotiation cost (NFR-04).
- Remote clients cannot silently invoke unavailable capabilities.
- Version-skew scenarios are recorded contract-evolution profile territory, not ad-hoc runtime behavior.
{% /consequence %}

{% consequence kind="negative" %}
- Two branches in the handshake increase the negotiation code path count; both branches must be covered by tests.
- Missing-version-field peers cause a strict downgrade that may confuse pre-Phase-1 peers; the fail-closed default is intentional.
{% /consequence %}

{% consequence kind="neutral" %}
- capabilities.schema.json is authored under protocol/; per-capability entries are additive-only per NFR-05.
{% /consequence %}

## Alternatives

- **Single-axis semver-range compatibility check applied uniformly** — Contradicts plan §Strategic decisions #5's explicit bundled-vs-remote framing; forces bundled clients to pay a negotiation round-trip for no observable benefit.

## Related

- decision inputs: `decision-input-capability-negotiation-axes`
- requirements: `FR-P1-03`, `FR-P1-04`, `NFR-04`
- contracts: `contract-capability-negotiation-policy`
