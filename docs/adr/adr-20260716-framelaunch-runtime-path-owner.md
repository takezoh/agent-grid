---
id: adr-20260716-framelaunch-runtime-path-owner
kind: adr
title: framelaunch owns runtime-authoritative PATH invariant via appid.RuntimeAuthoritativePathList()
status: accepted
created: '2026-07-16'
decision_makers:
- platform-team
tags:
- framelaunch
- path
- preexec
- appid
- ssot
owners:
- platform-team
relations:
- {type: partOf, target: change-20260716-framelaunch-runtime-path-preservation}
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
source_paths:
- src/platform/framelaunch/frame_exec.go
- src/platform/appid/appid.go
summary: Run() は preExec 完了後に appid.RuntimeAuthoritativePathList() (= [HostExecShimsPath,
  SecretEnvShimsPath]) を無条件 prepend + dedup し、appid が shim 名 SSOT + 順序付き runtime list
  を提供する (case D)
---

## Context

adr-20260711-0082 established `platform/framelaunch` as the sole owner of container-side frame sequencing including PreExec evaluation. The RCA found that PreExec (mise activate, dotfiles) silently reorders the process PATH after its own shell rc runs, pushing hostexec / secretenv shim directories to the tail — so `gh` resolves to `/usr/bin/gh` instead of the shim. Nothing between PreExec's shell and MainCommand re-asserts the intended shim-first ordering.

Two questions must be resolved together:

1. **Which layer owns re-asserting the shim-first PATH after PreExec?**
2. **What is the SSOT for the list of shim directories to prepend?**

## Decision

**`platform/framelaunch.Run()` is the sole owner of the runtime-authoritative PATH prefix, and the SSOT for the ordered list is a new function `appid.RuntimeAuthoritativePathList() []string` returning `[appid.HostExecShimsPath, appid.SecretEnvShimsPath]`.**

Concretely:

- Immediately at Run() entry (before any subprocess), capture `origPath = os.Getenv("PATH")`.
- Fetch `runtimeList := appid.RuntimeAuthoritativePathList()` once per invocation (via a `runtimePathListForTest` package-var seam for hermetic tests).
- After the unchanged PreExec env-merge loop, invoke `computeFinalPath(runtimeList, capturedEnv["PATH"], origPath)` and call `os.Setenv("PATH", ...)` with its result, unless the `AG_FRAMELAUNCH_DISABLE_PATH_REASSERT` toggle is truthy.
- Emit exactly one `framelaunch.path_reassert` slog record per invocation (contract-run-path-observability).
- Non-existent shim dirs on host-mode (where `/opt/agent-grid/run/hostexec-shims` does not exist on the host FS) are naturally skipped by POSIX exec's PATH lookup — no host-mode-specific code path is needed.

Case D pivots away from the case-C-1 approach of extracting `appid.ContainerRunDir`-rooted subdirectories from the pre-PreExec `origPath`. That approach was a 3-stage relay (provider `Env["PATH"]` → devcontainer merge → `origPath` → framelaunch extract predicate) that carried the same information redundantly through several boundaries, required a conservative extract predicate to defend against textual-prefix lookalikes and the bare mount root, required a devcontainer provider-order-stability pin test, and required a producer-vs-extractor SSOT panic. Case D collapses all of that into a single named-list SSOT.

## Consequences

{% consequence kind="positive" %}Sequencing (preExec evaluation, env merge, PATH re-assertion, pre-commands, main exec) stays inside the single owner adr-20260711-0082 already established, avoiding the daemon/bridge responsibility split.{% /consequence %}

{% consequence kind="positive" %}The invariant can be verified end-to-end at the same tier (T0 pure helpers + T1 fake-login-shell wiring) as the existing framelaunch tests — no new external dependency triple needed.{% /consequence %}

