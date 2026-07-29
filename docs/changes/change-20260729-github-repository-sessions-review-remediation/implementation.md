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
