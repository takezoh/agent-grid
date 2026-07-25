---
change: change-20260725-windows-service-wsl-supervision
role: implementation
contracts: ["contract-data-model","contract-control-flow","contract-state-lifecycle","contract-concurrency","contract-resource-management","contract-failure-recovery","contract-integration-contract","contract-security-boundary","contract-performance-budget","contract-migration-compatibility","contract-user-observability"]
contract_projections: [{"id":"contract-data-model","decision_rules":["decision-data-model-determinate"],"observable_effects":["observable-data-model"],"operational_inputs":["input-data-model"],"semantic_profiles":[],"failures":["failure-data-model"],"verifications":["verify-data-model"],"witnesses":["witness-data-model-normal","witness-data-model-adversarial"]},{"id":"contract-control-flow","decision_rules":["decision-control-flow-determinate"],"observable_effects":["observable-control-flow"],"operational_inputs":["input-control-flow"],"semantic_profiles":[],"failures":["failure-control-flow"],"verifications":["verify-control-flow"],"witnesses":["witness-control-flow-normal","witness-control-flow-adversarial"]},{"id":"contract-state-lifecycle","decision_rules":["decision-state-lifecycle-determinate","decision-state-lifecycle-unknown","decision-state-lifecycle-inconclusive","decision-state-lifecycle-conflicting"],"observable_effects":["observable-state-lifecycle"],"operational_inputs":["input-state-lifecycle"],"semantic_profiles":["profile-state-lifecycle-partition"],"failures":["failure-state-lifecycle"],"verifications":["verify-state-lifecycle"],"witnesses":["witness-state-lifecycle-normal","witness-state-lifecycle-adversarial"]},{"id":"contract-concurrency","decision_rules":["decision-concurrency-determinate"],"observable_effects":["observable-concurrency"],"operational_inputs":["input-concurrency"],"semantic_profiles":[],"failures":["failure-concurrency"],"verifications":["verify-concurrency"],"witnesses":["witness-concurrency-normal","witness-concurrency-adversarial"]},{"id":"contract-resource-management","decision_rules":["decision-resource-management-determinate"],"observable_effects":["observable-resource-management"],"operational_inputs":["input-resource-management"],"semantic_profiles":[],"failures":["failure-resource-management"],"verifications":["verify-resource-management"],"witnesses":["witness-resource-management-normal","witness-resource-management-adversarial"]},{"id":"contract-failure-recovery","decision_rules":["decision-failure-recovery-determinate","decision-failure-recovery-unknown","decision-failure-recovery-inconclusive"],"observable_effects":["observable-failure-recovery"],"operational_inputs":["input-failure-recovery"],"semantic_profiles":["profile-failure-recovery-partition"],"failures":["failure-failure-recovery"],"verifications":["verify-failure-recovery"],"witnesses":["witness-failure-recovery-normal","witness-failure-recovery-adversarial"]},{"id":"contract-integration-contract","decision_rules":["decision-integration-contract-determinate"],"observable_effects":["observable-integration-contract"],"operational_inputs":["input-integration-contract"],"semantic_profiles":[],"failures":["failure-integration-contract"],"verifications":["verify-integration-contract"],"witnesses":["witness-integration-contract-normal","witness-integration-contract-adversarial"]},{"id":"contract-security-boundary","decision_rules":["decision-security-boundary-determinate"],"observable_effects":["observable-security-boundary"],"operational_inputs":["input-security-boundary"],"semantic_profiles":[],"failures":["failure-security-boundary"],"verifications":["verify-security-boundary"],"witnesses":["witness-security-boundary-normal","witness-security-boundary-adversarial"]},{"id":"contract-performance-budget","decision_rules":["decision-performance-budget-determinate"],"observable_effects":["observable-performance-budget"],"operational_inputs":["input-performance-budget"],"semantic_profiles":["profile-performance-budget-cost"],"failures":["failure-performance-budget"],"verifications":["verify-performance-budget"],"witnesses":["witness-performance-budget-normal","witness-performance-budget-adversarial","witness-performance-budget-shutdown-derivation"]},{"id":"contract-migration-compatibility","decision_rules":["decision-migration-compatibility-determinate"],"observable_effects":["observable-migration-compatibility"],"operational_inputs":["input-migration-compatibility"],"semantic_profiles":["profile-migration-compatibility-evolution"],"failures":["failure-migration-compatibility"],"verifications":["verify-migration-compatibility"],"witnesses":["witness-migration-compatibility-normal","witness-migration-compatibility-adversarial","witness-migration-compatibility-registration-binding"]},{"id":"contract-user-observability","decision_rules":["decision-user-observability-determinate"],"observable_effects":["observable-user-observability"],"operational_inputs":["input-user-observability"],"semantic_profiles":[],"failures":["failure-user-observability"],"verifications":["verify-user-observability"],"witnesses":["witness-user-observability-normal","witness-user-observability-adversarial"]}]
adrs: ["adr-20260725-wsl-supervisor-registration-bound-json-config","adr-20260725-wsl-supervisor-direct-foreground-exec","adr-20260725-wsl-supervisor-detach-adr-nonapplicability","adr-20260725-wsl-supervisor-adopt-authority-classifier","adr-20260725-wsl-supervisor-retry-health-thresholds","adr-20260725-wsl-supervisor-shutdown-budget-escalation","adr-20260725-wsl-supervisor-machine-wide-record"]
decision_dispositions: [{"decision_input_ref":"decision-input-registration-binding","disposition":"adopted","rationale":"Registration-time binding is confirmed and closed by the atomic JSON snapshot ADR.","adr_refs":["adr-20260725-wsl-supervisor-registration-bound-json-config"],"contract_refs":["contract-data-model","contract-security-boundary","contract-migration-compatibility"]},{"decision_input_ref":"decision-input-launch-topology","disposition":"adopted","rationale":"Direct foreground wsl.exe --exec is fixed; no launcher or shell remains.","adr_refs":["adr-20260725-wsl-supervisor-direct-foreground-exec"],"contract_refs":["contract-control-flow"]},{"decision_input_ref":"decision-input-detach-spike-nonapplicability","disposition":"adopted","rationale":"The accepted detach ADR is explicitly non-applicable to a long-lived foreground parent.","adr_refs":["adr-20260725-wsl-supervisor-detach-adr-nonapplicability"],"contract_refs":["contract-control-flow"]},{"decision_input_ref":"decision-input-already-running","disposition":"adopted","rationale":"Typed pre-spawn lock/process fingerprinting establishes adoption and stop authority.","adr_refs":["adr-20260725-wsl-supervisor-adopt-authority-classifier"],"contract_refs":["contract-state-lifecycle","contract-integration-contract"]},{"decision_input_ref":"decision-input-health-config","disposition":"adopted","rationale":"Health target and all retry/readiness thresholds are fixed from the bound snapshot.","adr_refs":["adr-20260725-wsl-supervisor-retry-health-thresholds"],"contract_refs":["contract-state-lifecycle","contract-performance-budget"]},{"decision_input_ref":"decision-input-dedicated-log","disposition":"adopted","rationale":"The selected dedicated log is concretized with ProgramData path, ACL, retention and SCM complement.","adr_refs":["adr-20260725-wsl-supervisor-machine-wide-record"],"contract_refs":["contract-resource-management","contract-user-observability","contract-security-boundary"]},{"decision_input_ref":"decision-input-shutdown-timeout-provenance","disposition":"adopted","rationale":"Adopt OPT-DERIVE-FROM-SERVER-BUDGET: 8-second drain plus 5-second gateway grace plus 2-second measured unit-exit allowance yields the single 15-second graceful budget.","adr_refs":["adr-20260725-wsl-supervisor-shutdown-budget-escalation"],"contract_refs":["contract-failure-recovery","contract-performance-budget"]}]
milestones: [{"id":"milestone-1-registration-and-linux-control"},{"id":"milestone-2-lifecycle"},{"id":"milestone-3-stop-and-record"}]
reference_algorithms: [{"id":"alg-supervisor-lifecycle"},{"id":"alg-stop-escalation"}]
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Fixed decisions

