---
change: change-20260812-cli-terminal-client
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Content

Tier scheme per `docs/note/note-20260624-agent-testing.md`. T0 = pure unit over decoded values; T1 = component or scenario against a fake counterparty; T2 = enforcement, gate, and cross-module conformance. T3 items are the residue that only real hardware or a real backend can show and are listed separately at the end.

Every id, tier and command below is copied from the shared plan (`design-plan/spine.yaml`, `contracts[].verification`). No command is rewritten here. Every check is discriminating: it names an implementation it fails. A green run of the whole set is what "the CLI observes what the upstream experience contract requires" means mechanically.

These sixty-three are the CLI half. The other nineteen verifications of the shared plan exercise the execution plane, the gateway and the client configuration loaders; they are in `../change-20260814-cli-prerequisite-contracts/verification.md`.

### Attach data path

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-attach-topology-snapshot-two-clients | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestConcurrentAttachEachReceivesSnapshot` | uses `GET /ws?session=`, whose shared empty `SubscriberID` denies the second client its snapshot and drops both on one detach |
| ver-attach-capacity-refusal-is-transport-with-the-published-reason | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestCapacityRefusalExitsTransportWithThePublishedReason` | reports a `resource_exhausted` refusal as not-found, as a clean detach, or with a client-authored sentence in place of the code the plane published; or enters raw mode before the refusal is known |
| ver-attach-never-reports-accepted-as-established | T0 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestNonTerminalOutcomesAreNotEstablished` | treats `accepted` or `waiting` as an established subscription |
| ver-attach-topology-no-per-session-route | T2 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestAttachNeverUsesPerSessionRoute` | reintroduces the per-session attach route anywhere in the module |
| ver-first-payload-written-first | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestSubscribeSnapshotWrittenBeforeLiveOutput` | reorders or reconstructs the initial screen |
| ver-output-bytes-are-byte-faithful | T0 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestSurfaceOutputDecodesToExactBytes` | re-marshals the payload through a Go string and turns non-UTF-8 bytes into U+FFFD |
| ver-foreign-session-output-dropped | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestForeignSessionOutputNeverReachesStdout` | skips the element-3 session check on the multiplexed socket |
| ver-utf8-sequence-never-split | T0 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestInputFramesNeverSplitUTF8` | frames a partial multi-byte sequence, which the JSON string payload silently collapses |
| ver-input-fuzz-order-and-bytes | T2 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run FuzzInputFraming` | preserves boundaries only for the split points someone thought of |
| ver-ctrl-c-forwarded-as-0x03 | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestCtrlCForwardedNotTrapped` | takes Ctrl-C as a local interrupt |
| ver-lease-renews-under-stdout-backpressure | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestLeaseRenewsWhileStdoutBlocked` | sequences lease renewal behind a stdout write, losing the subscription after the 12-second TTL |
| ver-lease-revision-monotonic | T0 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestLeaseRevisionIsMonotonic` | reuses or decrements `clientRevision` |
| ver-initial-geometry-is-first-frame | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestGeometryDeclaredAtSubscribe` | omits geometry from the subscription, which the server closes as `invalid-geometry` |
| ver-geometry-clamped-into-range | T0 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestGeometryClampedBeforeSend` | sends an out-of-range size, which the server discards with only a warning |
| ver-resize-burst-coalesces-to-latest | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestResizeBurstCoalescesToLatestValue` | queues sizes and applies a stale one last |

### Terminal discipline

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-restore-on-every-exit-path | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestTerminalRestoredOnEveryExitPath` | restores on some departures and not others, including one that exits on SIGINT or SIGTERM without passing through restore |
| ver-pre-subscribe-failure-leaves-terminal-untouched | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestPreSubscribeFailureNeverTouchesTerminal` | enters raw mode at start-up and relies on `defer` |
| ver-restore-is-idempotent | T0 | `cd clients/cli && GOWORK=off go test ./internal/tty/ -run TestRestoreIsIdempotent` | corrupts the terminal on a double restore |
| ver-non-tty-stdin-fails-without-mode-change | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestNonTTYStdinFailsWithoutModeChange` | touches the controlling terminal before discovering stdin is a pipe |
| ver-post-detach-bytes-surface-nowhere | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestBytesAfterDetachKeySurfaceNowhere` | forwards or buffers post-detach keystrokes |
| ver-input-queue-flushed-before-restore | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestInputQueueFlushedBeforeRestore` | stops reading without discarding the tty queue, letting the shell consume the leftovers |
| ver-configured-detach-byte-only | T0 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestOnlyConfiguredDetachByteTriggersDetach` | keeps `0x1D` as a trigger after it was reconfigured |
| ver-detach-sends-no-terminate | T2 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestNoTerminateRequestOnAnyAttachShutdownPath` | emits a termination on any attach shutdown path |
| ver-tty-seam-drives-all-exit-paths | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestFakeTTYDrivesEveryExitPath` | leaves an exit path reachable only with a real terminal |

### Discovery, selection, and failure classification

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-rows-carry-owning-server | T0 | `cd clients/cli && GOWORK=off go test ./internal/view/ -run TestEveryRowCarriesItsServer` | prints a session without its server |
| ver-display-name-is-verbatim-card-title | T0 | `cd clients/cli && GOWORK=off go test ./internal/view/ -run TestDisplayNameIsServerCardTitleVerbatim` | takes `view.display_name` (the driver name), derives a path basename, or truncates |
| ver-cli-never-drops-a-published-session | T1 | `cd clients/cli && GOWORK=off go test ./internal/view/ -run TestEndedSessionsArePresentedNotFiltered` | filters ended sessions client-side |
| ver-json-separates-unreachable-from-empty | T1 | `cd clients/cli && GOWORK=off go test ./internal/view/ -run TestJSONDistinguishesUnreachableFromNoSessions` | omits an unreachable server or renders it like an empty one |
| ver-matrix-copy-matches-canonical | T2 | `cd clients/cli && GOWORK=off go test ./internal/view/ -run TestStatusMatrixCopyMatchesRepositoryCanonical` | lets the in-module copy drift from `protocol/session-status-display.json` |
| ver-precedence-collapses-like-existing-clients | T0 | `cd clients/cli && GOWORK=off go test ./internal/view/ -run TestStatusPrecedenceMatchesSharedMatrix` | uses the favicon aggregation order, which puts `idle` above `stopped` |
| ver-declared-kind-decides-resolution | T0 | `cd clients/cli && GOWORK=off go test ./internal/selector/ -run TestDeclaredKindDecidesResolution` | falls back to another selector kind |
| ver-qualifier-narrows-cross-server-collision | T1 | `cd clients/cli && GOWORK=off go test ./internal/selector/ -run TestServerQualifierNarrowsCollisionByNameAndByStableId` | accepts only one of the two qualifier vocabularies |
| ver-unreachable-only-match-is-transport | T1 | `cd clients/cli && GOWORK=off go test ./internal/selector/ -run TestUnreachableOnlyMatchIsTransportNotNotFound` | reports not-found when a server could not be read |
| ver-failure-paths-have-no-side-effect | T2 | `cd clients/cli && GOWORK=off go test ./internal/selector/ -run TestResolutionFailurePathsHaveNoSideEffect` | attaches or terminates on the way to reporting a failure |
| ver-departure-classes-follow-the-published-reason | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestDepartureReasonsFollowThePublishedCloseReason` | invents a departure reason locally instead of reading the published one, gives two departure classes the same reason text, or dresses an unattributed loss as one of the named causes |
| ver-cli-maps-each-close-reason-to-its-exit-class | T1 | `cd clients/cli && GOWORK=off go test ./internal/attach/ -run TestEachPublishedCloseReasonMapsToItsExitClass` | maps two published reasons to one class where the contract separates them, or guesses a class for a departure that published nothing |
| ver-exit-codes-match-the-fixed-assignment | T0 | `cd clients/cli && GOWORK=off go test ./internal/app/ -run TestFailureClassesMapToTheFixedExitCodes` | assigns any code other than 0/1/2/3/4/5/6 as specified, or maps two classes to one code |
| ver-tty-and-non-tty-identical | T1 | `cd clients/cli && GOWORK=off go test ./internal/app/ -run TestInteractiveAndPipedInvocationsAgree` | branches output content or exit code on terminal detection |
| ver-credential-never-reaches-diagnostics | T1 | `cd clients/cli && GOWORK=off go test ./internal/app/ -run TestAuthFailureDiagnosticsCarryNoCredentialTicketOrURL` | echoes the rejected bearer credential, the minted ticket, or a URL carrying either into the failure reason |

