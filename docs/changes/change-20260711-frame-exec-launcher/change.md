---
id: change-20260711-frame-exec-launcher
kind: change
title: 20260711 Frame Exec Launcher
status: draft
created: '2026-07-11'
summary: Imported legacy SDD package from docs/specs/frame-exec-launcher.
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
  path: changes/change-20260711-frame-exec-launcher/implementation.md
  required: true
- role: requirements
  path: changes/change-20260711-frame-exec-launcher/requirements.md
  required: true
- role: verification
  path: changes/change-20260711-frame-exec-launcher/verification.md
  required: true
promotion: []
unresolved_decisions: []
relations:
- {type: references, target: adr-20260624-0001-multiplexed-backends-shared-routing-contract}
- {type: references, target: adr-20260624-0081-codex-frame-init-serialize}
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
- {type: references, target: adr-20260711-0083-launchplan-argv-primary}
- {type: references, target: adr-20260711-0084-frame-spec-transport}
---

## Legacy Import

Imported losslessly from v1 artifacts.