- Shutdown provenance: `decision-input-shutdown-timeout-provenance` adopts `OPT-DERIVE-FROM-SERVER-BUDGET`; the typed trace closes through the shutdown ADR, `contract-failure-recovery`, `contract-performance-budget`, and `witness-performance-budget-shutdown-derivation`.

- Launch: direct `wsl.exe --distribution D --user U --exec server ...`; no shell, launcher, detach, setsid, or nohup.
- Adoption: `server supervisor-probe` runs before spawn and returns typed JSON. Exact data-dir lock holder, executable/argv fingerprint, Linux PID start token, and configured health evidence establish adoption and stop authority.
- Stop: `server supervisor-stop` revalidates the token and sends SIGTERM. Graceful budget is 15 seconds; SCM stop may kill the owned child then wait 2 seconds before bound-distro terminate; recovery skips child kill after 15 seconds.
- Retry: health timeout 2s, cadence 5s, resume recheck <=10s, startup budget 60s, backoff 1/2/5/15/60s, configuration terminal 5, transient terminal 10, health restart threshold 3.
- Config: versioned atomic JSON snapshot. No TOML dependency and no runtime default lookup.
- Record: `%ProgramData%\AgentGrid\logs\wsl-host.log`, 10 MiB, 5 files, 30 days; structured allowlist only and no raw stdout/stderr.

