---
id: spec-20260716-framelaunch-runtime-path-preservation
kind: spec
title: framelaunch runtime PATH preservation
status: approved
created: '2026-07-16'
methodology: sdd
tags: [framelaunch, path, preexec, hostexec, secretenv]
owners: [platform-team]
functional_requirements:
- id: FR-001
  statement: "The system shall ensure that the PATH inherited by PreCommands and MainCommand begins with a concatenation of appid.RuntimeAuthoritativePathList() entries in their declared order, and that no non-runtime-authoritative segment precedes any runtime-authoritative segment."
  priority: must
  rationale: "True ubiquitous invariant per issue-fr-d1-invariant-not-invariant fix, rewritten for case D: the invariant now points at a named static list (appid.RuntimeAuthoritativePathList()) rather than a computed extraction from origPath. Holds trivially for the empty-PreExec branch (PATH unchanged from origPath, which for a sandboxed launch was already correct — case D no longer relies on this because providers do not inject PATH anymore, but the invariant remains falsifiable) and non-trivially for the PreExec branch (unconditional prepend enforces it)."
- id: FR-002
  statement: "When PreExec evaluation returns a captured environment, the system shall prepend the entries of appid.RuntimeAuthoritativePathList() to the PATH produced by PreExec, deduplicated and order-preserving, before running PreCommands."
  priority: must
  rationale: "Event-driven statement of how FR-001 is maintained during the PreExec branch. Rewritten for case D: no extraction, direct prepend of the static list."
- id: FR-003
  statement: "When the runtime-authoritative prefix entries already appear elsewhere in the post-PreExec PATH, the system shall deduplicate the resulting PATH string by exact byte comparison of each ':'-separated segment, keeping only the first occurrence and preserving relative order."
  priority: must
  rationale: "Dedup contract preventing PATH growth on each frame launch."
- id: FR-004
  statement: "The system shall not produce a PATH that is empty or begins with an empty segment as a result of the merge wiring."
  priority: must
  rationale: "Preserved from case C-1 for defense in depth: since appid.RuntimeAuthoritativePathList() is a non-empty static list, the natural output is always non-empty; dedupPath drops any empty segment introduced by empty base. FR-004 is now a trivially-satisfied invariant, but a T0 test still pins it to catch a future regression that violates the invariant (e.g. someone deleting entries from RuntimeAuthoritativePathList())."
- id: FR-006
  statement: "If two PATH segments differ only by a trailing '/' (e.g. /opt/agent-grid/run/hostexec-shims vs /opt/agent-grid/run/hostexec-shims/), then the system shall treat them as distinct segments and shall not deduplicate one against the other."
  priority: must
  rationale: "Byte-exact matching bash's PATH resolver and the sibling deduplicateColonList in platform/sandbox/devcontainer/spec.go."
- id: FR-007
  statement: "When Run() is invoked with an empty PreExec (spec.PreExec == \"\"), the system shall not modify the process PATH at all."
  priority: must
  rationale: "No-op no-preExec launches, matching current behavior exactly. Host-direct launches naturally lack the runtime shim dirs on disk, so any unconditional prepend done by framelaunch produces PATH entries that POSIX exec silently skips — no host-mode-specific code path is needed."
- id: FR-008
  statement: "The system shall ensure that for every command name C such that a shim file exists in an appid.RuntimeAuthoritativePathList() directory on disk, PATH-based resolution of C in the environment Run() hands to execReplacer resolves to that shim path rather than to any /usr/bin/C or PreExec-inserted alternative."
  priority: must
  rationale: "Preserves the RCA user observation ('container 内 gh が host と異なる output を返す') as a testable postcondition rather than an internal string-ordering shadow. Case D wording pivots from 'shim dir found via extraction' to 'shim dir present in the named RuntimeAuthoritativePathList'."
- id: FR-009
  statement: "The system shall ensure that the container.Spec returned by hostexec.SpecBuilder and secretenv.SpecBuilder contains no Env entry for the key \"PATH\", and that the provider's sole PATH-adjacent contribution is the shim-directory Mounts."
  priority: must
  rationale: "Case-D structural invariant: framelaunch is the sole PATH owner. A regression that re-adds provider Env[\"PATH\"] would restore the vestigial 3-stage relay (provider→origPath→framelaunch) and re-introduce the SSOT drift class. Pinned by a per-package regression guard test."