### Lifecycle commands

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-create-outcomes-are-distinct-observations | T1 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestCreateOutcomesAreDistinctObservations` | reports a created-then-attach-failed run as a creation failure, hiding a live session, or collapses a `--no-attach` creation with an attach failure by exiting non-zero or withholding the identifier |
| ver-create-requires-explicit-server | T0 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestCreateWithoutQualifierIsUsageErrorAndCreatesNothing` | infers the target from `default_execution` or from the single configured server |
| ver-unresolvable-agent-creates-no-row | T1 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestUnresolvableAgentFailsBeforeAnyCreatingRequest` | issues the creation request before resolving the profile, leaving a row behind on failure |
| ver-attach-follows-created-identifier | T1 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestCreateAttachesToTheSessionItJustCreated` | attaches to a different session than the one it created |
| ver-terminate-affects-only-target | T1 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestTerminateAffectsOnlyTheTargetedSession` | ends a session that merely detached or received Ctrl-C |
| ver-rejected-terminate-exits-nonzero | T1 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestRejectedTerminateExitsNonZeroAndLeavesStateUnchanged` | reports success on a rejected request |
| ver-terminate-report-claims-only-acknowledgement | T0 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestTerminateReportAssertsNoUnreadLifecycleState` | prints a post-termination lifecycle state it never read back |
| ver-terminate-is-the-only-emitter | T2 | `cd clients/cli && GOWORK=off go test ./internal/ -run TestTerminateRequestHasExactlyOneEmissionSite` | adds a second site able to emit a termination |

### Configuration-contract consumption

The four CLI-side checks of contracts realized by `change-20260814-cli-prerequisite-contracts`. They read the loader's per-entry admission states; they do not re-implement the loader.

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-default-execution-never-infers-a-create-target | T0 | `cd clients/cli && GOWORK=off go test ./internal/lifecycle/ -run TestDefaultExecutionIsNeverReadAsACreateTarget` | reads `default_execution` — or the first-enabled fallback all three loaders apply — as a creation target |
| ver-invalid-entry-is-a-third-vocabulary-in-discovery | T1 | `cd clients/cli && GOWORK=off go test ./internal/view/ -run TestConfigurationInvalidIsDisjointFromReachabilityAndLifecycle` | renders a configuration-invalid entry as `unavailable`, or shares a value space with session lifecycle state, or omits the entry |
| ver-invalid-entry-is-never-connected-to | T1 | `cd clients/cli && GOWORK=off go test ./internal/fleet/ -run TestConfigurationInvalidEntryIsNeverProbed` | probes an entry the contract declared unusable, so it can acquire a reachability observation |
| ver-qualifier-precedence-unusable-outranks-ambiguous | T1 | `cd clients/cli && GOWORK=off go test ./internal/selector/ -run TestUnusableMatchesExitUsageAndAmbiguousOnlyAppliesToUsableOnes` | picks the first server matching a duplicated display name, offers unchoosable candidates as an ambiguity, or reports a configured-but-unusable entry as not-found |

### Wire shape, identity ownership, and gates

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-server-serialization-matches-golden-vector | T2 | `cd services && GOWORK=off go test ./host/state/view/ -run TestSessionInfoGoldenVectorIsStable` | renames or moves a session payload field the CLI reads |
| ver-cli-decodes-the-same-golden-vector | T0 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestDecodesCheckedInSessionInfoGoldenVector` | decodes a convenience shape instead of the server's |
| ver-golden-vector-renaming-fails-a-gate | T2 | `cd services && GOWORK=off go test ./gorules/ -run TestSessionInfoGoldenVectorIsRegistered` | adds a consumer without registering it, so the gate stops covering it |
| ver-identity-construction-has-one-owner | T2 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestServerQualifiedIdentityHasExactlyOneConstructor` | builds a server-qualified identity in a second package |
| ver-outbound-serialization-has-one-owner | T2 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestOutboundFrameSerializationHasExactlyOneSite` | serialises an outbound frame outside the codec |
| ver-ownership-gate-fails-on-synthesized-violation | T2 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestIdentityOwnershipGateFailsOnInjectedSecondOwner` | ships an ownership gate that never fires |
| ver-mismatch-blocks-operational-work | T1 | `cd clients/cli && GOWORK=off go test ./internal/gateway/ -run TestMismatchedServerPerformsNoOperationalWork` | continues to a REST call or a WebSocket application frame after a version mismatch |
| ver-mismatch-does-not-disable-peers | T1 | `cd clients/cli && GOWORK=off go test ./internal/fleet/ -run TestOneMismatchedServerDoesNotDisableOthers` | empties the whole list because one server is incompatible |
| ver-version-projection-registered | T2 | `cd services && GOWORK=off go test ./gorules/ -run TestProtocolVersionManifestMatchesEveryShippedProjection` | ships a projection the allow-list does not enumerate, so a CLI version drift goes undetected |
| ver-no-services-module-edge | T2 | `cd clients/cli && GOWORK=off go list -deps ./... \| grep -q 'takezoh/agent-grid/services' && exit 1 \|\| exit 0` | adds a dependency edge to the services module |
| ver-cli-registered-in-gates | T2 | `cd services && GOWORK=off go test ./gorules/ -run TestCLIModuleRegisteredInBuildAndVerificationEntryPoints` | introduces the module without build, test, vet, lint, and profile coverage |
| ver-dependency-triple-declared | T2 | `./scripts/check-harness-dependencies.sh` | registers an external boundary without a fake, a contract test, and a fidelity backstop |

### T3 residue (manual or nightly)

- Real-terminal echo and line editing actually disappearing on raw entry and returning on every restore path, including suspend and resume.
- Windows console behaviour: raw entry effectiveness, VT input arrival, VT output processing, Ctrl-C arrival form.
- Windows terminal size-change observation mechanism, latency, and loss.
- Whether a single `Ctrl-]` collides with Claude Code's or Codex's own key handling during real use.
- `FakeVsReal*` backstops for the gateway and the local-terminal boundaries. If one fails, fix the fake — never weaken the assertion.

Both backstops are registered and runnable:

| Boundary | Registry id | Backstop | Opt-in | Last observed |
|---|---|---|---|---|
| gateway | `cli-gateway` | `TestE2E_FakeVsRealGatewayAdmission` | `AG_E2E_SERVER_BIN` | PASS, 2026-08-15, after two defects it found |
| local terminal | `cli-local-terminal` | `TestE2E_FakeVsRealTerminalRestore`, `TestE2E_RealTerminalGeometryIsReadBack` | `AG_E2E_REAL_TTY` | PASS, 2026-08-14 |

`scripts/run-go-e2e.sh` now groups suites by module, because `clients/cli` is a
separate Go module and a package pattern from `services` cannot reach it. A
suite that names no module still belongs to `services`.

#### The terminal backstop was unreachable through its own entry point

As first written it required `os.Stdin` to be a terminal. No invocation of
`go test` can satisfy that — the go command hands the test binary a null device
— so the backstop could only ever skip, including on a machine with a real
console. A registered backstop that cannot run is worse than an absent one,
because the registry reads as coverage.

It now allocates its own pty (`github.com/creack/pty`, already pinned in the
services module) and opens the production `localTerminal` over it through
`openTerminal`, the named-files form of `Open`. A fresh pty is a real terminal
as far as termios is concerned, which is the entire fact under test, and it
belongs to the test rather than to the operator — so raw mode is never entered
on a console someone is looking at. `AG_E2E_REAL_TTY` remains the only gate, so
the suite stays T3.

A second case came with it: `TestE2E_RealTerminalGeometryIsReadBack` sets the
pty size behind the terminal's back and requires `Size()` to report the new
geometry. Nothing else in the suite could tell a `Size()` that queries the
kernel from one that returns a remembered constant, because the fake answers
from its own record either way.

What this still does not observe is a Windows console, which has no termios at
all. That remains `evidence-windows-console-behavior`.

#### The gateway backstop found two defects on its first real run

It first reported `the real server never became reachable`. That was the
environment: the sandbox it ran in denies `socket(AF_UNIX)`, so the daemon
exited on `listen …/server.sock: operation not permitted` before binding an HTTP
port. Run on a host that permits unix sockets, it got past start-up and found
two things nothing else could have.

**The harness spoke the wrong scheme.** The server serves HTTPS by default
(`tls=true`, self-signed). `startRealServer` built `http://` and got a 400 back,
which the CLI classified — correctly — as `invalid_request`. The fake was never
wrong. Fixed by keeping TLS on and trusting the child process's own certificate,
rather than by passing `-insecure`: the CLI derives the socket scheme from the
configured URL, so a plaintext run never exercises the `https` → `wss` mapping
every real deployment takes.

