---
change: change-20260725-windows-service-wsl-supervision
role: requirements
functional_requirements: [{"id":"FR-WSL-001","statement":"While the registered Windows Service is Running, the system shall report Ready only after the configured GET /healthz returns HTTP 200 with status ok; daemon-unavailable, transport success alone, and process existence shall remain non-Ready.","priority":"must"},{"id":"FR-WSL-002","statement":"The supervisor shall keep exactly one registration-bound host server supervised unit in the foreground and shall never leave a launcher or shell process in the supervision chain.","priority":"must"},{"id":"FR-WSL-003","statement":"When the Linux host server exits normally or abnormally, the supervised wsl.exe unit shall exit, the exit code shall be recorded, and no host server or launcher shell shall remain.","priority":"must"},{"id":"FR-WSL-004","statement":"When a transient crash or health failure occurs, the supervisor shall enter Backoff, record the delay and attempt number, retry according to the fixed schedule, and return to Ready only after health succeeds.","priority":"must"},{"id":"FR-WSL-005","statement":"If the same typed configuration failure occurs five consecutive times, the supervisor shall enter terminal Failed, record the specific invalid setting, and issue no further retry until a new applied registration generation or SCM restart resets the transaction.","priority":"must"},{"id":"FR-WSL-006","statement":"When the pre-spawn probe proves that a fingerprint-matched host already holds the registered data-directory lock, the supervisor shall adopt that instance before any spawn, record already-running, and shall not classify it as crash or configuration failure.","priority":"must"},{"id":"FR-WSL-007","statement":"If SCM cannot create the service process because of credentials, Log on as a service, profile, executable ACL, or configuration ACL, the Windows service-start record shall expose a credential or access category and a deterministic operator lookup path.","priority":"must"},{"id":"FR-WSL-008","statement":"When SCM requests stop, the supervisor shall use the registered instance control command to deliver SIGTERM, allow the existing 8-second drain and 5-second gateway grace within a 15-second graceful budget, and report Stopped only after the owned unit exits or bounded escalation completes.","priority":"must"},{"id":"FR-WSL-009","statement":"If graceful stop exceeds 15 seconds, the supervisor shall terminate only the owned wsl.exe child, wait 2 seconds, and only if the registered lock/health evidence persists shall invoke wsl.exe --terminate for the registration-bound distribution; it shall never invoke wsl.exe --shutdown.","priority":"must"},{"id":"FR-WSL-010","statement":"Every forced WSL action shall name the registration-bound distribution, and the record shall never contain an all-distribution shutdown action, including on a machine with only one distribution.","priority":"must"},{"id":"FR-WSL-011","statement":"When Windows resumes from sleep or hibernate, the supervisor shall invalidate pre-resume health evidence and start a fresh health probe within 10 seconds, recording its time, generation, and result.","priority":"must"},{"id":"FR-WSL-012","statement":"When three consecutive 5-second-cadence health probes fail for the same live generation, the supervisor shall start a graceful restart without entering terminal Failed; a later successful health probe shall restore Ready.","priority":"must"},{"id":"FR-WSL-013","statement":"If a graceful recovery restart exceeds the same 15-second shutdown budget and health remains failed, the supervisor shall skip child-only kill, terminate only the registration-bound distribution, and then restart from the pre-spawn probe.","priority":"must"},{"id":"FR-WSL-014","statement":"When Automatic service start follows a Windows or WSL update reboot with no signed-in user, the supervisor shall run probe-before-spawn and readiness from the beginning and shall expose Ready only after status ok.","priority":"must"},{"id":"FR-WSL-015","statement":"When the WSL subsystem becomes unavailable without a machine reboot, the supervisor shall retry as transient failure and shall not enter Failed before ten consecutive transient failures; recovery shall return to Ready without operator action.","priority":"must"},{"id":"FR-WSL-016","statement":"The Windows record surface shall expose service start/stop, registration-bound Windows identity, distribution and Linux user, generation, readiness result, exit code, retry count, budget expiry, and escalation at %ProgramData%\\AgentGrid\\logs\\wsl-host.log without secret, token, or environment values.","priority":"must"},{"id":"FR-WSL-017","statement":"A transient failure shall not be terminal at the five-failure configuration threshold; only the tenth consecutive transient failure may enter Failed, and any intervening Ready result shall reset the transient counter.","priority":"must"},{"id":"FR-WSL-018","statement":"The registration snapshot shall be the sole runtime authority for identity, distribution, Linux user, executable, data directory, address, TLS mode, health target, log path, and thresholds; interactive login and wsl --set-default changes shall not alter the running generation.","priority":"must"}]
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Provenance boundary