non_functional_requirements:
- id: NFR-001
  type: maintainability
  criteria: "The ordering-invariant computation (dedupPath, computeFinalPath) is directly T0-testable in isolation from Run()'s wiring, and Run()'s wiring adds one T1 integration test that snapshots/restores process PATH explicitly at test entry and overrides the runtime-list source via the runtimePathListForTest seam."
  measurement: "go test ./platform/framelaunch/... completes without setting or leaking process env other than via t.Setenv, and helper tests do not import syscall/exec."
- id: NFR-002
  type: performance
  criteria: "PATH prepend and deduplication add negligible latency to frame launch: O(n) string split/compare/join over a PATH of realistically fewer than 50 entries, executed once per Run() invocation. Case D removes the origPath scan of case C-1, saving ~1 µs per invocation."
  measurement: "Reasoned analysis: computeFinalPath is one strings.Join over 2 static entries + one dedupPath scan of the resulting string. Per-invocation cost stays under 100 µs on the reference CI machine for a 50-entry captured PATH."
- id: NFR-003
  type: compatibility
  criteria: "The fix does not alter Run()'s behavior when PreExec is empty, does not alter preExec failure handling, and preserves every pre-existing frame_exec_test.go test outcome unchanged; when AG_FRAMELAUNCH_DISABLE_PATH_REASSERT=1 is set, behavior is byte-identical to pre-fix behavior. Provider Env[\"PATH\"] removal does not change container Mounts or shim writing, so any existing test that inspects Mounts still passes."
  measurement: "go test ./platform/framelaunch/... ./platform/hostexec/... ./platform/secretenv/... passes with the toggle both on and off; every pre-existing test in frame_exec_test.go still passes without modification (provider tests are updated in-place to reflect the case-D Env-absence expectation)."
- id: NFR-004
  type: usability
  criteria: "Every Run() invocation that runs the PreExec-branch merge wiring (including toggle-disabled skip) emits exactly one framelaunch.path_reassert slog record with orig_path_len, runtime_prefix_count, dedup_dropped_count, post_reassert_changed_head, spec_frame_id, skip_branch fields, visible via the platform slog handler and captured by a slog test handler seam."
  measurement: "T0 test asserts exactly one record per invocation with all fields present across the normal-merge branch and the toggle-disabled branch."
acceptance:
- id: AC-001
  given: "A frame launch where PreExec's captured environment reconstructs PATH pushing the shim dirs to the tail (or omits them)"
  when: "Run() completes the PreExec branch and hands control to execReplacer"
  then: "The PATH in the argv env passed to execReplacer begins with the entries of appid.RuntimeAuthoritativePathList() in their declared order, followed by any additional entries from PreExec's PATH deduplicated and order-preserving"
  requirement_refs: [FR-001, FR-002, FR-003]
- id: AC-002
  given: "A frame launch where a shim file for the command name `gh` exists in the first appid.RuntimeAuthoritativePathList() directory (hostexec-shims)"
  when: "Run()'s post-merge environment is queried via PATH-based resolution for `gh`"
  then: "Resolution returns the shim path under appid.HostExecShimsPath, not /usr/bin/gh"
  requirement_refs: [FR-008]
- id: AC-003
  given: "A frame launch where the pre-PreExec PATH is empty (origPath == \"\") and PreExec's captured environment also has no PATH key"
  when: "Run() completes the PreExec branch"
  then: "PATH is set to strings.Join(appid.RuntimeAuthoritativePathList(), \":\"), which is non-empty and does not begin with an empty segment; exec.LookPath(spec.MainCommand[0]) does not fail because of the merge wiring"
  requirement_refs: [FR-004]
- id: AC-005
  given: "A PATH string containing both `/opt/agent-grid/run/hostexec-shims` and `/opt/agent-grid/run/hostexec-shims/` (trailing slash)"
  when: "dedupPath processes the string"
  then: "Both segments are retained as distinct entries in their original relative order"
  requirement_refs: [FR-006]
