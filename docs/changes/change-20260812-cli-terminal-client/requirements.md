---
change: change-20260812-cli-terminal-client
role: requirements
functional_requirements:
- id: fr-001
  statement: The CLI shall not emit a session-termination request on any code path
    other than the explicit terminate command, including detach, Ctrl-C, terminal
    close, stdin EOF, catchable signals, uncatchable process kill, WebSocket close,
    and post-raw-mode auth or protocol failure.
  priority: must
- id: fr-002
  statement: While the CLI holds raw mode, the CLI shall restore the invoking terminal's
    echo and line editing before the process exits, suspends, or returns control to
    the shell, on every exit path.
  priority: must
- id: fr-003
  statement: The CLI shall not create, modify, or delete any file under the client
    configuration directory or under the daemon data directory.
  priority: must
- id: fr-004
  statement: When attach is invoked and stdin is not a terminal, the CLI shall not
    apply any terminal mode change to the controlling terminal, shall not open a surface
    subscription, and shall exit with the non-TTY failure class and a non-zero code.
  priority: must
- id: fr-005
  statement: While attached, the CLI shall deliver every stdin byte to the remote
    PTY in read order without local echo, forwarding Ctrl-C as 0x03 rather than taking
    it as a local interrupt.
  priority: must
- id: fr-006
  statement: The CLI shall not place a partial UTF-8 sequence in a surface input frame,
    and shall hold an incomplete trailing sequence until the remaining bytes are read
    or the input stream ends.
  priority: must
- id: fr-007
  statement: When a surface subscription is established, the CLI shall write the first
    output payload it receives to stdout before any subsequent payload, so the session's
    current screen appears without local reconstruction.
  priority: must
- id: fr-008
  statement: The CLI shall base64-decode surface output payloads to raw bytes and
    write those bytes to stdout unmodified, and shall not parse, render, or re-encode
    terminal escape sequences.
  priority: must
- id: fr-009
  statement: The CLI shall drop every surface output frame whose carried session id
    differs from the attached session id.
  priority: must
- id: fr-010
  statement: When the detach key byte is consumed, the CLI shall discard every subsequent
    input byte, shall discard the terminal input queue before restoring cooked mode,
    and shall not forward any of those bytes to the remote PTY.
  priority: must
- id: fr-011
  statement: The CLI shall treat exactly the configured detach byte as the detach
    trigger and shall forward every other byte, including the default 0x1D when it
    is not configured as the trigger, to the remote PTY.
  priority: must
- id: fr-012
  statement: The CLI shall declare its terminal geometry when the subscription is
    established and shall forward every subsequent local size change, clamping columns
    and rows into the accepted range before sending.
  priority: must
- id: fr-013
  statement: While attached, the CLI shall renew its terminal lifecycle lease on a
    schedule that cannot be delayed by stdout write backpressure or by surface output
    volume.
  priority: must
- id: fr-014
  statement: When the CLI leaves the attached state, it shall restore the terminal
    first and then present a reason that distinguishes explicit detach, transport
    loss, and server-side session end from one another.
  priority: must
- id: fr-015
  statement: The CLI shall list sessions from every enabled configured server, shall
    not filter by creator, and shall not itself remove or hide any session the server
    publishes, including sessions whose lifecycle state indicates they have ended.
  priority: must
- id: fr-016
  statement: When one or more configured servers are unreachable or protocol-incompatible,
    the CLI shall still present the sessions of the healthy servers and shall present
    the connection-side condition in a vocabulary disjoint from session lifecycle
    state.
  priority: must
- id: fr-017
  statement: The CLI shall derive every session state label and the precedence used
    to collapse simultaneous conditions from the checked-in shared status display
    matrix, and shall not introduce a CLI-specific state word.
  priority: must
- id: fr-018
  statement: The CLI shall render the session display name as the server-provided
    card title verbatim, with no client-side fallback, derivation, or truncation.
  priority: must
- id: fr-019
  statement: The CLI shall present the owning server on every session row, shall emit
    server identity and session identity as separate machine-readable fields, and
    shall not emit any single concatenated identifier as the only way to name a session.
  priority: must
- id: fr-020
  statement: The CLI shall resolve a session selector only under an explicitly declared
    selector kind, shall never fall back to another kind, and shall present mutually
    distinguishable candidates with a non-zero exit and no side effect when more than
    one session matches.
  priority: must
