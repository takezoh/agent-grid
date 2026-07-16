---
id: change-20260626-2026-06-26-terminal-scrollback
kind: change
title: 20260626 2026 06 26 Terminal Scrollback
status: draft
created: '2026-06-26'
summary: Imported legacy SDD package from docs/specs/2026-06-26-terminal-scrollback.
profile: sdd@1
intent: Imported legacy SDD package; review before further promotion.
outcomes: []
scope: []
non_goals: []
change_classes: []
governance:
  gate: soft
  reasons:
  - legacy-import
members:
- role: implementation
  path: changes/change-20260626-2026-06-26-terminal-scrollback/implementation.md
  required: true
- role: requirements
  path: changes/change-20260626-2026-06-26-terminal-scrollback/requirements.md
  required: true
- role: verification
  path: changes/change-20260626-2026-06-26-terminal-scrollback/verification.md
  required: true
promotion: []
unresolved_decisions: []
relations:
- {type: references, target: adr-20260624-0010-surface-output-sequence-per-subscribe}
- {type: references, target: adr-20260624-0025-transcript-rest-backfill-then-ws-tail}
- {type: references, target: adr-20260624-0066-terminal-scrollback-via-vt-buffer}
---

## Legacy Import

Imported losslessly from v1 artifacts.
