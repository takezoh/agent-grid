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