- id: fr-021
  statement: The CLI shall accept a server qualifier supplied independently of the
    selector string, accepting both the user-configured display name and the derived
    stable server identifier as its value.
  priority: must
- id: fr-022
  statement: When session creation is invoked without an explicit target server, the
    CLI shall create no session and shall exit with the usage-error class, regardless
    of how many servers are configured or reachable.
  priority: must
- id: fr-023
  statement: When explicit terminate is invoked, the CLI shall report success only
    after the server has acknowledged that the termination was applied, shall exit
    non-zero when the request was rejected, and shall leave the target's observable
    state unchanged in that case.
  priority: must
- id: fr-024
  statement: The CLI shall exit 0 on success, 1 on an internal fault, 2 on a usage
    error, 3 on not-found, 4 on an ambiguous selector, 5 on an authentication-or-compatibility
    failure, and 6 on a transport failure; shall reproduce the same output and exit
    code for the same input; and shall classify both a selector that matches only
    on an unreachable server and a capacity refusal of attach establishment as transport
    failure, separating the latter by its stated reason.
  priority: must
- id: fr-025
  statement: The CLI shall produce identical output content and exit codes for the
    same input whether or not stdout is a terminal, and shall not add a confirmation
    prompt or a candidate-selection prompt in any environment.
  priority: must
- id: fr-027
  statement: The CLI shall not place the bearer credential, a minted WebSocket ticket,
    or any URL carrying either into a failure reason, a diagnostic line, a log, or
    a process argument.
  priority: must
- id: fr-029
  statement: When a server-qualifier value matches configured entries of which every
    match is unusable, the CLI shall exit with the usage-error class naming each matching
    entry and the rule it violates; when more than one usable entry matches, the CLI
    shall present those entries as mutually distinguishable candidates and exit with
    the ambiguous-selector class without selecting one; and it shall report neither
    case as not-found nor as a transport failure.
  priority: must
- id: fr-031
  statement: The CLI shall present every configured server entry including entries
    in the configuration-invalid admission state, shall express configuration invalidity
    in a vocabulary disjoint from both server reachability and session lifecycle state,
    and shall name each such entry by its position in the entry list and by its display
    name as written together with the rule it violates.
  priority: must
- id: nfr-001
  statement: The CLI shall carry exactly one compiled protocol version, shall treat
    any difference, absence, or malformation as incompatible protocol, and shall not
    enter any operational REST handler or WebSocket application frame for that server.
  priority: must
- id: nfr-002
  statement: The CLI module shall not require, replace, or import the repository's
    services module, and this shall be enforced by the module graph rather than by
    review.
  priority: must
- id: nfr-003
  statement: The CLI shall resolve its configuration directory to the same location
    the existing client hosts use on the same platform, and shall accept the same
    explicit directory override.
  priority: must
- id: nfr-004
  statement: The CLI shall not persist, reconstruct, or replay terminal screen state;
    the subscribe-time snapshot delivered by the execution plane shall be the only
    recovery mechanism.
  priority: must
- id: nfr-005
  statement: The CLI module shall be registered in the repository's build, test, vet,
    lint, verification-profile, and protocol-version-projection entry points at introduction
    time.
  priority: must
- id: nfr-006
  statement: The CLI shall not present any indication of other clients attached to
    the same session.
  priority: must
- id: nfr-007
  statement: Server-qualified session identity construction and outbound frame serialization
    shall each have exactly one owning package inside the CLI module, enforced by
    a test that fails when another package performs either.
  priority: must
- id: nfr-008
  statement: The session payload fields the CLI reads shall be pinned by a checked-in
    golden vector that the server's own serialization is tested against, so that renaming
    or moving one of those fields fails a repository gate.
  priority: must
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Content

Canonical plan: `design-plan/design.md` + `design-plan/spine.yaml`. Upstream experience contract: `requirements-plan/ux.md` (19 decision points, all `answered` / `user_consultation` / `confirmed`). This member restates the EARS requirements, the non-functional requirements, and the acceptance criteria; the reasoning, the counterexamples and the measured evidence live in the plan.

The design plan is **shared** with `change-20260814-cli-prerequisite-contracts` and lives here, under this change. It is neither split nor copied: both changes reference the same anchors. This change `dependsOn` that one.

### How the shared plan's requirements were divided

