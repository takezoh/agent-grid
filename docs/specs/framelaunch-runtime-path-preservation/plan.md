---
id: plan-20260716-framelaunch-runtime-path-preservation
kind: plan
title: framelaunch runtime PATH preservation
status: draft
created: '2026-07-16'
goal: "framelaunch を sole PATH owner にし、appid.RuntimeAuthoritativePathList() を無条件 prepend + dedup する。provider は Env[PATH] を返さず、appid が shim 名 SSOT を持つ (case D)。observability slog + rollback toggle + provider-side SSOT panic で silent-failure と drift を防ぐ。"
scope_in:
- "src/platform/appid/appid.go: HostExecShimsDir / SecretEnvShimsDir / HostExecShimsPath / SecretEnvShimsPath consts + RuntimeAuthoritativePathList() function"
- "src/platform/appid/appid_test.go: T0 tests for derivation + ordering + non-emptiness + fresh-slice"
- "src/platform/framelaunch/frame_exec.go: Run()'s PATH handling inside `spec.PreExec != \"\"` branch, slog wiring, AG_FRAMELAUNCH_DISABLE_PATH_REASSERT toggle"
- "New file src/platform/framelaunch/path_authority.go + path_authority_test.go: pure dedupPath + computeFinalPath(runtimeList, capturedPath, origPath) with T0 tests"
- "src/platform/hostexec/provider.go + shim.go: drop Env[\"PATH\"]; ShimDirName becomes `= appid.HostExecShimsDir`"
- "src/platform/hostexec/provider_test.go: regression guard TestProvider_DoesNotContributePATH + retained SSOT panic test"
- "src/platform/secretenv/provider.go: drop Env[\"PATH\"]; replace local `shimDirName` with `appid.SecretEnvShimsDir`"
- "src/platform/secretenv/provider_test.go: same regression guard + SSOT panic test"
- "docs/component/component-20260624-platform-sandbox.md: case-D PATH-preservation contract + shim enumeration appendix + rollback toggle documentation + docker-exec-bash observable-PATH migration note"
- "AG_FRAMELAUNCH_DISABLE_PATH_REASSERT env var: rollback toggle with T0 branch test"
scope_out:
- "Changing preExec's own PATH construction (mise/dotfiles)"
- "Changing preExec failure handling (adr-20260711-0082)"
- "Propagating preExec's non-PATH shell state"
- "Introducing a new AG_RUNTIME_PATH wire-transport field"
- "Modifying platform/sandbox/devcontainer/spec.go's RemoteEnv merge/dedup (with providers no longer contributing PATH, upstream ordering is no longer a functional dependency)"
- "Removing cfg.ContainerRunDir as a struct field (kept per adr-20260716-provider-shim-root-appid-ssot)"
milestones:
- {id: m1, title: "appid SSOT extension + pure PATH helpers (dedupPath / computeFinalPath) + T0 tests", status: todo}
- {id: m2, title: "Run() wiring (appid.RuntimeAuthoritativePathList prepend) + slog observability", status: todo}
- {id: m3, title: "Provider Env[PATH] removal + appid const rewire + SSOT panic (retained) + regression guard", status: todo}
- {id: m4, title: "Migration rollback toggle + sandbox component doc (case-D)", status: todo}
adrs:
- adr-20260716-framelaunch-runtime-path-owner
- adr-20260716-provider-shim-root-appid-ssot
- adr-20260716-shim-priority-hardening-and-migration
decision_dispositions:
- {decision_input_ref: decision-input-001, disposition: adopted, rationale: "Adopted per adr-20260716-framelaunch-runtime-path-owner (case D): the SSOT for the runtime prefix list is appid.RuntimeAuthoritativePathList() — a named static function in appid — not an extraction from origPath. The case-C-1 extraction predicate is explicitly recorded and rejected in that ADR's Alternatives, alongside the previously-rejected (b), (c), (d) alternatives, so the reader can verify all four options were compared symmetrically.", adr_refs: [adr-20260716-framelaunch-runtime-path-owner], contract_refs: [contract-appid-runtime-path-list, contract-run-path-merge-wiring]}
- {decision_input_ref: decision-input-002, disposition: adopted, rationale: "Adopted per contract-run-path-merge-wiring: when capturedEnv has no PATH key, computeFinalPath falls back to origPath as the base (dedup'd against the runtime list). If both are empty, the runtime list itself is the entire output — non-empty by construction (appid.RuntimeAuthoritativePathList() invariant).", adr_refs: [], contract_refs: [contract-run-path-merge-wiring]}
- {decision_input_ref: decision-input-003, disposition: not_applicable, rationale: "PreExec failure handling is unchanged per adr-20260711-0082 (constraint pass-through); no new decision needed here.", adr_refs: [], contract_refs: []}
- {decision_input_ref: decision-input-004, disposition: adopted, rationale: "Adopted per contract-path-dedup: byte-exact segment comparison, no trailing-slash normalization, mirrors platform/sandbox/devcontainer/spec.go:511 deduplicateColonList and bash's PATH resolver semantics.", adr_refs: [], contract_refs: [contract-path-dedup]}
- {decision_input_ref: decision-input-005, disposition: adopted, rationale: "Adopted per contract-path-dedup: local duplication in platform/framelaunch of the ~10-15 line dedup helper, because devcontainer already imports framelaunch (FrameSpec/EnvVar) so framelaunch cannot import devcontainer without an import cycle.", adr_refs: [], contract_refs: [contract-path-dedup]}
- {decision_input_ref: decision-input-006, disposition: adopted, rationale: "Adopted per adr-20260716-framelaunch-runtime-path-owner: platform/framelaunch.Run() is the sole owner of the runtime-authoritative PATH invariant, consistent with adr-20260711-0082's sequencing ownership. Case D reinforces this by moving the shim-name SSOT into appid so no producer's PATH contribution can compete with framelaunch's prepend.", adr_refs: [adr-20260716-framelaunch-runtime-path-owner], contract_refs: [contract-run-path-merge-wiring]}
- {decision_input_ref: decision-input-007, disposition: adopted, rationale: "Adopted per adr-20260716-framelaunch-runtime-path-owner (case D): the SSOT question and the ownership question are answered together — appid owns the shim-name+list SSOT, framelaunch owns the prepend. Previously subsumed under decision-input-001; now co-adopted under the same ADR that covers case D.", adr_refs: [adr-20260716-framelaunch-runtime-path-owner], contract_refs: [contract-appid-runtime-path-list]}
- {decision_input_ref: decision-input-008, disposition: adopted, rationale: "Adopted per contract-shim-priority-migration.migration_strategy: docs/component/component-20260624-platform-sandbox.md carries the case-D PATH-ownership contract, shim enumeration, rollback-toggle semantics, and the new docker-exec-bash observable-PATH migration note.", adr_refs: [], contract_refs: [contract-shim-priority-migration]}
contracts:
- contract-appid-runtime-path-list
- contract-path-dedup
- contract-run-path-merge-wiring
- contract-run-path-observability
- contract-provider-appid-ssot
- contract-provider-no-path-contribution
- contract-shim-priority-migration
contract_projections:
- id: contract-appid-runtime-path-list
  decision_rules: [decision-appid-list-order, decision-appid-derivation]
  observable_effects: [observable-appid-runtime-list]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-appid-list-degeneracy]
  verifications: [verify-appid-list-t0]
  witnesses: [witness-appid-list-normal, witness-appid-list-independence-adversarial]