**The two transport halves disagreed.** `New` accepts a `Doer` for REST but
installed a dialer that always used `http.DefaultClient`, so a server reachable
over REST through a private certificate authority or a proxy would admit and
then fail to attach, with nothing in the failure naming the asymmetry. This is a
production defect the backstop surfaced, not a test artifact.
`gateway.NewWebSocketDialer` now builds the production dialer over a named HTTP
client, and the two halves can share one transport policy.

**Then the assertion that survived was the real one.** The fake's hello frame
was missing `features`, `capabilities` and `serverTime` — three keys the real
server always sends and the CLI reads none of. A decoder that broke on their
presence would have passed every test in this repository. The fake now carries
them, as arrays rather than JSON null (the server replaces nil slices with empty
ones, and a fake must not be the laxer counterparty) and with a fixed
`serverTime`, because a hello frame that changed between runs would make every
byte-level assertion over it flaky for reasons unrelated to the code under test.

### The three platform observations (revised 2026-08-15)

This section previously said all three needed hardware that was not available,
and left the Windows paths unimplemented. That was too quick a conclusion: two
of the three were reachable, and the third was reachable *as a test* even where
it cannot run.

| Observation | State | Evidence |
|---|---|---|
| evidence-detach-key-chord-collision | **acquired — no collision** | `TestE2E_DetachByteIsNotBoundByRealAgents` drives the real `claude` and `codex` binaries in a pty. Both react visibly to a control key (140 and 670 bytes of redraw) and emit **nothing** for `0x1D`. The control is what makes it an answer rather than an absence. |
| evidence-windows-console-behavior | **implemented and covered; awaiting a run on Windows** | `DiscardInputQueue` now calls `FlushConsoleInputBuffer`. `TestE2E_WindowsDiscardInputQueueEmptiesTheConsoleBuffer` and `TestE2E_WindowsRawModeChangesAndRestoresConsoleMode` assert both halves against a real console via `CONIN$`/`CONOUT$`. |
| evidence-windows-resize-source | **decided and covered; the source is polling** | `watchSize` polls `term.GetSize` every 250 ms. Reading `WINDOW_BUFFER_SIZE_EVENT` was rejected on a stated ground: it consumes from the same console input buffer the input pump reads keystrokes from, so the observer would race the user's typing. |

