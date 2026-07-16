---
id: adr-20260716-vt-semantic-buffer-owner
kind: adr
title: VT semantic buffer owns row provenance
status: proposed
created: '2026-07-16'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: plan-20260716-vt-structural-reflow-contract}
- {type: references, target: adr-20260715-terminal-semantic-history-reattach}
source_paths:
- src/go.mod
methodology: sdd
summary: Keep cells and terminal row provenance behind one x/vt mutation owner.
consequences:
  positive:
  - Cell and boundary mutations share one enforcement point, eliminating stale parallel
    metadata and external mutable aliases.
  negative:
  - Existing Screen and Scrollback mutations must be routed through a new internal
    boundary, producing a broad review diff.
  neutral:
  - Ultraviolet remains the cell and damage-tracking implementation and gains no terminal-specific
    metadata.
confirmation: Structural tests permit raw RenderBuffer mutation only inside the owner;
  transition coverage and defensive-projection tests cover every semantic-affecting
  handler.
---

## Context

{% context %}
PR #919 associates wrap provenance with ultraviolet rows using parallel state and exposes mutable provenance operations. EL2 already demonstrates that a cell mutation can bypass the metadata update. Existing accepted history design requires a single owner and closure over erase, overwrite, horizontal edits, row edits, scrolling, reset, resize, and eviction.
{% /context %}

## Decision

{% decision %}
x/vt will introduce one private, deep semantic buffer owner. It owns `RenderBuffer`, row boundaries, truncation, and relevant pending-wrap transitions, and exposes complete semantic operations rather than raw storage. A row boundary is a single value `Hard | Soft | TruncatedHead`; invalid boolean combinations cannot be constructed. All semantic-affecting handlers use a closed operation transition table, and raw buffer mutation outside the owner is mechanically rejected.

Scrollback stores cloned internal semantic-row values. Pre-919 read APIs remain source-compatible but return deep defensive projections, including cells; PR #919-introduced `Rows`, `ReplaceRows`, `PushWrapped`, and provenance-bearing `PushN` are privatized. No public caller receives write or alias authority over retained state.
{% /decision %}

## Consequences

{% consequence kind="positive" %}Cell and provenance drift becomes structurally local and mutation closure is enforceable by tests.{% /consequence %}
{% consequence kind="negative" %}The migration touches all x/vt mutation paths and requires an explicit operation matrix rather than a small resize-only patch.{% /consequence %}
{% consequence kind="neutral" %}Ultraviolet APIs and ownership remain unchanged; x/vt wraps them as an implementation detail.{% /consequence %}

## Alternatives

- Add provenance to ultraviolet row types: rejected because terminal continuation semantics do not belong in the lower rendering library and would expand the upstream change boundary.
- Make a separate row store authoritative and project into `RenderBuffer`: rejected because projection synchronization would create a second owner and complicate damage tracking.
- Keep parallel `wrapped []bool` and patch known call sites: rejected because it cannot mechanically close future mutation paths or external aliases.

Confirmation: run the structural owner check, closed transition coverage, and defensive-copy mutation tests; each must fail when a bypass or unclassified handler is introduced.
