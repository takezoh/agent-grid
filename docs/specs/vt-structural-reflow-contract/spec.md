---
id: spec-20260716-vt-structural-reflow-contract
kind: spec
title: VT structural reflow contract
status: draft
created: '2026-07-16'
tags: []
owners: []
functional_requirements:
- id: FR-001
  statement: The system shall expose every retained physical row as cells and exactly one boundary value of Hard, Soft, or TruncatedHead from one mutation owner.
  priority: must
  rationale: Invalid combinations such as soft and truncated-head must be unrepresentable.
- id: FR-002
  statement: When autowrap advances to the next row, the system shall mark that row Soft, and when an explicit line transition advances rows, the system shall mark the destination Hard.
  priority: must
- id: FR-003
  statement: When any operation changes cells, row positions, row boundaries, truncation, or pending wrap, the system shall apply a classified transition through the semantic row owner or reject the unclassified operation in the contract suite.
  priority: must
  rationale: Covers erase, overwrite, horizontal edit, line edit, scroll, reset, resize, and screen transition rather than row-count changes only.
- id: FR-004
  statement: When the primary screen is resized, the system shall reflow retained history and visible rows at Hard and TruncatedHead boundaries while preserving cells, styles, wide and combining characters, cursor logical position, saved cursor, and pending wrap.
  priority: must
- id: FR-005
  statement: When the alternate screen width or height grows or shrinks, the system shall physically resize without semantic grouping or primary-history access, clip rather than join rows on width shrink, preserve clipped cells across an otherwise unmodified shrink-expand cycle, append blank cells or Hard rows on growth, and on height shrink remove rows below the cursor before trimming rows above it to keep the cursor visible.
  priority: must
- id: FR-006
  statement: When scrollback eviction removes the head of a Soft continuation chain, the system shall change the first retained fragment to TruncatedHead and shall not join it to a nonexistent predecessor.
  priority: must
- id: FR-007
  statement: When a reattach snapshot is requested, the system shall emit opaque ANSI bytes with no newline across Soft boundaries, CRLF across Hard and TruncatedHead boundaries, screen-specific history inclusion, and cursor and pending-wrap restoration.
  priority: must
- id: FR-008
  statement: If semantic row invariants are violated during snapshot construction, then the system shall return a typed failure with no partial snapshot bytes.
  priority: must
- id: FR-009
  statement: When ReattachSnapshot is called on SafeEmulator concurrently with Write or Resize, the system shall observe one lock-protected emulator state without a data race.
  priority: must
- id: FR-010
  statement: If the agent-grid terminal adapter receives a snapshot failure, then the system shall not publish a seed and shall mark the session unusable under the existing actor ordering contract.
  priority: must
- id: FR-011
  statement: When EL2, whole-row ED, EL0 from column zero, reset, or another operation removes a continuation origin, the system shall change the affected row boundary to Hard before subsequent writes are observed.
  priority: must
- id: FR-012
  statement: When ICH, DCH, or horizontal scroll moves the cell-to-column mapping, the system shall apply the operation-specific closed transition table to row boundary and pending-wrap state.
  priority: must
- id: FR-013
  statement: When resize races with a stale margin control sequence, the system shall keep horizontal and vertical scroll margins within the current screen bounds.
  priority: must
  rationale: Preserves the independent PR #908 invariant.
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: Go race detector reports zero races for concurrent SafeEmulator Write, Resize, and ReattachSnapshot tests.
  measurement: cd vt && go test -race ./...
- id: NFR-002
  type: performance
  criteria: Resize and snapshot time and additional memory are O(retained cells), while ordinary write and scroll do not scan all retained history.
  measurement: Complexity review plus bounded-history benchmarks at two history sizes.
- id: NFR-003
  type: compatibility
  criteria: Every exported Terminal, Screen, and Scrollback API present before PR #919 passes a compile fixture without source edits; only PR #919-introduced mutable provenance APIs may be removed or changed.
  measurement: Compile-time consumer fixture against the pre-919 exported API inventory.