## External dependency seams and mandatory triple

| Dependency | Injection seam | Fake | FakeVsReal* T3 | invariant-naming contract | Tier |
|---|---|---|---|---|---|
| Windows SCM | SCM interface wrapping svc.Run/status/control | fake control channel + status recorder | FakeVsRealWindowsSCMUnsignedIn | ContractSCMStopSerialization | T0 policy / T2 adapter / T3 real SCM |
| WSL runtime | WSLRunner for typed argv, wait and terminate(bound distro) | scripted runner fake | FakeVsRealWSLBoundDistribution | ContractNoShutdownAndBoundTarget | T0/T2/T3 |
| Linux process/lock | HostControl probe/stop JSON protocol | fake probe/stop client | FakeVsRealHostControlLockAndSignal | ContractProbeBeforeSpawnAndTokenRevalidation | T0/T2/T3 |
| HTTP health/TLS | HealthProber from bound addr/TLS config | scripted status/timeout/TLS fake | FakeVsRealHealthzSemantics | ContractHealthStatusOKOnly | T0/T1/T3 |
| Filesystem/config/ACL | ConfigStore + ACLApplier + RecordSink | memory store, failure/partial-write/rotation fake | FakeVsRealProgramDataACLAndRotation | ContractAtomicSnapshotAndLogACL | T0/T2/T3 |
| Clock/resume | Clock/Timer and ResumeSource interfaces | manual fake clock/resume source | FakeVsRealWindowsResumeNotification | ContractGenerationInvalidatesHealth | T0/T2/T3 |

Each external dependency ships the full fake + `FakeVsReal*` + invariant contract triple. A FakeVsReal failure is fixed in the fake or production adapter; its assertion is not weakened.

## Eleven-dimension closure

| Dimension | Owner | Unit | Contract | Typed failure | Adversarial witness | Verification |
|---|---|---|---|---|---|---|
| data_model | component-service-registration | Implement registration snapshot and ACL boundary | contract-data-model | failure-data-model | witness-data-model-adversarial | T0 / verify-data-model |
| control_flow | component-supervised-lifecycle | Implement supervised lifecycle and health state machine | contract-control-flow | failure-control-flow | witness-control-flow-adversarial | T0 / verify-control-flow |
| state_lifecycle | component-supervised-lifecycle | Implement supervised lifecycle and health state machine | contract-state-lifecycle | failure-state-lifecycle | witness-state-lifecycle-adversarial | T0 / verify-state-lifecycle |
| concurrency | component-supervised-lifecycle | Implement supervised lifecycle and health state machine | contract-concurrency | failure-concurrency | witness-concurrency-adversarial | T0 / verify-concurrency |
| resource_management | component-operator-recording | Implement structured machine-wide recording | contract-resource-management | failure-resource-management | witness-resource-management-adversarial | T0 / verify-resource-management |
| failure_recovery | component-stop-coordinator | Implement SCM stop and registered-distro escalation | contract-failure-recovery | failure-failure-recovery | witness-failure-recovery-adversarial | T2 / verify-failure-recovery |
| integration_contract | component-linux-control | Implement typed Linux probe and stop control | contract-integration-contract | failure-integration-contract | witness-integration-contract-adversarial | T2 / verify-integration-contract |
| security_boundary | component-service-registration | Implement registration snapshot and ACL boundary | contract-security-boundary | failure-security-boundary | witness-security-boundary-adversarial | T2 / verify-security-boundary |
| performance_budget | component-supervised-lifecycle | Implement supervised lifecycle and health state machine | contract-performance-budget | failure-performance-budget | witness-performance-budget-adversarial | T0 / verify-performance-budget |
| migration_compatibility | component-service-registration | Implement registration snapshot and ACL boundary | contract-migration-compatibility | failure-migration-compatibility | witness-migration-compatibility-adversarial | T0 / verify-migration-compatibility |
| user_observability | component-operator-recording | Implement structured machine-wide recording | contract-user-observability | failure-user-observability | witness-user-observability-adversarial | T0 / verify-user-observability |

