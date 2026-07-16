---
id: change-20260626-web-terminal-mobile-ux
kind: change
title: 20260626 Web Terminal Mobile Ux
status: draft
created: '2026-06-26'
summary: Imported legacy SDD package from docs/specs/web-terminal-mobile-ux.
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
  path: changes/change-20260626-web-terminal-mobile-ux/implementation.md
  required: true
- role: requirements
  path: changes/change-20260626-web-terminal-mobile-ux/requirements.md
  required: true
- role: ux
  path: changes/change-20260626-web-terminal-mobile-ux/ux.md
  required: false
- role: verification
  path: changes/change-20260626-web-terminal-mobile-ux/verification.md
  required: true
promotion: []
unresolved_decisions: []
relations:
- {type: references, target: adr-20260624-0029-terminal-host-flex-height}
- {type: references, target: adr-20260624-0030-terminal-keyed-remount}
- {type: references, target: adr-20260624-0034-refit-raf-coalesce-and-test-infra}
- {type: references, target: adr-20260624-0057-palette-single-aria-live-slot}
- {type: references, target: adr-20260624-0059-design-token-and-theme-bridge}
- {type: references, target: adr-20260624-0063-toast-single-live-and-undosnackbar}
- {type: references, target: adr-20260624-0064-reduced-motion-single-guard}
- {type: references, target: adr-20260624-0065-terminal-slot-absolute-overlay}
- {type: references, target: adr-20260624-0066-terminal-scrollback-via-vt-buffer}
- {type: references, target: adr-20260624-0067-mobile-gate-matchmedia}
- {type: references, target: adr-20260624-0068-mode-separation-focus-block-and-zoom-guard}
- {type: references, target: adr-20260624-0069-fab-overlay-layout-and-visualviewport-lift}
- {type: references, target: adr-20260624-0070-fontsize-persist-clamp}
- {type: references, target: adr-20260624-0071-touch-gesture-arbitration-and-long-press-selection}
- {type: references, target: adr-20260624-0072-coachmark-dismiss-and-once}
- {type: references, target: adr-20260624-0073-arialive-debounce-and-jump-fab-seed-stability}
- {type: references, target: adr-20260624-0074-migration-pc-only-to-pc-plus-mobile}
- {type: references, target: adr-20260624-0075-pattern-adoption-mode-affordances}
- {type: references, target: adr-20260624-0077-mobile-touch-gesture-swipe-to-arrow}
---

## Legacy Import

Imported losslessly from v1 artifacts.
