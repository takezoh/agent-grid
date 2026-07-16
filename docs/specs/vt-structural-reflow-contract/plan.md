---
id: plan-20260716-vt-structural-reflow-contract
kind: plan
title: VT structural reflow contract implementation plan
status: draft
created: '2026-07-16'
goal: Replace split cell/provenance ownership with one mutation-closed x/vt boundary
  and preserve it through resize, snapshot, concurrency, and integration.
scope_in:
- x/vt semantic row ownership and defensive projections
- primary semantic reflow and alternate physical resize
- typed concrete snapshot API and SafeEmulator locking
- 'agent-grid failure mapping and PR #908/#919 integration regression'
scope_out:
- ultraviolet changes
- raw PTY replay or browser-side terminal reconstruction
- Terminal interface widening
- 'replacement or folding of the independent PR #908 margin clamp'
milestones:
- id: m1
  title: Establish the semantic owner and closed mutation contract
  status: todo
- id: m2
  title: Separate primary reflow from alternate physical resize
  status: todo
- id: m3
  title: Close snapshot failure and concurrency contracts
  status: todo
- id: m4
  title: Prove compatibility and integrated PR behavior
  status: todo
contracts:
- Cells and RowBoundary are mutated only through the x/vt semantic owner.
- Primary resize reflows semantic history; alternate resize preserves physical row
  identity.
- Concrete ReattachSnapshot returns bytes or typed error; SafeEmulator holds its lock
  for the complete observation.
- 'Pre-919 exported API compile fixtures pass and PR #908 remains independently required.'
adrs:
- adr-20260716-vt-semantic-buffer-owner
- adr-20260716-vt-screen-specific-resize
- adr-20260716-vt-snapshot-failure-and-locking
- adr-20260716-vt-reflow-pr-migration
reference_algorithms:
- id: alg-row-transition
  purpose: Apply a closed operation-specific boundary and pending-wrap transition.
- id: alg-primary-reflow
  purpose: Group retained rows into logical lines and repack them at target width.
- id: alg-alternate-physical-resize
  purpose: Resize an alternate grid without semantic grouping or history access.
verification_profiles:
- profile: semantic-core
  milestone: m1
  tier: T0
  command: cd vt && go test ./...
  criterion: Mutation matrix, erase regression, cap, Unicode, and structural owner
    tests pass.
  milestone_dod: No raw RenderBuffer mutation or mutable provenance alias bypasses
    the owner.
- profile: resize-policy
  milestone: m2
  tier: T1
  command: cd vt && go test ./...
  criterion: Primary and all four alternate width/height fixtures pass against public
    emulator observations.
  milestone_dod: Primary and alternate policy selection cannot silently share semantic
    reflow.
- profile: concurrency
  milestone: m3
  tier: T1
  command: cd vt && go test -race ./...
  criterion: Concurrent Write, Resize, and ReattachSnapshot report zero races.
  milestone_dod: Successful snapshots are point-in-time consistent and failures contain
    no bytes.
- profile: consumer-contract
  milestone: m4
  tier: T2
  command: cd src && go test ./platform/termvt/...
  criterion: Agent-grid publishes only successful seeds and marks snapshot-failed
    sessions unusable.
  milestone_dod: Existing attach ordering tests and pre-919 compile fixtures pass.
- profile: full-integration
  milestone: m4
  tier: T2
  command: cd src && go test ./...
  criterion: 'Full agent-grid Go suite passes with the updated x pin and PR #908/#919 combined regressions.'
  milestone_dod: Both PR invariants coexist without weakening accepted ADR contracts.
implementation_checklist:
  required:
  - All mutation classes have an explicit transition and regression test.
  - Public read paths return defensive value projections with no mutable aliases.
  - Alternate resize fixtures cover width grow/shrink and height grow/shrink.
  - SafeEmulator race test and agent-grid failure test pass.
  recommended:
  - Benchmarks demonstrate linear retained-cell behavior at two scrollback sizes.
  - 'Migration note inventories pre-919 and PR #919 API changes.'
  operational:
  - 'Merge PR #908, rebase PR #919, and run combined verification before publication.'
