---
change: change-20260814-cli-prerequisite-contracts
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Content

Tier scheme per `docs/note/note-20260624-agent-testing.md`. T0 = pure unit over decoded values; T1 = component or scenario against a fake counterparty; T2 = enforcement, gate, and cross-module conformance.

Every id, tier and command below is copied from the shared plan (`../change-20260812-cli-terminal-client/design-plan/spine.yaml`, `contracts[].verification`). No command is rewritten here. Every check is discriminating: it names an implementation it fails.

All nineteen are **red today by construction** — that is the point. They are the falsifiable statement of the three gaps this change closes.

### A — Session terminal-state retention (`contract-session-terminal-state-retention`)

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-explicit-terminate-leaves-a-terminal-row | T1 | `cd services && GOWORK=off go test ./host/state/ -run TestStopSessionRetainsATerminalRow` | deletes the session record on explicit termination |
| ver-self-exit-paths-leave-a-terminal-row | T1 | `cd services && GOWORK=off go test ./host/state/ -run TestIntentionalExitAndFrameVanishRetainATerminalRow` | evicts on a clean or signalled self-exit or on a vanished frame |
| ver-terminal-transition-is-published-on-lifecycle | T1 | `cd services && GOWORK=off go test ./server/api/ -run TestLifecycleSocketPublishesTerminalTransition` | retains the row but never publishes the transition, leaving an attached client unable to attribute its departure |
| ver-ended-and-never-existed-are-distinguishable | T2 | `cd services && GOWORK=off go test ./server/api/ -run TestEndedSessionIsDistinguishableFromUnknownIdentifier` | makes an ended session and an unknown identifier look the same through the public contract |

### B — Transport close reason vocabulary (`contract-transport-close-taxonomy`)

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-every-server-initiated-close-carries-a-published-reason | T1 | `cd services && GOWORK=off go test ./server/api/ -run TestEveryServerInitiatedCloseCarriesAPublishedReason` | leaves a departure unwinding into an abrupt close with no reason, as graceful shutdown and every write-error path do today |
| ver-pre-upgrade-rejections-carry-the-same-vocabulary | T1 | `cd services && GOWORK=off go test ./server/api/ -run TestTicketAndVersionRejectionsCarryTheSameReasonVocabulary` | answers a rejected ticket with plain-text `unauthorized`, leaving it indistinguishable from a network drop |
| ver-close-reason-vocabulary-is-closed-and-registered | T2 | `cd services && GOWORK=off go test ./gorules/ -run TestTransportCloseReasonVocabularyIsRegisteredAndExhaustive` | adds a fifth ad-hoc close reason string with nothing to stop it |

The fourth verification of this contract, `ver-cli-maps-each-close-reason-to-its-exit-class`, exercises `clients/cli` and is discharged by `change-20260812-cli-terminal-client`.

### C — Client configuration contract, parity, and per-entry admission

`contract-config-and-fleet-identity`:

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-every-go-host-loads-through-one-owner | T2 | `cd services && GOWORK=off go test ./gorules/ -run TestClientConfigHasExactlyOneLoaderPerLanguage` | leaves or adds a second parser of `servers.json` in a client language |
| ver-server-id-digest-golden-vector | T0 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run TestServerIdMatchesCrossLanguageGoldenVector` | derives a different server identity than the other hosts |
| ver-config-never-written-and-directory-per-platform | T2 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run 'TestConfigDirectoryMatchesExistingHostsPerPlatform\|TestConfigDirectoryIsNeverWritten'` | keeps the missing Windows branch, so Windows sees a different fleet than the GUI; or writes into a configuration directory |

