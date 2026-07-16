---
id: adr-20260716-vt-reflow-pr-migration
kind: adr
title: VT reflow pull request migration
status: proposed
created: '2026-07-16'
decision_makers:
- unknown
tags:
- terminal
- compatibility
- github
owners: []
relations:
- {type: partOf, target: plan-20260716-vt-structural-reflow-contract}
- {type: references, target: adr-20260715-terminal-semantic-history-reattach}
- {type: references, target: adr-20260715-geometry-bearing-terminal-attach}
source_paths:
- src/go.mod
methodology: sdd
summary: 'Preserve the pre-919 source surface and keep independent PR #908 as a prerequisite.'
consequences:
  positive:
  - Released callers get a measurable compile-compatibility promise while unreleased
    unsafe APIs do not become permanent.
  - Margin safety and semantic reflow remain independently reviewable invariants.
  negative:
  - 'PR #919 must rebase after #908 and rerun cross-repository verification before leaving draft.'
  - Detached projections preserve source compatibility but can change callers that
    depended on undocumented write-through aliases.
  neutral:
  - The fork release and agent-grid module pin remain the delivery mechanism selected
    by the accepted semantic-history ADR.
confirmation: Exported API inventory, compile fixture, margin regression, and both
  repositories' test profiles pass on the final rebased commit.
---

## Context

PR #919 is still a draft and its mutable provenance APIs have not shipped. Older exported `Terminal`, `Screen`, and `Scrollback` APIs may already have external callers. Separately, PR #908 clamps stale DECSTBM/DECSLRM margins after resize and prevents an out-of-range mutation; semantic reflow neither implements nor supersedes that safety invariant.

Combining the PRs would obscure two independent review claims. Preserving every draft API would instead freeze the very ownership leak this design must remove.

## Decision

{% decision %}Treat PR #908 as an independent prerequisite. Merge it first when possible, then rebase PR #919 and retain its margin bounds code and regression tests unchanged in meaning. Revise PR #919 itself, rather than opening another reflow PR, because all semantic-owner, resize, and snapshot corrections are changes to its unmerged contract.{% /decision %}

Inventory all exported x/vt declarations immediately before PR #919. A compile-only consumer fixture is the source-compatibility criterion. Existing names and signatures remain, with defensive detached projections where write-through aliasing would violate the new owner. Only PR #919's unreleased provenance APIs and its old `ReattachSnapshot` signature may be removed or changed. The PR description must name those draft migrations, the detached-projection behavioral tightening, the pinned xterm fixture revision, and the relationship to #908.

After x/vt T0, race, and structure profiles pass on the rebased commit, publish the fork release/tag and update agent-grid's module pin. The old serializer is removed rather than retained as fallback.

## Consequences

### Positive

{% consequence kind="positive" %}Compatibility has a compile-level pass/fail test, unsafe unshipped APIs do not become legacy, and #908 remains independently reviewable and reusable.{% /consequence %}

### Negative

{% consequence kind="negative" %}Rebase ordering adds coordination and requires the full verification matrix to run again. Undocumented mutation through returned pointers becomes detached behavior even though source still compiles.{% /consequence %}

### Neutral

{% consequence kind="neutral" %}The delivery path remains fork tag plus `src/go.mod` pin. Neither PR changes multi-viewer geometry arbitration or terminal persistence.{% /consequence %}

## Alternatives

**Absorb #908 into #919 and close the old PR.** Rejected because margin bounds and semantic history are independent invariants and have different review/risk surfaces.

**Open a new structural reflow PR and leave #919 unchanged.** Rejected because #919 is unmerged draft work; splitting would preserve an incomplete contract and duplicate review context.

**Preserve every PR #919 draft API.** Rejected because no released compatibility value justifies exporting provenance decision authority.

**Break all existing aliases and signatures.** Rejected because defensive projections can close owner bypass while a compile fixture preserves the released source surface.

## Confirmation

The final rebased commit must pass the pre-919 compile fixture, stale margin regressions from #908, x/vt unit/race/structure profiles, and agent-grid wired/full tests before PR #919 is marked ready for review.