tags: []
owners: []
relations:
- {type: implements, target: spec-20260716-vt-structural-reflow-contract}
- {type: hasPart, target: adr-20260716-vt-semantic-buffer-owner}
- {type: hasPart, target: adr-20260716-vt-screen-specific-resize}
- {type: hasPart, target: adr-20260716-vt-snapshot-failure-and-locking}
- {type: hasPart, target: adr-20260716-vt-reflow-pr-migration}
source_paths:
- src/go.mod
methodology: sdd
summary: Implement one private x/vt semantic owner, explicit screen resize policies,
  and a synchronized typed snapshot boundary.
---

## Goal

Implement the contracts in the companion specification without replacing the accepted terminal-history or geometry-bearing-attach decisions. The implementation remains inside x/vt except for the narrow agent-grid consumer adaptation and module pin.

## Implementation Sequence

### m1

{% milestone id="m1" %}
Create the private semantic owner and move every cell, row, boundary, truncation, and pending-wrap mutation behind it. Use a single `RowBoundary` value (`Hard`, `Soft`, or `TruncatedHead`) instead of independent booleans. Keep ultraviolet as the cell/damage implementation detail, but make raw mutation inaccessible outside the owner using an x/vt internal package or an equivalently compiler-enforced private type boundary.

Task-grade units:

- **Semantic row value and owner** — output: internal owner/value types plus unit tests; files: x/vt screen/buffer internals; boundary: no resize or serializer policy; acceptance: invalid boundary combinations are unrepresentable and blank/moved rows preserve their specified boundary.
- **Mutation closure and projections** — output: closed transition table, handler migration, defensive `CellAt`/`Line`/`Lines`/scrollback projections, and regression tests; files: x/vt handlers, screen, scrollback; boundary: do not add public provenance mutation APIs; acceptance: EL2, ED, ICH, DCH, horizontal scroll, IL/DL, scroll, reset, and cap fixtures pass and structural checks find no bypass.
{% /milestone %}

### m2

{% milestone id="m2" %}
Implement `alg-primary-reflow` as a pure transformation over immutable semantic rows and `alg-alternate-physical-resize` as a separate operation. The emulator is the only policy chooser. Preserve current/saved cursor, tab stops, pending wrap, wide/combining cells, style, and margin bounds.

Task-grade units:

- **Primary reflow core** — output: grouping/packing code and T0 fixtures; files: x/vt reflow internals; acceptance: hard/soft/truncated grouping, cursor mapping, Unicode, and cap cases pass with O(retained cells) behavior.
- **Screen resize coordinator** — output: separate primary/alternate calls and public-observation tests; files: x/vt emulator/screen resize; acceptance: alternate width clipping/restoration and cursor-anchored height grow/shrink match the pinned xterm fixtures and never touch primary history; PR #908 margin tests remain green.
{% /milestone %}

### m3

{% milestone id="m3" %}
Keep `ReattachSnapshot() ([]byte, error)` on concrete `Emulator` and add an explicit locking method on `SafeEmulator`; do not widen the existing `Terminal` interface. Agent-grid owns a narrow consumer-side snapshot interface. Serialize an immutable owner view, and return a typed invariant failure with no partial bytes.

Task-grade units:

- **Snapshot serializer and API** — output: pure ANSI serializer, concrete API, typed error, snapshot fixtures; files: x/vt reattach/emulator; acceptance: primary/alternate inclusion, CRLF/Soft joining, cursor/pending-wrap restoration, and zero-byte failure pass.
- **SafeEmulator and agent-grid mapping** — output: explicit lock wrapper, race test, narrow adapter seam, actor fail-closed test; files: x/vt safe emulator and agent-grid termvt adapter/actor; acceptance: race detector is clean and snapshot failure cannot publish a seed or allow a usable session.
{% /milestone %}

### m4

{% milestone id="m4" %}
Inventory the exported API at the pre-919 base and add a compile fixture. Remove or privatize only PR #919-introduced mutable provenance APIs. Merge PR #908 independently first, rebase PR #919, retain the margin regression, update agent-grid's x pin, and run the complete verification set.

