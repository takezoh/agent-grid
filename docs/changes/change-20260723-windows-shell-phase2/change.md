---
id: change-20260723-windows-shell-phase2
kind: change
title: 20260723 Windows Shell Phase 2 UX
status: active
created: '2026-07-23'
summary: UX requirements for the Windows desktop supervision surface (Phase 2 of native-clients
  plan).
profile: sdd@1
intent: Fix the observable user experience for the Windows shell + workspace vertical
  slice S1-S5 before design and implementation.
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
promotion:
- action: none
  reason: Phase 2 change stays scoped to native-client behavior/capability additions;
    the 14 ADRs published under docs/adr/adr-20260724-*.md carry the design contracts.
    No persistent design/*.md upsert or retire is required in this pass — Windows
    Shell / Workspace / hosted-mode SPA are not represented in docs/design as first-class
    responsibility/boundary/ownership records today. Promotion is revisited if boundaries
    stabilize post-S5.
unresolved_decisions: []
relations: []
updated: '2026-07-24'
---

## Source plans

- `plans/plan-20260723-windows-shell-design.md` — Phase 2 詳細設計
- `plans/plan-20260723-native-clients.md` — 親計画 (Phase 2 は S1-S5 vertical slice)

## Scheduled gates

Change status は `ready`。以下 4 gate は S1 entry を塞がない conditional regression detector で、実装フェーズで発火条件を監視する。

| id | chunk | covers | on_fail |
|---|---|---|---|
| `s3-prototypes-gate` | `chunk-s1a-s3-prototypes-gate` | `assumption-com-background-activation-unpackaged`, `assumption-appnotification-textbox-ime`, engage-restore AttachThreadInput screen-reader 互換性 | reopen `DP-SUPERVISION-PRIMARY-ENTRY` via new user consultation; blocks entry into `chunk-s2-panel-glance` and `chunk-s3-approval-round-trip` until resolved |
| `wsl-detach-survival-verification` | `chunk-s1-connection-supervision` (`unit-wsl-detach-spike`) | `adr-20260724-boundary-3-wsl-detach-spike` (setsid+nohup survival) | supersede the ADR with a systemd --user alternative; re-run S1 |
| `deep-link-upstream-additive-pr` | post-S5 follow-up (Track B of `adr-20260724-deep-link-schema-additive-extension`) | additive extension of `protocol/deep-links.schema.json` for `question` + `/jump` variants | amend the ADR to make Track A (client-side alias) the permanent implementation; no Phase 2 code change needed |
| `reconnect-delay-p95-monitor` | post-S1 measurement | `NFR-daemon-restart-reconnect-delay=5s` | revise NFR value based on histogram; regression alarm at p95 > 5s |

## Scaffold

Explore-mode requirements package for Phase 2 of the native clients plan. The `ux.md` member is authored by the requirements integrator and is the SoT for Phase 2 UX. Other members are stubs; design / implementation / verification will fill them in later stages.
