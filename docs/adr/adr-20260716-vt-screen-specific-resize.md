---
id: adr-20260716-vt-screen-specific-resize
kind: adr
title: VT screen-specific resize policies
status: proposed
created: '2026-07-16'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: references, target: adr-20260715-terminal-semantic-history-reattach}
source_paths:
- src/go.mod
methodology: sdd
summary: Use semantic reflow for primary screens and physical resize for alternate
  screens.
consequences:
  positive:
  - Primary history reconstructs at new widths while alternate applications retain
    physical-grid semantics.
  negative:
  - Two resize operations and four alternate geometry fixtures must be maintained.
  neutral:
  - Emulator remains the policy chooser and existing cursor, tab-stop, and margin
    state still participates in resize.
confirmation: Public emulator fixtures distinguish primary reflow from alternate width
  and height growth and shrinkage, and verify primary history is untouched by alternate
  resize.
---

## Context

{% context %}
PR #919 currently applies semantic reflow to both screens. Primary history needs logical-line reconstruction, while alternate screen applications depend on stable physical rows and have no primary scrollback semantics. The accepted history ADR explicitly excludes alternate history.
{% /context %}

## Decision

{% decision %}
The emulator will call two distinct private operations: primary semantic reflow and alternate physical resize. Primary reflow groups at `Hard` and `TruncatedHead`, repacks at the target width, and preserves cell clusters, styles, current/saved cursor, pending wrap, tab stops, and bounded margins. Alternate resize never groups Soft rows and never accesses primary history. It follows pinned xterm physical-buffer behavior: width shrink clips without joining rows and retains clipped cells for an otherwise unmodified re-expansion; width growth restores retained cells or appends blanks; height growth appends blank Hard rows; height shrink removes rows below the cursor before trimming rows above it and adjusts current/saved cursors with that row shift.
{% /decision %}

## Consequences

{% consequence kind="positive" %}The screen identity determines one explicit, testable policy and prevents alternate state from contaminating primary history.{% /consequence %}
{% consequence kind="negative" %}Resize code cannot share a single semantic path and must retain separate fixtures and coordinator branches.{% /consequence %}
{% consequence kind="neutral" %}The policy split complements rather than replaces geometry-bearing attach and PR #908 margin normalization.{% /consequence %}

## Alternatives

- One `Screen.Resize` that infers policy from incidental state: rejected because decision authority would be implicit and easy to bypass.
- Distinct exported PrimaryScreen and AlternateScreen types: rejected because it creates a broad pre-919 API break for no additional observable requirement.
- Reflow both screens: rejected because it changes physical alternate rows and reproduced the diagnostic failure.

Confirmation: run primary semantic fixtures plus alternate width shrink-expand restoration and cursor-at-top/middle/bottom height grow/shrink fixtures through public Emulator operations; assert primary history remains byte-for-byte unchanged during alternate resize.