`contract-client-config-parity`:

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-expected-outcome-corpus-agrees-in-three-languages | T1 | `./scripts/run-verification-profile.sh pr schema` | lets Go, TypeScript and C# reach different outcomes on the same fixture, or asserts only that each throws |
| ver-go-loader-matches-the-expected-outcome-corpus | T1 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run TestExpectedOutcomeCorpus` | passes a fixture without matching its recorded decision, admission states, canonical URL and derived identifier |
| ver-corpus-covers-every-per-entry-and-file-level-rule | T2 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run TestExpectedOutcomeCorpusCoversEveryDeclaredRule` | declares a contract rule no fixture exercises — the state duplicate URL, multiple default execution, zero enabled and unparseable are in today |
| ver-measured-divergences-are-closed-by-the-corpus | T1 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run 'TestUnknownFieldIsRejected\|TestUrlCredentialsAreRejectedNotStripped'` | keeps Go silently dropping unknown fields, or keeps a URL canonicalisation that splits the server identity across hosts |

`contract-config-entry-admission`:

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-host-starts-and-only-the-offending-entries-are-unusable | T1 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run TestHostStartsAndOnlyOffendingEntriesAreInvalid` | refuses the whole configuration for one bad entry, or lets a bad entry cost a sibling that is fine |
| ver-unreadable-entry-list-still-stops-the-host | T0 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run TestUnreadableEntryListIsAFileLevelStop` | starts with an empty fleet on a truncated or unversioned document instead of stopping |

`contract-server-naming-prerequisite`:

| ID | Tier | Command | Fails an implementation that |
|---|---|---|---|
| ver-duplicate-display-name-flagged-per-entry-by-every-loader | T1 | `./scripts/run-verification-profile.sh pr schema` | enforces uniqueness in only one language, or enforces it by stopping the host |
| ver-charset-violation-flagged-identically | T0 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run TestDisplayNameCharsetRejection` | accepts a control character or a leading `-` in a display name |
| ver-naming-violation-names-the-entry-without-renaming | T1 | `cd clients/goclient && GOWORK=off go test ./clientconfig/ -run TestNamingViolationReportsOffendingEntryWithoutRenaming` | auto-suffixes, disables, or arbitrarily picks a winner among colliding names, or reports without naming the offending entry |

The remaining verifications of these two contracts — `ver-invalid-entry-is-a-third-vocabulary-in-discovery`, `ver-invalid-entry-is-never-connected-to` and `ver-qualifier-precedence-unusable-outranks-ambiguous` — exercise `clients/cli` and are discharged by `change-20260812-cli-terminal-client`.

### Acceptance-to-verification coverage

| Acceptance | Verifications in this change | Verifications in the CLI change |
|---|---|---|
| AC-ended-session-remains-observable | ver-explicit-terminate-leaves-a-terminal-row, ver-self-exit-paths-leave-a-terminal-row, ver-ended-and-never-existed-are-distinguishable | — |
| AC-transport-close-is-typed | ver-every-server-initiated-close-carries-a-published-reason, ver-pre-upgrade-rejections-carry-the-same-vocabulary, ver-close-reason-vocabulary-is-closed-and-registered | ver-cli-maps-each-close-reason-to-its-exit-class |
| AC-config-parity-across-languages | ver-expected-outcome-corpus-agrees-in-three-languages, ver-go-loader-matches-the-expected-outcome-corpus, ver-corpus-covers-every-per-entry-and-file-level-rule, ver-measured-divergences-are-closed-by-the-corpus, ver-every-go-host-loads-through-one-owner | — |
| AC-config-entry-admission | ver-host-starts-and-only-the-offending-entries-are-unusable, ver-unreadable-entry-list-still-stops-the-host | ver-invalid-entry-is-a-third-vocabulary-in-discovery, ver-invalid-entry-is-never-connected-to |
| AC-server-naming-and-qualifier-outcomes | ver-duplicate-display-name-flagged-per-entry-by-every-loader, ver-charset-violation-flagged-identically, ver-naming-violation-names-the-entry-without-renaming | ver-qualifier-precedence-unusable-outranks-ambiguous, ver-qualifier-narrows-cross-server-collision |

This change also contributes to two criteria the CLI change owns: `ver-terminal-transition-is-published-on-lifecycle` to `AC-disconnect-classes-distinct`, and `ver-config-never-written-and-directory-per-platform` to `AC-boundary-and-gates`.

### Regression surface

These units change code the whole product already runs. The existing suites are the guard, and none of them may be weakened to make the new checks pass.

```sh
cd services && GOWORK=off go test ./host/... ./server/...
cd clients/web-host && GOWORK=off go test ./...
cd clients/workspace && npm run test:unit
./scripts/run-verification-profile.sh pr schema
```

### T3 residue

None owned here. This change introduces no external boundary, so it adds no `test-harness/dependencies.json` entry and no `FakeVsReal*` backstop; the gateway and local-terminal boundary registrations belong to `change-20260812-cli-terminal-client`. The real-terminal and Windows-console residue is likewise the CLI change's.
