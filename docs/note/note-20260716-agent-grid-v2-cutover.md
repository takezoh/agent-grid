---
id: note-20260716-agent-grid-v2-cutover
kind: note
title: Agent-grid dev-docs v2 cutover
status: published
created: '2026-07-16'
tags: []
owners: []
relations:
- {type: references, target: design-client}
- {type: references, target: design-orchestrator}
- {type: references, target: design-platform}
source_paths: []
summary: Records the transactional v1-to-v2 migration, semantic design consolidation,
  compatibility aliases, and verification gates.
updated: '2026-07-16'
---

## Summary

Agent-grid の canonical development docs を format v1 から format v2 へ transaction migration した。24 個の legacy SDD package は `docs/changes/` の change package へ、18 個の active component は client / orchestrator / platform の 3 area design へ統合した。

旧 plan / spec / ux / component ID は `docs/aliases.yaml` で解決できる。legacy component 本文は各 design の migration history として完全保存し、先頭の stable statements を今後の governing design surface とする。

## Migration record

- Exact manifest: `docs/migration/agent-grid-v2-manifest.yaml`
- Source snapshot: migration branch based on the repository state through the runtime PATH ownership implementation.
- Inventory: 279 documents, 277 structured, 2 passthrough, no duplicate IDs.
- Consolidation: 8 client components → `design-client`; 2 orchestrator components → `design-orchestrator`; 8 platform components → `design-platform`.
- Recent PATH ownership knowledge was promoted into `design-platform` as responsibility `RESP-006` and invariant `INV-008`.

## Verification

- Migration manifest check passed with no unresolved blocker.
- Reviewed stage and post-apply `docs lint --conformance` both indexed successfully with zero warnings.
- Representative legacy component, plan, and spec IDs resolve through aliases.
- Maintainer and implementer context generation succeeds. Imported unfinished changes intentionally remain `draft`; missing owner and acceptance diagnostics are not inferred away.
- Every legacy component source remains embedded verbatim under the corresponding design's `Legacy Source` sections.

## Operating policy

All new development work uses v2 change packages. Imported draft changes are historical work-in-progress and must be reviewed before lifecycle promotion. Stable architecture changes update a design through an explicit change package and promotion operation rather than editing legacy history.