A requirement belongs to the change whose deliverable **makes it true**, not to the change whose contracts happen to reference it. Requirements satisfied by the observable behaviour of the `clients/cli` executable are here; requirements satisfied by the execution plane, the public protocol surface, or the client configuration loaders are in `change-20260814-cli-prerequisite-contracts/requirements.md`. Four functional requirements and one non-functional requirement moved there — FR-026, FR-028, FR-030, FR-032 and NFR-009 — and are **not** restated below.

Three of those carry a trailing clause that binds the CLI rather than the plane. The clause is not promoted into a separate requirement here; it is discharged inside this change by the contract named in the dependency table below, so each sentence keeps exactly one owner.

### Functional requirements

| ID | Type | Statement |
|---|---|---|
| FR-001 | invariant | The CLI shall not emit a session-termination request on any code path other than the explicit terminate command, including detach, Ctrl-C, terminal close, stdin EOF, catchable signals, uncatchable process kill, WebSocket close, and post-raw-mode auth or protocol failure. |
| FR-002 | invariant | While the CLI holds raw mode, the CLI shall restore the invoking terminal's echo and line editing before the process exits, suspends, or returns control to the shell, on every exit path. |
| FR-003 | invariant | The CLI shall not create, modify, or delete any file under the client configuration directory or under the daemon data directory. |
| FR-004 | event | When attach is invoked and stdin is not a terminal, the CLI shall not apply any terminal mode change to the controlling terminal, shall not open a surface subscription, and shall exit with the non-TTY failure class and a non-zero code. |
| FR-005 | state | While attached, the CLI shall deliver every stdin byte to the remote PTY in read order without local echo, forwarding Ctrl-C as `0x03` rather than taking it as a local interrupt. |
| FR-006 | state | The CLI shall not place a partial UTF-8 sequence in a surface input frame, and shall hold an incomplete trailing sequence until the remaining bytes are read or the input stream ends. |
| FR-007 | event | When a surface subscription is established, the CLI shall write the first output payload it receives to stdout before any subsequent payload, so the session's current screen appears without local reconstruction. |
| FR-008 | state | The CLI shall base64-decode surface output payloads to raw bytes and write those bytes to stdout unmodified, and shall not parse, render, or re-encode terminal escape sequences. |
| FR-009 | invariant | The CLI shall drop every surface output frame whose carried session id differs from the attached session id. |
| FR-010 | event | When the detach key byte is consumed, the CLI shall discard every subsequent input byte, shall discard the terminal input queue before restoring cooked mode, and shall not forward any of those bytes to the remote PTY. |
| FR-011 | state | The CLI shall treat exactly the configured detach byte as the detach trigger and shall forward every other byte, including the default `0x1D` when it is not configured as the trigger, to the remote PTY. |
| FR-012 | state | The CLI shall declare its terminal geometry when the subscription is established and shall forward every subsequent local size change, clamping columns and rows into the accepted range before sending. |
| FR-013 | state | While attached, the CLI shall renew its terminal lifecycle lease on a schedule that cannot be delayed by stdout write backpressure or by surface output volume. |
| FR-014 | event | When the CLI leaves the attached state, it shall restore the terminal first and then present a reason that distinguishes explicit detach, transport loss, and server-side session end from one another. |
| FR-015 | ubiquitous | The CLI shall list sessions from every enabled configured server, shall not filter by creator, and shall not itself remove or hide any session the server publishes, including sessions whose lifecycle state indicates they have ended. |
| FR-016 | event | When one or more configured servers are unreachable or protocol-incompatible, the CLI shall still present the sessions of the healthy servers and shall present the connection-side condition in a vocabulary disjoint from session lifecycle state. |
| FR-017 | invariant | The CLI shall derive every session state label and the precedence used to collapse simultaneous conditions from the checked-in shared status display matrix, and shall not introduce a CLI-specific state word. |
| FR-018 | state | The CLI shall render the session display name as the server-provided card title verbatim, with no client-side fallback, derivation, or truncation. |
| FR-019 | invariant | The CLI shall present the owning server on every session row, shall emit server identity and session identity as separate machine-readable fields, and shall not emit any single concatenated identifier as the only way to name a session. |
| FR-020 | state | The CLI shall resolve a session selector only under an explicitly declared selector kind, shall never fall back to another kind, and shall present mutually distinguishable candidates with a non-zero exit and no side effect when more than one session matches. |
| FR-021 | state | The CLI shall accept a server qualifier supplied independently of the selector string, accepting both the user-configured display name and the derived stable server identifier as its value. |
| FR-022 | event | When session creation is invoked without an explicit target server, the CLI shall create no session and shall exit with the usage-error class, regardless of how many servers are configured or reachable. |
| FR-023 | event | When explicit terminate is invoked, the CLI shall report success only after the server has acknowledged that the termination was applied, shall exit non-zero when the request was rejected, and shall leave the target's observable state unchanged in that case. |
| FR-024 | ubiquitous | The CLI shall exit `0` on success, `1` on an internal fault, `2` on a usage error, `3` on not-found, `4` on an ambiguous selector, `5` on an authentication-or-compatibility failure, and `6` on a transport failure; shall reproduce the same output and exit code for the same input; and shall classify both a selector that matches only on an unreachable server and a capacity refusal of attach establishment as transport failure, separating the latter by the reason the execution plane publishes rather than by a client-authored string. |
| FR-025 | ubiquitous | The CLI shall produce identical output content and exit codes for the same input whether or not stdout is a terminal, and shall not add a confirmation prompt or a candidate-selection prompt in any environment. |
| FR-027 | invariant | The CLI shall not place the bearer credential, a minted WebSocket ticket, or any URL carrying either into a failure reason, a diagnostic line, a log, or a process argument. |
| FR-029 | event | When a server-qualifier value matches configured entries of which every match is unusable, the CLI shall exit with the usage-error class naming each matching entry and the rule it violates; when more than one usable entry matches, the CLI shall present those entries as mutually distinguishable candidates and exit with the ambiguous-selector class without selecting one; and it shall report neither case as not-found nor as a transport failure. |
| FR-031 | state | The CLI shall present every configured server entry including entries in the configuration-invalid admission state, shall express configuration invalidity in a vocabulary disjoint from both server reachability and session lifecycle state, and shall name each such entry by its position in the entry list and by its display name as written together with the rule it violates. |

