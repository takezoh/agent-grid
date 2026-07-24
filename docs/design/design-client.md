---
id: design-client-redirect
kind: design
title: design-client redirect stub (see design-host)
status: superseded
created: '2026-07-23'
updated: '2026-07-24'
summary: Redirect stub. The client/ layer was renamed to host/ (see plan-20260723-repo-structure.md
  M2); this document was replaced by design-host.md and is kept only so legacy links
  in ADRs and change packages continue to resolve.
tags:
- redirect-stub
- legacy
owners: []
relations: []
source_paths: []
scope_type: system
responsibilities:
- {id: RESP-001, statement: 'Redirect legacy links to design-host; hold no live design content.'}
invariants:
- {id: INV-001, statement: 'This document holds no authoritative design content; readers must follow the reference to design-host.', enforcement: review}
boundaries:
  provides: []
  consumes: []
  forbidden: []
variability:
  fixed: []
  free: []
capabilities: []
failure_responsibilities: []
trust_boundaries: []
compatibility_policies: []
---

# design-client.md は design-host.md に改名されました

`client/` 層の `host/` への rename (plans/plan-20260723-repo-structure.md M2) に伴い、この文書は [design-host.md](./design-host.md) に移動しました。歴史文書 (ADR・docs/changes) 内の旧リンクのためにこのスタブを残しています。