- id: contract-path-dedup
  decision_rules: [decision-dedup-first-occurrence, decision-dedup-empty-segment, decision-dedup-trailing-slash]
  observable_effects: [observable-deduped-path]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-dedup-empty-input]
  verifications: [verify-dedup-t0]
  witnesses: [witness-dedup-normal, witness-dedup-trailing-slash]
- id: contract-run-path-merge-wiring
  decision_rules: [decision-run-path-normal, decision-run-path-no-preexec]
  observable_effects: [observable-run-final-path-env, observable-run-lookpath-shim]
  operational_inputs: [input-run-orig-path, input-run-captured-env, input-run-runtime-list]
  semantic_profiles: []
  failures: [failure-run-path-preexec-error]
  verifications: [verify-run-path-t0-compute, verify-run-path-t1-wiring]
  witnesses: [witness-run-path-normal, witness-run-path-empty-inputs]
- id: contract-run-path-observability
  decision_rules: [decision-slog-emit-once]
  observable_effects: [observable-slog-record]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-slog-handler-error]
  verifications: [verify-slog-t0]
  witnesses: [witness-slog-normal, witness-slog-toggle-disabled]
- id: contract-provider-appid-ssot
  decision_rules: [decision-provider-ssot-check, decision-provider-ssot-ok]
  observable_effects: [observable-provider-panic]
  operational_inputs: [input-provider-cfg-container-run-dir]
  semantic_profiles: []
  failures: [failure-provider-ssot-mismatch]
  verifications: [verify-provider-ssot-t0]
  witnesses: [witness-provider-ssot-ok, witness-provider-ssot-panic-mismatch]