FR-029 and FR-031 are the CLI-side consequences of the prerequisite change's FR-028 and FR-030 — how a qualifier that cannot be honoured reports itself, and how an unusable entry stays visible. The per-entry admission partition they read is defined once, in `design-plan/design.md` under the `fr-030` anchor and restated in `change-20260814-cli-prerequisite-contracts/requirements.md`.

### Requirements this change depends on but does not satisfy

Satisfied by `change-20260814-cli-prerequisite-contracts`. Their statements are not duplicated here; the dependency edge is what this table records.

| Prerequisite requirement | What depends on it here | Where the CLI-side clause is discharged |
|---|---|---|
| FR-026 — an ended session stays observable, and the transition is published | FR-014 (the session-end departure class), FR-015 (ended rows are not removed by the client), AC-disconnect-classes-distinct | "the CLI shall never reconstruct that observation from local memory" — `contract-explicit-terminate` and the `unit-lifecycle-commands` task boundary, under NFR-004 |
| FR-032 — every server-initiated close carries a published reason | FR-014 (transport-loss reason), FR-024 (transport class separated by its stated reason), `contract-exit-classification`, `contract-attach-transport-topology` | "the CLI shall derive both its transport failure classification and its presented reason from that value" — `contract-exit-classification`, verified by `ver-cli-maps-each-close-reason-to-its-exit-class` |
| FR-028 — a naming violation costs one server, not the host | FR-021 (a display name that can be addressed), FR-029 | — |
| FR-030 — the host starts whenever the entry list is readable | FR-029, FR-031, `contract-config-entry-admission` CLI half | — |
| NFR-009 — one loader per language, pinned by one corpus | FR-003 and NFR-003 (the loader that resolves and reads the configuration directory), FR-019 and FR-021 (one derived server identifier across hosts), FR-022 (`default_execution` is loaded but never read as a target) | — |

Unit-level, this is a single edge: `unit-transport-admission` depends on the prerequisite change's `unit-shared-goclient-config`. Everything else above is a contract dependency that does not gate the CLI build order.

### Non-functional requirements