- id: AC-006
  given: "A Run() invocation with an empty PreExec (spec.PreExec == \"\")"
  when: "Run() completes and hands control to execReplacer"
  then: "The PATH in the argv env passed to execReplacer is byte-identical to os.Getenv(\"PATH\") observed at Run() entry"
  requirement_refs: [FR-007]
- id: AC-007
  given: "A Run() invocation that runs the PreExec-branch merge wiring"
  when: "The Run() call completes"
  then: "Exactly one `framelaunch.path_reassert` slog record has been emitted with fields orig_path_len, runtime_prefix_count, dedup_dropped_count, post_reassert_changed_head, spec_frame_id, and skip_branch"
  requirement_refs: [NFR-004]
- id: AC-008
  given: "AG_FRAMELAUNCH_DISABLE_PATH_REASSERT=1 is set in the process environment"
  when: "Run() completes the PreExec branch"
  then: "The merge wiring is skipped and PATH is left as whatever PreExec produced (pre-fix behavior), and the slog record fires with skip_branch=\"toggle_disabled\""
  requirement_refs: [NFR-003, NFR-004]
- id: AC-009
  given: "A hostexec.SpecBuilder or secretenv.SpecBuilder constructed with cfg.ContainerRunDir == appid.ContainerRunDir"
  when: "A container.Spec is built and inspected"
  then: "container.Spec.Env has no \"PATH\" key (nil or empty map); Mounts are unchanged from pre-case-D"
  requirement_refs: [FR-009]
relations:
- {type: implementedBy, target: plan-20260716-framelaunch-runtime-path-preservation}
source_paths:
- src/platform/framelaunch/frame_exec.go
- src/platform/hostexec/provider.go
- src/platform/secretenv/provider.go
- src/platform/appid/appid.go
summary: "framelaunch が preExec 後に appid.RuntimeAuthoritativePathList() を無条件 prepend して runtime 権威 shim dir 群を PATH 先頭に固定する契約 — provider は Env[PATH] を返さず、appid が shim 名 SSOT を持ち、observability slog と rollback toggle と provider-side SSOT panic で silent-failure と drift を防ぐ"
---

## Goal

Make platform/framelaunch.Run() the sole owner of the runtime-authoritative (appid.ContainerRunDir-rooted) PATH prefix by unconditionally prepending the static list `appid.RuntimeAuthoritativePathList()` to the PATH produced by PreExec, dedup'd and order-preserving. Provider (hostexec / secretenv) stops contributing `Env["PATH"]` to the container spec; `appid` becomes the single source of truth for shim subdirectory names. This removes the vestigial extract-from-origPath predicate, collapses the provider→origPath→framelaunch 3-stage relay into a single named-list lookup, and closes the silent-failure class the RCA identified.

## Functional Requirements

{% req id="FR-001" %}
The system shall ensure that the PATH inherited by PreCommands and MainCommand begins with a concatenation of `appid.RuntimeAuthoritativePathList()` entries in their declared order, and that no non-runtime-authoritative segment precedes any runtime-authoritative segment.
{% /req %}

{% req id="FR-002" %}
When PreExec evaluation returns a captured environment, the system shall prepend the entries of `appid.RuntimeAuthoritativePathList()` to the PATH produced by PreExec, deduplicated and order-preserving, before running PreCommands.
{% /req %}

{% req id="FR-003" %}
When the runtime-authoritative prefix entries already appear elsewhere in the post-PreExec PATH, the system shall deduplicate the resulting PATH string by exact byte comparison of each ':'-separated segment, keeping only the first occurrence and preserving relative order.
{% /req %}

{% req id="FR-004" %}
The system shall not produce a PATH that is empty or begins with an empty segment as a result of the merge wiring.
{% /req %}

{% req id="FR-006" %}
If two PATH segments differ only by a trailing '/' (e.g. /opt/agent-grid/run/hostexec-shims vs /opt/agent-grid/run/hostexec-shims/), then the system shall treat them as distinct segments and shall not deduplicate one against the other.
{% /req %}

{% req id="FR-007" %}
When Run() is invoked with an empty PreExec (spec.PreExec == ""), the system shall not modify the process PATH at all.
{% /req %}

{% req id="FR-008" %}
The system shall ensure that for every command name C such that a shim file exists in an `appid.RuntimeAuthoritativePathList()` directory on disk, PATH-based resolution of C in the environment Run() hands to execReplacer resolves to that shim path rather than to any /usr/bin/C or PreExec-inserted alternative.
{% /req %}