- id: NFR-004
  type: maintainability
  criteria: Raw RenderBuffer mutation exists only inside the semantic owner package and every parser handler that can affect semantic state maps to one closed transition-table entry.
  measurement: Structural grep/import test and transition coverage test.
- id: NFR-005
  type: portability
  criteria: The implementation adds no third-party dependency and no agent-grid-specific type to x/vt.
  measurement: go.mod diff and dependency-boundary review.
acceptance:
- id: AC-001
  given: A primary screen at width 5 contains the logical text abcdefghij wrapped over two physical rows.
  when: The screen is resized to width 10 and a reattach snapshot is consumed at width 10.
  then: The consumer observes abcdefghij as one logical line with the cursor at its preserved logical position.
  requirement_refs: [FR-004, FR-007]
- id: AC-002
  given: An alternate screen has distinct physical rows, content to the right of a narrower viewport, and a cursor near the bottom.
  when: Its width is shrunk then restored and its height is shrunk.
  then: Rows are never semantically joined, clipped right-side cells reappear after the unmodified width restoration, rows below the cursor are removed before rows above it, the cursor remains visible, and primary history is unchanged.
  requirement_refs: [FR-005]
- id: AC-003
  given: A Soft second row is cleared by EL2 and then receives Z.
  when: A reattach snapshot is generated.
  then: The snapshot contains a hard break between the preceding row and Z and does not emit abcdeZ as one line.
  requirement_refs: [FR-011, FR-007]
- id: AC-004
  given: SafeEmulator receives concurrent Write, Resize, and ReattachSnapshot calls.
  when: The race-enabled contract test runs repeatedly.
  then: No race is reported and every successful snapshot represents one complete emulator state.
  requirement_refs: [FR-009, NFR-001]
- id: AC-005
  given: Semantic state violates a boundary invariant before agent-grid attachment.
  when: The adapter requests a snapshot.
  then: No seed is published and the session becomes unusable without live output overtaking a seed.
  requirement_refs: [FR-008, FR-010]
- id: AC-006
  given: PR #908 margin clamps and the structural reflow changes are both applied.
  when: Resize is followed by stale horizontal and vertical margin sequences.
  then: Margins remain within the resized screen and no out-of-range mutation occurs.
  requirement_refs: [FR-013]
entities:
- name: RowBoundary
  fields:
  - 'kind: Hard | Soft | TruncatedHead'
  invariants:
  - Exactly one kind exists for each retained physical row.
  - Soft requires a retained predecessor in the same semantic chain.
  - TruncatedHead is a hard serialization boundary and records missing predecessor content.
  normalization_rules:
  - Evicting a Soft predecessor converts the new retained head to TruncatedHead.
  - A new blank row starts Hard.
- name: SemanticRow
  fields:
  - 'cells: defensive value projection of ultraviolet cells'
  - 'boundary: RowBoundary'
  invariants:
  - Cells and boundary are created, moved, erased, and removed by the same owner operation.
  normalization_rules:
  - Public reads return a deep value projection and never a mutable owner alias.
- name: SnapshotFailure
  fields:
  - 'class: invariant violation'
  - 'cause: diagnostic detail'
  invariants:
  - A failure carries no partial ANSI bytes.
  normalization_rules:
  - The agent-grid adapter maps it to seed non-publication and session unusable.
failure_modes:
- class: invariant-violation
  detection: Semantic owner validation during snapshot construction.
  recovery: fail_fast
  related_fr: [FR-008, FR-010]
- class: unclassified-mutation
  detection: Closed transition coverage and raw-mutation structural test.
  recovery: fail_fast
  related_fr: [FR-003]
- class: snapshot-concurrency-race
  detection: Go race detector on SafeEmulator public operations.
  recovery: fail_fast
  related_fr: [FR-009]
