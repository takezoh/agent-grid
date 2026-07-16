---
id: adr-20260716-shim-priority-hardening-and-migration
kind: adr
title: Shim-priority hardening with migration rollback and docker-exec-bash note
status: accepted
created: '2026-07-16'
decision_makers:
- platform-team
tags: [framelaunch, migration, security, path]
owners:
- platform-team
relations:
- {type: partOf, target: plan-20260716-framelaunch-runtime-path-preservation}
- {type: references, target: adr-20260716-framelaunch-runtime-path-owner}
- {type: references, target: adr-20260716-provider-shim-root-appid-ssot}
source_paths:
- src/platform/framelaunch/frame_exec.go
- src/platform/hostexec/provider.go
- src/platform/secretenv/provider.go
- docs/component/component-20260624-platform-sandbox.md
summary: "runtime-authoritative shim dir を preExec 以降も先頭に固定する trust-boundary shift を承認し、rollback env toggle と shim enumeration と docker-exec-bash 観測差分 note を移行契約に含める (case D)"
---

## Context

The critic identified that the fix is not merely a bugfix but a shim-priority policy change (issue-migration-mise-first-user-toolchains, issue-security-shim-priority-attack): post-fix, any binary placed under `appid.ContainerRunDir/<any shim subdir>/` is unconditionally exec'd in preference to `/usr/bin`, and PreExec/user-side PATH shadowing (mise activate, dotfiles) can no longer shadow it. This shifts the trust boundary — writers of `ContainerRunDir` gain implicit exec priority — and can change the observed binary for any command name currently both hostexec-shimmed and mise-shimmed.

Case D additionally has a user-visible cosmetic consequence: because providers no longer inject `Env["PATH"]` into the container spec, an operator running `docker exec bash` interactively on a running container sees the image's default PATH, not a PATH containing the shim dirs. This does not affect production frames (they observe environment via framelaunch's prepared env, not via interactive `docker exec bash`), but must be documented so an operator diagnosing shim resolution knows to consult framelaunch's slog rather than `echo $PATH`.

`adr-20260711-0082` says PreExec propagates env-only, not that shim priority is unchanged.

## Decision

Adopt the shim-priority hardening as an intentional policy decision, subject to four explicit safeguards:

- **(a) Enumeration.** Every command name shimmed at fix-adoption time (`hostexec.ShimDirName` contents (= `appid.HostExecShimsDir`) + secretenv `credproxy` + any overlay registered via `hostexec/provider.go:173`) is listed in `docs/component/component-20260624-platform-sandbox.md` appendix. Confirm no overlap with the standard mise/asdf/dotfile shim targets `{git, python, node, go, ruby, java}` at fix-adoption time.
- **(b) Rollback toggle.** `AG_FRAMELAUNCH_DISABLE_PATH_REASSERT=1` (or `true`, `yes`, case-insensitive) skips the merge wiring and restores pre-fix behavior, tested by a dedicated T0 branch and honored by contract-run-path-observability's `skip_branch="toggle_disabled"`.
- **(c) Drift check.** The enumeration appendix must be updated whenever a new shim is added (documentation drift check, folded into contract-shim-priority-migration.migration_strategy).
- **(d) `docker exec bash` observable-PATH migration note.** Documented in the same sandbox component doc: providers no longer inject `Env["PATH"]` into `ContainerEnv`, so an interactive `docker exec bash` sees the image PATH (no shim dirs). This is cosmetic — actual command lookups still hit the shims via framelaunch's per-frame prepend, and shell rc / login shells rebuild PATH themselves.

## Consequences

{% consequence kind="positive" %}Any consumer whose frame relies today on mise-shim resolution of a name also wrapped by hostexec can be identified from the enumeration before deploy.{% /consequence %}

{% consequence kind="positive" %}Operators have a documented rollback path (`AG_FRAMELAUNCH_DISABLE_PATH_REASSERT=1`) if a regression surfaces post-deploy, with the slog record visibly showing rollback state.{% /consequence %}

{% consequence kind="positive" %}The trust-boundary shift is a named, evidenced decision — an auditor can see who can write `ContainerRunDir` and what monitoring exists (`framelaunch.path_reassert` slog).{% /consequence %}

{% consequence kind="negative" %}The rollback toggle is a permanent env-var-shaped kludge until this ADR is superseded; there is no automatic removal timeline.{% /consequence %}

{% consequence kind="negative" %}Overlay registration via `hostexec/provider.go:173` continues to be an implicit trust delegation to whoever registers the overlay; the ADR names but does not tighten this today (deferred).{% /consequence %}

{% consequence kind="negative" %}Case-D cosmetic change: `docker exec bash` interactive PATH no longer contains the shim dirs (a change from pre-case-D). Since production frames never observe their environment via `docker exec bash` (they observe via framelaunch's prepared env), this is cosmetic — but must be documented so an operator investigating shim resolution knows to check framelaunch's slog record rather than the interactive `$PATH`.{% /consequence %}

{% consequence kind="neutral" %}The enumeration appendix in the sandbox component doc becomes a maintenance surface (drift check via contract-shim-priority-migration.migration_strategy).{% /consequence %}

## Alternatives

**Ship the fix without a rollback toggle or shim enumeration.** 却下: reintroduces the silent-behavior-change class the fix itself was motivated to eliminate; operators would have no rollback path if a real frame breaks.

**Restrict the fix to only commands not already in a known-mise-shim set (whitelist).** 却下: requires framelaunch to know about mise's shim targets, coupling it to an external tool and re-introducing hardcoded name lists (same objection as the (c) alternative in adr-20260716-framelaunch-runtime-path-owner).

{% transition from="draft" to="proposed" date="2026-07-16" %}
Integrated from design skill artifacts/plan.json — case-D redesign resolves issue-migration-mise-first-user-toolchains + issue-security-shim-priority-attack and adds the docker-exec-bash cosmetic migration note.
{% /transition %}