The claim that Windows has "no terminal driver queue to flush — the console host
owns the input buffer" was simply **wrong**. The console input buffer is exactly
what `FlushConsoleInputBuffer` discards, and discarding it is the same act the
Unix path performs with `TCIFLUSH`.

What backs the Windows code without a Windows machine: two documented console
APIs, a clean `GOOS=windows` build and `vet` on amd64 and arm64, unit coverage
of the polling decision that runs everywhere (`size_poll_test.go`), and a
registered e2e file that asserts the rest wherever a console exists. That is
weaker than an observed run and much stronger than an unimplemented stub.

The polling decision is deliberately **not** in the platform file. `pollSize`
takes a size source and an interval, so "poll, compare, notify only on change,
stop synchronously and idempotently" is tested on Linux; only the one call that
reads the geometry is Windows-specific. Writing it that way immediately caught a
real defect: the first version's stop function panicked when called twice, which
`localTerminal.Close` does on a signal exit.

The BSD input-queue flush was listed here as genuinely open, on the ground that
there was no second documented API to reach for. That was wrong in the same way
the Windows claim was: `golang.org/x/sys/unix` exports `TIOCFLUSH` for darwin,
dragonfly, freebsd, netbsd and openbsd, and the read-side flag it takes is
`FREAD 0x0001` from `sys/file.h`. It is implemented in `flush_bsd.go` and
cross-compiles for all five.