non_goals:
  must_not:
  - Move terminal semantic reconstruction into agent-grid or the browser.
  - Replace bounded semantic history with raw PTY transcript replay.
  - Treat PR #919 as a replacement for the independent PR #908 margin clamp.
  should_not:
  - Change ultraviolet to carry terminal-specific provenance.
  - Add ReattachSnapshot to the existing Terminal interface and break external implementers.
worked_examples:
- id: ex-001
  fr: FR-004
  cases:
  - input: width 5 rows [abcde(Hard), fghij(Soft)] -> width 10
    expected: width 10 visible row abcdefghij(Hard)
  - input: width 10 row abcdefghij(Hard) -> width 5
    expected: rows abcde(Hard), fghij(Soft)
  - input: retained head fghij(TruncatedHead) -> width 10
    expected: fghij remains a standalone truncated logical fragment
  counterexample:
    input: EL2 changes the second row cells but leaves its Soft marker
    wrong_output: snapshot joins the new second-row text to the preceding row
    why_wrong: FR-011 requires removal of the obsolete continuation origin before later writes are observed.
relations:
- {type: implementedBy, target: plan-20260716-vt-structural-reflow-contract}
source_paths:
- src/go.mod
methodology: sdd
summary: Define a mutation-closed x/vt row owner, screen-specific resize behavior, and synchronized typed snapshot contract.
---

## Overview

This specification repairs the ownership and public contracts behind x/vt semantic history reflow. It complements the accepted terminal-history and geometry-bearing-attach ADRs and preserves the independent PR #908 margin guarantee.

## Requirements

{% req id="FR-001" %}A retained row has cells and one valid boundary value under one owner.{% /req %}
{% req id="FR-002" %}Autowrap creates Soft; explicit line transitions create Hard.{% /req %}
{% req id="FR-003" %}Every semantic mutation is classified and owner-mediated.{% /req %}
{% req id="FR-004" %}Primary resize performs history-aware semantic reflow.{% /req %}
{% req id="FR-005" %}Alternate resize is physical and history-independent in all width/height directions.{% /req %}
{% req id="FR-006" %}Evicted continuation heads become TruncatedHead.{% /req %}
{% req id="FR-007" %}Snapshots serialize semantic boundaries as opaque ANSI.{% /req %}
{% req id="FR-008" %}Invariant failure returns typed failure without partial bytes.{% /req %}
{% req id="FR-009" %}SafeEmulator snapshots are synchronized.{% /req %}
{% req id="FR-010" %}Agent-grid fails attachment closed on snapshot failure.{% /req %}
{% req id="FR-011" %}Erase-origin operations clear obsolete Soft provenance.{% /req %}
{% req id="FR-012" %}Horizontal edits follow a closed boundary transition table.{% /req %}
{% req id="FR-013" %}Resize and stale margins retain PR #908 bounds safety.{% /req %}

The NFRs make race freedom, complexity, source compatibility, structural ownership, and dependency scope measurable.

## Acceptance Criteria

{% acceptance id="AC-001" %}Primary width 5 to 10 reconstructs one logical line.{% /acceptance %}
{% acceptance id="AC-002" %}Alternate resize follows cursor-anchored physical-row behavior and never reflows.{% /acceptance %}
{% acceptance id="AC-003" %}EL2 cannot leave a stale Soft join.{% /acceptance %}
{% acceptance id="AC-004" %}Concurrent SafeEmulator operations are race-free and point-in-time consistent.{% /acceptance %}
{% acceptance id="AC-005" %}Invalid state publishes no seed and makes the session unusable.{% /acceptance %}
{% acceptance id="AC-006" %}The #908 margin invariant survives integration.{% /acceptance %}

## Data Model

{% data_model %}{% /data_model %}

## Failure Modes

{% failure_modes %}{% /failure_modes %}

## Non-Goals

{% non_goals %}{% /non_goals %}

## Worked Examples

{% worked_example id="ex-001" %}Primary reflow, truncation, and stale-boundary examples.{% /worked_example %}
