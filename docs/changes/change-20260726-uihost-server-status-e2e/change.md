---
id: change-20260726-uihost-server-status-e2e
kind: change
title: Define and verify uihost-server UI status E2E
status: done
created: '2026-07-26'
profile: sdd@1
intent: Define UI/UX E2E as the real uihost-to-server path and verify composite Codex
  status rendering end to end.
outcomes:
- Real server and real uihost render spinner, bell, attention, and combined status
  patterns from fake Codex events.
scope:
- clients/ui real-stack Playwright harness, deterministic fake Codex scenarios, and
  E2E ownership documentation
non_goals:
- Windows shared UI duplication and real Codex nondeterministic browser execution
change_classes:
- boundary
governance:
  gate: auto
  reasons: []
evidence_refs:
- type: test
  ref: clients/ui:npm run test:e2e:real
- type: test
  ref: go:test ./host/runtime/subsystem/stream/fake ./host/runtime/subsystem/stream
    ./server/api/...
- type: test
  ref: clients/ui:npm run test:unit + lint + typecheck
members:
- role: requirements
  path: changes/change-20260726-uihost-server-status-e2e/requirements.md
  required: true
- role: implementation
  path: changes/change-20260726-uihost-server-status-e2e/implementation.md
  required: true
- role: verification
  path: changes/change-20260726-uihost-server-status-e2e/verification.md
  required: true
promotion:
- action: none
  reason: The durable E2E ownership boundary was recorded directly in design-client
    and note-20260624-agent-testing by this change.
unresolved_decisions: []
tags: []
owners: []
relations:
- {type: modifies, target: design-client}
source_paths:
- clients/ui/e2e
- clients/ui/playwright.real.config.ts
- clients/ui/scripts/e2e-real-stack.sh
- src/host/runtime/subsystem/stream/fake/appserver.go
- docs/design/design-host.md
- docs/note/note-20260624-agent-testing.md
summary: Establish real uihost-to-server Playwright E2E for composite session status
  while keeping Windows E2E native-surface scoped.
updated: '2026-07-26'
promotion_applied_at: '2026-07-26T10:17:11.419510+00:00'
closure:
  closed_at: '2026-07-26T10:25:58.479064+00:00'
  content_hash: sha256:46b77281744deda7f7145581b4366fbe3167a121aec8ec9988c424c42e6c1fc3
---

## Summary

## Closure Notes


{% transition from="draft" to="ready" date="2026-07-26" %}
Requirements and E2E boundary accepted by the user.
{% /transition %}


{% transition from="ready" to="active" date="2026-07-26" %}
Implementation and real-stack execution started.
{% /transition %}


{% transition from="active" to="closing" date="2026-07-26" %}
Implementation, tests, real-stack E2E, documentation, and independent critique completed.
{% /transition %}