More usefully, the assertion that will judge it **runs here today**.
`TestE2E_DiscardInputQueueEmptiesTheDriverQueue` writes into a pty master, calls
`DiscardInputQueue`, and requires nothing to come back — with a control case
beside it proving the bytes do arrive when nothing discards them. Replacing
`flushTerminalInput` with a no-op makes it fail, naming the nine bytes that
survived, while the control still passes. So the same test that passes on Linux
is a real check of the BSD path wherever it runs, rather than a hope.

`flush_other.go` now covers only solaris, aix, plan9 and js, where no ioctl has
been identified.

### Acceptance-to-verification coverage

Acceptance criteria owned by this change. A superscript **P** marks a verification discharged by `change-20260814-cli-prerequisite-contracts`.

| Acceptance | Verifications |
|---|---|
| AC-attach-current-screen | ver-first-payload-written-first, ver-output-bytes-are-byte-faithful, ver-foreign-session-output-dropped |
| AC-input-roundtrip | ver-utf8-sequence-never-split, ver-input-fuzz-order-and-bytes, ver-ctrl-c-forwarded-as-0x03 |
| AC-detach-restores-and-discards | ver-post-detach-bytes-surface-nowhere, ver-input-queue-flushed-before-restore, ver-configured-detach-byte-only, ver-detach-sends-no-terminate |
| AC-restore-on-every-exit | ver-restore-on-every-exit-path, ver-pre-subscribe-failure-leaves-terminal-untouched, ver-restore-is-idempotent, ver-tty-seam-drives-all-exit-paths, ver-detach-sends-no-terminate |
| AC-lease-survives-backpressure | ver-lease-renews-under-stdout-backpressure, ver-lease-revision-monotonic |
| AC-non-tty-fail-fast | ver-non-tty-stdin-fails-without-mode-change |
| AC-disconnect-classes-distinct | ver-departure-classes-follow-the-published-reason, ver-cli-maps-each-close-reason-to-its-exit-class, ver-terminal-transition-is-published-on-lifecycle **P** |
| AC-resize-reaches-remote | ver-initial-geometry-is-first-frame, ver-geometry-clamped-into-range, ver-resize-burst-coalesces-to-latest |
| AC-discovery-parity | ver-rows-carry-owning-server, ver-display-name-is-verbatim-card-title, ver-matrix-copy-matches-canonical, ver-precedence-collapses-like-existing-clients, ver-cli-never-drops-a-published-session |
| AC-partial-outage | ver-json-separates-unreachable-from-empty, ver-mismatch-does-not-disable-peers, ver-mismatch-blocks-operational-work |
| AC-selector-and-failures | ver-declared-kind-decides-resolution, ver-unreachable-only-match-is-transport, ver-failure-paths-have-no-side-effect, ver-exit-codes-match-the-fixed-assignment, ver-tty-and-non-tty-identical |
| AC-create-outcomes-distinct | ver-create-outcomes-are-distinct-observations, ver-create-requires-explicit-server, ver-default-execution-never-infers-a-create-target, ver-unresolvable-agent-creates-no-row, ver-attach-follows-created-identifier |
| AC-terminate-is-explicit-only | ver-terminate-affects-only-target, ver-rejected-terminate-exits-nonzero, ver-terminate-report-claims-only-acknowledgement, ver-terminate-is-the-only-emitter, ver-detach-sends-no-terminate |
| AC-boundary-and-gates | ver-no-services-module-edge, ver-cli-registered-in-gates, ver-version-projection-registered, ver-dependency-triple-declared, ver-config-never-written-and-directory-per-platform **P** |
| AC-concurrent-attach | ver-attach-topology-snapshot-two-clients, ver-attach-capacity-refusal-is-transport-with-the-published-reason, ver-attach-never-reports-accepted-as-established, ver-attach-topology-no-per-session-route |
| AC-credential-never-echoed | ver-credential-never-reaches-diagnostics |
| AC-wire-shape-and-identity-pinned | ver-server-serialization-matches-golden-vector, ver-cli-decodes-the-same-golden-vector, ver-golden-vector-renaming-fails-a-gate, ver-identity-construction-has-one-owner, ver-outbound-serialization-has-one-owner, ver-ownership-gate-fails-on-synthesized-violation |

