---
id: change-20260723-windows-shell-phase2
kind: change
title: 20260723 Windows Shell Phase 2 UX
status: draft
created: '2026-07-23'
summary: UX requirements for the Windows desktop supervision surface (Phase 2 of native-clients plan).
profile: sdd@1
intent: Fix the observable user experience for the Windows shell + workspace vertical slice S1-S5 before design and implementation.
outcomes:
- Local supervision flow (approval / question / jump-back) has no browser involvement
- Panel glance/engage focus discipline is specified before implementation
- Workspace window discipline (reuse, restore, deep-link) is observably testable
non_goals:
- macOS shell, mobile clients, or distribution (installer/signing/auto-update)
- Approval/question server-side domain design (Phase 0)
- Protocol schema definition (Phase 1)
scope:
- clients/windows-shell (native panel, toast, deep link, daemon supervisor UX)
- clients/workspace (Electron window host UX)
- clients/ui hosted mode UX
change_classes:
- behavior
- capability
governance:
  gate: soft
  reasons:
  - explore-mode-open-decisions
members:
- role: requirements
  path: changes/change-20260723-windows-shell-phase2/requirements.md
  required: true
- role: implementation
  path: changes/change-20260723-windows-shell-phase2/implementation.md
  required: true
- role: verification
  path: changes/change-20260723-windows-shell-phase2/verification.md
  required: true
- role: ux
  path: changes/change-20260723-windows-shell-phase2/ux.md
  required: true
promotion: []
unresolved_decisions:
- panel-primary-vs-toast-primary-supervision-entry
- panel-engage-focus-return-policy
- jump-back-target-inventory-provenance
- hosted-mode-visual-integration-scope
relations: []
---

## Scaffold

Explore-mode requirements package for Phase 2 of the native clients plan. The `ux.md` member is authored by the requirements integrator and is the SoT for Phase 2 UX. Other members are stubs; design / implementation / verification will fill them in later stages.
