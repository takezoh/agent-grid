# Resilient server reconnect and fleet health — UX requirements

This specify plan fixes the observable recovery and diagnosis experience for the shared browser and Workspace UI. It does not require preservation of the old WebSocket, output replay, a user-configurable retry policy, a second notification surface, or persistence of runtime health.

## Goals

<!-- anchor: goal-recover-work -->
### Continue work after a server-side disconnect

**Actor.** A person using Agent Grid through the shared UI, with or without a selected session.

**Context.** The selected server is unavailable at startup or an established connection closes unexpectedly.

**Desired outcome.** The client attempts recovery, distinguishes progress from stopped recovery, and provides a reload-free Retry path after attempts stop.

<!-- anchor: goal-recover-work.success-observation -->
#### Success observation

The person sees recovery progress, can perceive when it has stopped, can operate Retry with pointer or keyboard, and sees recovery confirmed only after a daemon-derived hello.

<!-- anchor: goal-diagnose-fleet -->
### Identify the configured server that needs attention

**Actor.** A person using one or more configured Agent Grid servers in the browser or Workspace host.

**Context.** Enabled servers can be healthy, unavailable, or protocol-incompatible while disabled servers remain configured but intentionally excluded.

**Desired outcome.** The person can distinguish aggregate degradation from active-connection recovery and can reach server-specific current health and an available reason from either host.

<!-- anchor: goal-diagnose-fleet.success-observation -->
#### Success observation

The status bar summarizes enabled-server degradation, its Connections action reaches per-server diagnosis in either host, and disabled servers neither inflate the aggregate denominator nor appear failed.

## Reference UX and legacy boundary

The adopted pattern is one persistent compact status surface with progressive disclosure: active-connection recovery and enabled-fleet summary in the status bar, with a visible Connections action leading to per-server diagnosis. A header, sidebar, toast, or modal does not duplicate active recovery state or Retry. WebSocket open proves gateway reachability only. ADR 0011's typed close remains intact; this plan creates a new connection lifecycle instead of preserving and re-subscribing the old WebSocket. ADR 0012's gateway-to-daemon supervisor remains unbounded and is not constrained by the finite shared-UI lifecycle.

Replaced legacy observations are: reload is the only recovery after daemon restart; exhausted client recovery has no Retry; active connection failure may have multiple shell owners; and enabled-server failure has neither an aggregate status-bar condition nor server-specific current health and reason. Every acceptance scenario below is `must-fail` against the applicable replaced behavior.

## Experience dimensions

- **connection-origin:** initial unavailability; unexpected established-connection close.
- **recovery-phase:** client automatic attempts; automatic attempts exhausted; manual Retry lifecycle; daemon-confirmed recovery.
- **recovery-proof:** gateway socket open without hello; daemon-derived hello received.
- **fleet-condition:** all enabled healthy; partially unavailable; partially incompatible; all enabled unavailable; disabled present; health refresh unavailable.
- **access-modality:** pointer; keyboard; assistive-technology status text.
- **host:** browser; Workspace.
- **session-context:** session selected; no session selected.

## Experience contracts

<!-- anchor: exp-recovery-continuity -->
### Recovery continuity

The persistent status bar owns the active connection lifecycle. Initial connection failure and unexpected close both enter client-side automatic recovery. One lifecycle performs no more than 16 automatic retry attempts after the initiating attempt fails. Success requires a daemon-derived hello. Exhaustion presents connection closed with Retry; Retry starts a fresh lifecycle and remains available again if that lifecycle also exhausts.

<!-- anchor: exp-recovery-continuity.interaction-rules -->
#### Interaction rules

- Initial unavailability presents connecting/reconnecting progress without requiring an existing hello.
- Unexpected close presents reconnecting while automatic attempts continue.
- Retry is named `Retry`, is reachable with no selected session, and has equivalent pointer and keyboard outcomes.
- While an attempt is active, repeated activation cannot create visibly competing attempts and Retry is not offered as a competing action.
- Status transitions are perceivable through status text exposed to assistive technology; changing status does not take focus away from the user's current control. If Retry held focus when a new attempt started, focus returns to a stable shell location from which the changed connection status is perceivable.

<!-- anchor: exp-recovery-continuity.invariants -->
#### Invariants