- id: contract-provider-no-path-contribution
  decision_rules: [decision-provider-no-env-path]
  observable_effects: [observable-provider-env-absence]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-provider-path-reintroduction]
  verifications: [verify-provider-no-path-t0]
  witnesses: [witness-provider-no-path-normal, witness-provider-no-path-regression]
- id: contract-shim-priority-migration
  decision_rules: [decision-migration-toggle-on, decision-migration-toggle-off]
  observable_effects: [observable-migration-toggle-effect]
  operational_inputs: [input-migration-toggle-env]
  semantic_profiles: [profile-shim-priority-evolution]
  failures: [failure-migration-toggle-parse]
  verifications: [verify-migration-toggle-t0]
  witnesses: [witness-migration-toggle-off, witness-migration-toggle-on]
tags: [framelaunch, path, preexec, hostexec, secretenv]
owners: [platform-team]
relations:
- {type: implements, target: spec-20260716-framelaunch-runtime-path-preservation}
- {type: hasPart, target: adr-20260716-framelaunch-runtime-path-owner}
- {type: hasPart, target: adr-20260716-provider-shim-root-appid-ssot}
- {type: hasPart, target: adr-20260716-shim-priority-hardening-and-migration}
source_paths:
- src/platform/framelaunch/frame_exec.go
- src/platform/hostexec/provider.go
- src/platform/secretenv/provider.go
- src/platform/appid/appid.go
- docs/component/component-20260624-platform-sandbox.md
summary: "case D 実装計画 — appid が shim 名 SSOT + RuntimeAuthoritativePathList() を提供、framelaunch が sole PATH owner として無条件 prepend + dedup、provider は Env[PATH] を返さない、observability slog + rollback toggle で shim resolution を確定化"
---

## Goal

Case D — framelaunch is the sole PATH owner. It unconditionally prepends the static list `appid.RuntimeAuthoritativePathList()` onto whatever PATH PreExec produced (or origPath if PreExec omitted PATH), deduplicated and order-preserving. Providers (hostexec / secretenv) no longer contribute `Env["PATH"]` to the container spec — they only emit `Mounts`. Shim subdirectory names live in `appid` as the SSOT (`HostExecShimsDir`, `SecretEnvShimsDir`), and `appid.RuntimeAuthoritativePathList()` returns them fully qualified in a fixed order. Provider construction still panics on `cfg.ContainerRunDir != appid.ContainerRunDir` for defense in depth, and a per-package regression guard test pins the "provider does not contribute PATH" invariant. Observability slog and rollback toggle are retained.

## Implementation Sequence

{% milestone id="m1" %}
**appid SSOT extension + pure PATH helpers + T0 tests** — establish the case-D SSOT and the T0-testable core before any process-env wiring.

Chunk members: `component:component-appid-identity`, `component:component-framelaunch-path-authority`, requirements `FR-001 FR-002 FR-003 FR-006 NFR-001`, ADR `adr-20260716-framelaunch-runtime-path-owner`.

Task-grade units:

- **appid-runtime-path-ssot**
  - objective: Add `appid.HostExecShimsDir`, `appid.SecretEnvShimsDir`, `appid.HostExecShimsPath`, `appid.SecretEnvShimsPath` consts and `appid.RuntimeAuthoritativePathList() []string` backed by T0 tests. Order = `[HostExecShimsPath, SecretEnvShimsPath]`. Return a fresh slice each call.
  - files_touched: `src/platform/appid/appid.go`, `src/platform/appid/appid_test.go`
  - acceptance: derivations pinned, ordering pinned, fresh-slice pinned, `go test ./platform/appid/...` passes.
  - contract_refs: `contract-appid-runtime-path-list`
  - max_diff_loc: 150

- **path-authority-helpers** (depends_on: `appid-runtime-path-ssot`)
  - objective: Add pure `dedupPath(pathStr)` and `computeFinalPath(runtimeList []string, capturedPath, origPath string) (string, ComputeFinalPathDecision)` to `platform/framelaunch` backed by T0 table-driven tests. No extract predicate — the runtime list is caller-supplied.
  - output_format: New `src/platform/framelaunch/path_authority.go` + `path_authority_test.go`.
  - tool_guidance: `computeFinalPath` picks `base = capturedPath if non-empty else origPath`; joins the runtime list first, then base (if non-empty), then `dedupPath`. `ComputeFinalPathDecision` records `{Branch, PrefixCount, DroppedCount, HeadChanged}`. Do NOT touch `os.Setenv`.
  - task_boundaries: No `Run()` wiring; no provider changes; no extract predicate.
  - files_touched: `src/platform/framelaunch/path_authority.go`, `src/platform/framelaunch/path_authority_test.go`
  - contract_refs: `contract-path-dedup`, `contract-run-path-merge-wiring`
  - max_diff_loc: 300
{% /milestone %}

