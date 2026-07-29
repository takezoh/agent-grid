---
id: change-20260729-github-repository-sessions-review-remediation
kind: change
title: GitHub repository sessions review remediation
status: done
created: '2026-07-29'
profile: sdd@1
intent: Close independently reviewed correctness and user-reachable coverage gaps
  in GitHub repository sessions before landing the delivery branch.
outcomes:
- Production repository lease routes reject exact-version mismatches before mutation.
- Retained catalogs, in-flight preparation, and focus recovery survive scoped transient
  failures and invalidation.
- Repository preparation enforces revision fencing, durable environment reuse, bounded
  never-available storage, and cleanup.
- Shipped SDKs and UI tests preserve the repository discriminator and observable UAC
  behavior.
scope:
- .github/workflows/ cross-SDK executable conformance gate
- services/server/api/ repository lease admission and route-level mismatch tests
- clients/ui/ retained projection, preparation/focus behavior, and DOM/Playwright
  tests
- services/host/state/ repository quarantine revision fencing
- services/host/runtime/ durable environment reuse, storage accounting, and cleanup
- services/cmd/server/ production repository reconciliation, sweep, and environment
  composition wiring
- services/platform/sandbox/ repository environment mount and container identity
- services/platform/lib/gitclone/ credential result and broker concurrency hardening
- services/gorules/ executable SDK harness and fake-green enforcement
- clients/sdk/ repository discriminator conformance
- docs/changes/change-20260729-github-repository-sessions-review-remediation/ review
  evidence and closure
non_goals:
- Re-splitting the completed repository-session implementation
- Weakening assertions, thresholds, or external fidelity backstops
- Unrelated repository cleanup or refactoring
change_classes:
- behavior
- invariant
- boundary
governance:
  gate: auto
  reasons: []
members:
- role: requirements
  path: changes/change-20260729-github-repository-sessions-review-remediation/requirements.md
  required: true
- role: implementation
  path: changes/change-20260729-github-repository-sessions-review-remediation/implementation.md
  required: true
- role: verification
  path: changes/change-20260729-github-repository-sessions-review-remediation/verification.md
  required: true
promotion:
- target: none
  action: none
  reason: The governing invariants were already promoted by the source change; this
    remediation makes implementation and tests conform to them.
unresolved_decisions: []
evidence_refs:
- type: command
  ref: make lint
- type: command
  ref: cd services && go test ./...
- type: command
  ref: cd clients/ui && npm run lint && npm run test:unit
- type: command
  ref: cd clients/ui && PW_USE_SYSTEM_CHROME=1 npm run test:e2e
- type: command
  ref: cd clients/ui && PW_USE_SYSTEM_CHROME=1 npm run test:e2e:real
- type: command
  ref: make test-e2e
- type: command
  ref: clients/windows-shell AgentGrid.Shell.Core.Tests
- type: command
  ref: clients/sdk/scripts/run-session-target-conformance.sh
- type: contract
  ref: services/gorules/repository_flagday_test.go
- type: test
  ref: services/server/api/repository_route_mismatch_test.go
- type: test
  ref: services/host/runtime/repository_preparation_test.go
- type: test
  ref: services/host/runtime/repository_devcontainer_e2e_test.go
- type: test
  ref: clients/ui/src/components/palette/TargetPicker.test.tsx
- type: test
  ref: clients/ui/e2e/repository-fleet-ux.spec.ts
- type: contract
  ref: independent cross-task correctness and test-discipline approved
tags:
- github
- repository-session
- review-remediation
owners: []
relations:
- {type: references, target: change-20260728-github-repository-sessions}
- {type: conformsTo, target: design-client}
- {type: conformsTo, target: design-clients-ui}
- {type: conformsTo, target: design-platform}
source_paths:
- .github/workflows/
- services/server/api/
- clients/ui/
- services/host/state/
- services/host/runtime/
- services/cmd/server/
- services/platform/sandbox/
- services/platform/lib/gitclone/
- services/gorules/
- clients/sdk/
summary: Validate and close independent correctness and user-reachable coverage findings
  before landing GitHub repository sessions.
updated: '2026-07-29'
promotion_applied_at: '2026-07-29T05:43:18.721863+00:00'
closure:
  closed_at: '2026-07-29T05:46:27.802642+00:00'
  content_hash: sha256:a11aa2e2ed86fa23a20cd5340457a9c7092a09fa868171c207baa5419f8e667a
---

## Summary

Independent correctness and test-discipline review found production wiring and
user-observable coverage gaps after the source change closed. Each finding must be
reproduced or disproved against the real path; confirmed findings are fixed without
repartitioning the cohesive implementation.

## Closure Notes

All confirmed blocker and major findings were fixed and independently re-reviewed.
The remaining minor observations are external branch-protection configuration and a
tested frame-CWD translation helper that is not called directly by the runtime; neither
changes the verified production mount and launch behavior.

{% transition from="draft" to="ready" date="2026-07-29" %}
Independent review findings normalized into scoped requirements and verification
{% /transition %}


{% transition from="ready" to="active" date="2026-07-29" %}
Grok remediation implementation authorized on the delivery branch
{% /transition %}


{% transition from="active" to="closing" date="2026-07-29" %}
All confirmed review findings fixed; mechanical gates and independent cross-task reviews approved
{% /transition %}
