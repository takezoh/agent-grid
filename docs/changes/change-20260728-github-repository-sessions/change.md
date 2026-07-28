---
id: change-20260728-github-repository-sessions
kind: change
title: GitHub repository-backed sessions
status: ready
created: '2026-07-28'
profile: sdd@1
intent: Add authenticated GitHub repositories to the existing fleet-wide New Session
  discovery while preserving project availability, credential confinement, server-owned
  lifecycle authority, and observable staged recovery.
outcomes:
- Users can distinguish and select categorized projects or GitHub repositories in
  browser and Electron without losing fleet-wide project behavior.
- Repository sessions use an isolated clone and a repository-scoped per-project devcontainer
  and appear available only after durable preparation succeeds.
- Authentication, source/member failure, cancellation, retry, restart, and invalidation
  preserve healthy context and expose typed accessible recovery.
- Credential transfer, protocol migration, and every external dependency are bounded
  and independently verifiable.
scope:
- Client-host GitHub OAuth device-flow account and retained catalog
- Shared categorized target picker and Settings recovery
- Authenticated one-use remote clone authority
- Durable server repository preparation, independent clones, repository environment
  reuse
- Accessible progress and exact-version all-consumer migration
non_goals:
- GitHub Enterprise or multiple accounts
- Unvalidated extended search metadata
- Branch/fork chooser
- Clone sharing or per-session/shared devcontainers
- Mixed-version compatibility
- Completed workspace retention policy
- New third-party dependencies
change_classes:
- behavior
- responsibility
- boundary
- invariant
- internal_design
- capability
governance:
  gate: hard
  reasons:
  - Four architecture decisions required explicit user consultation; all were approved
    on 2026-07-28 and recorded in consultations/adr-approval.md before implementation.
members:
- role: requirements
  path: changes/change-20260728-github-repository-sessions/requirements.md
  required: true
- role: implementation
  path: changes/change-20260728-github-repository-sessions/implementation.md
  required: true
- role: verification
  path: changes/change-20260728-github-repository-sessions/verification.md
  required: true
- role: ux
  path: changes/change-20260728-github-repository-sessions/ux.md
  required: false
- role: consultation
  path: changes/change-20260728-github-repository-sessions/consultations/adr-approval.md
  required: false
promotion:
- target: design-clients-ui
  section: invariants
  action: upsert
  item:
    id: INV-901
    statement: New Session target rows expose source and fully qualified identity
      in every modality; independently failing sources never globally block healthy
      targets.
    enforcement: contract
  reason: The categorized repository target capability changes the durable shared-SPA
    observation boundary.
- target: design-client
  section: responsibilities
  action: upsert
  item:
    id: RESP-901
    statement: Own durable repository preparation separately from Session and publish
      a Session only after clone, repository environment, and launch success.
  reason: Repository preparation adds a durable server-domain lifecycle responsibility.
- target: design-client
  section: invariants
  action: upsert
  item:
    id: INV-901
    statement: Incomplete, failed, cancelled, interrupted, or corrupt repository preparation
      never appears in the available Sessions collection.
    enforcement: contract
  reason: Availability ownership is a persistent invariant for future changes.
- target: design-platform
  section: trust_boundaries
  action: upsert
  item:
    id: TRUST-901
    statement: Remote GitHub clone authority is authenticated, repository-bound, one-use,
      memory-only, replay-resistant, and absent from durable or process-visible surfaces.
  reason: The remote clone path adds a durable credential mediation boundary.
- target: design-platform
  section: invariants
  action: upsert
  item:
    id: INV-901
    statement: Repository devcontainer identity is keyed by immutable repository scope
      while every session clone remains an independent frame workspace.
    enforcement: contract
  reason: Repository environment scope changes the canonical sandbox identity contract.
- target: design-platform
  section: compatibility_policies
  action: upsert
  item:
    id: COMPAT-901
    statement: Repository identity, preparation, lease, and progress protocol changes
      migrate daemon IPC, REST/WS, all shipped host parsers, SPA, SDKs, simulator,
      fixtures, and browser fakes under one exact token.
  reason: The public repository surface extends the accepted exact-version policy
    to a closed consumer inventory.
unresolved_decisions: []
tags:
- github
- repository-session
- client-host
- security
- devcontainer
owners: []
relations:
- {type: conformsTo, target: design-client}
- {type: conformsTo, target: design-clients-ui}
- {type: conformsTo, target: design-platform}
- {type: references, target: change-20260727-multi-server-session-palette}
- {type: modifies, target: design-client}
- {type: modifies, target: design-clients-ui}
- {type: modifies, target: design-platform}
source_paths:
- clients/ui/
- clients/web-host/
- clients/workspace/
- clients/windows-shell/
- services/server/api/
- services/host/
- services/platform/agentlaunch/
- services/platform/sandbox/
- services/platform/lib/gitclone/
- protocol/
- clients/sdk/
summary: Specify categorized GitHub repository sessions with host-owned authentication,
  durable server preparation, isolated clones, repository-scoped devcontainers, and
  exact-version fidelity gates.
updated: '2026-07-28'
---

## Summary

This package translates the specified GitHub repository-session UX into EARS requirements, closed implementation contracts, four accepted architecture decisions, dependency-ordered task units, and tiered verification. The design became implementation-ready when the user approved all four ADRs on 2026-07-28.

## Decision governance

- [20260728-client-host-github-device-flow](../../adr/adr-20260728-client-host-github-device-flow.md)
- [20260728-ephemeral-github-clone-authority](../../adr/adr-20260728-ephemeral-github-clone-authority.md)
- [20260728-durable-repository-preparation](../../adr/adr-20260728-durable-repository-preparation.md)
- [20260728-repository-devcontainer-scope](../../adr/adr-20260728-repository-devcontainer-scope.md)

## Closure Notes

Closure requires accepted/rejected ADR dispositions, applied promotion entries, terminal tasks, evidence for every success outcome, and clean documentation conformance.
