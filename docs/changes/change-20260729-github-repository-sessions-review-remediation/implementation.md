---
change: change-20260729-github-repository-sessions-review-remediation
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Plan

1. Reproduce or refute each independent review finding on its production path.
2. Wire exact-version admission into registered lease routes and add route-level tests.
3. Preserve retained projection and in-flight UI state; add DOM-observable interaction
   and focus-recovery tests.
4. Fence quarantine by revision and complete durable environment lookup plus bounded
   clone accounting and cleanup.
5. Restore cross-SDK discriminator conformance and harden confirmed credential/concurrency
   edges.
6. Run focused tests, fake-green and diff gates, then the complete delivery gate set
   and an independent cross-task re-review.

## Delivered

- Registered lease routes now apply the production exact-version gate before mutation.
- Projection failures retain the last complete catalog, selection, preparation state,
  and deterministic focus; the real `reused` wire value drives reuse presentation.
- Preparation recovery now performs startup reconciliation, bounded sweeping,
  revision fencing, durable environment lookup, and immediate capacity release.
- Repository devcontainers mount the sessions root while retaining repository-generation
  container identity and per-session clone workspaces.
- TypeScript, C#, Kotlin, and Swift ship executable `SessionTarget` discriminator
  harnesses; CI requires all four and success sentinels reject zero-exit fake tools.