{% req id="FR-009" %}
The `container.Spec` returned by `hostexec.SpecBuilder` and `secretenv.SpecBuilder` shall NOT contain an `Env` entry for the key `"PATH"`. The provider's sole PATH-adjacent contribution is the shim-directory `Mounts`.
{% /req %}

## Non-Functional Requirements

- **NFR-001 (maintainability)**: `dedupPath` / `computeFinalPath` are T0-testable pure helpers; Run() wiring adds one T1 test with explicit process-env snapshot/restore + `runtimePathListForTest` seam.
- **NFR-002 (performance)**: O(n) join/dedup for a 50-entry PATH per Run() invocation; case D removes case C-1's origPath scan.
- **NFR-003 (compatibility)**: pre-fix behavior when PreExec is empty; existing test outcomes unchanged; `AG_FRAMELAUNCH_DISABLE_PATH_REASSERT=1` restores byte-identical pre-fix behavior; provider Mounts unchanged.
- **NFR-004 (usability)**: exactly one `framelaunch.path_reassert` slog record per PreExec-branch invocation with the six declared fields.

## Acceptance

{% acceptance id="AC-001" %}
Given a frame launch where PreExec's captured environment reconstructs PATH pushing the shim dirs to the tail (or omits them), when Run() completes the PreExec branch and hands control to execReplacer, then the PATH in the argv env passed to execReplacer begins with the entries of `appid.RuntimeAuthoritativePathList()` in their declared order, followed by any additional entries from PreExec's PATH deduplicated and order-preserving.
{% /acceptance %}

{% acceptance id="AC-002" %}
Given a frame launch where a shim file for the command name `gh` exists in the first `appid.RuntimeAuthoritativePathList()` directory (hostexec-shims), when Run()'s post-merge environment is queried via PATH-based resolution for `gh`, then resolution returns the shim path under `appid.HostExecShimsPath`, not `/usr/bin/gh`.
{% /acceptance %}

{% acceptance id="AC-003" %}
Given a frame launch where the pre-PreExec PATH is empty (origPath == "") and PreExec's captured environment also has no PATH key, when Run() completes the PreExec branch, then PATH is set to `strings.Join(appid.RuntimeAuthoritativePathList(), ":")`, which is non-empty and does not begin with an empty segment; `exec.LookPath(spec.MainCommand[0])` does not fail because of the merge wiring.
{% /acceptance %}

{% acceptance id="AC-005" %}
Given a PATH string containing both `/opt/agent-grid/run/hostexec-shims` and `/opt/agent-grid/run/hostexec-shims/` (trailing slash), when `dedupPath` processes the string, then both segments are retained as distinct entries in their original relative order.
{% /acceptance %}

{% acceptance id="AC-006" %}
Given a Run() invocation with an empty PreExec (spec.PreExec == ""), when Run() completes and hands control to execReplacer, then the PATH in the argv env passed to execReplacer is byte-identical to `os.Getenv("PATH")` observed at Run() entry.
{% /acceptance %}

{% acceptance id="AC-007" %}
Given a Run() invocation that runs the PreExec-branch merge wiring, when the Run() call completes, then exactly one `framelaunch.path_reassert` slog record has been emitted with fields orig_path_len, runtime_prefix_count, dedup_dropped_count, post_reassert_changed_head, spec_frame_id, and skip_branch.
{% /acceptance %}

{% acceptance id="AC-008" %}
Given `AG_FRAMELAUNCH_DISABLE_PATH_REASSERT=1` is set in the process environment, when Run() completes the PreExec branch, then the merge wiring is skipped and PATH is left as whatever PreExec produced (pre-fix behavior), and the slog record fires with `skip_branch="toggle_disabled"`.
{% /acceptance %}

{% acceptance id="AC-009" %}
Given a `hostexec.SpecBuilder` or `secretenv.SpecBuilder` constructed with `cfg.ContainerRunDir == appid.ContainerRunDir`, when a `container.Spec` is built and inspected, then `container.Spec.Env` has no `"PATH"` key (nil or empty map); `Mounts` are unchanged from pre-case-D.
{% /acceptance %}