## Milestones and task-grade units

### milestone-1-registration-and-linux-control

- **Implement registration snapshot and ACL boundary** — Create atomic JSON registration/apply semantics and fixed target binding.
  - Files: src/cmd/agent-grid-wsl-supervisor/, src/platform/wslhost/config.go
  - Contracts: contract-data-model, contract-security-boundary, contract-migration-compatibility
  - Acceptance: Atomic validated generation is the only runtime config source.; Default identity/distro are resolved only at registration.
- **Implement typed Linux probe and stop control** — Add additive server CLI commands that classify lock ownership and deliver ownership-checked SIGTERM.
  - Files: src/cmd/server/supervisor_control.go, src/cmd/server/daemon_lock.go
  - Contracts: contract-integration-contract
  - Acceptance: Probe distinguishes absent, matched running, mismatch, unknown.; Stop rejects stale or mismatched ownership and reaches SIGTERM.

### milestone-2-lifecycle

- **Implement supervised lifecycle and health state machine** — Implement probe-before-spawn, direct foreground launch, readiness, resume invalidation, classifier, and fixed retry thresholds.
  - Files: src/platform/wslhost/supervisor_windows.go, src/platform/wslhost/process_windows.go, src/platform/wslhost/health_windows.go
  - Contracts: contract-control-flow, contract-state-lifecycle, contract-concurrency, contract-performance-budget
  - Acceptance: All UAC-001..006,011,012,014,015,017 transitions are deterministic.; No spawn occurs before determinate absence.

### milestone-3-stop-and-record

- **Implement SCM stop and registered-distro escalation** — Bridge SCM controls to ownership-checked SIGTERM and bounded escalation.
  - Files: src/platform/wslhost/service_windows.go, src/platform/wslhost/stop_windows.go, src/cmd/agent-grid-wsl-supervisor/main_windows.go
  - Contracts: contract-failure-recovery
  - Acceptance: 15-second graceful budget preserves existing drain/grace.; Only owned child and registered distro are escalation targets.
- **Implement structured machine-wide recording** — Provide operator-readable structured records, safe retention and pre-start lookup instructions.
  - Files: src/platform/wslhost/record_windows.go, src/platform/wslhost/config.go, docs/design/design-host.md
  - Contracts: contract-resource-management, contract-user-observability
  - Acceptance: ProgramData path/ACL/10MiB x5/30-day policy is enforced.; SCM pre-process failures have a deterministic documented lookup.

## Reference algorithms

### alg-supervisor-lifecycle

serialize event; discard if event.generation != current; on start probe before spawn; matched=>adopt, absent=>spawn once, mismatch=>config failure, unknown=>transient; Ready only on status ok; third health failure=>graceful restart; config count 5=>Failed; transient count 10=>Failed; Ready resets counters; resume invalidates health and schedules immediate probe.

Bounds: At most one spawn, health request and timer per generation. Backoff is 1,2,5,15,60 seconds capped at 60. All terminal partitions are exhaustive and mutually exclusive.

### alg-stop-escalation

revalidate token; invoke supervisor-stop(SIGTERM); report pending checkpoints while waiting <=15s; if exited finish; if SCM-stop terminate owned child, wait 2s, probe, and terminate bound distro only if evidence persists; if recovery terminate bound distro immediately after 15s; after terminate re-probe absence before reporting completion; never invoke --shutdown.

Bounds: Graceful wait 15s; optional child wait 2s; one bound-distro terminate. Duplicate/late stop controls cannot start another transaction.

## Persistent design promotion

At closure, promote only future-governing rules: Windows SCM is the sole supervisor; registration snapshot owns target identity; probe-before-spawn and ownership token guard spawn/stop; all force actions are bound-distro-only; external Windows/WSL dependencies require the testing triple. Private filenames and threshold implementation details remain in this change package/ADR, not persistent design.
