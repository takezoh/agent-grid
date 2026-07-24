---
id: adr-20260724-deep-link-schema-additive-extension
kind: adr
title: DeepLinkRouter implements client-side alias for question/jump variants in Phase
  2 AND proposes an additive protocol/deep-links.schema.json PR upstream
status: accepted
created: '2026-07-24'
summary: DeepLinkRouter implements client-side alias for question/jump variants in
  Phase 2 AND proposes an additive protocol/deep-links.schema.json PR upstream
decision_makers:
- agent-grid-maintainers
consulted:
- windows-shell-maintainers
- workspace-maintainers
- server-api-maintainers
informed:
- agent-grid-users
tags:
- native-clients
- windows-shell
- phase2
owners:
- agent-grid-maintainers
relations:
- type: originatedFrom
  target: change-20260723-windows-shell-phase2
source_paths: []
consequences:
  positive:
  - Phase 2 S4-S5 deep-link entry points to F-004/F-005 are not blocked on cross-team
    schema sign-off.
  - Track B keeps the wire contract honest long-term; the alias is documented as removable
    rather than a permanent divergence.
  negative:
  - The alias table is a documented interim; a future Phase 1 schema change that goes
    non-additive would require alias removal + generated-SDK migration. Track B keeps
    additive-only per adr-20260724-deep-link-shape-adopts-remote-control-plan.
  - Generated SDKs (C#/TS) continue to type deep links against the current session/approval
    enum; router-side alias code is the only path that recognizes the extra kinds.
    contract-deep-link-question-jump-kind-gap's outcome partition now includes {routed-via-alias,
    routed-via-typed-helper, explicitly-rejected}.
  neutral:
  - Track B PR is a follow-up outside this change; when it lands, a supersedes-this
    ADR removes the alias.
---
# DeepLinkRouter implements client-side alias for question/jump variants in Phase 2 AND proposes an additive protocol/deep-links.schema.json PR upstream

## Context

{% context %}
Plan §3.4 assumes agent-grid://question/<id> and agent-grid://session/<id>/jump routes. protocol/deep-links.schema.json's currently accepted kind enum is [session, approval] only, with no path-suffix variant. User consultation (2026-07-24) resolved that Phase 2 must not block on Phase 1 protocol sign-off; we implement the routes client-side now and pursue the upstream schema change in parallel.
{% /context %}

## Decision

{% decision %}
Two-track: (Track A, Phase 2 in-scope) DeepLinkRouter implements a documented client-side alias table that maps `agent-grid://question/<id>` to the `approval`-kind panel-focus routing pathway (question requests use the same panel item-focus UI as approval requests) and interprets `agent-grid://session/<id>/jump` as a `session`-kind route with a `jumpBack=true` router-local hint (never emitted on the wire). The alias table lives in `clients/windows-shell/AgentGrid.Shell.Core/DeepLinkRouter/AliasTable.cs` and is explicitly documented as an interim convention. (Track B, upstream) A PR is opened against protocol/deep-links.schema.json extending the kind enum with 'question' and adding an optional 'path_suffix' variant for '/jump' — additive-only per adr-20260724-deep-link-shape-adopts-remote-control-plan. Once the schema PR is merged and generated SDKs regenerate, DeepLinkRouter's alias table is removed and routes flow through the typed helper directly (superseding ADR filed at that time).
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- Phase 2 S4-S5 deep-link entry points to F-004/F-005 are not blocked on cross-team schema sign-off.
- Track B keeps the wire contract honest long-term; the alias is documented as removable rather than a permanent divergence.
{% /consequence %}

{% consequence kind="negative" %}
- The alias table is a documented interim; a future Phase 1 schema change that goes non-additive would require alias removal + generated-SDK migration. Track B keeps additive-only per adr-20260724-deep-link-shape-adopts-remote-control-plan.
- Generated SDKs (C#/TS) continue to type deep links against the current session/approval enum; router-side alias code is the only path that recognizes the extra kinds. contract-deep-link-question-jump-kind-gap's outcome partition now includes {routed-via-alias, routed-via-typed-helper, explicitly-rejected}.
{% /consequence %}

{% consequence kind="neutral" %}
- Track B PR is a follow-up outside this change; when it lands, a supersedes-this ADR removes the alias.
{% /consequence %}

## Alternatives

- **Wait for Phase 1 additive extension to land before enabling question/jump routes** — Blocks Phase 2 S4-S5 on cross-team sign-off with no compensating benefit; the alias table is documented and removable.
- **Client-side alias only, no upstream PR** — Leaves the wire contract permanently divergent from what plan §3.4 assumes; violates the additive-evolution intent of the parent ADR.

## Related

- decision inputs: (none)
- requirements: `FR-B1-01`
- contracts: `contract-deep-link-question-jump-kind-gap`
- change: `change-20260723-windows-shell-phase2`
