---
id: change-20260715-credproxy-materialization-contract
kind: change
title: 20260715 Credproxy Materialization Contract
status: draft
created: '2026-07-15'
summary: Imported legacy SDD package from docs/specs/credproxy-materialization-contract.
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
  path: changes/change-20260715-credproxy-materialization-contract/implementation.md
  required: true
- role: requirements
  path: changes/change-20260715-credproxy-materialization-contract/requirements.md
  required: true
- role: verification
  path: changes/change-20260715-credproxy-materialization-contract/verification.md
  required: true
promotion: []
unresolved_decisions: []
relations:
- {type: references, target: adr-20260715-credproxy-fork-remote-replace}
- {type: references, target: adr-20260715-credproxy-materialize-method}
- {type: references, target: adr-20260715-credproxy-metadata-handler-async-materialize}
- {type: references, target: adr-20260715-credproxy-recovery-lever-accepts-degraded-window}
- {type: references, target: adr-20260715-credproxy-retry-owner-caller-side}
- {type: references, target: adr-20260715-credproxy-runner-readonly-aggregation}
---

## Legacy Import

Imported losslessly from v1 artifacts.