- The recovered or nominal connection state is never shown before daemon-derived hello, even if a WebSocket remains open long enough to receive hello later.
- Each shared-UI recovery lifecycle is finite at 16 automatic retries; this limit does not apply to the gateway's unbounded daemon-client supervisor in ADR 0012.
- Exhaustion always has a reload-free Retry path.
- Active recovery state and Retry appear in exactly one persistent shell surface: the status bar. Connections is diagnosis, not a second owner of Retry.
- No promise is made about replaying missed output, reconciling sequence gaps, restoring scrollback, or preserving the old WebSocket.

<!-- anchor: exp-fleet-health-legibility -->
### Fleet health legibility

The status bar presents aggregate runtime health over enabled servers. When degraded, a visible Connections action reaches a server-specific health surface in both browser and Workspace hosts. Each configured server is identified as Connected, Unavailable, Incompatible protocol, Disabled, Checking, or Health unavailable as applicable, with an available failure reason. Health observation never becomes saved configuration truth.

<!-- anchor: exp-fleet-health-legibility.interaction-rules -->
#### Interaction rules

- Partial degradation states affected counts over the enabled-server total and distinguishes unavailable from incompatible outcomes.
- If all enabled servers are unavailable, the summary says all enabled servers are unavailable rather than using partial-failure wording.
- The degraded summary exposes a consistently named Connections action operable by pointer and keyboard in either host.
- If current health cannot be refreshed, an enabled server reads Health unavailable; a retained prior result is explicitly labeled stale. Configuration remains editable.
- When every enabled server becomes healthy, the aggregate warning and degraded Connections action state clear without changing the independently observed active-connection state.

<!-- anchor: exp-fleet-health-legibility.invariants -->
#### Invariants

- Disabled servers are omitted from both numerator and denominator and read Disabled.
- Aggregate status never substitutes for server identity, current-state qualification, and available reason.
- Browser and Workspace expose the same fleet meaning and diagnosis reachability; only delivery transport may differ.
- Runtime health is not written to `servers.json` or another persisted configuration source.

## Primary flows and acceptance scenarios

<!-- anchor: f-001 -->
### F-001 — Automatic recovery

**Entry:** the initial connection cannot complete, or a hello-confirmed connection closes unexpectedly. **Progress:** the status bar reports connecting/reconnecting while up to 16 automatic retries proceed. **Success:** a daemon-derived hello changes the presentation to the post-recovery state. **Failure:** the sixteenth unsuccessful retry ends progress at connection closed with Retry. **Recovery:** Retry enters F-002 without reload.

<!-- anchor: uac-001 -->
#### UAC-001 — Unexpected disconnect recovers automatically
<!-- anchor: uac-001.given -->
##### Given
The client has received hello and presents the connection as available.
<!-- anchor: uac-001.when -->
##### When
The connection closes unexpectedly and a later automatic attempt receives a new daemon-derived hello.
<!-- anchor: uac-001.then -->
##### Then
The status bar presents reconnecting throughout the interruption, never presents the recovered state before hello, and presents the post-recovery state after hello without Retry or reload.
<!-- anchor: uac-001.counterexample -->
##### Counterexample
An implementation that shows Connected on WebSocket open and later receives hello fails because it exposes a false recovered interval before hello.

<!-- anchor: uac-002 -->
#### UAC-002 — Socket open without hello is not recovery
<!-- anchor: uac-002.given -->
##### Given
The client is automatically recovering from initial failure or unexpected close.
<!-- anchor: uac-002.when -->
##### When
A gateway WebSocket opens but no daemon-derived hello has arrived, whether the socket remains open or later closes.
<!-- anchor: uac-002.then -->
##### Then
The status text remains connecting/reconnecting or advances to closed after exhaustion; it never presents daemon recovery or the nominal state before hello.
<!-- anchor: uac-002.counterexample -->
##### Counterexample
An on-open Connected state fails even if the same socket later receives hello, because the user observes success before its required proof.

<!-- anchor: uac-010 -->
#### UAC-010 — Initial unavailability reaches a recoverable stop
<!-- anchor: uac-010.given -->
##### Given
The application opens before any hello and the selected server remains unavailable.
<!-- anchor: uac-010.when -->
##### When
The initiating attempt and all 16 automatic retries fail without hello.
<!-- anchor: uac-010.then -->
##### Then
The status bar first exposes connecting/reconnecting progress, then connection closed with Retry, including when no session is selected.
<!-- anchor: uac-010.counterexample -->
##### Counterexample
An implementation that only reconnects sockets which were once healthy fails because startup remains indefinitely connecting or ends without Retry.