The upstream `ux.md` remains `phase: explore`. Confirmed constraints and consultation answers are normative inputs; launch/classifier/budget candidates are provisional seeds resolved here as proposed ADRs. This document does not rewrite Explore hypotheses as accepted product fact. The change remains draft until ADR consultation.

## Functional requirements

### FR-WSL-001 (state_driven, must)

While the registered Windows Service is Running, the system shall report Ready only after the configured GET /healthz returns HTTP 200 with status ok; daemon-unavailable, transport success alone, and process existence shall remain non-Ready.

### FR-WSL-002 (ubiquitous, must)

The supervisor shall keep exactly one registration-bound host server supervised unit in the foreground and shall never leave a launcher or shell process in the supervision chain.

### FR-WSL-003 (event_driven, must)

When the Linux host server exits normally or abnormally, the supervised wsl.exe unit shall exit, the exit code shall be recorded, and no host server or launcher shell shall remain.

### FR-WSL-004 (event_driven, must)

When a transient crash or health failure occurs, the supervisor shall enter Backoff, record the delay and attempt number, retry according to the fixed schedule, and return to Ready only after health succeeds.

### FR-WSL-005 (unwanted, must)

If the same typed configuration failure occurs five consecutive times, the supervisor shall enter terminal Failed, record the specific invalid setting, and issue no further retry until a new applied registration generation or SCM restart resets the transaction.

### FR-WSL-006 (event_driven, must)

When the pre-spawn probe proves that a fingerprint-matched host already holds the registered data-directory lock, the supervisor shall adopt that instance before any spawn, record already-running, and shall not classify it as crash or configuration failure.

### FR-WSL-007 (unwanted, must)

If SCM cannot create the service process because of credentials, Log on as a service, profile, executable ACL, or configuration ACL, the Windows service-start record shall expose a credential or access category and a deterministic operator lookup path.

### FR-WSL-008 (event_driven, must)

When SCM requests stop, the supervisor shall use the registered instance control command to deliver SIGTERM, allow the existing 8-second drain and 5-second gateway grace within a 15-second graceful budget, and report Stopped only after the owned unit exits or bounded escalation completes.

### FR-WSL-009 (unwanted, must)

If graceful stop exceeds 15 seconds, the supervisor shall terminate only the owned wsl.exe child, wait 2 seconds, and only if the registered lock/health evidence persists shall invoke wsl.exe --terminate for the registration-bound distribution; it shall never invoke wsl.exe --shutdown.

### FR-WSL-010 (ubiquitous, must)

Every forced WSL action shall name the registration-bound distribution, and the record shall never contain an all-distribution shutdown action, including on a machine with only one distribution.

### FR-WSL-011 (event_driven, must)

When Windows resumes from sleep or hibernate, the supervisor shall invalidate pre-resume health evidence and start a fresh health probe within 10 seconds, recording its time, generation, and result.

### FR-WSL-012 (event_driven, must)

When three consecutive 5-second-cadence health probes fail for the same live generation, the supervisor shall start a graceful restart without entering terminal Failed; a later successful health probe shall restore Ready.

### FR-WSL-013 (event_driven, must)

If a graceful recovery restart exceeds the same 15-second shutdown budget and health remains failed, the supervisor shall skip child-only kill, terminate only the registration-bound distribution, and then restart from the pre-spawn probe.

### FR-WSL-014 (event_driven, must)

When Automatic service start follows a Windows or WSL update reboot with no signed-in user, the supervisor shall run probe-before-spawn and readiness from the beginning and shall expose Ready only after status ok.

### FR-WSL-015 (event_driven, must)

When the WSL subsystem becomes unavailable without a machine reboot, the supervisor shall retry as transient failure and shall not enter Failed before ten consecutive transient failures; recovery shall return to Ready without operator action.