| ID | Type | Statement |
|---|---|---|
| NFR-001 | compatibility | The CLI shall carry exactly one compiled protocol version, shall treat any difference, absence, or malformation as incompatible protocol, and shall not enter any operational REST handler or WebSocket application frame for that server. |
| NFR-002 | invariant | The CLI module shall not require, replace, or import the repository's services module, and this shall be enforced by the module graph rather than by review. |
| NFR-003 | portability | The CLI shall resolve its configuration directory to the same location the existing client hosts use on the same platform, and shall accept the same explicit directory override. |
| NFR-004 | invariant | The CLI shall not persist, reconstruct, or replay terminal screen state; the subscribe-time snapshot delivered by the execution plane shall be the only recovery mechanism. |
| NFR-005 | maintainability | The CLI module shall be registered in the repository's build, test, vet, lint, verification-profile, and protocol-version-projection entry points at introduction time. |
| NFR-006 | invariant | The CLI shall not present any indication of other clients attached to the same session. |
| NFR-007 | invariant | Server-qualified session identity construction and outbound frame serialization shall each have exactly one owning package inside the CLI module, enforced by a test that fails when another package performs either. |
| NFR-008 | compatibility | The session payload fields the CLI reads shall be pinned by a checked-in golden vector that the server's own serialization is tested against, so that renaming or moving one of those fields fails a repository gate. |

### Acceptance criteria

Criteria whose primary requirement is owned here. Five further criteria of the shared plan — AC-ended-session-remains-observable, AC-transport-close-is-typed, AC-config-parity-across-languages, AC-config-entry-admission and AC-server-naming-and-qualifier-outcomes — are owned by `change-20260814-cli-prerequisite-contracts`; this change contributes the CLI-side verifications listed at the end of `verification.md`.

| ID | Criteria | Requirements |
|---|---|---|
| AC-attach-current-screen | Attaching to a running session shows the content that was already on its screen and continues from there, and output from any other subscribed session never appears. | FR-007, FR-008, FR-009, NFR-004 |
| AC-input-roundtrip | A known input containing multi-byte characters produces the remote side's own response even when reads split it, and an interrupt reaches the remote operation without ending the CLI. | FR-005, FR-006 |
| AC-detach-restores-and-discards | Characters typed immediately after the detach key appear neither in the returning shell's input line nor on reattach, echo and line editing are restored, and the session is still running afterwards. | FR-010, FR-011, FR-002, FR-001 |
| AC-restore-on-every-exit | A pre-subscription failure, a catchable terminating signal, suspend and resume, end of input, and a server disconnect all leave the shell with working echo and line editing, and the pre-subscription failure leaves the terminal settings unchanged throughout; after the catchable terminating signal the session that was attached is still listed as running. | FR-002, FR-004, FR-001 |
| AC-lease-survives-backpressure | A stalled stdout reader for longer than the lease lifetime does not lose the subscription, and output resumes when the reader resumes. | FR-013 |
| AC-non-tty-fail-fast | An attach with a non-terminal stdin exits non-zero without changing the invoking terminal, and the target session is unchanged. | FR-004, FR-024 |
| AC-disconnect-classes-distinct | Explicit detach, lost connection, and server-side session end are told apart from what remains on screen after returning to the shell. The third class rests on the accepted terminal-state retention publishing the transition; within the second class, the reason shown is the one the execution plane published rather than a client guess, so a capacity refusal, a going-away shutdown and an unexplained drop do not read alike. | FR-014, FR-024, FR-026, FR-032 |
| AC-resize-reaches-remote | Resizing during a full-screen redraw produces a redraw at the new dimensions, and no size value is silently dropped by the server. | FR-012 |
| AC-discovery-parity | Each row pairs a session with its owning server, the display name and state label match the graphical clients, the name is never truncated, and rows are not filtered by creator nor removed by the client. | FR-015, FR-017, FR-018, FR-019 |
| AC-partial-outage | With one server unreachable or protocol-incompatible the list is not empty, and machine-readable output alone distinguishes an unreachable server from a server with no sessions. | FR-016, FR-019, NFR-001 |
| AC-selector-and-failures | Resolution follows the declared selector kind, ambiguity yields distinguishable candidates with a non-zero exit and no side effect, and the failure classes carry the fixed exit codes 2 through 6 reproducibly with an unreachable-only match classified as transport. | FR-020, FR-021, FR-024, FR-025 |
| AC-create-outcomes-distinct | Creation failure, creation success with attach failure, and full success are three different observations; a creation with `--no-attach` is a fourth, attempting no attach and exiting zero while still reporting an identifier that appears as a running row on the named server; a missing target server creates nothing and ends as a usage error; an unresolvable agent argument creates no row; and the attach that follows creation lands on the new session's freshly started screen. | FR-022, FR-024 |
| AC-terminate-is-explicit-only | Only the explicitly terminated session ends, sessions that merely experienced detach, interrupt, terminal close, disconnect, or an uncatchable kill keep running, a rejected termination exits non-zero without changing the target, and the success report claims only the acknowledgement it received. | FR-001, FR-023 |
| AC-boundary-and-gates | The CLI module has no dependency edge to the services module, is covered by build, test, vet, lint, verification profile, and protocol-version projection checks, and neither its runtime nor its tests write into a user configuration directory. | NFR-002, NFR-003, NFR-005, NFR-001, FR-003 |
| AC-concurrent-attach | An existing graphical attach neither blocks nor gates the CLI attach and nothing in the CLI output indicates that other clients are attached; when the shared per-connection surface capacity is already exhausted the attach fails as a transport failure whose reason is the capacity code the execution plane published, which is a capacity limit of the execution plane rather than an ownership policy. | NFR-006, FR-007, FR-024, FR-032 |
| AC-credential-never-echoed | An authentication failure, a protocol mismatch, and a socket dial failure each produce a reason on stderr that contains neither the bearer credential, nor the minted ticket, nor a URL carrying either, and neither value appears in the process arguments. | FR-027, FR-003 |
| AC-wire-shape-and-identity-pinned | Renaming or moving a session payload field the CLI reads fails a repository gate on the server side, and a second package constructing a server-qualified identity or serialising an outbound frame fails a gate on the client side. | NFR-008, NFR-007, FR-018 |