{% milestone id="m2" %}
**Run() wiring + slog observability** — plumb `computeFinalPath` (with `appid.RuntimeAuthoritativePathList()`) into `Run()` and emit one `framelaunch.path_reassert` slog record per invocation. depends_on: m1.

Chunk members: `component:component-framelaunch-run`, requirements `FR-001 FR-002 FR-004 FR-007 FR-008 NFR-003 NFR-004`, ADR `adr-20260716-framelaunch-runtime-path-owner`.

Task-grade units:

- **run-wiring-integration** (depends_on: `path-authority-helpers`)
  - objective: Wire `computeFinalPath(appid.RuntimeAuthoritativePathList(), capturedEnv["PATH"], origPath)` into `Run()` inside the existing `spec.PreExec != ""` branch. Add a `runtimePathListForTest` package-var seam for hermetic T1 tests. Add T1 test asserting captured envv PATH begins with the runtime list + `exec.LookPath('gh')` resolves to a shim under the first runtime-list dir.
  - files_touched: `src/platform/framelaunch/frame_exec.go`, `src/platform/framelaunch/frame_exec_test.go`
  - contract_refs: `contract-run-path-merge-wiring`
  - max_diff_loc: 300

- **path-reassert-slog** (depends_on: `run-wiring-integration`)
  - objective: Emit exactly one `framelaunch.path_reassert` slog record per PreExec-branch invocation with fields `{orig_path_len, runtime_prefix_count, dedup_dropped_count, post_reassert_changed_head, spec_frame_id, skip_branch}`. Add T0 test via slog test handler seam.
  - contract_refs: `contract-run-path-observability`
  - max_diff_loc: 200
{% /milestone %}

{% milestone id="m3" %}
**Provider Env[PATH] removal + appid const rewire + SSOT panic + regression guard** — collapse the vestigial 3-stage relay.

Chunk members: `component:component-hostexec-provider`, `component:component-secretenv-provider`, requirements `FR-001 FR-002 FR-009`, ADR `adr-20260716-provider-shim-root-appid-ssot`.

Task-grade units:

- **provider-env-path-removal**
  - objective: Remove `Env["PATH"]` from both `hostexec.SpecBuilder.Build` and `secretenv.SpecBuilder.Build` returned `container.Spec` (case-D invariant). Rewire the shim subdirectory-name references to `appid.HostExecShimsDir` / `appid.SecretEnvShimsDir`. Keep the construction-time SSOT panic. Add regression-guard tests `TestProvider_DoesNotContributePATH` per package.
  - output_format: Edits in `src/platform/hostexec/provider.go`, `src/platform/hostexec/shim.go` (ShimDirName alias re-export = `appid.HostExecShimsDir`), `src/platform/secretenv/provider.go`; test edits in both `*_test.go` files (update existing Env expectations + add the regression pin).
  - task_boundaries: Do not modify Mounts semantics. Do not delete `cfg.ContainerRunDir` field. Do not modify devcontainer/spec.go.
  - files_touched: `src/platform/hostexec/provider.go`, `src/platform/hostexec/shim.go`, `src/platform/hostexec/provider_test.go`, `src/platform/secretenv/provider.go`, `src/platform/secretenv/provider_test.go`
  - contract_refs: `contract-provider-appid-ssot`, `contract-provider-no-path-contribution`
  - max_diff_loc: 350
{% /milestone %}

{% milestone id="m4" %}
**Migration rollback toggle + sandbox component doc (case-D)** — ship the toggle and document the case-D contract + shim enumeration + docker-exec-bash observable-PATH migration note. depends_on: m2.

Chunk members: `component:component-framelaunch-run`, `component:component-docs-sandbox-path-ordering`, requirements `FR-001 NFR-003`, ADR `adr-20260716-shim-priority-hardening-and-migration`.

Task-grade units:

