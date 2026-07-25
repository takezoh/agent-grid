---
change: change-20260725-windows-service-wsl-supervision
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## UAC / FR / acceptance / verification trace

| Acceptance | Requirements | Verification ID | Tier | Criterion |
|---|---|---|---|---|
| AC-UAC-001 | FR-WSL-001, FR-WSL-014 | V-UAC-001 | T1/T2 | status ok is returned only after the full readiness sequence; daemon-unavailable fails |
| AC-UAC-002 | FR-WSL-002 | V-UAC-002 | T1/T2 | the foreground unit remains healthy and retry count does not increase |
| AC-UAC-003 | FR-WSL-003 | V-UAC-003 | T1/T2 | wsl.exe exits with it, the code is recorded, and no shell or server remains |
| AC-UAC-004 | FR-WSL-004 | V-UAC-004 | T1/T2 | Backoff delay and attempt increase, then status ok restores Ready |
| AC-UAC-005 | FR-WSL-005 | V-UAC-005 | T1/T2 | Failed is recorded with the setting and retries stop |
| AC-UAC-006 | FR-WSL-006 | V-UAC-006 | T1/T2 | probe/adopt occurs before spawn and already-running is recorded without a failure |
| AC-UAC-007 | FR-WSL-007 | V-UAC-007 | T3 | the OS service-start record identifies credential/access failure and the dedicated file is not falsely claimed |
| AC-UAC-008 | FR-WSL-008 | V-UAC-008 | T1/T2 | SIGTERM reaches the existing drain path and Stopped follows unit exit within the graceful budget |
| AC-UAC-009 | FR-WSL-009 | V-UAC-009 | T3 | only the bound child then bound distribution are affected and the other distribution survives |
| AC-UAC-010 | FR-WSL-010 | V-UAC-010 | T3 | the record names --terminate target and contains no --shutdown action |
| AC-UAC-011 | FR-WSL-011 | V-UAC-011 | T3 | old health is invalidated and a timestamped fresh probe starts within ten seconds |
| AC-UAC-012 | FR-WSL-012 | V-UAC-012 | T1/T2 | graceful restart begins without terminal Failed and successful health restores Ready |
| AC-UAC-013 | FR-WSL-013 | V-UAC-013 | T3 | the bound distribution is terminated directly, then probe-before-spawn restarts |
| AC-UAC-014 | FR-WSL-014 | V-UAC-014 | T3 | the complete readiness sequence runs and only status ok produces Ready |
| AC-UAC-015 | FR-WSL-015 | V-UAC-015 | T3 | transient retries continue beyond five and recovery before ten restores Ready |
| AC-UAC-016 | FR-WSL-016, FR-WSL-018 | V-UAC-016 | T1/T2 + T3 | Register generation G for A/D, change wsl --set-default and interactive defaults to A2/D2, restart/Ready/stop, and assert records plus captured launch/health/probe/stop/terminate targets remain G and A/D; current-default re-resolution must fail. |
| AC-UAC-017 | FR-WSL-017 | V-UAC-017 | T1/T2 | retry has not stopped, no Failed record exists, and the next healthy generation becomes Ready |

## Tier strategy

- **T0 pure:** classifier, state machine, generation linearization, fake-clock budgets/backoff, config validation, redaction allowlist, target selection.
- **T1 wired:** controller with fake SCM/WSL/health/process/record adapters; existing Go gateway scenarios prove `/healthz` status semantics.
- **T2 contract:** JSON probe/stop exit classes, atomic config codec, ACL policy model, SCM adapter, no-`--shutdown` invariant, daemon lock/PID start-token contract.
- **T3 fidelity:** opt-in Windows SCM Session 0 and unsigned-in startup; real WSL distribution ownership; direct-exec exit linkage; lock adoption and SIGTERM drain; other-distro survival; sleep/resume; ProgramData ACL/rotation; SCM credential-failure record.

## External dependency triples

- **Windows SCM:** seam `SCM interface wrapping svc.Run/status/control`; fake `fake control channel + status recorder`; T3 `FakeVsRealWindowsSCMUnsignedIn`; contract `ContractSCMStopSerialization`; T0 policy / T2 adapter / T3 real SCM.
- **WSL runtime:** seam `WSLRunner for typed argv, wait and terminate(bound distro)`; fake `scripted runner fake`; T3 `FakeVsRealWSLBoundDistribution`; contract `ContractNoShutdownAndBoundTarget`; T0/T2/T3.
- **Linux process/lock:** seam `HostControl probe/stop JSON protocol`; fake `fake probe/stop client`; T3 `FakeVsRealHostControlLockAndSignal`; contract `ContractProbeBeforeSpawnAndTokenRevalidation`; T0/T2/T3.
- **HTTP health/TLS:** seam `HealthProber from bound addr/TLS config`; fake `scripted status/timeout/TLS fake`; T3 `FakeVsRealHealthzSemantics`; contract `ContractHealthStatusOKOnly`; T0/T1/T3.
- **Filesystem/config/ACL:** seam `ConfigStore + ACLApplier + RecordSink`; fake `memory store, failure/partial-write/rotation fake`; T3 `FakeVsRealProgramDataACLAndRotation`; contract `ContractAtomicSnapshotAndLogACL`; T0/T2/T3.
- **Clock/resume:** seam `Clock/Timer and ResumeSource interfaces`; fake `manual fake clock/resume source`; T3 `FakeVsRealWindowsResumeNotification`; contract `ContractGenerationInvalidatesHealth`; T0/T2/T3.

## Commands

```sh
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/wslhost/... ./cmd/server/...
cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...
GOCACHE=/tmp/gocache-agent-grid make test-e2e
# Windows T3 (opt-in, real SCM + WSL): project target introduced by implementation
make test-windows-wsl-supervisor-e2e
```

## Success gate

All 17 rows must pass, every external dependency has the full triple, every `FakeVsReal*` assertion remains strict, the direct process tree contains no shell, no command trace contains `wsl.exe --shutdown`, and config/transient counters demonstrably terminate at 5/10 respectively.

## Delivery evidence (2026-07-26)

- `go test ./platform/wslhost/... ./cmd/server/... ./cmd/agent-grid-wsl-supervisor/...`: passed.
- `go test -tags=e2e ./platform/wslhost/...`: passed; real-WSL subtests remain opt-in when the Windows/WSL environment marker is absent.
- `GOOS=windows GOARCH=amd64 go test ./platform/wslhost ./cmd/agent-grid-wsl-supervisor -run '^$'`: passed.
- `go test ./internal/harnesspolicy`: passed with the WSL runtime and Windows ACL process boundaries registered.
- `go vet ./...`: passed.
- Independent cross-task review found production-path defects in probe fingerprinting, ACL application, stopped-service gating, and persistent recording. Those defects were corrected and the affected Linux tests plus Windows cross-compilation were rerun.

Real Session 0, unsigned-in SCM startup, and cross-distribution survival require a Windows host and remain opt-in T3 checks; they were not claimed as executed in this WSL delivery environment.