<!-- anchor: uac-005 -->
#### UAC-005 — The client retry bound is exact
<!-- anchor: uac-005.given -->
##### Given
Every automatic client retry fails without hello.
<!-- anchor: uac-005.when -->
##### When
The recovery lifecycle completes its sixteenth automatic retry after the initiating failure.
<!-- anchor: uac-005.then -->
##### Then
The status bar stops reconnecting and presents connection closed with Retry; it neither stops after fewer than 16 retries nor starts a seventeenth automatic retry.
<!-- anchor: uac-005.counterexample -->
##### Counterexample
A one-retry implementation and an unbounded retry loop both fail at the user-visible stopping boundary required by this scenario.

<!-- anchor: f-002 -->
### F-002 — Manual recovery after automatic exhaustion

**Entry:** connection closed and Retry are visible. **Progress:** one pointer or keyboard activation starts a fresh lifecycle and exposes connecting/reconnecting status. **Success:** hello presents the post-recovery state. **Failure:** 16 unsuccessful automatic retries return to closed with Retry. **Recovery:** Retry can start another lifecycle without reload; status changes remain perceivable and focus remains usable.

<!-- anchor: uac-003 -->
#### UAC-003 — Retry and hello complete recovery
<!-- anchor: uac-003.given -->
##### Given
Automatic recovery has stopped and connection closed with Retry is visible.
<!-- anchor: uac-003.when -->
##### When
The user activates Retry once and the fresh lifecycle later receives daemon-derived hello.
<!-- anchor: uac-003.then -->
##### Then
The status changes to progress, no competing Retry is offered during the attempt, and the post-recovery state appears only after hello without reload.
<!-- anchor: uac-003.counterexample -->
##### Counterexample
A Retry button that leaves accounting exhausted or only reloads the page fails because activation does not begin a fresh observable lifecycle.

<!-- anchor: uac-011 -->
#### UAC-011 — A failed manual lifecycle remains recoverable
<!-- anchor: uac-011.given -->
##### Given
The user has started a fresh lifecycle with Retry.
<!-- anchor: uac-011.when -->
##### When
Its 16 automatic retries all fail without hello.
<!-- anchor: uac-011.then -->
##### Then
Progress ends at connection closed with Retry, and activating Retry again can begin another fresh lifecycle without reload.
<!-- anchor: uac-011.counterexample -->
##### Counterexample
A manual Retry that remains indefinitely reconnecting or becomes permanently disabled after one failed lifecycle fails because recovery has no finite reusable exit.

<!-- anchor: uac-004 -->
#### UAC-004 — Retry is reachable in every active modality
<!-- anchor: uac-004.given -->
##### Given
No session is selected and the status bar presents connection closed with Retry.
<!-- anchor: uac-004.when -->
##### When
A pointer user selects Retry, a keyboard user reaches and activates Retry, or an assistive-technology user perceives the closed state and Retry name before activation.
<!-- anchor: uac-004.then -->
##### Then
Each modality starts the same recovery progress; the transition is exposed as status text, and focus remains at or moves to a stable shell location from which the updated status is perceivable.
<!-- anchor: uac-004.counterexample -->
##### Counterexample
A session-only status bar or visual-text-only transition fails because at least one required user cannot perceive or operate the sole recovery path.

<!-- anchor: uac-006 -->
#### UAC-006 — Active recovery has one presentation owner
<!-- anchor: uac-006.given -->
##### Given
The active connection is reconnecting or closed.
<!-- anchor: uac-006.when -->
##### When
The user inspects the shell and its status through visual or assistive-technology output.
<!-- anchor: uac-006.then -->
##### Then
Active recovery state and Retry appear only in the persistent status bar, not in a header, sidebar, toast, modal, or another shell surface; Connections contains diagnosis but no duplicate Retry.
<!-- anchor: uac-006.counterexample -->
##### Counterexample
Keeping the status-bar Retry while adding the same action to a sidebar fails even if the former header banner was removed.

<!-- anchor: f-003 -->
### F-003 — Diagnose and clear fleet degradation