### FR-WSL-016 (ubiquitous, must)

The Windows record surface shall expose service start/stop, registration-bound Windows identity, distribution and Linux user, generation, readiness result, exit code, retry count, budget expiry, and escalation at %ProgramData%\AgentGrid\logs\wsl-host.log without secret, token, or environment values.

### FR-WSL-017 (unwanted, must)

A transient failure shall not be terminal at the five-failure configuration threshold; only the tenth consecutive transient failure may enter Failed, and any intervening Ready result shall reset the transient counter.

### FR-WSL-018 (ubiquitous, must)

The registration snapshot shall be the sole runtime authority for identity, distribution, Linux user, executable, data directory, address, TLS mode, health target, log path, and thresholds; interactive login and wsl --set-default changes shall not alter the running generation.

## Non-functional requirements

- **NFR-WSL-RELIABILITY / reliability:** Configuration failures terminate after 5 attempts; transient failures terminate after 10; retry delays are 1,2,5,15,60 seconds capped at 60. Measurement: Fake-clock T0/T1 tests and Windows/WSL T3 timestamps.
- **NFR-WSL-PERFORMANCE / performance:** Health request timeout is 2 seconds, cadence 5 seconds, resume recheck begins within 10 seconds, startup readiness budget is 60 seconds, and graceful stop budget is 15 seconds. Measurement: Fake-clock deterministic tests plus FakeVsReal Windows duration measurements.
- **NFR-WSL-SECURITY / security:** Config is administrator/service-identity readable only; logs are service-writable and operator-readable; no raw child stdout/stderr, token, secret, or environment value is persisted. Measurement: ACL fidelity test and adversarial record scan.
- **NFR-WSL-MAINTAINABILITY / maintainability:** SCM, WSL command execution, process inspection/signalling, HTTP health, filesystem/ACL, clock, resume notification, and record sink are injected boundaries. Measurement: Contract tests instantiate the pure controller with fakes and no Windows or WSL process.
- **NFR-WSL-COMPATIBILITY / compatibility:** Existing server SIGTERM, /healthz status semantics, and daemon flock remain authoritative; additive probe/stop commands preserve existing CLI behavior. Measurement: Baseline and target Go gateway/CLI tests plus FakeVsReal Linux behavior.

## UAC trace matrix

| UX must-fail | Requirement | Acceptance | Verification |
|---|---|---|---|
| UAC-001 | FR-WSL-001, FR-WSL-014 | AC-UAC-001 | V-UAC-001 |
| UAC-002 | FR-WSL-002 | AC-UAC-002 | V-UAC-002 |
| UAC-003 | FR-WSL-003 | AC-UAC-003 | V-UAC-003 |
| UAC-004 | FR-WSL-004 | AC-UAC-004 | V-UAC-004 |
| UAC-005 | FR-WSL-005 | AC-UAC-005 | V-UAC-005 |
| UAC-006 | FR-WSL-006 | AC-UAC-006 | V-UAC-006 |
| UAC-007 | FR-WSL-007 | AC-UAC-007 | V-UAC-007 |
| UAC-008 | FR-WSL-008 | AC-UAC-008 | V-UAC-008 |
| UAC-009 | FR-WSL-009 | AC-UAC-009 | V-UAC-009 |
| UAC-010 | FR-WSL-010 | AC-UAC-010 | V-UAC-010 |
| UAC-011 | FR-WSL-011 | AC-UAC-011 | V-UAC-011 |
| UAC-012 | FR-WSL-012 | AC-UAC-012 | V-UAC-012 |
| UAC-013 | FR-WSL-013 | AC-UAC-013 | V-UAC-013 |
| UAC-014 | FR-WSL-014 | AC-UAC-014 | V-UAC-014 |
| UAC-015 | FR-WSL-015 | AC-UAC-015 | V-UAC-015 |
| UAC-016 | FR-WSL-016, FR-WSL-018 | AC-UAC-016 | V-UAC-016 |
| UAC-017 | FR-WSL-017 | AC-UAC-017 | V-UAC-017 |

## Acceptance

### AC-UAC-001

