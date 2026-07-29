---
change: change-20260729-github-repository-sessions-review-remediation
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Required evidence

- Registered HTTP route mismatch test proves no lease mutation.
- Reducer and runtime tests prove revision fencing, retained state, durable reuse,
  storage accounting, terminal cleanup, and age-based sweep.
- Real DOM or Playwright tests drive repository selection, preparation progression,
  availability gating, projection invalidation, and focus recovery.
- SDK contract tests reject invalid project/repository discriminator combinations.
- `make lint`, all Go tests, UI lint/unit/E2E/real-stack, Windows Shell core tests,
  `make test-e2e`, fake-green guard, and independent cross-task review pass.

## Delivery evidence

On 2026-07-29 the remediation branch passed Go lint, the complete Go suite, UI lint
and 1,786 unit tests, Playwright smoke, real-stack Playwright, Windows Shell core
tests, Go fidelity E2E, focused repository runtime/sandbox/API tests, the SDK
TypeScript harness, locally available Kotlin harness, fake-green guard, and the
zero-exit fake-tool regression. C#, Kotlin, Swift, and TypeScript are configured as
a fail-closed CI job with stable language-specific success sentinels. Independent
correctness and test-discipline re-reviews returned `approved` with no blocker or
major findings.

Two first-run failures were independently reproduced as unrelated flakes and passed
alone plus on full rerun: terminal relay timing and stream-server wired delivery.
No assertion, threshold, or test was weakened.
