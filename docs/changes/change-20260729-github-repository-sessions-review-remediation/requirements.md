---
change: change-20260729-github-repository-sessions-review-remediation
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Requirements

- The actual authenticated repository lease HTTP routes must reject protocol-version
  mismatch before issuing or revoking authority.
- A transient GitHub projection failure must retain the last complete catalog, valid
  selection, and any in-flight preparation view.
- Quarantine mutations must reject stale observed revisions.
- Never-available clone bytes must be accounted, capped, deleted on terminal abandon,
  and swept after the declared cleanup age.
- Ready repository environments must be discoverable from the durable store after restart.
- Every shipped SDK must preserve the `SessionTarget` source discriminator contract.
- Repository selection, preparation-stage availability gating, and invalidation focus
  recovery must have user-reachable DOM or Playwright evidence.
- No fix may reduce assertions, introduce direct skips, expose credentials, or weaken
  thresholds.