- Given: no user is signed in and Automatic start runs
- When: the configured endpoint is queried
- Then: status ok is returned only after the full readiness sequence; daemon-unavailable fails
- Trace: FR-WSL-001, FR-WSL-014

### AC-UAC-002

- Given: the service has reached Ready
- When: time passes without a failure
- Then: the foreground unit remains healthy and retry count does not increase
- Trace: FR-WSL-002

### AC-UAC-003

- Given: the foreground unit is Ready
- When: the Linux server exits normally or abnormally
- Then: wsl.exe exits with it, the code is recorded, and no shell or server remains
- Trace: FR-WSL-003

### AC-UAC-004

- Given: a Ready generation suffers a transient crash
- When: the lifecycle controller observes the exit
- Then: Backoff delay and attempt increase, then status ok restores Ready
- Trace: FR-WSL-004

### AC-UAC-005

- Given: a distribution, executable, permission, or JSON setting is invalid
- When: the same typed cause reaches five consecutive failures
- Then: Failed is recorded with the setting and retries stop
- Trace: FR-WSL-005

### AC-UAC-006

- Given: a fingerprint-matched locked server is already healthy
- When: the service starts
- Then: probe/adopt occurs before spawn and already-running is recorded without a failure
- Trace: FR-WSL-006

### AC-UAC-007

- Given: SCM cannot start the process because its service identity or ACL is invalid
- When: SCM attempts Automatic start
- Then: the OS service-start record identifies credential/access failure and the dedicated file is not falsely claimed
- Trace: FR-WSL-007

### AC-UAC-008

- Given: an owned Ready generation may contain active frames
- When: SCM sends stop
- Then: SIGTERM reaches the existing drain path and Stopped follows unit exit within the graceful budget
- Trace: FR-WSL-008

### AC-UAC-009

- Given: another WSL distribution is active and graceful stop times out
- When: escalation runs
- Then: only the bound child then bound distribution are affected and the other distribution survives
- Trace: FR-WSL-009

### AC-UAC-010

- Given: only the target distribution exists and graceful stop times out
- When: escalation runs
- Then: the record names --terminate target and contains no --shutdown action
- Trace: FR-WSL-010

### AC-UAC-011

- Given: Windows resumes while the unit process survives
- When: resume is delivered
- Then: old health is invalidated and a timestamped fresh probe starts within ten seconds
- Trace: FR-WSL-011

### AC-UAC-012

- Given: a live process has failing health after resume
- When: the third consecutive probe fails
- Then: graceful restart begins without terminal Failed and successful health restores Ready
- Trace: FR-WSL-012

### AC-UAC-013

- Given: graceful recovery restart has exceeded 15 seconds
- When: health still fails
- Then: the bound distribution is terminated directly, then probe-before-spawn restarts
- Trace: FR-WSL-013

### AC-UAC-014

- Given: an update reboot finishes with no user signed in
- When: SCM Automatic start runs
- Then: the complete readiness sequence runs and only status ok produces Ready
- Trace: FR-WSL-014

### AC-UAC-015

- Given: WSL becomes unavailable without a reboot
- When: the supervisor observes exit or health loss
- Then: transient retries continue beyond five and recovery before ten restores Ready
- Trace: FR-WSL-015

### AC-UAC-016

- Given: a service was registered with snapshot generation G for identity A and distribution D, then wsl --set-default and the interactive identity/default are changed to A2/D2
- When: the same registered service is restarted, reaches Ready, is stopped, and the operator opens the machine-wide record
- Then: the record, direct launch argv, health target, supervisor-probe/supervisor-stop authority, and any terminate target all remain bound to generation G and A/D; no event or command uses A2/D2, while required fields remain readable and secrets/environment values remain absent
- Trace: FR-WSL-016, FR-WSL-018

### AC-UAC-017

- Given: a previously Ready configuration suffers transient failures
- When: five failures occur and the cause then clears
- Then: retry has not stopped, no Failed record exists, and the next healthy generation becomes Ready
- Trace: FR-WSL-017

## Non-goals

- GUI or Windows Shell client behavior
- WSL systemd or dual supervision
- Windows-native host, containers, cross-OS bridge redesign
- Credential broker, session, frame, driver, sandbox, or devcontainer ownership