**Entry:** the status bar reports enabled-fleet degradation. **Progress:** pointer or keyboard activation of Connections opens per-server health in the current host. **Success:** the affected server and available reason are identifiable, or all-enabled recovery clears aggregate degradation. **Failure:** a refresh failure is labeled Health unavailable and any retained result is stale while editing remains available. **Recovery:** a later current observation replaces stale/unavailable presentation; independent active-connection recovery does not hide remaining fleet degradation.

<!-- anchor: uac-007 -->
#### UAC-007 — Partial fleet summary excludes disabled servers
<!-- anchor: uac-007.given -->
##### Given
Enabled servers include healthy and unavailable or incompatible outcomes, and at least one configured server is disabled.
<!-- anchor: uac-007.when -->
##### When
The status bar presents fleet health.
<!-- anchor: uac-007.then -->
##### Then
It reports affected counts over enabled servers, distinguishes unavailable from incompatible outcomes, omits disabled entries from both counts, and exposes Connections.
<!-- anchor: uac-007.counterexample -->
##### Counterexample
Counting all configured entries fails because a deliberately disabled server changes the displayed ratio.

<!-- anchor: uac-012 -->
#### UAC-012 — Total enabled-server outage is explicit
<!-- anchor: uac-012.given -->
##### Given
Every enabled server is unavailable and disabled servers may also be configured.
<!-- anchor: uac-012.when -->
##### When
The status bar summarizes fleet health.
<!-- anchor: uac-012.then -->
##### Then
It states that all enabled servers are unavailable and does not describe the condition as a partial ratio; Connections identifies every configured server.
<!-- anchor: uac-012.counterexample -->
##### Counterexample
Displaying `N/N servers unavailable` as ordinary partial degradation fails because it does not distinguish total enabled-fleet outage.

<!-- anchor: uac-008 -->
#### UAC-008 — Diagnosis is reachable in browser and Workspace
<!-- anchor: uac-008.given -->
##### Given
The status bar reports fleet degradation in either the browser or Workspace host and an affected server has an available reason.
<!-- anchor: uac-008.when -->
##### When
The user activates the visible Connections action by pointer or keyboard.
<!-- anchor: uac-008.then -->
##### Then
The current host opens its Connections surface, where each configured server is named with its current qualified state and the affected server's available reason; configuration remains editable.
<!-- anchor: uac-008.counterexample -->
##### Counterexample
A Workspace-only settings route fails because a browser user can see degradation but cannot identify the affected server and reason.

<!-- anchor: uac-013 -->
#### UAC-013 — Health refresh failure cannot masquerade as current health
<!-- anchor: uac-013.given -->
##### Given
An enabled server previously appeared Connected and its next current-health refresh cannot complete.
<!-- anchor: uac-013.when -->
##### When
Connections presents the server after the failed refresh.
<!-- anchor: uac-013.then -->
##### Then
The server reads Health unavailable; any retained Connected result is explicitly labeled stale, Disabled entries remain Disabled, and configuration remains editable.
<!-- anchor: uac-013.counterexample -->
##### Counterexample
Leaving the old Connected label unqualified fails because the user cannot distinguish stale evidence from current health.

<!-- anchor: uac-009 -->
#### UAC-009 — Active recovery does not hide fleet degradation
<!-- anchor: uac-009.given -->
##### Given
The active connection receives hello while another enabled server remains unavailable or incompatible.
<!-- anchor: uac-009.when -->
##### When
The status bar updates after active-connection recovery.
<!-- anchor: uac-009.then -->
##### Then
The active reconnecting indication clears, but the independent enabled-fleet degradation and Connections action remain visible.
<!-- anchor: uac-009.counterexample -->
##### Counterexample
Replacing the whole status with Connected on active hello fails because another enabled server's current failure disappears.

<!-- anchor: uac-014 -->
#### UAC-014 — Fleet recovery clears aggregate degradation
<!-- anchor: uac-014.given -->
##### Given
The status bar presents fleet degradation and later every enabled server has a current healthy observation.
<!-- anchor: uac-014.when -->
##### When
The fleet presentation refreshes.
<!-- anchor: uac-014.then -->
##### Then
The aggregate degradation clears and the nominal fleet state appears, while the active connection continues to show its independently correct state.
<!-- anchor: uac-014.counterexample -->
##### Counterexample
A warning latched after all enabled servers recover fails because the user never observes the successful fleet exit.

