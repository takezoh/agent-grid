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