### What this change contributes to the prerequisite change's acceptance

Five acceptance criteria of the shared plan are owned by `change-20260814-cli-prerequisite-contracts`. Four of them are only fully discharged once the CLI-side check below is green as well.

| Acceptance (owned there) | Contributed from here |
|---|---|
| AC-transport-close-is-typed | ver-cli-maps-each-close-reason-to-its-exit-class |
| AC-config-entry-admission | ver-invalid-entry-is-a-third-vocabulary-in-discovery, ver-invalid-entry-is-never-connected-to |
| AC-server-naming-and-qualifier-outcomes | ver-qualifier-precedence-unusable-outranks-ambiguous, ver-qualifier-narrows-cross-server-collision |
| AC-ended-session-remains-observable | — (fully discharged there) |
| AC-config-parity-across-languages | — (fully discharged there) |

Until the prerequisite change lands, five CLI-side checks read a contract that does not exist yet and are expected to be red: `ver-cli-maps-each-close-reason-to-its-exit-class`, `ver-invalid-entry-is-a-third-vocabulary-in-discovery`, `ver-invalid-entry-is-never-connected-to`, `ver-qualifier-precedence-unusable-outranks-ambiguous`, and `ver-cli-never-drops-a-published-session` for the ended-session case. That redness is the dependency edge made visible, not a defect in this change.