## Closed product decisions

<!-- anchor: dp-recovery-policy -->
### Recovery policy

Selected `OPT-SIXTEEN-AUTO-THEN-MANUAL`: after the initiating attempt fails, a shared-UI lifecycle makes at most 16 automatic retries, then exposes Retry. This preserves the repository's existing `MAX_ATTEMPTS = 16` behavior while closing the observable boundary. It does not constrain ADR 0012's gateway supervisor. `OPT-UNBOUNDED-AUTO` and `OPT-ONE-AUTO-THEN-MANUAL` are rejected.

<!-- anchor: dp-recovery-proof -->
### Recovery proof

Selected `OPT-DAEMON-HELLO`; `OPT-SOCKET-OPEN` is rejected because it proves only gateway reachability.

<!-- anchor: dp-status-owner -->
### Connection-status owner

Selected `OPT-STATUS-BAR-ONLY`; duplicate shell recovery surfaces are rejected.

<!-- anchor: dp-fleet-denominator -->
### Fleet denominator

Selected `OPT-ENABLED-SERVERS`; disabled configuration is not a health outcome.

<!-- anchor: dp-health-boundary -->
### Health data boundary

Selected `OPT-RUNTIME-ONLY`; stale runtime evidence is never persisted as configuration truth.

<!-- anchor: dp-diagnosis-entry -->
### Diagnosis entry

Selected `OPT-STATUS-BAR-CONNECTIONS-BOTH-HOSTS`: degraded fleet status exposes a visible Connections action that opens server-specific diagnosis in browser and Workspace. `OPT-WORKSPACE-SETTINGS-ONLY` is rejected because the architectural fleet contract forbids browser-only loss of server-qualified semantics.

## Design and verification handoffs

### Retry reachability

Design must preserve the status bar with no selected session and implement focus continuity without creating another recovery surface. Verification covers pointer, keyboard, assistive-technology status perception, exact 16-retry exhaustion, and repeatable manual lifecycles.

### Status announcement semantics

Design chooses the least noisy platform-native semantics that make connecting/reconnecting, closed-with-Retry, and hello-confirmed recovery perceivable. This is implementation guidance, not permission to add toast or modal notifications.

### Runtime health boundary

Design keeps server identity stable across aggregate and detail views, represents current-versus-stale evidence, and keeps health outside configuration persistence.

### Shared Connections surface

Design chooses host-appropriate delivery for opening Connections while preserving the same action name, server-qualified contents, pointer/keyboard reachability, and outcome in browser and Workspace.

## Assumptions and exclusions

The assembled shell must validate status-bar reachability without a selected session, status-text perception and focus continuity, and the browser Connections delivery. Failure requires repairing shell composition or delivery; it does not permit weakening these scenarios. User-configurable retry bounds, background notifications, transparent old-WebSocket re-subscription, missed-output replay, scrollback restoration, sequence-gap reconciliation, and Windows Shell tray-health integration remain excluded.

## Resolved issues and critique resolutions

All 12 reviewed issues are materially resolved and traced to `critique_resolutions` in the spine:

1. Initial unavailability is covered from entry through closed-with-Retry by UAC-010.
2. Premature socket-open success is rejected whether the socket stays open or closes by UAC-001 and UAC-002.
3. Manual Retry failure has a finite reusable exit in UAC-011.
4. The stopping boundary is fixed at 16 automatic client retries by UAC-005 and DP-RECOVERY-POLICY.
5. Assistive-technology perception and focus continuity are required by UAC-004 and UAC-006.
6. Browser and Workspace diagnosis reachability is fixed by UAC-008 and DP-DIAGNOSIS-ENTRY.
7. Refresh failure and stale evidence are qualified by UAC-013.
8. All-enabled healthy recovery is observed by UAC-014.
9. All-enabled unavailable wording is observed by UAC-012.
10. Every duplicate shell-level recovery owner is rejected by UAC-006.
11. Fleet legacy behavior is stated as concrete missing aggregate and per-server observations.
12. The finite shared-UI lifecycle is explicitly separated from ADR 0012's unbounded gateway supervisor.

## Open Questions

None. Presentation details may vary only when they preserve the entry, progress, success, failure, recovery, modality, host, and compatibility observations above.
