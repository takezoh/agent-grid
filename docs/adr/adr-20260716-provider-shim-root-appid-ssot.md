---
id: adr-20260716-provider-shim-root-appid-ssot
kind: adr
title: Provider shim root and PATH contribution both anchored to appid SSOT
status: accepted
created: '2026-07-16'
decision_makers:
- platform-team
tags:
- hostexec
- secretenv
- appid
- ssot
- security
owners:
- platform-team
relations:
- {type: partOf, target: change-20260716-framelaunch-runtime-path-preservation}
- {type: references, target: adr-20260716-framelaunch-runtime-path-owner}
source_paths:
- src/platform/hostexec/provider.go
- src/platform/hostexec/shim.go
- src/platform/secretenv/provider.go
- src/platform/appid/appid.go
summary: hostexec/secretenv SpecBuilder は cfg.ContainerRunDir が appid.ContainerRunDir
  と一致することを起動時に fail-fast で検証し、Env[PATH] を返さず、shim 名は appid の const を SSOT として参照する
  (case D)
---

## Context

The critic identified a blocker (issue-ssot-cfg-vs-constant): `hostexec/provider.go:115` and `secretenv/provider.go:106` build shim dirs from `b.cfg.ContainerRunDir + "/" + <name>` where the name lived in a package-local const. Any producer constructed with `cfg.ContainerRunDir != appid.ContainerRunDir` would silently drift.

Case D additionally requires that **providers stop contributing `Env["PATH"]` to the container spec** — framelaunch is the sole PATH owner (adr-20260716-framelaunch-runtime-path-owner), and provider `Env["PATH"]` would be a vestigial second path source that recreates the case-C-1 3-stage relay.

## Decision

Three invariants enforced together at the provider boundary:

- **(a) Construction-time SSOT panic (retained).** Both `hostexec.NewSpecBuilder` and `secretenv.NewSpecBuilder` fail-fast panic at construction time if `cfg.ContainerRunDir != appid.ContainerRunDir`. The panic message names the offending value, `appid.ContainerRunDir`, and this ADR id.
- **(b) Providers MUST NOT return `Env["PATH"]`.** Case-D invariant: `container.Spec.Env` from either provider has no `"PATH"` key. Only `Mounts` contribute. Pinned by `TestProvider_DoesNotContributePATH` per package.
- **(c) Shim subdirectory names are owned by appid.** `hostexec.ShimDirName` is re-declared as `= appid.HostExecShimsDir` (alias re-export for external callers), and secretenv's package-local `shimDirName` is replaced by direct reference to `appid.SecretEnvShimsDir`. The framelaunch-side runtime list `appid.RuntimeAuthoritativePathList()` is built from the same appid consts.

Together these three invariants ensure provider, framelaunch, and shim writing cannot drift on either the shim root or the shim subdirectory name, and that the vestigial `Env["PATH"]` contribution cannot be silently reintroduced.

`cfg.ContainerRunDir` is retained as a struct field (some tests explicitly set it, and removing it would expand blast radius) but its permitted value set is the singleton `{appid.ContainerRunDir}`.

## Consequences

{% consequence kind="positive" %}Producer and framelaunch cannot drift on shim root (panic guard) or on shim subdirectory names (appid consts shared).{% /consequence %}

{% consequence kind="positive" %}A regression that re-adds provider `Env["PATH"]` fails `TestProvider_DoesNotContributePATH` loudly, preventing the case-C-1 3-stage relay from silently returning.{% /consequence %}

{% consequence kind="positive" %}The invariants are enforceable by T0 tests in the same units as the code changes.{% /consequence %}

{% consequence kind="negative" %}Any test harness that previously exercised `cfg.ContainerRunDir != appid.ContainerRunDir` must migrate to `appid.ContainerRunDir` (assessment: no such harness exists in the current repo).{% /consequence %}

{% consequence kind="negative" %}Any test that inspected `container.Spec.Env["PATH"]` for the provider's contribution must be updated to inspect the new absence (in-place edits in `provider_test.go`).{% /consequence %}

{% consequence kind="negative" %}If a future feature legitimately requires per-instance `ContainerRunDir` override or a provider that DOES need to inject `Env["PATH"]`, this ADR must be superseded (not silently relaxed).{% /consequence %}

{% consequence kind="neutral" %}The Mount emission behavior of both providers is unchanged; only the `Env` field is removed and the const references are rewired.{% /consequence %}

## Alternatives

**Refactor to drop `Config.ContainerRunDir` as a field entirely and reference `appid.ContainerRunDir` directly at the shim-writing site.** 却下: bigger blast radius (some tests set the field explicitly); the panic + appid-const approach achieves the same invariant with a smaller change surface. Can be revisited when the field is provably unused after a full grep.

**Keep provider `Env["PATH"]` contribution and rely on framelaunch's prepend to be idempotent via dedup.** 却下: this is exactly the case-C-1 vestigial redundancy. Two writers of the same information (provider `Env`, framelaunch prepend) is a drift surface; deduplication masks the drift instead of preventing it. Case D eliminates it structurally.

**Keep the drift risk and accept it in framelaunch documentation only.** 却下: reproduces the exact silent-failure class the RCA identified.

{% transition from="draft" to="proposed" date="2026-07-16" %}
Integrated from design skill artifacts/plan.json — case-D redesign resolves critique blocker issue-ssot-cfg-vs-constant and adds the case-D no-Env[PATH] invariant.
{% /transition %}