### Full local verification run, 2026-08-14

Everything below was executed in this repository on the date given; the two
failing groups are named with their cause rather than folded into a pass.

| Leg | Result |
|---|---|
| `make lint` (services, clients/cli) | 0 issues in both modules |
| `make build-all` | 6 binaries, including `dist/bin/agent-grid` |
| `cd clients/cli && go test ./...` | all packages ok |
| `cd clients/cli && go list -deps` filtered for `services` | 0 matches |
| `services/gorules`, `services/internal/harnesspolicy` | ok |
| `scripts/check-harness-dependencies.sh` | 16 external dependencies admitted |
| `scripts/check-test-skips.sh` | ok |
| `scripts/vet-go-e2e.sh` | ok (both modules, `-tags e2e`) |
| `scripts/check-client-config-parity.sh` | Go, TypeScript **and C#** all green — 28 C# cases, the first run of that leg here |
| `clients/cli` `cli-local-terminal` T3 with `AG_E2E_REAL_TTY=1` | 2 tests PASS |
| `clients/ui` `npm run test:unit` | 135 files, 1911 tests pass |
| `clients/ui` `npm run test:e2e` (Playwright) | 19 passed, 5 skipped |
| `clients/workspace` `npm run typecheck` | pass |
| `clients/workspace` vitest | 186 pass, 1 skip, **1 fail** — `control-endpoint.test.ts` cannot `listen` on a unix socket here |
| `cd services && go test ./...` | **10 packages fail**, every one on `socket: operation not permitted` or an outbound network call |
| `clients/cli` `cli-gateway` T3 with `AG_E2E_SERVER_BIN` | **fails to start the backend** — same unix-socket denial |
| `docs lint --conformance` | this change and the prerequisite change are clean; two July changes still need HARD approval evidence |