Two rows reference requirements owned by the prerequisite change: AC-disconnect-classes-distinct cites FR-026 and FR-032, and AC-concurrent-attach cites FR-032. Those references are dependencies, not obligations restated here; AC-boundary-and-gates likewise takes `ver-config-never-written-and-directory-per-platform` from the prerequisite change.

### Upstream UX deltas that changed a requirement

These are the places where measurement against this repository changed what the upstream experience contract can be satisfied by. None of them reopens a settled decision point. Four of them produced the prerequisite requirements now carried by `change-20260814-cli-prerequisite-contracts`; they are summarised there and named here only where a CLI requirement changed shape as a result.

- **Ended sessions (UAC-007, UAC-029).** DP-LIST-LIFECYCLE-SCOPE settled that ended sessions appear in the listing. Measurement shows the execution plane deletes the session record on three of four termination paths, so no client-side implementation can satisfy the observation. **FR-015 was narrowed to the client's own obligation** — remove nothing the server publishes — and the plane-side half became the prerequisite change's FR-026.
- **Server-side session end as a departure class (UAC-018, UAC-019).** The frame that would attribute a session end on the lifecycle socket is unmeasured, so the Phase 0 observation still runs in `unit-phase0-contract-pin` — but a negative result no longer degrades the contract, because the accepted retention prerequisite publishes the transition either way.
- **Transport loss as an informative class (UAC-018, UAC-019 again).** The public contract carries no typed reason for a connection ending. **FR-014 and FR-024 now read the published reason** rather than a client-authored string; the vocabulary itself is the prerequisite change's FR-032.
- **`--server` by display name (UAC-032, UAC-025).** The upstream Given assumes a naming prerequisite that does not hold today. The enforcement point is the prerequisite change's FR-028 and FR-030; on this side it **restated FR-029's firing condition** and added **FR-031** so an unusable entry stays visible in the CLI's own output.
- **Configuration parity across client languages.** The three loaders of the shared `servers.json` contract disagree on unknown fields, on URL credentials — which splits the derived server identity between hosts — and on the Windows configuration directory. **FR-019's machine-readable server identity depends on that being closed**; the closing itself is the prerequisite change's NFR-009.
- **Model column.** The upstream exclusion cites unavailability of a model field. That premise is measured false (`view.model` carries a JSON tag on the same payload the CLI already reads, and the shipping SPA reads it). Design takes no position on the column; the corrected fact is returned to requirements as `open-model-column-premise-falsified`.
- **Selector kinds.** Reduced to full identifier and identifier prefix. This is the invalidation behaviour the upstream plan itself recorded for `assumption-stable-name-field-unknown`, fired by measurement, not a design override.
