---
id: change-20260729-github-repository-sessions-review-remediation
kind: change
title: GitHub repository sessions review remediation
status: active
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
- services/server/api/ repository lease admission and route-level mismatch tests
- clients/ui/ retained projection, preparation/focus behavior, and DOM/Playwright
  tests
- services/host/state/ repository quarantine revision fencing
- services/host/runtime/ durable environment reuse, storage accounting, and cleanup
- services/platform/lib/gitclone/ credential result and broker concurrency hardening
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
- services/server/api/
- clients/ui/
- services/host/state/
- services/host/runtime/
- services/platform/lib/gitclone/
- clients/sdk/
summary: Validate and close independent correctness and user-reachable coverage findings
  before landing GitHub repository sessions.
updated: '2026-07-29'
---

## Summary

Independent correctness and test-discipline review found production wiring and
user-observable coverage gaps after the source change closed. Each finding must be
reproduced or disproved against the real path; confirmed findings are fixed without
repartitioning the cohesive implementation.

## Closure Notes


{% transition from="draft" to="ready" date="2026-07-29" %}
Independent review findings normalized into scoped requirements and verification
{% /transition %}


{% transition from="ready" to="active" date="2026-07-29" %}
Grok remediation implementation authorized on the delivery branch
{% /transition %}