The environment denies `socket(AF_UNIX)`. That single fact accounts for every
failure in the last three rows, and it is why `cli-gateway` remains unobserved
rather than green: a run that cannot start the server has not compared the fake
to anything.

### Re-run with unix sockets available, 2026-08-15

The same legs, on a host where `socket(AF_UNIX)` is permitted.

| Leg | Result |
|---|---|
| `clients/cli` `cli-gateway` T3 | **PASS** — after the three defects above were fixed |
| `cd services && go test ./cmd/... ./host/... ./platform/...` | all green; every one of the ten earlier failures was the socket denial and nothing else |
| `clients/workspace` `control-endpoint.test.ts` | PASS |
| `make lint` | 0 issues in both modules |
| `scripts/vet-go-e2e.sh` | ok |

### Getting to a repository that is actually green, 2026-08-15

Four defects stood between "the CLI's own checks pass" and "the suite passes".
None was caused by this change; all four are fixed at their cause, because a
suite that is red for reasons nobody owns stops being read at all.

| Defect | Nature | Fix |
|---|---|---|
| Four gateway scenario tests | asserted the pre-retention contract; two passed **vacuously** on a helper that any frame satisfied | `waitForSessionTerminal`, which requires the published terminal state *and* the retained row |
| `TestTerminalRelay_SeversSlowSubscriberWithoutBlockingOthers` | raced its own subject: drained before severance was reached, and required a buffered event to survive severance | waits for the observable outcome (`awaitSevered`); the prefix bound is now upper-only |
| `TestWorkspaceWriteResourceBoundConcurrentRSS` | read **process-wide** RSS inside a shared test binary, charging other tests' allocations to itself | the measurement re-runs alone in a child process; the 32 MiB bound is untouched |
| `gitclone.FakeGitClone` | a real data race under `-race` — recorded calls without synchronisation while the real cloner is concurrency-safe | mutex plus locked accessors, per "fix the fake, never the assertion" |

Two of these were **false green**, which is the worse failure mode: the relay
test's vacuous sibling and `waitForSessionAbsent` both reported success while
asserting nothing. A test that cannot fail is indistinguishable from coverage
until someone checks.

Final state on a host that permits unix sockets:

| Leg | Result |
|---|---|
| `cd services && go test ./...` | green, five consecutive full runs |
| `make test-race` | green |
| `TestTerminalRelay_SeversSlowSubscriber…` | 60 runs green; 40 more under `-race` |
| `make lint` | 0 issues in both modules |
| `scripts/check-test-skips.sh` | ok |
| `clients/cli` unit suite | green |