Task-grade units:

- **Compatibility and PR integration** — output: API inventory, compile fixture, migration note, rebased draft, module pin, and combined tests; files: x/vt API tests, PR notes, agent-grid `src/go.mod`; acceptance: pre-919 consumer fixture compiles, new mutable provenance APIs are absent, and both #908 and #919 invariants pass.
{% /milestone %}

## Targets

- **x/vt private semantic owner**: sole mutation seam around ultraviolet `RenderBuffer`; no new external dependency is introduced.
- **Pure seams**: immutable row values feed `alg-row-transition`, `alg-primary-reflow`, and the ANSI serializer without I/O or locks.
- **Resize policy seam**: `Emulator` explicitly selects primary semantic reflow or alternate physical resize; tests inject screen state through public terminal operations.
- **Snapshot seam**: concrete x/vt method returns `([]byte, error)`; agent-grid declares the narrow interface it consumes, avoiding a breaking addition to x/vt `Terminal`.
- **Concurrency seam**: `SafeEmulator` owns the lock and holds it through state capture and serialization.
- **External reference seam**: pinned xterm.js fixtures are generated/recorded as test data only; xterm.js is not a runtime dependency.
- **Integration targets**: x fork PR #919 and its vt tests, independent PR #908 margin tests, agent-grid terminal adapter/actor tests, and `src/go.mod`.

Structural fitness functions:

- A grep/import test fails when raw `RenderBuffer` mutation appears outside the owner boundary.
- A closed transition coverage test fails when a semantic-affecting handler has no transition-table entry.
- A compile fixture fails when a pre-919 exported API no longer compiles.
- The Go race detector fails when any SafeEmulator state path bypasses synchronization.

## Verification

| Profile | Tier | Command | Criterion / milestone DoD |
|---|---|---|---|
| semantic-core | T0 pure | `cd vt && go test ./...` | Mutation matrix, projections, erase/cap/Unicode regressions pass; no owner bypass exists. |
| resize-policy | T1 wired | `cd vt && go test ./...` | Public emulator fixtures distinguish primary reflow from alternate width clipping/restoration and cursor-anchored height resizing. |
| concurrency | T1 wired | `cd vt && go test -race ./...` | No race; each snapshot is one state or a zero-byte typed failure. |
| consumer-contract | T2 contract | `cd src && go test ./platform/termvt/...` | Agent-grid seed ordering and fail-closed session behavior pass. |
| full-integration | T2 contract | `cd src && go test ./...` | Updated x pin, accepted attach contracts, and #908/#919 combined regressions pass. |

## Reference Algorithms

### alg-row-transition

```text
transition(operation, rows, cursor):
  class = closedTable.lookup(operation)
  if class is missing: fail contract
  mutate cells and row placement through owner
  apply class.boundaryRule and class.pendingWrapRule
  normalize new blanks to Hard and evicted Soft head to TruncatedHead
  assert every retained row has exactly one valid boundary
```

### alg-primary-reflow

```text
primaryReflow(history, visible, geometry, cursor):
  clone immutable semantic rows
  split logical fragments at Hard or TruncatedHead
  preserve cell clusters and map cursor to logical cell offset
  pack fragments to target width, marking continuation rows Soft
  divide bounded result into history and visible target height
  restore cursor, saved cursor, pending wrap, tab stops, and bounded margins
```

### alg-alternate-physical-resize

```text
alternatePhysicalResize(grid, geometry):
  never group rows or read/write primary history
  on width shrink, clip the viewport while retaining hidden right cells
  on width growth, restore retained cells or append blanks when no cells exist
  on height growth, append blank Hard rows
  on height shrink, remove rows below the cursor first, then trim rows above it
  adjust current and saved cursors using the same cursor-anchored row shift
```

## Implementation Checklist

- Required: mutation closure, defensive projections, screen-specific fixtures, concurrency, and failure mapping.
- Recommended: retained-cell benchmarks and an API migration inventory.
- Operational: merge #908, rebase #919, and verify both invariants together before publication.