- **run-migration-toggle** (depends_on: `path-reassert-slog`)
  - objective: Honor `AG_FRAMELAUNCH_DISABLE_PATH_REASSERT=1|true|yes` (case-insensitive) as a rollback toggle skipping merge wiring; slog still fires with `skip_branch="toggle_disabled"`.
  - files_touched: `src/platform/framelaunch/frame_exec.go`, `src/platform/framelaunch/frame_exec_test.go`
  - contract_refs: `contract-shim-priority-migration`
  - max_diff_loc: 150

- **docs-sandbox-path-ordering** (depends_on: `run-migration-toggle`)
  - objective: Update `docs/component/component-20260624-platform-sandbox.md` with case-D contract (framelaunch is sole PATH owner; providers only mount), shim enumeration appendix, trust-boundary note, rollback toggle documentation, and the docker-exec-bash observable-PATH migration note.
  - files_touched: `docs/component/component-20260624-platform-sandbox.md`
  - contract_refs: `contract-shim-priority-migration`
  - max_diff_loc: 250
{% /milestone %}

## Targets

- **owners / boundary**: `platform/framelaunch.Run()` is the sole owner of the runtime PATH invariant (adr-20260716-framelaunch-runtime-path-owner). `platform/appid` owns the shim-name+list SSOT (`HostExecShimsDir`, `SecretEnvShimsDir`, `RuntimeAuthoritativePathList()`). Both `hostexec` and `secretenv` SpecBuilders panic on drift and MUST NOT contribute `Env["PATH"]` (adr-20260716-provider-shim-root-appid-ssot).
- **seams**: existing `execReplacer`, `readPasswd`, `currentUser` package vars in `frame_exec.go`; new `runtimePathListForTest` package var seam (T1 hermeticity for `appid.RuntimeAuthoritativePathList()`); new pure `computeFinalPath` seam callable without process env; new slog handler seam swap in `frame_exec_test.go` for the observability contract test; `t.Setenv("AG_FRAMELAUNCH_DISABLE_PATH_REASSERT", …)` for the migration toggle test.
- **contract projections**: mirrored in `contract_projections[]` above so materialization validation can reconcile stable IDs with the canonical plan.

## Verification

| Tier | Command | Criterion | Milestone DoD |
|------|---------|-----------|---------------|
| T0 | `cd src && go test -run TestRuntimeAuthoritativePathList ./platform/appid/...` | Derivations + ordering + fresh-slice invariants hold. | m1 done when this test passes. |
| T0 | `cd src && go test -run 'TestDedupPath\|TestComputeFinalPath' ./platform/framelaunch/...` | All table-driven cases in `contract-path-dedup.witnesses` and `contract-run-path-merge-wiring.witnesses` (T0 subset) pass. | m1 done when these tests pass. |
| T0 | `cd src && go test -run 'TestRun_PathReassertSlog\|TestRun_MigrationToggle' ./platform/framelaunch/...` | Slog records + toggle branch assertions per `witness-slog-*` and `witness-migration-toggle-*`. | m2 (slog) and m4 (toggle) done when these pass. |
| T0 | `cd src && go test -run TestSpecBuilder_ContainerRunDirSSOT ./platform/hostexec/... ./platform/secretenv/...` | Mismatched cfg panics; matched cfg no-ops; `hostexec.ShimDirName == appid.HostExecShimsDir`. | m3 done when both provider T0 tests pass. |
| T0 | `cd src && go test -run TestProvider_DoesNotContributePATH ./platform/hostexec/... ./platform/secretenv/...` | Case-D regression guard: `container.Spec.Env` has no `"PATH"` key. | m3 done when both packages report pass. |
| T1 | `cd src && go test -run TestRun_PathReassert ./platform/framelaunch/...` | Captured envv PATH begins with runtime-list entries (via `runtimePathListForTest` seam); `exec.LookPath('gh')` resolves to the shim file; FR-007 empty-preExec case leaves PATH byte-identical. | m2 done when the T1 test passes. |
| T1 | `cd src && go test ./platform/framelaunch/... ./platform/hostexec/... ./platform/secretenv/... ./platform/appid/...` | Full package test success. | overall done. |
| T0 | `docs lint` (or the docs lint bootstrap the repo uses) over `docs/specs/framelaunch-runtime-path-preservation/*` and `docs/adr/adr-20260716-*` | Docs lint zero errors. | overall done. |

## Reference Algorithms

(none — `computeFinalPath` is a straightforward pure function that Go source expresses more precisely than pseudocode. Left empty per adr-20260709-reference-algorithms-in-plan guidance to add reference algorithms only when they are the essence of a contract.)