{% consequence kind="positive" %}Because computeFinalPath is a pure function AND the runtime list is a static named SSOT, the ordering-invariant is testable without process env mutation AND without a separate provider-order-stability pin test (issue-testability-t0-envmutation-gap and issue-upstream-order-inherited-implicitly both resolved by construction).{% /consequence %}

{% consequence kind="positive" %}Case D collapses the case-C-1 3-stage relay (provider Env[PATH] → devcontainer merge → origPath → framelaunch extract) into one lookup, eliminating the vestigial extract predicate, its textual-prefix / bare-mount-root edge cases, and its provider-vs-extractor SSOT drift surface.{% /consequence %}

{% consequence kind="negative" %}framelaunch acquires a direct import dependency on appid (already OK — appid is a leaf package). Adding a broker whose shim dir is NOT under appid.ContainerRunDir requires extending appid.RuntimeAuthoritativePathList() rather than relying on prefix-matching over an origPath scan.{% /consequence %}

{% consequence kind="negative" %}The static list must be kept in sync with actual shim writing by hostexec/secretenv — pinned by contract-provider-appid-ssot (both providers reference the appid consts, so a rename ripples correctly). A third-party broker that writes to a new shim subdir must add its entry to `RuntimeAuthoritativePathList()` (a single-file edit) and add an ADR.{% /consequence %}

{% consequence kind="neutral" %}preExec failure handling (adr-20260711-0082) is unchanged.{% /consequence %}

{% consequence kind="neutral" %}Host-direct launches naturally see runtime shim dirs absent on disk; POSIX exec silently skips them — no separate code path.{% /consequence %}

## Alternatives

**(previous case C-1) Provider contributes `Env["PATH"]` to the container spec, devcontainer merges/dedups, framelaunch extracts `appid.ContainerRunDir`-rooted subdirs from the resulting origPath and prepends them.** 却下: vestigial redundancy. The same information (list of runtime-authoritative shim dirs) travels through provider → devcontainer merge → origPath → framelaunch extract predicate. Requires an extract predicate to defend against textual-prefix lookalikes and the bare mount root; requires a devcontainer-provider-order-stability pin test to catch upstream reordering; requires an SSOT panic to prevent producer-vs-extractor drift. Rejected on user feedback: replaced by a direct named-list SSOT (case D).

**(b) New `AG_RUNTIME_PATH` env var enumerated by the daemon and consumed by framelaunch.** 却下: grows the FrameSpec wire contract (adr-20260711-0084 scope). Same information as `appid.RuntimeAuthoritativePathList()` but with runtime-wire cost.

**(c) Hardcoded `{hostexec-shims, secretenv-shims}` constants inside framelaunch.** 却下: couples framelaunch to every broker's exact shim-dir name and violates the appid SSOT principle. Case D places the SSOT in appid instead, so framelaunch reads from the SSOT rather than duplicating.

**(d) Prepend the entire pre-PreExec origPath as a block onto PreExec's PATH, then dedup.** 却下: elevates ALL pre-PreExec entries to authoritative status — any /tmp or workspace dir the sandbox happened to inject also wins over PreExec's toolchain, broadening the trust-boundary shift adr-20260716-shim-priority-hardening-and-migration acknowledges. Selection condition: pick `appid.RuntimeAuthoritativePathList()` (case D) when 'runtime authoritative' means 'strictly shim-broker roots'; pick (d) when it means 'whatever the daemon curated'. Current requirements point at case D.

**Fix in `platform/sandbox/devcontainer` via a second `docker exec -e PATH=…` re-injection after PreExec.** 却下: there is no second `docker exec` in the current per-frame sequencing (PreExec runs inside bridge's own process). Reintroduces the daemon/bridge responsibility split adr-20260711-0082 eliminated.

{% transition from="draft" to="proposed" date="2026-07-16" %}
Integrated from design skill artifacts/plan.json — case-D redesign. decision-input-001, decision-input-006, decision-input-007 all adopted here.
{% /transition %}
