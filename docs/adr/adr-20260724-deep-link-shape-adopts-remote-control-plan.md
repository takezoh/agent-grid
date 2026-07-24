---
id: adr-20260724-deep-link-shape-adopts-remote-control-plan
kind: adr
title: deep-links.schema.json adopts the URI shape recorded in plans/remote-control-mobile-session-deep-link.md
status: accepted
created: 2026-07-24
summary: deep-links.schema.json adopts the URI shape recorded in plans/remote-control-mobile-session-deep-link.md
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
    - 'The upstream design''s URI shape is adopted rather than reinvented (audit trail is clean).'
    - SDKs expose a typed helper; shells cannot hand-parse and regress.
  negative:
    - 'Any future shape addition (thread/<id>, approval/<id>?ack=...) requires an additive schema evolution per NFR-05.'
  neutral:
    - 'Codex''s inability to deep-link an arbitrary thread from a mobile app (per the upstream analysis) is a Codex-side limitation; the agent-grid:// shape stays useful because agent-grid owns the resolution target.'
---


# deep-links.schema.json adopts the URI shape recorded in plans/remote-control-mobile-session-deep-link.md

## Context

{% context %}
FR-P1-09 requires an agent-grid:// URI shape declared in deep-links.schema.json. The user's earlier investigation is captured in plans/remote-control-mobile-session-deep-link.md, which analyzed the notification-to-mobile-deep-link path for Claude vs Codex. Silently inventing an alternate shape here would drop that upstream candidate (issue-d2-remote-control-deep-link-decision-input-drop).
{% /context %}

## Decision

{% decision %}
deep-links.schema.json declares two path shapes verbatim from plans/remote-control-mobile-session-deep-link.md's analysis: agent-grid://session/<id> (mobile-deep-link-open-session) and agent-grid://approval/<id> (in-panel approval jump). Every generated SDK exposes a typed parse/construct helper for both. Extended URI shapes (thread, notification-actioned approval) are deferred to Phase 3 mobile implementation and are called out in the ADR's Alternatives, not silently deferred.
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- The upstream design's URI shape is adopted rather than reinvented (audit trail is clean).
- SDKs expose a typed helper; shells cannot hand-parse and regress.
{% /consequence %}

{% consequence kind="negative" %}
- Any future shape addition (thread/<id>, approval/<id>?ack=...) requires an additive schema evolution per NFR-05.
{% /consequence %}

{% consequence kind="neutral" %}
- Codex's inability to deep-link an arbitrary thread from a mobile app (per the upstream analysis) is a Codex-side limitation; the agent-grid:// shape stays useful because agent-grid owns the resolution target.
{% /consequence %}

## Alternatives

- **Invent an ad-hoc shape here** — Silently drops an upstream candidate; loses the analysis the plan already recorded.
- **Defer deep links entirely to Phase 3** — Phase 1 is the plan's own SDK-generation exit criterion for deep-links.schema.json (FR-P1-09).

## Related

- decision inputs: `decision-input-deep-link-uri-shape`
- requirements: `FR-P1-09`
- contracts: `contract-deep-link-uri-shape`
