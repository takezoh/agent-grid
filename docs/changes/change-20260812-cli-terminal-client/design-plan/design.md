# Agent Grid terminal CLI client — technical design

granularity_profile: `algorithm-bound`. Evidence is graded **[measured]** (read in this repository at a named path), **[inferred]** (derived from measured facts, falsifiable as stated), **[unmeasured]** (routed to a spike; never written as settled).

This plan integrates draft 1 (winner) with seven grafted elements from draft 2, closes all sixteen critique issues, and then applies the user consultation `consultation-20260814-cli-terminal-client-design`. Of the ten decision ADRs, **seven were accepted** in that consultation; **three were returned for rework** and, with **one new prerequisite ADR** the consultation asked for, stay `proposed` pending a second verdict. §I records the rework line by line.

---

## A. Governing design and promotion (prose only)

`clients/cli` is a new client host of the same rank as the existing ones. The governing design is **not** `design-host.md` (id `design-client`): its `source_paths` are `services/host/`, `services/server/api/`, `services/cmd/server/`, `clients/web-host/`, which do not govern clients in general [measured]. Each existing client host carries its own `scope_type: component` design.

Promotion candidates, recorded but **not created in this run**:

- **upsert** `design-clients-cli` (component). Responsibility: a terminal client of the public wire contract. Boundary: `clients/cli/`. Forbidden: any dependency on `services/*`, PTY ownership, ownership or takeover control, a replay store. Capability: terminal session attach presentation.
- **upsert** `design-clients-goclient` (component) if the Go loader module lands. Responsibility: the **Go projection** of the client configuration contract — `servers.json` loading, canonical URL derivation, server identity derivation, per-entry admission, and platform configuration-directory resolution. Boundary: `clients/goclient/`. It is a projection, not the contract; the contract's owner is the next entry.
- **upsert (escalate first)** `design-windows-shell` currently owns `clients/config/` and the server naming contract [measured: RESP-005 / INV-009 / `source_paths`]. This change makes that contract explicitly canonical for every client host and binds three languages to it, which sharpens the mismatch rather than resolving it: a design named for one client would own a contract every client obeys. Carving out `design-clients-config` is recorded as a candidate and is not decided here (`open-config-contract-design-ownership`).
- **upsert** `design-host` — the terminal-state retention prerequisite is **accepted**, so retained terminal state and its lifecycle publication become an execution-plane responsibility and an invariant, not an implementation detail.
- **upsert (conditional)** `design-host` again, on a second and independent axis, if `adr-transport-close-reason-taxonomy` is accepted: a published closed vocabulary for connection departure becomes a public-contract invariant of the gateway, and the two-step close shape that `adr-20260624-0011` records becomes schema-checked rather than conventional.
- **none**: `design-clients-ui`, `design-workspace-desktop`. Both gain a capability from the transport-close prerequisite; neither is modified by it.

---

## B. Requirements

<!-- anchor: fr-001 -->
### fr-001 — a termination request is emitted only by the explicit terminate command (invariant)

The CLI shall not emit `DELETE /api/sessions/{id}` — or any other session-termination request — on any code path other than the explicit terminate command.

Detach, Ctrl-C, terminal close, stdin EOF, catchable signals, an uncatchable process kill, a WebSocket close, and an authentication or protocol failure after raw mode was entered are all inside this prohibition. Direct translation of the upstream constraint `cc-termination-only-explicit`; observed by UAC-013 / UAC-024 / UAC-029 / UAC-030.

Structural enforcement: the function that can emit a termination lives in `comp-lifecycle` alone and is unreachable from `comp-attach`. [measured] The absence of a purge side effect is corroborated on the server side: `reduceSurfaceUnsubscribe` and `reduceConnClosed` never touch `s.Sessions`, and no "terminate when the last subscriber leaves" mechanism exists in the repository.

<!-- anchor: fr-002 -->
### fr-002 — raw mode is restored on every exit path (invariant)

While the CLI holds raw mode, the CLI shall restore the invoking terminal's echo and line editing before the process exits, suspends, or returns control to the shell, on every exit path.

Covered paths: ordinary detach, WebSocket close, server disconnect, a post-entry authentication or protocol failure, a catchable terminating signal, suspend, stdin EOF, and the panic boundary. Upstream `cc-raw-mode-restore-all-exits`.

<!-- anchor: fr-003 -->
### fr-003 — user configuration is never written (invariant)

The CLI shall not create, modify, or delete any file under the client configuration directory or under the daemon data directory.

[measured] Of the three existing loaders, the TypeScript and C# ones create a default file through `loadOrCreate`; the Go `clientconfig.Load` only reads. The CLI aligns with the read-only side. `~/.agent-grid` is the daemon data directory and is distinct from the client configuration directory (`$XDG_CONFIG_HOME/agent-grid` and its platform equivalents) [measured].

<!-- anchor: fr-004 -->
### fr-004 — a non-TTY attach fails without touching the terminal

When attach is invoked and stdin is not a terminal, the CLI shall not apply any terminal mode change to the controlling terminal, shall not open a surface subscription, and shall exit with the non-TTY failure class and a non-zero code.

UAC-015. `term.IsTerminal(int(os.Stdin.Fd()))` is an existing repository idiom [measured: `services/host/event/event.go:51`].

<!-- anchor: fr-005 -->
### fr-005 — input reaches the remote PTY in order and unaltered

While attached, the CLI shall deliver every stdin byte to the remote PTY in read order without local echo, forwarding Ctrl-C as `0x03` rather than taking it as a local interrupt.

Upstream `cc-input-and-resize-passthrough`. UAC-010 / UAC-012.

<!-- anchor: fr-006 -->
### fr-006 — an input frame never splits a UTF-8 sequence

The CLI shall not place a partial UTF-8 sequence in a surface input frame, and shall hold an incomplete trailing sequence until the remaining bytes are read or the input stream ends.

[measured] The input payload is a JSON string, not base64 (`case "i": WriteRaw(..., []byte(in.D))`, `services/server/api/gateway.go:544-547`). A JSON string can carry only well-formed UTF-8, so a multi-byte character split at a read boundary is collapsed to U+FFFD before it reaches the remote side. This is why the upstream batching handoff cannot be delegated as "order-preserving is enough".

<!-- anchor: fr-007 -->
### fr-007 — the current screen appears immediately after attach

When a surface subscription is established, the CLI shall write the first output payload it receives to stdout before any subsequent payload, so the session's current screen appears without local reconstruction.

[measured] The snapshot is the direct result of subscribing: termvt sends it as the first output chunk (`platform/termvt/session_actor.go:53-75`). No marker distinguishes snapshot from live output on the wire, so the CLI neither identifies nor reconstructs it. UAC-009 / UAC-022 / UAC-023.

<!-- anchor: fr-008 -->
### fr-008 — output reaches stdout byte for byte

The CLI shall base64-decode the surface output payload to raw bytes and write those bytes to stdout unmodified, and shall not parse, render, or re-encode terminal escape sequences.

[measured] An output frame is `[timeSec,"o",dataB64,sessionId]`. Element 2 is a pass-through of the daemon's base64 string; re-marshalling it through a Go string replaces non-UTF-8 bytes with U+FFFD (`services/server/api/wire.go:28-41` records the same trap on the server side).

<!-- anchor: fr-009 -->
### fr-009 — output belonging to another session never reaches stdout (invariant)

The CLI shall drop every surface output frame whose element-3 session id differs from the attached session id.

[measured] The lifecycle-multiplexed socket carries several sessions' surfaces on one connection. Skipping the element-3 check lets another session's output bleed in during a switch (`services/server/api/wire.go:20-27` records this as a past code-review finding).

<!-- anchor: fr-010 -->
### fr-010 — bytes typed after the detach key surface nowhere

When the detach key byte is consumed, the CLI shall discard every subsequent input byte, shall discard the terminal input queue before restoring cooked mode, and shall not forward any of those bytes to the remote PTY.

UAC-041, DP-DETACHING-INPUT-DISPOSITION. [inferred, falsifiable] Merely stopping the read leaves bytes in the kernel tty input queue, which the restored shell then reads as its own input. Falsifier: on real hardware, type after detaching and observe an empty shell input line — that would show the queue discard was unnecessary.

<!-- anchor: fr-011 -->
### fr-011 — only the configured detach byte detaches

The CLI shall treat exactly the configured detach byte as the detach trigger and shall forward every other byte, including the default `0x1D` when it is not configured as the trigger, to the remote PTY.

UAC-014, upstream `cc-detach-key-configurable`.

<!-- anchor: fr-012 -->
### fr-012 — geometry is declared and propagated within range

The CLI shall declare its terminal geometry when the subscription is established and shall forward every subsequent local size change, clamping columns and rows into the accepted range before sending.

[measured] `readInitialGeometry` requires the first frame to be `{"k":"r",cols,rows}` and closes the socket with `invalid-geometry` otherwise. The accepted range is `1..2000` (`termvt.MaxDim`). An out-of-range resize is **silently discarded** by the server (`tryResize` only warns), so without clamping the screen and the remote diverge with no observable signal. UAC-011.

<!-- anchor: fr-013 -->
### fr-013 — lease renewal is not starved by output

While attached, the CLI shall renew its terminal lifecycle lease on a schedule that cannot be delayed by stdout write backpressure or by surface output volume.

[measured] `lifecycleLeaseTTL = 12s`, `lifecycleApplyDeadline = 4s` (`services/host/runtime/terminal_lifecycle_runtime.go`); renewal is a re-sent `ld` with a monotonically increasing `clientRevision`; the existing client renews every 4 seconds (`clients/ui/src/socket/terminalSubscription.ts`); expiry unsubscribes with `RevisionReleased` reason `lease_expired` and does not kill the session.

<!-- anchor: fr-014 -->
### fr-014 — the three departure classes are told apart after restore

When the CLI leaves the attached state, it shall restore the terminal first and then present a reason that distinguishes explicit detach, transport loss, and server-side session end from one another.

UAC-013 / UAC-018 / UAC-019. Presentation happens after restore because the CLI owns no output channel while raw mode is held (DP-ATTACH-OBSERVABILITY-CHANNEL). The third class depends on the execution plane publishing a session-end observation; see `contract-exit-classification` and `contract-session-terminal-state-retention` for the closed negative branch.

<!-- anchor: fr-015 -->
### fr-015 — the CLI removes nothing the server publishes

The CLI shall list sessions from every enabled configured server, shall not filter by creator, and shall not itself remove or hide any session the server publishes, including sessions whose lifecycle state indicates they have ended.

This is the client-side half of DP-LIST-SERVER-SCOPE / DP-LIST-LIFECYCLE-SCOPE and upstream `cc-list-population-parity`. The server-side half — that an ended session is still published at all — is `fr-026`, because measurement shows the record is deleted on three of four termination paths. UAC-005 / UAC-007.

<!-- anchor: fr-016 -->
### fr-016 — one server's failure does not empty the list

When one or more configured servers are unreachable or protocol-incompatible, the CLI shall still present the sessions of the healthy servers and shall present the connection-side condition in a vocabulary disjoint from session lifecycle state.

UAC-002 / UAC-040 / UAC-046.

<!-- anchor: fr-017 -->
### fr-017 — state words and precedence come from the shared matrix (invariant)

The CLI shall derive every session state label and the precedence used to collapse simultaneous conditions from the checked-in shared status display matrix, and shall not introduce a CLI-specific state word.

[measured] `protocol/session-status-display.json` holds six labels (`Working`, `Action required`, `Idle`, `Stopped`, `Failed`, `Unknown`) and the precedence `failed > human_input > active > stopped > idle`, and **no code in the repository reads it**. The effective definitions are hand-mirrored in TypeScript (`clients/ui/src/components/StatusIcon.tsx`) and C# (`PanelPresentation.cs`) with no cross-language parity test. UAC-006 / UAC-039, `adr-20260726-shared-status-display-matrix`.

<!-- anchor: fr-018 -->
### fr-018 — the display name is the server's card title, verbatim

The CLI shall render the session display name as the server-provided card title verbatim, with no client-side fallback, derivation, or truncation.

[measured] The SPA's `titleText(card) = card.title ?? ""` has no fallback; the chain is resolved by the driver layer (`services/host/driver/view_builder.go`) per `adr-20260726-server-owned-session-card-title` and `adr-20260624-0079`. `view.display_name` is the driver name (`codex` and similar), not the row title — confusing the two lands directly on UAC-004's counterexample. `SessionInfo.Name()` (`baseName(project)`) is a non-serialised Go helper and must not be used.

The prohibition on truncation is absolute and therefore removed from what table rendering may choose; see `discretion-table-rendering`.

<!-- anchor: fr-019 -->
### fr-019 — rows and machine-readable output recover the owning server (invariant)

The CLI shall present the owning server on every session row, shall emit server identity and session identity as separate machine-readable fields, and shall not emit any single concatenated identifier as the only way to name a session.

DP-JSON-SESSION-IDENTIFIER-SHAPE; `adr-20260727-server-qualified-identity-partial-failure` ("identity is `(serverId, localId)` end-to-end"). UAC-001 / UAC-003 / UAC-008.

<!-- anchor: fr-020 -->
### fr-020 — a selector resolves only under a declared kind

The CLI shall resolve a session selector only under an explicitly declared selector kind, shall never fall back to another kind, and shall present mutually distinguishable candidates with a non-zero exit and no side effect when more than one session matches.

[measured] Measurement reduces the kinds to two — the full identifier and an identifier prefix: `proto.SessionInfo` has no `name` field, and the name-like values (`view.card.title`, `view.display_name`) are mutable and non-unique. This is precisely the `invalidation_behavior` the upstream plan recorded for `assumption-stable-name-field-unknown`, not a design override of a product choice. UAC-031 / UAC-033 / UAC-034.

<!-- anchor: fr-021 -->
### fr-021 — the server qualifier accepts a display name and a stable id

The CLI shall accept a server qualifier supplied independently of the selector string, accepting both the user-configured display name and the derived stable server identifier as its value.

DP-SERVER-QUALIFIER-SYNTAX / DP-SERVER-QUALIFIER-VALUE-VOCABULARY. Display-name acceptance rests on a prerequisite (`fr-028`), and the behaviour while it is outstanding is `fr-029` — not prose. UAC-032 / UAC-025.

<!-- anchor: fr-022 -->
### fr-022 — creation requires an explicit target server

When session creation is invoked without an explicit target server, the CLI shall create no session and shall exit with the usage-error class, regardless of how many servers are configured or reachable.

[measured] The trap is real: `servers.json` carries `default_execution`, and when it is absent the first enabled server is auto-promoted (`clientconfig.validateServers`, TypeScript `hasExplicitDefault`, C# `fallbackDefaultIndex`). The CLI must never use that value to infer a creation target. DP-CREATE-TARGET-SERVER, UAC-045.

<!-- anchor: fr-023 -->
### fr-023 — explicit termination returns on an acknowledgement, and claims only that

When explicit terminate is invoked, the CLI shall report success only after the server has acknowledged that the termination was applied, shall exit non-zero when the request was rejected, and shall leave the target's observable state unchanged in that case.

[measured] `DELETE /api/sessions/{id}` returns 204 only after the daemon has applied the reducer: `SendCommand` waits for the daemon's `okResp`, and that `okResp` is emitted by the same reduce that removed the session. So the 204 **is** the acknowledgement of application.

What this requirement deliberately does **not** claim is that the target is afterwards observable as an ended session. That observation is `fr-026`, and it is not satisfiable under the current public contract. The success report must therefore state the acknowledgement it received and must not assert a lifecycle state the CLI has not read back. UAC-044 is satisfied here; UAC-029's second half is not, and is escalated.

<!-- anchor: fr-024 -->
### fr-024 — failure classes carry fixed, reproducible exit codes

The CLI shall exit `0` on success, `1` on an internal fault, `2` on a usage error, `3` on not-found, `4` on an ambiguous selector, `5` on an authentication-or-compatibility failure, and `6` on a transport failure; shall reproduce the same output and exit code for the same input; and shall classify both a selector that matches only on an unreachable server and a capacity refusal of attach establishment as transport failure, separating the latter by the reason the execution plane publishes rather than by a client-authored string.

DP-EXIT-CODE-GRANULARITY and DP-UNREACHABLE-SERVER-SELECTOR-OUTCOME. The numbering is closed here (`adr-cli-failure-class-exit-codes`) because `handoff-exit-code-numeric-assignment` is a confirmed design handoff, not an open product decision. **The assignment `0`–`6` is accepted and is not reopened**; the only clause the consultation changed is how capacity is separated within code `6`.

The clause about capacity is a correction, not an addition. The earlier formulation asserted a closed five-class partition while the same design measured a sixth establishment failure that fits none of them: the per-connection surface-subscribe cap of 8 is keyed on `ConnID`, and `ConnID` is the identity of a **daemon-side Unix-socket IPC connection** allocated in `services/host/runtime/ipc.go:101-108` — the gateway process holds exactly one, so every WebSocket client in the fleet shares one budget of eight [measured: `services/host/state/reduce_surface.go:108-110`, `services/host/state/state.go:70-71`]. (The earlier revision cited `services/server/api/gateway.go:100` for `ConnID`; that citation is wrong — there is no `ConnID` in `gateway.go` — while the substance it supported is right, and the per-browser identities `web-<n>` and `owner-<random24>` at `gateway.go:309-310` take no part in the cap.)

What changed is the discriminator. The interim rule was "code `6` plus a reason text that names capacity", which makes the discriminator a CLI-authored sentence — exactly the per-implementer variance this requirement forbids one clause earlier for the codes themselves. `fr-032` puts the discriminator in the public contract instead, so the reason the CLI prints and branches on is the one the execution plane published. UAC-035 / UAC-036 / UAC-045.

<!-- anchor: fr-025 -->
### fr-025 — observation does not branch on terminal detection

The CLI shall produce identical output content and exit codes for the same input whether or not stdout is a terminal, and shall not add a confirmation prompt or a candidate-selection prompt in any environment.

DP-TTY-BEHAVIOR-BRANCHING. The non-TTY stdin branch in `fr-004` is not a behaviour the CLI chooses; it follows from the physical capability of the device, and the upstream plan states this explicitly. UAC-037 / UAC-038.

<!-- anchor: fr-026 -->
### fr-026 — an ended session stays observable as an ended session (invariant, prerequisite)

The execution plane shall keep a session observable with a terminal lifecycle state after it ends by explicit termination, by clean or signalled self-exit, or by its backend frame vanishing, and shall publish that transition on the lifecycle socket; the CLI shall never reconstruct that observation from local memory.

This requirement exists because the earlier design contained two related errors, both of which measurement falsifies.

**The explicit-termination path.** `reduceStopSession` executes `delete(s.Sessions, sid)` [measured: `services/host/state/reduce_session.go:313-329`] and the durable snapshot is deleted by the runtime's removal path [measured: `services/host/runtime/runtime.go:789-792`]. After an explicit terminate the target appears neither in `GET /api/sessions` nor in `k:"v"`; the only observable is absence.

**The self-exit path, which the earlier design treated as already settled.** It is not. `reduceFrameCommandExited` evicts the whole session when the exit code is one of `{0, 129, 130, 137, 143}` [measured: `services/host/state/reduce_tick.go:75-81, 106-115`, and `evictRootFrame` deletes the session at `services/host/state/reduce_frame_evict.go:29-32`], and `reduceFrameVanished` evicts on a vanished backend window [measured: `services/host/state/reduce_tick.go:49-57`]. A row with `status.lifecycle == "stopped"` therefore survives on **one** of the four termination paths — the abnormal exit code — and UAC-007's Given (a clean self-exit) is precisely one of the three that do not.

**Why the shape of the fix belongs to the execution plane.** The public protocol already declares the state: `protocol/session-status.schema.json` has `lifecycle ∈ {active, idle, stopped}` and the shared display matrix carries a `Stopped` label with its own precedence slot [measured]. `view.Status` maps `StatusStopped` to `LifecycleStopped` [measured: `services/host/state/view/status.go:112, 140-150`], but that mapping can only run while the session record still exists. The contract declares a terminal state the plane deletes rather than publishes.

A client-side compensation is forbidden: remembering terminated sessions locally would violate `nfr-004` and the upstream `cc-execution-plane-ownership`, and would make two clients disagree about which sessions ever existed.

<!-- anchor: fr-027 -->
### fr-027 — credentials never appear in diagnostics (invariant)

The CLI shall not place the bearer credential, a minted WebSocket ticket, or any URL carrying either into a failure reason, a diagnostic line, a log, or a process argument.

`fr-003` keeps the credential out of files and the transport unit keeps it out of the socket URL, but every departure must print a reason, and the authentication failure path is exactly the one most likely to echo the rejected value back. The prohibition is enforced at the single point where a transport error becomes a reason, not at each call site.

<!-- anchor: fr-028 -->
### fr-028 — a naming violation costs one server, not the host (prerequisite)

When a client host loads a configuration entry whose display name duplicates another entry's or falls outside the declared CLI-safe character set, it shall start, shall place that entry **and every entry it collides with** in the configuration-invalid admission state, shall not connect to any of them, and shall not rename, normalise, deduplicate, or silently drop any of them.

Upstream `cc-server-display-name-unique` and `cc-server-display-name-cli-safe-charset`. [measured] Today nothing enforces either: `clients/config/servers.schema.json:22` constrains a display name only by `minLength: 1`, no code reads that schema at all — all three loaders hand-roll their validation — and every loader keys its uniqueness check on the URL digest (`clients/web-host/internal/clientconfig/config.go:91-93` `"duplicate server url %q"`, `clients/workspace/src/main/desktop-config.ts:302-304`, `clients/windows-shell/.../DesktopConfig.cs:132-133` `"servers.json: server ids must be unique"`). A constraint with no enforcement point is an assertion; this requirement gives it one.

**Why the enforcement is per entry rather than per file.** The earlier revision made a naming violation reject the whole configuration and stop the host, which matched the documented behaviour [measured: `clients/config/README.md:41-43`, "malformed files, unsupported versions, duplicate server URLs, and invalid values stop startup with a file-specific error"]. That is the wrong blast radius for a rule about one entry: a typo in the display name of a server the operator is not even using would take away every other server, on every host, at once — and the rule is being added precisely because display names are about to become addressable, so the class of configurations that can trip it is about to grow. The general rule is `fr-030`; this requirement is its naming instance.

**Both members, not a winner.** A duplicate is a property of a pair, so blaming one member requires choosing which, and every choice rule (first wins, last wins, enabled wins) silently changes which server an operator's existing `--server <name>` reaches. Marking the whole collision group invalid is the only rule that is decidable without a choice.

<!-- anchor: fr-029 -->
### fr-029 — a qualifier that cannot be honoured says why, and never guesses

When a server-qualifier value matches configured entries of which every match is unusable, the CLI shall exit with the usage-error class naming each matching entry and the rule it violates; when more than one usable entry matches, the CLI shall present those entries as mutually distinguishable candidates and exit with the ambiguous-selector class without selecting one; and it shall report neither case as not-found nor as a transport failure.

Picking one would contradict DP-AMBIGUITY-INTERACTIVITY; failing as ambiguous is the only outcome consistent with the settled decision. The requirement exists rather than being ADR prose because an implementation that picks the first match would otherwise be inside the contract.

**Why the firing condition had to be restated.** Under whole-file rejection, a duplicated display name never reached resolution — the host had already stopped. Under `fr-028`'s per-entry admission the host runs, both colliding entries exist and neither is usable, so the old wording ("matches more than one configured enabled server → ambiguous") would have produced a candidate list from which nothing can be picked. Resolution therefore runs over **every configured entry**, not only the usable ones, and unusability outranks ambiguity: with no choosable candidate the only next action is editing the configuration, and the exit code must say so. `contract-server-naming-prerequisite` carries the precedence.

**And the old absolute claim is false under the new rule.** The earlier text asserted that a stable server identifier "shall always resolve to at most one server". [measured] The identifier is `srv_` + SHA-256 of the canonical URL (`clients/web-host/internal/clientconfig/config.go`), so two entries with the same URL have the *same* identifier — a case the current loaders never produce only because duplicate URLs stop the file. Once duplicate URLs become per-entry invalidity, an identifier can match two entries, and both are invalid by the duplicate-URL rule. The corrected claim is that a stable identifier resolves to at most one **usable** entry.

<!-- anchor: fr-030 -->
### fr-030 — the host starts whenever the entry list is readable (prerequisite)

When a client host loads a client configuration whose entry list is readable, it shall start and admit every entry independently, placing each entry that violates a per-entry rule — and every member of a colliding group — in the configuration-invalid admission state rather than refusing the whole configuration; it shall stop at load only when the entry list itself cannot be read.

**The partition, and the criterion that draws it.** An error is per-entry exactly when it can be *attributed*: decidable from one entry alone, or from a relation among a named set of entries. It is file-level exactly when no entry can be named, which happens only when the entry list itself is not available.

| Class | Today [measured] | Under this requirement |
|---|---|---|
| bytes unreadable, content not JSON | file-level stop | file-level stop (unchanged) |
| `schema_version` absent or not 2 | file-level stop | file-level stop (unchanged) |
| `servers` absent or not an array | file-level stop | file-level stop (unchanged) |
| display name blank / outside charset | *unenforced* | per-entry |
| display name duplicated | *unenforced* | per-entry, whole collision group |
| URL duplicated | file-level stop | **per-entry, whole collision group** |
| URL invalid (scheme, credentials, host) | file-level stop | **per-entry** |
| auth shape invalid | file-level stop | **per-entry** |
| unknown field on an entry | file-level stop (TS/C#); silently ignored (Go) | **per-entry**, and the divergence is closed |
| `enabled` / `default_execution` not boolean | file-level stop | **per-entry** |
| more than one `default_execution` | file-level stop | **per-entry**, whole marked group |
| `servers` empty, or zero enabled | file-level stop | not an error — an empty usable fleet, fully observable |

The last row is the one that goes a step beyond the classes named above, and deliberately. Once entries can individually become unusable, "no usable server" is already a reachable state that the host must survive and present; keeping a *separate* fatal path to the same observable would mean the same end state stops the host on one route and not on another. One rule — the entry list is readable or the host stops — is what "per-entry" actually costs, and stating it as a list of exceptions would leave the incoherence in place. [measured] This also relaxes `clients/web-host/internal/webhost/host.go:30-31`, which refuses a snapshot with no servers.

<!-- anchor: fr-031 -->
### fr-031 — an unusable entry is visible, and in its own vocabulary

The CLI shall present every configured server entry including entries in the configuration-invalid admission state, shall express configuration invalidity in a vocabulary disjoint from both server reachability and session lifecycle state, and shall name each such entry by its position in the entry list and by its display name as written together with the rule it violates.

Without this, per-entry admission would be strictly worse than the whole-file stop it replaces: the host would start, a server would silently be missing, and the operator would get no signal at all. The stop was at least loud.

**Why a third vocabulary and not a reuse of "unreachable".** DP-JSON-SERVER-REACHABILITY-FIELD settled that machine-readable output carries each server's reachability so a script can tell "fix the connection" from "nothing to do here" (UAC-046), and UAC-040 requires the connection-side word and the session-lifecycle word to differ. A configuration-invalid entry is a third thing: nothing is wrong with the connection, and there is no session. Folding it into `unavailable` would send the operator to the network for a problem in their own file, and would make UAC-046's counterexample true again one level up — an entry the output cannot distinguish from a different condition.

**Where it lands.** [measured] The health observation kind is already a closed set carrying a non-reachability admission value: `checking | connected | unavailable | incompatible_protocol | disabled | health_unavailable` (`clients/ui/src/fleet/session-protocol.ts:73-90`, produced at `clients/web-host/internal/webhost/fleet.go:176-596`, validated by an allow-list at `fleet.go:1046-1112`), and `disabled` is exactly the precedent — an entry that exists, is never probed, and is not a reachability fact. Configuration invalidity joins that set as one more kind with its own closed reason set, rather than becoming a free-form string. [measured] Nothing of the sort exists today: the only configuration-flavoured signal anywhere is fleet-wide prose shown when health delivery is lost before any server could be loaded (`clients/ui/src/fleet/connections-presentation.ts:33-37, 92-95`).

**Naming an entry whose name is the problem.** The position in `servers[]` is always available and always unique; the display name as written may be blank, duplicated, or full of control characters, and is reported as data rather than used as the handle. Reporting position first is what makes the message actionable when the name itself is what must change.

<!-- anchor: fr-032 -->
### fr-032 — a departure carries a published reason, not a client guess (prerequisite)

Every server-initiated close of a client socket shall carry a reason drawn from a closed published vocabulary, and the CLI shall derive both its transport failure classification and its presented reason from that value rather than from the close code alone or from a client-authored string.

[measured] The public contract has no such vocabulary. The gateway has exactly one close helper, `writeTypedClose(c, reason)` (`services/server/api/gateway.go:179-181`), which always uses `StatusGoingAway` (1001) and carries one of four ad-hoc strings — `invalid-geometry` (`:192`), `subscribe-failed` (`:199-201`), `lifecycle-subscribe-failed` (`:303`), `daemon-disconnected` (`:343, :352, :484`). Every other departure — context cancellation, a write error, graceful server shutdown — unwinds into `defer c.CloseNow()` (`services/server/api/mux.go:613`) and sends **no close frame at all**. None of these strings appears in any schema: `protocol/` contains seven `*.schema.json` files and none models the `{"k":"c"}` control frame, and the only `reason` field in `events.schema.json:24` is a free-form justification on an approval request, unrelated to transport. `controlMsg.Code` is `int` with `omitempty` and every production call site passes `0` (`services/server/api/wire.go:45-55`), so the numeric slot is always absent from the wire and the free-string `data` is the sole discriminator.

The consequence is measurable at the client: a rejected ticket is an HTTP 401 with the plain-text body `unauthorized` before the socket is ever accepted (`services/server/api/mux.go:198-204`, `auth.go:20-23`), and `ticketStore.consume` collapses empty, unknown, already-used and expired into a single boolean (`services/server/api/ticket.go:66-81`). A browser sees code 1006 with an empty reason for that, for a graceful server shutdown, and for a pulled network cable alike. The rule this requirement states is what makes `fr-014`'s transport-loss class carry information instead of a shrug.

Note the scope: this is about **connection-level** departures. A subscribe-time refusal already carries a type — the per-connection surface cap answers with `{"k":"e", …, "code":"resource_exhausted", …}` on an open lifecycle socket [measured: `services/host/state/reduce_surface.go:108-110`, `services/server/api/gateway.go:443-471`] — and that typing is kept, not replaced.

<!-- anchor: nfr-001 -->
### nfr-001 — exact-match protocol admission, fail closed

The CLI shall carry exactly one compiled protocol version, shall treat any difference, absence, or malformation as `incompatible_protocol`, and shall not enter any operational REST handler or WebSocket application frame for that server.

[measured] `adr-20260727-exact-match-client-server-protocol`: REST requires the `X-Agent-Grid-Protocol-Version` header and answers a mismatch with HTTP 426 and `{"code":"incompatible_protocol",...}`; only `GET /api/capabilities` is ungated. Version ranges, capability degradation, and fallback decoders are permanently forbidden by `contracts/compatibility-policy.md`.

<!-- anchor: nfr-002 -->
### nfr-002 — no dependency edge to the services module (invariant)

The CLI module shall not require, replace, or import the repository's services module, and this shall be enforced by the module graph rather than by review.

The mechanism named by `adr-20260727-independent-web-host-module` (a nested `go.mod`, `GOWORK=off`, and no root module edge). The root-module alternative was rejected in `adr-20260727-root-go-module-flag-day` [measured].

<!-- anchor: nfr-003 -->
### nfr-003 — configuration discovery matches the existing hosts per platform

The CLI shall resolve its configuration directory to the same location the existing client hosts use on the same platform, and shall accept the same explicit directory override.

[measured] The existing implementations diverge: Go's `clientconfig.DefaultDirectory()` returns `$XDG_CONFIG_HOME/agent-grid` or `~/.config/agent-grid` with **no Windows branch**, while TypeScript and C# use `%APPDATA%\agent-grid\config` on Windows. Copying the Go loader as-is would make `ag ls` on Windows see a different fleet from the GUI. The override is `--config-dir`, the existing contract shared by all three hosts.

<!-- anchor: nfr-004 -->
### nfr-004 — no client-side replay or snapshot store (invariant)

The CLI shall not persist, reconstruct, or replay terminal screen state; the subscribe-time snapshot delivered by the execution plane shall be the only recovery mechanism.

Upstream `cc-execution-plane-ownership`. [measured] No scrollback or replay API exists in the repository; the termvt reattach snapshot is the only recovery mechanism.

<!-- anchor: nfr-005 -->
### nfr-005 — a new module is not ungated by default

The CLI module shall be registered in the repository's build, test, vet, lint, verification-profile, and protocol-version-projection entry points at introduction time.

[measured] `make lint` runs only inside `services`, so `clients/web-host` has no lint gate today. The same hole must not be reproduced.

<!-- anchor: nfr-006 -->
### nfr-006 — concurrent attachment by other clients is never displayed (invariant)

The CLI shall not present any indication of other clients attached to the same session.

[measured] No presence or subscriber-count field exists on `SessionInfo`, in the hello frame, in `viewUpdate`, or in any `k:` frame. DP-CONCURRENT-ATTACH-VISIBILITY, UAC-021.

<!-- anchor: nfr-007 -->
### nfr-007 — identity construction and frame serialization have one owner each (invariant)

Server-qualified session identity construction and outbound frame serialization shall each have exactly one owning package inside the CLI module, enforced by a test that fails when another package performs either.

The design already applies single-origin enforcement to the termination request. Identity construction is the other place where `fr-019`'s separate-fields rule and the bare-upstream-id-on-the-wire disposition can silently diverge across packages. The repository's existing ownership meta-test covers the SPA tree only and explicitly does not scan other client trees [measured], so the CLI needs its own equivalent. A source-level guard converts a review finding into a test failure.

<!-- anchor: nfr-008 -->
### nfr-008 — the payload shape the CLI reads is pinned across languages

The session payload fields the CLI reads shall be pinned by a checked-in golden vector that the server's own serialization is tested against, so that renaming or moving one of those fields fails a repository gate.

[measured] `SessionInfo` and `View` appear in neither `protocol/*.schema.json` nor `clients/sdk/quicktype-emit.json`; the TypeScript side is a hand mirror. The status matrix is pinned to a canonical file, but `view.card.title` and the working-directory field are pinned nowhere. A test that asserts the CLI's own behaviour cannot detect a server-side rename; only a test on the server's serializer can.

<!-- anchor: nfr-009 -->
### nfr-009 — the client configuration contract has one loader per language, pinned by one corpus

The client configuration contract shall have exactly one loader implementation per client language, and the accept-or-reject decision, the per-entry admission state, the canonical URL and the derived server identifier produced by the Go, TypeScript and C# loaders shall be pinned to one shared fixture corpus in which every fixture carries its own expected outcome.

[measured] `clients/config/README.md:3-4` already declares the contract shared — "Web host, Windows Shell, and Workspace share the `servers.json` version 2 contract" — and three loaders read it, with the SPA consuming a credential-free projection of the result (`GET /host/config` on the web host, `agent-grid:session-fleet-configs` over preload IPC in Workspace). What does not exist is any mechanism that makes the three agree. `clients/config/servers.schema.json` is read by no code at all; each loader hand-rolls its validation. The three fixture tests that do exist assert only shape: Go reads **one** of the four fixtures (`clients/web-host/internal/clientconfig/config_test.go:10-35`), TypeScript asserts a bare `.not.toThrow()` / `.toThrow()` (`clients/workspace/tests/desktop-config.test.ts:339-365`), and C# asserts `Assert.Throws<DesktopConfigException>` (`.../DesktopConfigTests.cs:215-243`). None compares *which* error, and no fixture covers duplicate URLs, multiple default-execution entries, zero enabled entries, or an unparseable document — the four cases the README's own sentence promises.

The divergence that produces is already measurable, in three places at once:

| Behaviour | Go | TypeScript | C# |
|---|---|---|---|
| unknown field on an entry | silently dropped (`encoding/json` default) | rejected, `"unsupported field '…'"` (`desktop-config.ts:276-282`) | rejected via `UnmappedMemberHandling.Disallow` (`DesktopConfig.cs:66`) |
| credentials / query / fragment in a URL | rejected (`config.go:143`) | **silently stripped** (`desktop-config.ts:328-332`) | rejected (`DesktopConfig.cs:159-162`) |
| configuration directory on Windows | **no branch** — `~/.config/agent-grid` (`config.go:40-49`) | `%APPDATA%\agent-grid\config` (`desktop-config.ts:82-89`) | `%APPDATA%\agent-grid\config` (`DesktopConfig.cs:69-75`) |
| duplicate-URL error names the URL | yes (`config.go:91-93`) | yes (`desktop-config.ts:302-304`) | **no** — `"server ids must be unique"` (`DesktopConfig.cs:132-133`) |

The second row is the sharpest: the same `servers.json` yields a different canonical URL, therefore a different `srv_` digest, therefore a **different server identity** in Workspace than in the web host. `fr-019` and DP-JSON-SESSION-IDENTIFIER-SHAPE make that identity the value a script passes back as a server qualifier. A fifth consumer added on top of that without a parity mechanism inherits a coin flip, which is why the parity obligation is a requirement of this change rather than a follow-up.

---

## C. Components

<!-- anchor: comp-entry -->
### comp-entry — verb dispatch and the single exit

`clients/cli/cmd/agent-grid/main.go`, `clients/cli/internal/app/`. The first argument is always a verb and a selector is never accepted positionally (DP-ENTRY-DISPATCH-STYLE). The whole failure-class-to-exit-code mapping lives here, once. No `os.Exit` outside `clients/cli/internal/app/`. Planned.

<!-- anchor: comp-goclient-config -->
### comp-goclient-config — the Go projection of the client configuration contract

`clients/goclient/go.mod`, `clients/goclient/clientconfig/`. Schema v2 reading, per-platform configuration directory resolution, `--config-dir` override, the `srv_` + SHA-256 of the canonical URL server identity, per-entry admission (`fr-030`) and the naming-contract validation of `fr-028`. Read-only (`fr-003`).

This component is **one language's projection** of the contract owned by `comp-config-contract`, not the contract itself and not an extraction of any particular host's loader. It exists because Go has two client hosts — the web host and the CLI — and `nfr-009` allows one loader per language; see `adr-shared-go-client-config-module`. Planned (the existing package is moved out of `clients/web-host/internal/clientconfig`, not written fresh, because moving preserves the canonical-URL and digest derivations that `fr-019`'s identity rests on).

<!-- anchor: comp-fleet -->
### comp-fleet — reachability observation and partial-failure aggregation

`clients/cli/internal/fleet/`. Probes `GET /api/capabilities` per server and maps to `checking` / `connected` / `unavailable` / `incompatible_protocol` / `disabled` with the existing closed reason set. One server's failure never suppresses another's result. Two implementations already exist (`clients/web-host/internal/webhost/fleet.go` and the TypeScript one) [measured]; the CLI reuses their vocabulary verbatim and invents no new word. Planned.

<!-- anchor: comp-transport -->
### comp-transport — auth bootstrap and WebSocket admission

`clients/cli/internal/gateway/client.go`, `auth.go`. REST carries `Authorization: Bearer` plus `X-Agent-Grid-Protocol-Version`; the socket is `POST /api/ws-ticket` → `{"ticket","client_instance_id"}` → `GET /ws?ticket=`. [measured] The ticket is single-use with a 30-second TTL and the `client_instance_id` is bound to it and returned in the hello frame. Adding header auth to the WebSocket upgrade is forbidden by `adr-20260724-boundary-2-native-ws-auth-reuse-ticket`. This component is also the single point where a transport error becomes a redacted reason (`fr-027`). Planned.

<!-- anchor: comp-wire -->
### comp-wire — hand-written stdlib-only frame codec and identity owner

`clients/cli/internal/gateway/wire.go`, `identity.go`. Encoding and decoding of the inbound (`i`, `r`, `ld`) and outbound (`h`, `v`, `p`, `c`, `lo`, `e`, `r`, output array) frames, plus session id requalification. It is the single constructor of a server-qualified identity and the single serialiser of an outbound frame (`nfr-007`).

[measured] Terminal and lifecycle frames are defined only in Go code — they are absent from `protocol/*.schema.json`, and no Go SDK is generated (`clients/sdk` emits TypeScript, C#, Kotlin and Swift). A hand-written stdlib-only codec is therefore the only shape consistent with the accepted SDK ADR [measured: `adr-20260724-protocol-cross-language-sdks-supersedes-0021`]. Planned.

<!-- anchor: comp-selector -->
### comp-selector — resolution under a declared kind

`clients/cli/internal/selector/`. Builds a fleet-wide candidate set from the declared kind alone (full identifier or identifier prefix) and separates zero / one / several / unreachable-origin outcomes. Applies the server qualifier over every configured entry, usable or not, and resolves the outcome by `fr-029`'s precedence. Planned.

<!-- anchor: comp-view -->
### comp-view — columns and the machine-readable DTO

`clients/cli/internal/view/`. Labels and precedence from the shared display matrix, the default columns, and the `--json` DTO. Reachability is a property of the server element, never collapsed into the session's lifecycle value space. Planned.

<!-- anchor: comp-attach -->
### comp-attach — the attach runtime

`clients/cli/internal/attach/`. Subscription establishment, the raw-mode critical section, both pumps, lease renewal, the detach boundary, and departure classification. It owns the input and output pass-through contracts, because the delegated choices inside them live in this component's files and an escalation must have one adjudicating owner. Planned.

<!-- anchor: comp-tty -->
### comp-tty — the platform terminal adapter and sole owner of terminal I/O

`clients/cli/internal/tty/` (`tty.go`, `tty_unix.go`, `tty_windows.go`, `tty_fake.go`). Raw entry and restore, size query, size-change events, input-queue discard, terminal detection. `comp-attach` touches the terminal only through this interface. It is also the primary test seam: a fake terminal drives every exit path deterministically. Planned.

<!-- anchor: comp-lifecycle -->
### comp-lifecycle — creation and explicit termination

`clients/cli/internal/lifecycle/`. Creation is `POST /api/sessions` (201 with `{"id","project","command","created_at"}`) after reading the profile set from `GET /api/session-config`; termination is `DELETE /api/sessions/{id}` (204). [measured] Session creation and termination do **not** exist on the lifecycle socket — `k:"ld"` is the desired state of a surface subscription, not of a session. The "hypothetical `POST /api/sessions`" the source plan warned about is not hypothetical; it is the current public route. Planned.

<!-- anchor: comp-fakegateway -->
### comp-fakegateway — in-process fake gateway

`clients/cli/internal/gateway/fakegateway/`. Drives ticket minting, admission, hello, subscription, output, capacity refusal, and every disconnect shape deterministically. It is the fake element of the repository's three-part external-dependency rule (`adr-20260705-test-tier-taxonomy`). Planned.

<!-- anchor: comp-config-contract -->
### comp-config-contract — the canonical client configuration contract (prerequisite, not CLI-owned)

`clients/config/servers.schema.json`, `clients/config/README.md`, `clients/config/fixtures`, the credential-free projection (`clients/web-host/internal/clientconfig/projection.go`), and the three language loaders.

This component owns the **contract**, not any host's loader: the document shape, the admission rules, the canonical URL and server-identity derivations, the per-platform directory resolution, and the parity corpus that binds the three languages to all of it. Every client host — CLI, web host, Workspace, Windows Shell — is a consumer of equal rank; the SPA consumes the derived projection rather than the file [measured: `GET /host/config` at `clients/web-host/internal/webhost/host.go:41-46`, and `agent-grid:session-fleet-configs` over preload IPC at `clients/workspace/src/preload/electron-preload.cjs:28-74`].

[measured] Neither display-name uniqueness nor a character set is constrained today, and the schema file that would constrain them is read by no code. `cc-server-display-name-unique` and `cc-server-display-name-cli-safe-charset` are satisfied here and nowhere else. Governed by `design-windows-shell`, whose name has diverged from that scope (`open-config-contract-design-ownership`). Existing.

**Explicitly not in this component.** A host's own runtime settings. [measured] The web host takes its listen address, TLS material and `--config-dir` from flags only (`clients/web-host/cmd/web-host/main.go:28-37`) and embeds its static assets (`internal/webhost/embed.go:22-23`); Workspace and Windows Shell read `appearance.json` / `shell.json` / `workspace.json` beside `servers.json`. Those are per-host concerns that happen to share a directory, and folding them into a cross-language parity obligation would bind three languages to settings only one of them has. Recorded as `open-web-host-server-side-config`.

<!-- anchor: comp-execution-plane-lifecycle -->
### comp-execution-plane-lifecycle — session record lifetime and lifecycle publication (prerequisite, not CLI-owned)

`services/host/state/reduce_session.go`, `reduce_frame_evict.go`, `reduce_tick.go`, `services/host/state/view/status.go`, `services/server/api/gateway.go`. This component owns whether a session survives its own end and whether that transition is published. It is listed because `fr-026` cannot be satisfied anywhere else, and it is listed as a prerequisite so that no part of the responsibility drifts into the CLI. Existing.

<!-- anchor: comp-gateway-transport -->
### comp-gateway-transport — socket admission and departure (prerequisite, not CLI-owned)

`services/server/api/gateway.go`, `mux.go`, `auth.go`, `protocol_version.go`, `push_wire.go`, and `protocol/`. This component owns every way a client socket can be refused or ended: the single close helper, the pre-upgrade ticket and version gates, the control-frame shape, and — after `fr-032` — the published vocabulary that names the cause.

It is listed separately from `comp-execution-plane-lifecycle` because the two answer different questions. That component owns whether a *session* survives its own end; this one owns whether a *connection*'s end is explicable. They share a file (`gateway.go`) and nothing else, and conflating them would put a session-retention change and a transport-vocabulary change in one unit that has to land atomically for no reason. Existing.

<!-- anchor: comp-enforcement -->
### comp-enforcement — build and gate registration

`Makefile`, `test-harness/profiles.json`, `test-harness/dependencies.json`, `services/gorules/static_enforcement_test.go`, `protocol/session-status-display.json`. Connects the new module and the new golden vector to the existing gates. Existing (appended to).

---

## D. Implementation contracts

<!-- anchor: contract-attach-transport-topology -->
### contract-attach-transport-topology — attach uses the lifecycle-multiplexed socket

Owner: `comp-attach`.

**Decision rule.** Attach opens `GET /ws?ticket=<t>` with **no** session query parameter and makes the target session's surface desired via `k:"ld"`. `GET /ws?session=` (`AttachWS`) is not used.

**Why `?session=` is disqualified — two independent counterexamples, each sufficient** [measured]:

1. `writeOutbound` (`services/server/api/gateway.go:476-520`) forwards only `EvtSurfaceOutput`, `EvtSessionFileLine` and `EvtAgentNotification`, and explicitly drops `EvtSessionsChanged`; the channel closes only when the *daemon* disconnects. So if the attached session ends server-side, no frame reaches the CLI at all and the user keeps typing at a frozen screen. `fr-014` and UAC-018 both fail.
2. `DaemonAdapter.SubscribeSurface` sends an empty `SubscriberID` (`gateway.go:100`), and the daemon's subscription key is `(ConnID, SessionID, SubscriberID)` with a `ConnID` shared by every WebSocket client. A second `?session=` attach to the same session collides on the key, `reduceSurfaceSubscribe` returns ok without emitting `EffSurfaceSubscribeStart` (`services/host/state/reduce_surface.go:117-119`), the second client never receives a snapshot (`fr-007` / UAC-009 fail), and the first client's detach unsubscribes the shared key and drops both (UAC-013 fails).

The lifecycle route assigns a distinct subscriber identity per socket (`web-<n>` for surface subscription and `owner-<random24>` for lifecycle ownership, `gateway.go:308-310`) and is the route the existing browser and workspace clients already use. This is a choice between two existing public routes, not a new CLI-specific one.

**Observable effects.** The target session's surface output appears on stdout; lifecycle state changes are observable through `k:"v"`; concurrent attachment to the same session does not break either party's snapshot or subscription.

**Operational inputs.** `client_instance_id` — owner `comp-transport`, producer the server's ticket mint, boundary the `POST /api/ws-ticket` response, needed before the socket is opened, not preserved across invocations, and its absence fails attach as an authentication-class failure before the terminal is touched. `connectionGeneration` and `clientRevision` — owner and producer `comp-attach` itself, monotonically increasing, with zero rejected by the server as `invalid_argument` [measured].

**Outcome partition.** `determinate` (`lo` reports `applied`); `unknown` (`accepted` or `waiting` still outstanding at the apply deadline); `inconclusive` (an `e` frame, including `resource_exhausted`); `conflicting` (`superseded` — a newer revision took the single slot). `accepted` and `waiting` are non-terminal and must never be reported as established [measured: `RevisionOutcomeKind` = accepted / waiting / applied / rejected / superseded / released / degraded / no_output]. Coverage rests on that enumeration being the complete set of outcome kinds; exclusivity rests on one outcome per revision.

**Capacity, and why the acceptance criterion had to be qualified.** [measured] `reduce_surface.go:108-110` refuses a new subscription with `resource_exhausted` and the message `per-conn surface subscribe cap (8) exceeded` once `len(existing) >= 8`, keyed on `ConnID` — which is the identity of a daemon-side Unix-socket IPC connection (`services/host/runtime/ipc.go:101-108`, `services/host/state/state.go:70-71`), and the gateway process holds exactly one. Eight browser tabs therefore consume the whole fleet-wide budget, and the per-browser identities `web-<n>` / `owner-<random24>` (`gateway.go:309-310`) take no part in the check. UAC-020 requires that a graphical attach neither block nor gate the CLI attach, and that still holds: nothing here is an ownership policy, a lock, or a takeover. What does not hold is an unconditional claim of success under load.

**Where the capacity discriminator comes from — a correction.** The earlier revision said the CLI maps this to the transport code "with a capacity reason", meaning a reason the CLI writes. Two measured facts change that. First, on the route this contract selects — the lifecycle socket — the refusal is **already typed and arrives on an open socket**: `{"k":"e","reqId":…,"code":"resource_exhausted","message":…}` (`gateway.go:443-471` via `gateway_lifecycle_inbound.go:206-217`). It is not a close, and the CLI should present the published `code`, not a sentence of its own. Second, the untyped case is the *connection-level* one, and that is `contract-transport-close-taxonomy`'s subject, not this contract's. So the interim "exit 6 plus a capacity reason string" is replaced by "exit 6 plus the reason the plane published", and the dedicated-code question is answered rather than escalated — see `adr-cli-failure-class-exit-codes`. `ac-concurrent-attach` states the qualified property.

**Adversarial witness (capacity, risk: resource).** Open eight browser tabs against one server, then `ag attach`. Expected: attach fails with exit code 6, the reason is the `resource_exhausted` code the plane published, the terminal is never switched to raw mode, and the session is unaffected. Forbidden: reporting not-found, reporting a clean detach, entering raw mode before the refusal is known, and substituting a CLI-authored sentence for the published code. Verified by `ver-attach-capacity-refusal-is-transport-with-the-published-reason`.

**Adversarial witness (concurrency, risk: correctness).** Attach twice to the same session over two lifecycle sockets. Expected: both receive their own snapshot, and one detaching does not drop the other. Verified by `ver-attach-topology-snapshot-two-clients`.

<!-- anchor: contract-attach-lease-progress -->
### contract-attach-lease-progress — renewal is not starved by output

Owner: `comp-attach`.

**Decision rule.** Lease renewal is driven on a time base independent of the output pump, and sending a renewal never waits for a stdout write to complete. The interval is at most half the lease TTL, and `clientRevision` increases monotonically.

**Why it is a contract** [measured]: `lifecycleLeaseTTL = 12s`; an expiry timer is armed for each `desired=true`; expiry releases the subscription with `RevisionReleased` reason `lease_expired`. The existing client renews at 4 seconds.

**Adversarial witness (backpressure, risk: liveness).** Attach with stdout connected to a pipe whose reader has stopped (a paused pager), while the remote produces continuous output. Expected: renewals continue, and output resumes when the reader resumes. Forbidden: losing the subscription after the TTL and freezing the screen permanently — the failure mode of a single goroutine that interleaves "read socket, write stdout, renew". The session survives either way (`fr-001`). Verified by `ver-lease-renews-under-stdout-backpressure`.

**Forbidden.** Detecting lease expiry and silently re-subscribing without telling the user; that collapses `fr-014`'s classes.

**Delegated.** `discretion-pump-goroutine-topology` — the goroutine and channel arrangement of the two pumps and the renewal timer. Private to `unit-attach-runtime`'s files, mechanically checked by the same backpressure test, and escalated if renewal can only be made to work by depending on output progress, if more than one path can restore the terminal, or if cancellation would require a change to the gateway frame vocabulary.

<!-- anchor: contract-terminal-mode-lifecycle -->
### contract-terminal-mode-lifecycle — the raw-mode critical section

Owner: `comp-tty` (the sole owner of terminal state); consumer: `comp-attach`.

**Decision rule.** Raw entry happens exactly once, and only after selector resolution, ticket acquisition, socket establishment, initial geometry acceptance, and a terminal `applied` subscription outcome have all completed. Restore is implemented as the single exit from the critical section, and every departure — ordinary detach, socket close, server disconnect, a post-entry authentication or protocol failure, a catchable terminating signal, suspend, stdin EOF, the panic boundary — passes through it.

**Invariants.** (a) A failure before raw entry never touches the terminal (UAC-016). (b) Restore is idempotent and a double restore does not corrupt the terminal. (c) Suspend restores first, then falls through to the default behaviour, and re-enters raw mode when the process is foregrounded (UAC-042). (d) A non-TTY stdin never enters the critical section (`fr-004` / UAC-015). (e) A catchable terminating signal (SIGINT / SIGTERM) received while raw mode is held is a departure **inside** the critical section, not beside it: the handler restores first and the process exits afterwards, and since restore is not a termination request the attached session is still running in the following listing (`fr-002` + `fr-001` / UAC-017).

**Why (e) is stated rather than left to the enumeration.** The decision rule lists a catchable signal among the departures, but a signal path is the one departure that can bypass an ordinary deferred restore without any code saying so — the process is gone before the deferred call runs. Making it an invariant is what forces a registered handler, and pairing it with `fr-001` is what stops the handler from "cleaning up" the session it was attached to. Both halves of UAC-017 — the shell that echoes again and the session that is still there — are claimed here.

**Normal witness (risk: correctness).** Attach, detach, type in the shell. Expected: characters echo and line editing works.

**Adversarial witness 1 (ordering, risk: resource).** Inject a failure immediately before subscription establishment. Expected: the terminal settings are byte-identical to their pre-invocation value throughout. This is the witness that kills the "enter raw mode at startup and restore with defer" shape. Verified by `ver-pre-subscribe-failure-leaves-terminal-untouched`.

**Adversarial witness 2 (signals, risk: lifecycle).** Send SIGTSTP during raw mode, type a few characters in the shell, then foreground. Expected: keystrokes echo while suspended, and raw mode returns afterwards. Rejects both of UAC-042's counterexamples.

**Adversarial witness 3 (catchable termination, risk: lifecycle).** Hold raw mode and send SIGINT; in a second run, send SIGTERM. Expected: in both runs echo and line editing are back before the process exits, and the following listing still shows the target session running because no termination request was emitted on this path. Forbidden: exiting on the signal without passing through restore, and a handler that terminates the session as part of its cleanup. Rejects UAC-017's counterexample. Verified by `ver-restore-on-every-exit-path` for the restore half — the signal path is one of the enumerated exit paths — and by `ver-detach-sends-no-terminate` for the still-running half, which asserts over *any* attach shutdown path and therefore over this one.

**Adversarial witness 4 (uncatchable kill, risk: mutation).** Kill the CLI with an uncatchable signal. Expected: the session keeps running and the same selector reattaches (UAC-024). The terminal being left raw is out of this witness's scope; what is in scope is that no "terminate the session if this connection is lost" registration was ever made — and no such registration exists in the public contract [measured].

<!-- anchor: contract-detach-boundary -->
### contract-detach-boundary — the input is cut at the detach byte

Owner: `comp-attach`; terminal-seam participant: `comp-tty`.

**Decision rule.** On finding the detach byte inside a read buffer, `comp-attach` forwards everything **before** it normally, discards that byte and everything **after** it, and stops reading stdin; it then calls, through `comp-tty`'s interface and in this order, the terminal input-queue discard and the restore. Send no termination request. The cut position, the stdin stop, and the order of the last two calls belong to `comp-attach`; the two terminal effects belong to `comp-tty` and are exposed as two separately callable operations rather than folded into a single restore, because a discard that only ever happens inside restore cannot be observed to have happened *before* it.

**Which carrier holds which part.** `unit-attach-runtime` holds the buffer cut, the stdin stop, and the call order, and its files are where all four verifications run. `unit-tty-adapter` holds the two terminal operations this rule ends in — input-queue discard and restore, already required by that unit to be reachable only through the one interface and drivable by a fake — so it is the carrier that makes `ver-input-queue-flushed-before-restore` a T1 assertion over a recorded call order instead of a T3 observation on real hardware. `unit-platform-hardening-spikes` holds the platform residue: the Windows implementation of those same two operations, and the recorded answer to whether the default detach byte collides with the remote agents' own key handling. Ownership stays with `comp-attach` because what can be got wrong here is the cut and the ordering; `comp-tty` supplies the effects and does not decide when they fire.

**Why the queue discard is part of the contract** [inferred, falsifiable]: merely stopping the read leaves bytes in the kernel tty input queue for the restored shell to consume as its own input. UAC-041 requires that nothing typed after the detach key appears in the returning shell's input line, and without the discard the implementation is observationally identical to the counterexample that buffers post-detach keystrokes and replays them into the shell. Falsifier: on real hardware, type immediately after detaching; an empty shell input line would show the discard was unnecessary.

**Normal witness (risk: correctness).** Type a known string, press the detach key, type a second known string. Expected: the shell prompt's input line is empty, and reattaching shows the first string on screen and not the second. This rejects both of UAC-041's counterexamples at once.

**Adversarial witness (configuration, risk: correctness).** Configure a non-default detach byte, attach, and press `0x1D` first. Expected: it reaches the remote side visibly and the attach continues; pressing the configured byte then detaches (UAC-014).

<!-- anchor: contract-input-passthrough -->
### contract-input-passthrough — input byte fidelity and the send boundary

Owner: `comp-attach`; codec participant: `comp-wire`.

Ownership sits with `comp-attach` because the delegated choice inside this contract lives in `clients/cli/internal/attach/input.go`; an escalation raised there must reach the same owner that holds the contract.

**Decision rule.** Bytes read from raw stdin are placed into surface input frames unaltered and in read order, except for the cut made by `contract-detach-boundary`. A frame boundary may fall only where it does not split a UTF-8 sequence; an incomplete trailing sequence is held until the remaining bytes arrive or the stream ends.

**Why the boundary changes what is observed** [measured]: the input payload `d` is a JSON string, not base64 (`gateway.go:544-547` writes `[]byte(in.D)`). A JSON string carries only well-formed UTF-8, so a split multi-byte character is replaced with U+FFFD by the encoder and a different character reaches the remote side. The upstream handoff's premise — "order preservation is enough" — is true only *given* the boundary rule, so the boundary rule is contracted and only the granularity is delegated.

**Normal witness (risk: correctness).** Type a known string containing multi-byte characters one byte at a time, so reads are guaranteed to split it. Expected: the remote side's response matches the original string.

**Adversarial witness (risk: data).** Send a four-byte emoji split at a read boundary. Expected: no U+FFFD reaches the remote side. Verified by `ver-utf8-sequence-never-split` and the fuzz over arbitrary split points.

**Forbidden.** Base64-encoding the input; the server would write the literal characters into the PTY.

**Delegated.** `discretion-input-read-buffering` — read buffer size and coalescing granularity, inside the boundary rule and the detach cut position.

<!-- anchor: contract-output-passthrough -->
### contract-output-passthrough — output byte fidelity and session selection

Owner: `comp-attach`; codec participant: `comp-wire`.

**Decision rule.** Decode the output frame as the four-element array `[timeSec,"o",dataB64,sessionId]`, accept only those whose element 3 equals the attached session id, base64-decode element 2, and write the **raw bytes** to stdout. Do not interpret, re-render, or re-encode escape sequences.

**Adversarial witness 1 (byte fidelity, risk: data).** Have the remote emit output containing non-UTF-8 bytes (256-colour sequences, binary). Expected: the stdout byte sequence equals the original. An implementation that re-marshals element 2 through a Go string emits U+FFFD here [measured: `wire.go:28-37` records the same trap server-side].

**Adversarial witness 2 (session selection, risk: correctness).** Subscribe two sessions on the same lifecycle socket and have the non-target session produce output. Expected: not one byte of it appears on stdout. [measured] Element 3 was added because a past code review found exactly this leak during session switching.

**Note.** `sequence` is stripped from browser-visible surface output [measured: the encoder drops `EvtSurfaceOutput.Sequence`], so the CLI cannot detect a gap. Ordering is guaranteed by the socket; the public contract offers no other means.

<!-- anchor: contract-wire-shape-conformance -->
### contract-wire-shape-conformance — the payload shape the CLI reads is gated

Owner: `comp-wire`.

**Decision rule.** A canonical `SessionInfo` payload — carrying at minimum the identifier, `view.card.title`, the working-directory field chosen by the Phase 0 observation, and `status` — is captured once as a checked-in golden vector. Two tests read it: the server's own serializer must reproduce it, and the CLI's decoder must consume it. The golden vector is registered in the repository's enumerating gate so that adding a consumer without registering it fails.

**Why a CLI-side test is not enough.** [measured] `SessionInfo` and `View` are absent from `protocol/*.schema.json` and from `clients/sdk/quicktype-emit.json`. A test that asserts the CLI takes the display name from `view.card.title` proves only the CLI's behaviour; renaming `view.card` on the server leaves it green while the CLI silently breaks. Only a test over the server's serialization fails on a rename.

**Alternatives considered.** Adding `SessionInfo` to `protocol/*.schema.json` and registering it in the quicktype emitter would be the stronger fix and would benefit the TypeScript mirror as well, but it changes the generated SDK surface for four languages and needs its own ADR; that is recorded as a follow-up rather than taken here. Vendoring the Go types from `services` is excluded by `nfr-002`.

**Normal witness (risk: correctness).** The CLI decodes the checked-in vector and produces the expected row.

**Adversarial witness (migration, risk: migration).** Rename `view.card.title` on the server. Expected: `ver-server-serialization-matches-golden-vector` fails. Forbidden: every gate staying green while the CLI's display name becomes empty.

**Contract evolution.** Changed surface: the serialized shape of `SessionInfo.View` and the fields the CLI reads. Typed consumers: the CLI decoder, the SPA hand mirror, the C# panel presentation. Compatibility: additive fields are compatible; renaming or moving a pinned field is breaking and must update the golden vector in the same change. Rollout: the golden vector lands with the Phase 0 unit, before any consumer depends on it. Rollback: deleting the vector and its two tests restores the current (ungated) state. Baseline verification is the current absence of any gate; target verification is `ver-server-serialization-matches-golden-vector` failing on an injected rename.

<!-- anchor: contract-identity-single-origin -->
### contract-identity-single-origin — one constructor, one serializer

Owner: `comp-wire`.

**Decision rule.** Exactly one package constructs a server-qualified session identity, and exactly one package serialises an outbound frame. Every other package receives an already-constructed identity and calls the serializer; none re-derives either. A source-level test enumerates the permitted sites and fails when a second appears.

**Why a test rather than a review rule.** The repository already applies this pattern to the SPA and records that its scan does not reach other client trees [measured]. The CLI carries two rules that can diverge silently across packages — the separate-fields rule of `fr-019` and the "bare upstream id on the wire" disposition — and a divergence is invisible in review because both spellings compile.

**Adversarial witness (risk: correctness).** Add a second construction site in a test fixture. Expected: `ver-ownership-gate-fails-on-synthesized-violation` fails, proving the gate itself fires.

<!-- anchor: contract-geometry-propagation -->
### contract-geometry-propagation — declaring and propagating geometry

Owner: `comp-attach`; adapter: `comp-tty`.

**Decision rule.** Carry the current columns and rows on the subscription request. Afterwards send the latest value on every terminal size-change event. Clamp into `1..2000` before sending. Coalesce bursts to the latest value; keep no queue.

**Why clamping is contractual** [measured]: an out-of-range resize is discarded by the server with only a warning log (`tryResize`) — no error frame, no acknowledgement — so without clamping "sent but not applied" is unobservable. The initial geometry is stricter still: `readInitialGeometry` closes the socket as `invalid-geometry` on a bad value.

**Platform branch.** Unix uses `SIGWINCH`. The Windows console has no `SIGWINCH` and observes size changes by another mechanism [inferred]. `comp-tty` owns the event source per platform and `comp-attach` sees one channel. The Windows mechanism is **[unmeasured]** and routed to a spike.

**Adversarial witness (risk: correctness).** Resize while the remote is continuously redrawing a full-screen UI. Expected: a redraw at the new dimensions. Rejects the implementation that sends the value but loses it during a redraw (UAC-011).

**Out of scope.** The rendering outcome of concurrent attachments at different geometries. [measured] Subscribe and resize are last-writer-wins for the whole session; `SurfaceGeometry` is recorded in state but is never read for arbitration, and no geometry-change notification frame exists. The upstream plan hands this to the execution plane (`handoff-shared-pty-geometry-resolution`) and this design leaves it there — while recording that the CLI is now a party that can silently overwrite a browser's geometry.

<!-- anchor: contract-exit-classification -->
### contract-exit-classification — departure and failure classification

Owner: `comp-entry`; departure observation: `comp-attach`.

**Decision rule — departure.** (a) Explicit detach: the CLI's own detach byte was consumed — the only class observed locally. (b) Server-side session end: an observation on the lifecycle plane attributes the end to the session (the target's state transition or disappearance in `k:"v"`, or a surface severance naming the session). (c) Transport loss: the connection ended, and the reason is the published value `contract-transport-close-taxonomy` carries, or — when nothing was published — an explicitly unattributed loss. In every case the terminal is restored first and the reason is presented afterwards. The three reason texts do not coincide, and the detach text coincides with neither failure text.

**Decision rule — exit codes.** `0` success, `1` internal fault, `2` usage error, `3` not found, `4` ambiguous selector, `5` authentication or compatibility failure, `6` transport failure. A selector that matches only on an unreachable server is `6`, not `3`. A qualifier whose every match is a `configuration-invalid` entry is `2`, not `3` and not `6` (`fr-029`). See `adr-cli-failure-class-exit-codes`, whose numbering is accepted and is not reopened here.

**Decision rule — the published reason decides the class.** Within class (c), the CLI does not invent a classification; it reads one:

| Published reason | Exit | Why |
|---|---|---|
| `unauthenticated` | `5` | the credential or ticket was refused — retrying the connection cannot help |
| `incompatible_protocol` | `5` | fail-closed admission, same class as the REST 426 path (`nfr-001`) |
| `capacity` | `6` | the plane refused for a resource budget; retrying later can help |
| `going_away` | `6` | the server is draining; retrying later can help |
| `backend_unavailable` | `6` | the daemon behind the gateway is gone |
| `internal_error` | `6` | the server failed and will not attribute it further |
| `invalid_request` | `1` | the CLI sent a frame outside the contract — its own fault, not the user's |
| *(nothing published)* | `6` | unattributed loss; presented as such, never dressed as one of the named reasons |

**Outcome partition and the sentinel.** `determinate` (a published reason, a locally observed detach byte, or a published session-end transition) and `inconclusive` (the connection went away carrying nothing). Coverage rests on the departure either carrying a reason or not; exclusivity on detach being locally observed and therefore never confused with a remote cause. The unattributed case is the **only** one a client may classify on its own, and it is presented as an absence rather than as a guess — which is the difference between "the connection ended and the server did not say why" and the current state of the art, where every remote cause reads the same.

**Two discriminators, both now sourced rather than guessed.** The earlier revision contracted a three-way departure distinction against an unmeasured discriminator for class (b) and recorded a degraded fallback. Both discriminators now have owners:

- Class (b) — the session-end observation — is supplied by `contract-session-terminal-state-retention`, whose ADR the consultation **accepted**. The Phase 0 question `evidence-session-end-frame-shape` still runs, because knowing which frame carries it shapes the implementation; but a negative result no longer degrades the contract, since the retention prerequisite publishes the transition either way.
- Class (c)'s internal structure is supplied by `contract-transport-close-taxonomy`. Without it, `fr-014`'s transport-loss class is a single word covering a rejected ticket, a draining server and a pulled cable [measured — see `fr-032`], which satisfies the letter of "distinguishes transport loss from the other two" while telling the operator nothing.

**Forbidden (credentials).** The bearer credential, the minted ticket, and any URL carrying either must not appear in the reason text on standard error, in a transport error string, in a log line, or in the process arguments (`fr-027`). Redaction happens where a transport error becomes a reason, in `comp-transport`, so no later call site can reintroduce it.

**Adversarial witness (secrecy, risk: security).** Present an invalid bearer credential and let the authentication failure print its reason. Expected: standard error names the failure class and the server, and contains neither the credential nor the ticket nor a URL carrying either; `ps` shows neither value. Verified by `ver-credential-never-reaches-diagnostics`.

**Delegated.** `discretion-flag-parsing-implementation` lives under `contract-module-boundary` but preserves this contract's exit-code assignment; it escalates if the chosen parser decides the usage exit code itself.

<!-- anchor: contract-transport-close-taxonomy -->
### contract-transport-close-taxonomy — why a connection ended is a published fact (prerequisite)

Owner: `comp-gateway-transport`. This contract is **not** CLI-owned; its ADR is `proposed`.

**Decision rule.** Every server-initiated departure of a client socket carries exactly one reason from a closed vocabulary published under `protocol/`, delivered in the WebSocket close frame's reason slot and — whenever the connection is still writable — in a control frame first, preserving the two-step shape that `adr-20260624-0011-two-step-ws-close-on-daemon-disconnect` established. A rejection that happens **before** the socket is accepted cannot carry a close reason, so it carries the same vocabulary in its JSON error body's `code` field. One vocabulary therefore spans both sides of the upgrade.

**The vocabulary.** `going_away`, `backend_unavailable`, `unauthenticated`, `incompatible_protocol`, `capacity`, `invalid_request`, `internal_error`. A client-side sentinel for "the socket ended carrying nothing" is deliberately **not** in it: by construction nothing arrives, so it cannot be a server value, and making it one would invite a server to send it.

**Why the reason slot and not numeric close codes.** Close codes outside the reserved ranges need a registry the design would have to invent and every client would have to mirror, and the repository already has one code in use — `StatusGoingAway` (1001) at `services/server/api/gateway.go:180`, the sole close-code site on the gateway path [measured]. Adding numbers would create a second vocabulary parallel to the string one, and the two would drift. Keeping one code and typing the reason also means the same value works in the pre-upgrade error body, where no close code exists at all.

**The measured gap.**

| Departure | Today [measured] | Under this contract |
|---|---|---|
| daemon dies | `{"k":"c","data":"daemon-disconnected"}` + close 1001 `daemon-disconnected` (`gateway.go:343-344, :352-353, :484-485`) | `backend_unavailable`, same two-step shape |
| server shuts down gracefully | **nothing** — unwinds into `defer c.CloseNow()` (`mux.go:613`); client sees 1006, empty reason | `going_away` |
| ticket rejected | HTTP 401, body `unauthorized` as plain text, before `websocket.Accept`; empty / unknown / used / expired collapse to one boolean (`mux.go:198-204`, `auth.go:20-23`, `ticket.go:66-81`) | `unauthenticated` in a JSON error body |
| protocol mismatch | REST answers 426 `{"code":"incompatible_protocol",…}` (`protocol_version.go:39-50`); the **authenticated** WS upgrade performs no version check at all (`mux.go:198-204`) | `incompatible_protocol` on both paths |
| capacity | subscribe-time refusal is already typed as `resource_exhausted` — `{"k":"e",…,"code":…}` on the lifecycle route, `{"k":"c","data":…}` + close `subscribe-failed` on the per-session route (`reduce_surface.go:108-110`, `gateway.go:197-202`, `:443-471`) | kept; connection-level refusal gains `capacity` |
| network drops | nothing; 1006, empty reason | nothing — and that absence is now the *defined* unattributed case rather than an ambiguity |

So three of six departures are mutually indistinguishable today, and the one that is distinguishable is so only through an application frame that no schema models.

**Source-requirement §17, all three conditions.**

1. *The capability is absent from the execution plane.* Measured above. One close code, four ad-hoc reason strings, no schema entry: `protocol/` holds seven `*.schema.json` files and none models the `{"k":"c"}` control frame; `controlMsg.Code` is `int,omitempty` and every production call passes `0` (`wire.go:45-55`, call sites `gateway.go:199, :343, :352, :484`), so the numeric slot is always absent and the free string `data` is the sole discriminator.
2. *It cannot be expressed through the current public client contract.* A client that wants to tell a rejected ticket from a pulled cable has nothing to read: both are a failed handshake or a bare close. The one field named `reason` in `protocol/` (`events.schema.json:24`) is an approval justification on `ApprovalWire`, and the only enumerated reason fields there — `resolution_reason` on approvals and questions — describe why a human-input request resolved. No CLI-side rule can manufacture the distinction.
3. *The browser and workspace clients benefit from the same fix.* Measured, and this is the condition worth stating carefully because it is the one an author is tempted to assert. The SPA does not merely ignore the close reason — it does not bind the event: `this.ws.onclose = () => this.handleClose(auth)` (`clients/ui/src/socket/connection.ts:400`), so `code` and `reason` are structurally unreachable, and a repository-wide grep for `CloseEvent` across `clients/` returns nothing. Its `handleControl` recognises exactly two `data` literals and silently drops everything else including `resource_exhausted` (`connection.ts:586-595`). The Workspace relay publishes `close` with no payload at all, discarding both fields (`clients/workspace/src/main/electron-entry.ts:488-491`), and its fleet prober classifies failures by regular expressions over `Error.message` strings (`clients/workspace/src/main/server-health.ts:126-131`) — a technique that exists precisely because there is no typed value to read. All three clients would read one value instead of guessing three different ways. The prohibition in §17 on CLI-motivated server changes is satisfied: the CLI is the client that makes the gap visible, not the client the fix is for.

**A corroborating divergence.** `adr-20260624-0011` (accepted) records the decision as sending `controlMsg {k:'c', code:'daemon-disconnected'}` — a *string* in the `code` field. The implementation puts it in `data` and leaves `code` an always-omitted int. Neither the ADR nor the code is checkable against a schema, because none exists, so the divergence has survived since June. Publishing the frame shape alongside the vocabulary is what stops the next one.

**Contract evolution.** Changed surface: the reason carried on every server-initiated close, the error body of the ticket and version rejections, and the `{"k":"c"}` frame's published shape. Typed consumers: the SPA socket, the Workspace relay and fleet prober, the Windows Shell, and the CLI. Compatibility: additive for readers — a client that ignores the reason today keeps working, because the close still happens and the two-step frame keeps its shape; the plain-text `unauthorized` body becomes JSON, which is the one breaking edge and affects a body nothing currently parses. Migration: none for data. Rollout: schema and vocabulary first, then the close sites, then the pre-upgrade bodies — in that order, because a close site emitting a reason the schema does not list is exactly what the gate must be able to catch. Rollback: reverting the close sites restores today's strings with no data change. Baseline verification: `ver-every-server-initiated-close-carries-a-published-reason` and `ver-pre-upgrade-rejections-carry-the-same-vocabulary` both fail today. Target verification: the same two plus `ver-close-reason-vocabulary-is-closed-and-registered`.

**Adversarial witness (indistinguishability, risk: failure).** In four runs, attach and then: stop the daemon, shut the server down gracefully, let the ticket expire mid-connection, and sever the network. Expected: four different reasons, three of them published and the fourth explicitly unattributed. Forbidden: any two of the four presenting alike — which is what all four do today for three of the cases. Verified by `ver-cli-maps-each-close-reason-to-its-exit-class`.

**Adversarial witness (closed vocabulary, risk: migration).** Add a close site that emits a reason outside the published list. Expected: `ver-close-reason-vocabulary-is-closed-and-registered` fails. Forbidden: a fifth ad-hoc string joining the four that exist today with nothing to stop it.

**Out of scope.** Changing the SPA or Workspace to consume the value. They gain the capability here and adopt it in their own change; requiring adoption would make this prerequisite a three-client refactor and would delay the CLI behind work it does not need.

<!-- anchor: contract-protocol-admission -->
### contract-protocol-admission — exact-match admission, fail closed

Owner: `comp-transport`.

**Decision rule.** The CLI carries exactly one compiled protocol version and always sends `X-Agent-Grid-Protocol-Version`. On HTTP 426, or when `GET /api/capabilities` reports a version that differs, is missing, or is malformed, that server is `incompatible_protocol` and no further operational REST call or WebSocket application frame is made to it. No other server's result changes.

[measured] `contracts/compatibility-policy.md` permanently forbids version ranges, capability degradation, legacy/tolerant/fallback decoders, downgrade, user override, and mixed operation. The CLI becomes a new "shipped projection", so it must carry the value of `protocol/protocol-version.json` exactly once in a projection file and be added to the enumeration in `TestProtocolVersionManifestMatchesEveryShippedProjection` (`services/gorules/static_enforcement_test.go:142-162`). That test is an allow-list: without the addition, nothing detects a CLI version drift [measured].

**Adversarial witness (risk: correctness).** Mix in one fake server returning a mismatched version. Expected: that server is shown as protocol-incompatible, the other servers' sessions still appear, and the CLI issues no session listing or attach against the mismatched one.

<!-- anchor: contract-config-and-fleet-identity -->
### contract-config-and-fleet-identity — configuration semantics and server identity

Owner: `comp-config-contract`; Go projection: `comp-goclient-config`.

Ownership moved here from `comp-goclient-config` in the consultation rework. The rules below are properties of the *contract* — every host must obey them in every language — so vesting them in one language's package would have made a Go implementation detail the definition of a cross-language rule.

**Decision rule.** `servers.json` schema v2 is read once at process start and never reloaded [measured: `adr-20260727-host-neutral-fleet-provider`, "Runtime reload is not supported"]. The configuration directory resolves per platform to the same place the existing hosts use, and `--config-dir` replaces the whole directory. The server identity is `srv_` plus the lowercase hex SHA-256 of the canonical URL; the file's `id` field is ignored as deprecated [measured: `servers.schema.json:17-21`, "Ignored read-only compatibility field; clients derive the identifier from url"]. Authentication is `none` XOR `bearer`; no token file and no environment override are read [measured: `adr-20260727-single-user-server-auth`].

**Invariants.** Nothing is written by the CLI (`fr-003`). `default_execution` is never used to infer a creation target (`fr-022`) — [measured] the Go loader promotes the first enabled entry to default when the field is absent (`config.go:123-130`), the TypeScript loader has the same fallback in `hasExplicitDefault`, and the C# loader in `fallbackDefaultIndex`, so the trap is live in all three and a verification is cheaper than a review rule. Admission is per entry and reported, never repaired (`fr-030`).

**Operational input.** The configuration file — owner `comp-config-contract`, producer the operator, boundary the configuration directory, read once at start-up, not preserved after parse, mutable between runs but not within one. If the **entry list** cannot be read, the command stops with an actionable error rather than a default fleet; if individual entries are unusable, the command proceeds with the rest and reports them (`contract-config-entry-admission`).

**Adversarial witness 1 (portability, risk: correctness).** On Windows, place the configuration the Workspace and Shell already use. Expected: `ag ls` sees the same fleet. [measured] The current Go loader has no Windows branch and reads `~/.config/agent-grid` (`config.go:40-49`), so a naive copy fails this witness.

**Adversarial witness 2 (identity divergence, risk: data).** Configure one server whose URL carries a query string, and read the server identifier from the CLI and from Workspace. Expected: the same `srv_` digest. [measured] Today they differ — Go rejects the URL (`config.go:143`) while TypeScript silently strips the query before canonicalising (`desktop-config.ts:328-332`) — so the same file yields two identities for one server. `contract-client-config-parity` is what closes this; this contract is what says the identity must be one value.

<!-- anchor: contract-client-config-parity -->
### contract-client-config-parity — three languages, one contract, one corpus

Owner: `comp-config-contract`.

**Decision rule.** The client configuration contract has exactly one loader implementation per client language. Every rule the contract declares is expressed as a fixture in one shared corpus, and each fixture carries its **expected outcome as data**: the accept-or-reject decision, the admission state of each entry, the canonical URL, and the derived server identifier. All three loaders read the same corpus and assert against the same record. Adding a rule to the contract means adding a fixture; it must not mean editing three test bodies that can disagree about what they assert.

**Why the expected outcome has to be data.** [measured] Three fixture tests exist today and every one of them is shape-only: Go reads one of four fixtures and asserts `Load` succeeds (`config_test.go:10-35`); TypeScript asserts `.not.toThrow()` / bare `.toThrow()` (`desktop-config.test.ts:339-365`); C# asserts `Assert.Throws<DesktopConfigException>` (`DesktopConfigTests.cs:215-243`). None compares *which* rule fired, so three loaders can reject the same document for three different reasons and the suite stays green — which is exactly the state the measured divergences are in. A test that cannot distinguish "rejected correctly" from "rejected by accident" is not a parity test.

**The scope of the claim.** This contract binds the **client configuration contract** — the `servers.json` document, its admission rules, and the identity derived from it. It does not bind a host's own runtime settings, which only some hosts have and which no other language implements (`open-web-host-server-side-config`). Nor does it bind the SPA: [measured] the SPA never reads the file, it reads a credential-free projection (`projection.go:3-13`), so it inherits parity through whichever host serves it rather than needing a fourth loader.

**Invariant.** No client language holds a second parser of the configuration document. This is checked structurally, not by review, because a second parser is invisible in a diff that adds it.

**Adversarial witness (identity, risk: data).** Put a URL with a query string in a fixture and record the expected `srv_` digest. Expected: all three loaders produce that digest or all three reject the URL — whichever the contract declares — and none of them silently canonicalises differently. This is the witness that fails today: TypeScript strips (`desktop-config.ts:328-332`) where Go rejects (`config.go:143`), so the same server has two identities. Verified by `ver-expected-outcome-corpus-agrees-in-three-languages`.

**Adversarial witness (coverage, risk: correctness).** Delete a fixture that covers a declared rule. Expected: `ver-corpus-covers-every-per-entry-and-file-level-rule` fails. Forbidden: a contract rule that no fixture exercises — the state the four README-promised cases (duplicate URL, multiple default execution, zero enabled, unparseable) are in today.

**Contract evolution.** Changed surface: `clients/config/README.md`, `servers.schema.json`, the fixtures directory, and the test surface of all three loaders. Typed consumers: the Go loader, the Workspace main-process loader, the Windows Shell loader, and, transitively, the SPA through the projection. Compatibility: for a configuration that all three loaders already accept identically, nothing changes. Where they diverge today, the corpus forces one answer, so one host's behaviour changes — deliberately, and the corpus is the record of which. Rollout: corpus format and reader first, then each language, then the rules that need it (`unit-server-naming-contract`). Rollback: deleting the corpus restores the current shape-only tests. Baseline verification: the three shape-only tests pass today while the loaders disagree. Target verification: `ver-expected-outcome-corpus-agrees-in-three-languages` and `ver-measured-divergences-are-closed-by-the-corpus`.

<!-- anchor: contract-config-entry-admission -->
### contract-config-entry-admission — one bad entry costs one server

Owner: `comp-config-contract`.

**Decision rule.** Each entry is admitted independently into exactly one state: `usable`, `disabled`, or `configuration-invalid`. A `configuration-invalid` entry is never probed, never connected to, never repaired, and never dropped; it is carried through to presentation with its position, its display name as written, and the rule it violates. The host starts whenever the entry list is readable.

**The attribution criterion.** An error is per-entry exactly when it can be attributed — decidable from one entry alone, or from a relation among a named set of entries. It is file-level exactly when no entry can be named, which is true only of an unreadable byte stream, content that is not JSON, an absent or non-conforming `schema_version`, and a `servers` value that is absent or not an array. Everything else is attributable. The full partition, including the classes this moves off the file-level path, is the table in `fr-030`.

**Why this is worth the change to three shipping hosts.** [measured] `clients/config/README.md:41-43` documents whole-file rejection and all three loaders implement it — Go returns `(Snapshot{}, error)` from `Load` for every failure, TypeScript throws out of `loadOrCreateDesktopConfig`, C# throws `DesktopConfigException`. That was defensible while a display name was decoration. It stops being defensible the moment a display name is an addressable identifier, because the set of configurations that can trip a rule grows and the cost of tripping one is the entire fleet on every host. The consultation chose the per-entry direction; this contract is where the choice becomes checkable.

**Outcome partition.** `determinate` (an entry passes, or violates a rule decidable from itself alone), `conflicting` (a rule violated jointly by a group — duplicate display name, duplicate URL, more than one default execution — making every member invalid with no winner chosen), `inconclusive` (the entry list cannot be read, the only class that stops the host). Coverage rests on every declared rule being classified into exactly one of the three, which `ver-corpus-covers-every-per-entry-and-file-level-rule` enforces; exclusivity rests on attribution being a property of the rule rather than of the document instance.

**Why no winner is chosen in a collision.** Every winner rule — first, last, enabled-first, lexical — silently redirects an operator's existing `--server <name>` to a different machine. There is no rule that is both decidable and harmless, so the contract declines to have one.

**Adversarial witness (blast radius, risk: correctness).** Configure three servers and give the second a display name containing a control character. Expected: the host starts, servers one and three are usable and probed, server two is `configuration-invalid` and unprobed, and `ag ls` shows all three. Forbidden: the host refusing to start, and server two vanishing from the output. Verified by `ver-host-starts-and-only-the-offending-entries-are-unusable`.

**Adversarial witness (vocabulary, risk: correctness).** In one run, have one server unreachable, one `configuration-invalid`, one healthy with an ended session. Read `--json`. Expected: three values from three disjoint value spaces. Forbidden: the invalid entry carrying `unavailable`, and the ended session sharing a word with either. Rejects UAC-046's counterexample at the admission level and UAC-040's at the vocabulary level. Verified by `ver-invalid-entry-is-a-third-vocabulary-in-discovery`.

**Adversarial witness (file-level residue, risk: failure).** Truncate `servers.json` mid-object. Expected: the host stops with a file-level error that names no entry, because none can be named. Forbidden: starting with an empty fleet and pretending the file was empty. Verified by `ver-unreadable-entry-list-still-stops-the-host`.

**Contract evolution.** Changed surface: the failure model of all three loaders and the presentation of a server that was never connectable. Typed consumers: the same four hosts, plus the credential-free projection, which keeps its current shape in this change — the CLI reads the loader directly and widening the projection so the SPA can show the state is a separate change. Compatibility: a configuration all loaders accept today is unaffected. A configuration that stops a host today will now start it with some entries unusable, which is the point and is the observable that changes. Migration: none — the change is in what happens to a bad configuration, not in the document. Rollout: corpus first (`unit-client-config-parity`), then the admission path and the naming rule together (`unit-server-naming-contract`), because a naming rule landing on a whole-file failure path would stop hosts that work today. Rollback: restoring whole-file rejection, with no data change. Baseline verification: today every one of the classes in `fr-030`'s table stops the host. Target verification: `ver-host-starts-and-only-the-offending-entries-are-unusable` plus `ver-unreadable-entry-list-still-stops-the-host` as the pair that pins both sides of the line.

<!-- anchor: contract-server-naming-prerequisite -->
### contract-server-naming-prerequisite — the display name becomes addressable

Owner: `comp-config-contract`.

**Decision rule — the constraint.** Among enabled servers, display names are unique by exact code-point equality after trimming leading and trailing whitespace. A display name is CLI-safe when, after trimming, it is non-empty, contains no ASCII control character (U+0000–U+001F, U+007F), and does not begin with `-`. Validation happens at configuration load; a violation makes the offending entry and its collision partners `configuration-invalid` (`contract-config-entry-admission`) and the loader never renames, normalises, or deduplicates.

Case-insensitive or NFC-folded comparison was considered and rejected: it would introduce a locale- and library-dependent fold that the Go, TypeScript and C# loaders would have to agree on exactly, which is a new cross-language divergence source in the very code path this change exists to make consistent, and no measured need for it exists. Exact equality after trimming is decidable identically in all three.

**Decision rule — resolving a qualifier.** Resolution runs over **every configured entry**, not only the usable ones, so a qualifier naming a configured server never reports not-found. Then, in order:

1. every match is `configuration-invalid` → exit `2`, naming each entry by position and written display name together with the rule it violates;
2. two or more usable matches → exit `4`, listing them distinguishably, selecting none;
3. exactly one usable match → resolved;
4. zero matches → falls to `contract-selector-resolution` (not-found, or transport if a server could not be read).

**Why unusability outranks ambiguity.** They can both be true — two entries sharing a display name are both invalid *and* both matches. Reporting "ambiguous, here are two candidates" would be a lie about what the user can do next: neither candidate is choosable, and no re-invocation with a narrower qualifier succeeds. The only next action is editing the configuration, and exit `2` with the violated rule is the outcome that says so. Exit `2` rather than `6` because the failure is decided locally from operator-authored input before any I/O; calling it transport would send the operator to the network for a problem in their own file, and calling it not-found would deny that the entry exists.

**Outcome partition.** `determinate` (exactly one usable match), `conflicting` (two or more usable matches), `inconclusive` (every match unusable). Coverage rests on the match set being partitioned by usability and then by size; precedence is the ordering above. A stable server identifier resolves to at most one **usable** entry — not, as this design previously claimed, to at most one entry, because two entries sharing a URL share a digest and are both invalid by the duplicate-URL rule.

**Contract evolution.** Changed surface: `clients/config/servers.schema.json` and the loader behaviour of all three hosts. Typed consumers: the Go loader, the Workspace main-process loader, the Windows Shell loader, the SPA's collision-disambiguation presentation, and the CLI. Compatibility: a configuration that already satisfies the constraint is unaffected; one that does not keeps its host running and loses the offending entries. Migration: the report names the offending entry and the required change. Rollout: corpus, then admission and the naming rule together, then the CLI's use of names as qualifiers. Rollback: reverting the loader change restores the current permissive behaviour without data change. Baseline verification: today no loader constrains either property. Target verification: `ver-duplicate-display-name-flagged-per-entry-by-every-loader` and `ver-charset-violation-flagged-identically`.

**Adversarial witness (migration, risk: migration).** Load an existing configuration whose two enabled servers share a display name. Expected: the host starts, both entries are `configuration-invalid` naming the duplication, and every other server still works. Forbidden: renaming one, appending a suffix, disabling one, continuing with an arbitrary winner, and — now equally — refusing to start. Verified by `ver-naming-violation-names-the-entry-without-renaming`.

**Adversarial witness (precedence, risk: correctness).** Run `ag ls --server <duplicated name>` against that configuration, then again against a configuration with two *usable* servers whose stable ids share a prefix the qualifier matches. Expected: exit `2` naming both entries in the first case, exit `4` listing both candidates in the second, and no session touched in either. Verified by `ver-qualifier-precedence-unusable-outranks-ambiguous`.

**Not design's to change.** Whether the constraint should exist is settled upstream. This contract fixes what the upstream handoff explicitly assigned to design — the enforcement point, the character set, the migration shape — plus the qualifier-resolution rule that follows from the consultation's per-entry decision.

<!-- anchor: contract-discovery-projection -->
### contract-discovery-projection — listing and machine-readable output

Owner: `comp-view`; reachability source: `comp-fleet`.

**Decision rule.** The population is every enabled server crossed with every session it publishes, unfiltered by creator, with nothing removed by the client. The default columns are: an identifier that can be used to address the session, the display name, the owning server, the working directory, and the lifecycle state.

**Column-to-field mapping** [measured except where noted]: identifier = `id`; display name = `view.card.title` verbatim (`fr-018`); owning server = client-side identity, because the server does not put its own identity on the wire; lifecycle state = `status` resolved through the shared matrix precedence; agent kind, where shown, comes from `root_driver`. The working-directory column is **[unmeasured]**: `project`, `workspace` and `workspace_root` all exist on `SessionInfo`, but which of them the graphical row presents is not measured, and UAC-004 requires the CLI row and the GUI row to be matchable as the same session. The choice is deferred to `evidence-cwd-worktree-field` in Phase 0 rather than fixed here with a fallback chain.

**`view.model` — a falsified upstream premise, returned to requirements.** The upstream basis for excluding a model column is an availability claim ("model 相当は公開されていないため含めない"). That claim is false: `Model` carries a JSON tag on the same `View` struct the CLI reads for the card title and status [measured: `services/host/state/view/view.go:23`], it rides in `SessionInfo.View` on the same payload, and the shipping SPA reads `session.view.model` [measured: `clients/ui/src/components/SessionList.tsx:83`]. Excluding the column is therefore a product decision, not a consequence of availability. Design does not decide it either way and does not ratify the exclusion by citing the decision the measurement invalidates; the falsified premise is returned to requirements as `open-model-column-premise-falsified`.

The earlier revision of this plan wrote the exclusion into `unit-discovery-render`'s boundaries as "do not add a model column", which is the same overreach in a different place: a design boundary that fixes a column set is a column decision, and the escalation would have been decorative. The boundary is now symmetric — the unit adds no column and removes none on design's authority, and renders whatever `DP-LS-DEFAULT-COLUMNS` resolves to once requirements has the corrected fact. Nothing else in this contract depends on which way that resolves: the column-to-field mapping, the machine-readable field-breadth rule (`cc-json-field-breadth`) and the verifications are all stated over "the default columns", not over an enumerated list.

**Machine-readable output.** Server identity and session identity are separate fields; the field breadth matches the default columns (`cc-json-field-breadth`); each server's reachability is a property of the server element and never shares a value space with session lifecycle state — nor with configuration invalidity, which is a third space carried by `contract-config-entry-admission` (`fr-031`). A configured entry appears in the output whether it is usable, disabled, unreachable or configuration-invalid; the client removes nothing it was configured with, exactly as it removes no session the server publishes.

**Invariant.** The identifier fields for a given session are stable across runs, and changing the human-readable columns or their order does not change the meaning of the machine-readable output.

**Adversarial witness (partial outage, risk: correctness).** Three servers: one unreachable, one healthy with no sessions, one healthy with sessions. Read `--json`. Expected: the first carries an unreachability value, the second carries none and an empty session collection, the third is non-empty (UAC-046). Rejects both the implementation that omits the unreachable server and the one that renders it identically to an empty server.

**Delegated.** `discretion-table-rendering` — column widths, alignment, and colouring. Truncation was removed from the delegated question: `fr-018` forbids it absolutely, so a delegation that admitted truncation would have permitted a choice violating an invariant it claims to preserve. The delegation now carries "the display-name column is never truncated" as a constraint and escalates when a name cannot be laid out without it.

<!-- anchor: contract-status-vocabulary -->
### contract-status-vocabulary — the CLI's status vocabulary source

Owner: `comp-view`.

**Decision rule.** Labels and precedence come from the checked-in shared display matrix. The in-module copy is asserted equal to the repository's canonical file by a test that reads it over a relative path — the pattern `clients/web-host/internal/clientconfig/config_test.go` already uses for `../../../config/fixtures/` [measured].

**Input and mapping.** The inputs are `status.failure`, `status.human_input` and `status.lifecycle`, all three required [measured: `protocol/session-status.schema.json`]. Precedence is `failed > human_input > active > stopped > idle`. `Unknown` is a client-side sentinel for a missing status object; a conforming server never produces it. `signal` (bell) is an orthogonal axis and does not participate in precedence.

**Scope of the claim.** This contract makes `protocol/session-status-display.json` canonical **for the CLI consumer only**. It does not make it the repository's single source of truth: the TypeScript (`StatusIcon.tsx`) and C# (`PanelPresentation.cs`) copies remain hand-maintained with no cross-language parity test [measured], and pinning them is outside this change. The residual drift is recorded as `open-status-matrix-cross-language-drift` rather than implied solved — the CLI adds a pinned copy, it does not convert the other two.

**Forbidden.** CLI-specific state words such as `running`, `live`, `connected` (UAC-006). Confusing this precedence with the favicon aggregation precedence in `clients/ui/src/lib/favicon.ts`, which places `idle` above `stopped` — a partly reversed order for cross-session aggregation [measured].

**Adversarial witness (risk: correctness).** Construct a session with `human_input` true and `lifecycle` active. Expected: the label is `Action required`, matching the GUI. An implementation printing `Working` has the precedence backwards.

<!-- anchor: contract-selector-resolution -->
### contract-selector-resolution — resolution under a declared kind, and the closure of ambiguity

Owner: `comp-selector`.

**Decision rule.** A selector resolves only under its declared kind. The kinds are the full identifier and an identifier prefix — a reduction forced by measurement, since `proto.SessionInfo` has no `name` field and the name-like values are mutable and non-unique, so they cannot ground identity. There is no implicit fallback to another kind.

**Server qualifier.** Supplied independently of the selector string; accepts a display name and a stable server identifier. When present, the candidate set is confined to that one server. Collision handling while the naming prerequisite is outstanding is `contract-server-naming-prerequisite`.

**Outcome partition.** `determinate` (exactly one match on a reachable server); `unknown` (no match anywhere among reachable servers — not found); `conflicting` (two or more matches — ambiguous, never auto-selected); `inconclusive` (zero matches while an unreachable server is in the population — transport, per DP-UNREACHABLE-SERVER-SELECTOR-OUTCOME). Coverage rests on the match count crossed with the reachability of the population; precedence is that unreachability outranks a zero count.

**Invariant.** No ambiguous, not-found, or transport path attaches to or terminates any session.

**Adversarial witness 1 (kind precedence, risk: correctness).** Construct a string that is one session's full identifier and simultaneously a prefix of another's. Expected: the prefix-declared run reports two candidates and exits `4`; the id-declared run resolves to one and attaches (UAC-034).

**Adversarial witness 2 (cross-server collision, risk: correctness).** Put a matching session on each of two servers and give both servers the same display name. Expected: unqualified is ambiguous; qualifying by stable id reaches the one session; qualifying by the duplicated display name is never an arbitrary pick — it exits `2` naming both entries and the rule they violate once the naming contract is enforced, and exits `4` with both listed while it is not (UAC-031 / UAC-032 / UAC-033).

**Unresolved.** How much context an ambiguous candidate row carries (`open-ambiguous-candidate-breadth`). Acceptance requires only mutual distinguishability, and design does not settle the breadth.

<!-- anchor: contract-create-and-attach -->
### contract-create-and-attach — creation and the attach that follows it

Owner: `comp-lifecycle`; attach: `comp-attach`.

**Decision rule.** Resolve the target server from the mandatory server qualifier. Read the launch profile set from that server. Resolve the agent argument to one profile identifier and its revision. Issue `POST /api/sessions` with an absolute project path, the profile pair, the parameter map, and the current geometry [measured: 201 Created with `{"id","project","command","created_at"}`]. On success take the returned identifier; unless `--no-attach` is given, attach to it.

**Operational input — the launch profile set.** Owner `comp-lifecycle`; producer the target server; boundary `GET /api/session-config`, which returns `command_profiles` and `base_command_profiles` each carrying an `ID` and a `Revision` [measured: `services/server/api/mux.go:236`, `mux_repository_session_test.go:63-79`]; acquired at invocation time; needed before the creation request; not preserved across invocations and never cached; mutable on the server between invocations, its stability grounded in the server-validated revision; and if it cannot be read, or the revision is stale, or the agent argument resolves to no profile, creation fails **before any request that could create a session**. That ordering is what makes UAC-043 observable: a failed creation leaves no new row.

[measured] The server's validation codes are `project_not_absolute`, `invalid_cols_rows`, `invalid_create_version`, `stale_profile_revision` (409) and `invalid_profile_parameters` (400). What remains **[unmeasured]** is only the parameter map's vocabulary — how agent kind and working directory are expressed inside `parameters` — which is narrowed to Phase 0 rather than left as the whole request shape.

**Outcome partition.** `determinate-created-and-attached`, `determinate-created-not-attached` (`--no-attach`: creation succeeded and no attach was attempted), `determinate-created-attach-failed` (creation succeeded, the attach that followed it failed), `determinate-not-created`, plus `unknown` for an unacknowledged creation, which must not report an identifier. Coverage rests on the creation being acknowledged or not, and — when it was — on the attach being requested and then either succeeding or failing; exclusivity rests on `--no-attach` being decided locally before any attach is attempted, so the two not-attached outcomes can never both apply.

The two created-but-not-attached outcomes are separated by exit classification rather than by wording alone: `determinate-created-not-attached` is a success and exits `0` while still reporting the identifier (UAC-027), and `determinate-created-attach-failed` exits with the attach failure's own class and is presented as its own observation, never as a creation failure — collapsing it hides an existing session and invites a duplicate on retry (UAC-026). An implementation that treated `--no-attach` as an attach failure would exit non-zero on a path where nothing failed, and one that treated an attach failure as a deliberate no-attach would exit `0` on a path where the user asked for a screen and did not get one.

**Invariant.** The identifier the creation reports, the identifier the following attach uses, and the identifier of the resulting listing row are the same, and the session appears on the server that was named. This holds on the `--no-attach` path too, where there is no following attach: the reported identifier is still the identifier of the running row on the named server, so a creation that deliberately does not attach is never left as an orphan the user cannot name afterwards (UAC-027).

**Normal witness (risk: correctness).** Create with `--no-attach` against a named server. Expected: exit `0`, the created identifier on standard output, the terminal never entering the raw-mode critical section, and the following listing showing that identifier as a running row on the server that was named. Verified by `ver-create-outcomes-are-distinct-observations`, which enumerates all four determinate outcomes rather than only the three that involve an attempted attach.

**Adversarial witness (risk: correctness).** Create and attach against a server that already has a session with a busy screen. Expected: the screen after attach is the new session just starting, distinguishable on the terminal from the existing session's in-progress screen (UAC-025).

**Adversarial witness (risk: mutation).** Invoke creation with exactly one server configured and no server qualifier. Expected: nothing is created and the exit code is `2`, distinct from every failure class.

<!-- anchor: contract-explicit-terminate -->
### contract-explicit-terminate — explicit termination claims only its acknowledgement

Owner: `comp-lifecycle`.

**Decision rule.** Explicit termination is `DELETE /api/sessions/{id}`. The graceful-versus-forceful distinction is **not** exposed [measured and settled: `state.StopSessionParams` has the single field `session_id`, and `reduceStopSession` unconditionally tears down every frame; the distinction does not exist in the execution plane]. The 204 is returned after the daemon applied the reducer, so 204 **is** the acknowledgement of application [measured: `SendCommand` waits for the daemon's `okResp`, which the same reduce emits]. A rejection exits non-zero and leaves the target's observable state unchanged.

**What the success report may say.** Only that the termination was acknowledged as applied. It must not assert that the target is now observable in a terminated state, because it is not: the reducer deleted the record. The earlier design shipped a success report on a path it simultaneously stated fails UAC-029; that is now separated. `ver-terminate-report-claims-only-acknowledgement` fails an implementation that prints a post-termination lifecycle state it never read back.

**Outcome partition.** `determinate` (acknowledged as applied), `unknown` (no acknowledgement — never reported as success), `inconclusive` (rejected — non-zero, state unchanged). Coverage rests on the request being acknowledged, rejected, or neither.

**Escalated, not closed.** UAC-029's second half — that the immediately following listing shows the target in a terminated state — is not satisfiable here and is not restated as satisfied. It is carried by `fr-026` and `contract-session-terminal-state-retention`, and `ac-terminate-is-explicit-only` therefore claims only the half this contract delivers, while `ac-ended-session-remains-observable` carries the other half and depends on the prerequisite.

**Adversarial witness (risk: mutation).** Run three sessions; terminate one explicitly, detach from the second, send Ctrl-C to the third. Expected: only the first ends; the other two keep running. Verified by `ver-terminate-affects-only-target`.

<!-- anchor: contract-session-terminal-state-retention -->
### contract-session-terminal-state-retention — an ended session stays observable (prerequisite)

Owner: `comp-execution-plane-lifecycle`. This contract is **not** CLI-owned. Its ADR, `adr-cli-session-terminal-state-retention-prerequisite`, is **accepted**, so the unit is no longer gated on a verdict; it is sequenced outside the CLI chain because nothing in the CLI depends on it landing first.

**Decision rule.** A session that ends — by explicit termination, by a clean or signalled self-exit, or by its backend frame vanishing — is retained with a terminal lifecycle state instead of being deleted from the session map, and the transition is published on the lifecycle socket. Frame and process teardown are unchanged; only the fate of the session record and what is published change. No client keeps a private record to compensate.

**The measured gap, on all four paths.**

| Path | Reducer | Today's outcome |
|---|---|---|
| explicit terminate | `reduceStopSession` | `delete(s.Sessions, sid)`; durable snapshot deleted [`reduce_session.go:313-329`, `runtime.go:789-792`] |
| self-exit, code in `{0,129,130,137,143}` | `reduceFrameCommandExited` → `evictFrame(…, true)` | session deleted by `evictRootFrame` [`reduce_tick.go:75-115`, `reduce_frame_evict.go:29-32`] |
| backend frame vanished | `reduceFrameVanished` → `evictFrame(…, false)` | session deleted [`reduce_tick.go:49-57`] |
| self-exit, other codes | `reduceFrameCommandExited` | frame kept with driver `Stopped`; **row survives** |

So a `lifecycle == "stopped"` row is reachable on exactly one of four paths. DP-LIST-LIFECYCLE-SCOPE and UAC-007 hold on that one; UAC-029's second half holds on none.

**Source-requirement §17, all three conditions.**

1. *The capability is absent from the execution plane.* Measured above: three of four paths delete the record, and the fourth retains it only as a side effect of a crash path.
2. *It cannot be expressed through the current public client contract.* Once the record is gone, `GET /api/sessions` and `k:"v"` carry only absence; no public field distinguishes "ended" from "never existed".
3. *The browser and workspace clients benefit from the same fix.* This is the condition the earlier design asserted without evidence, and it is measurable. `protocol/session-status.schema.json` declares `lifecycle ∈ {active, idle, stopped}`; `protocol/session-status-display.json` gives `stopped` the label `Stopped`, a glyph, and its own precedence slot; `services/host/state/view/status.go:112,140-150` maps `StatusStopped` to `LifecycleStopped`; and both the SPA and the C# panel render that label [measured]. Every shipping client therefore carries presentation for a terminal state that the plane can essentially never publish — the SPA shows a row disappearing where the contract says it should show `Stopped`. The defect is shared, not CLI-specific, and the fix is a public-contract fix rather than a CLI accommodation. The prohibition in §17 on CLI-motivated server changes is satisfied.

**Contract evolution.** Changed surface: the lifetime of a session record and the lifecycle publication of its terminal transition. Typed consumers: the SPA session list, the Workspace-hosted SPA, the Windows Shell panel, and the CLI. Compatibility: additive for readers — a state that was previously unreachable becomes reachable, and no field changes shape; a consumer that assumed "absent means ended" will now see the row, which is the point. Migration: sessions that ended before the change are already gone and are not reconstructed. Rollout: the reducer change and the publication land together, because a retained record that is not published would leave an attached client unable to attribute its departure. Rollback: restoring the deletes returns to today's behaviour with no data migration. Baseline verification: `ver-explicit-terminate-leaves-a-terminal-row` and `ver-self-exit-paths-leave-a-terminal-row` fail today. Target verification: the same two tests plus `ver-terminal-transition-is-published-on-lifecycle`.

**The one open point.** How long a terminal row is retained and what reclaims it. It is a product-visible choice (it decides what a listing shows an hour later), it belongs with the durable-snapshot owner, and design does not settle it. Escalated as `open-terminate-observability`.

**Adversarial witness (risk: failure).** Terminate one session explicitly, let a second exit cleanly with code 0, and make a third's backend frame vanish. Expected: all three are still listed with a terminal state, and each is distinguishable from an identifier that never existed. Forbidden: any of the three disappearing, and any client-side memory being used to fake the distinction.

<!-- anchor: contract-module-boundary -->
### contract-module-boundary — module boundary and gate registration

Owner: `comp-enforcement`.

**Decision rule.** `clients/cli` is an independent Go module (`clients/cli/go.mod`) with no `require` or `replace` on `github.com/takezoh/agent-grid/services`. No `go.mod` or `go.work` is created at the repository root. Registration into build, test, vet, lint, and the verification profile happens at introduction.

**Why this is a mechanism and not a convention** [measured]: the repository root has neither `go.mod` nor `go.work` and every Go command runs with `GOWORK=off`, so without a `require` a services import **cannot compile**. This is the mechanism `adr-20260727-independent-web-host-module` named; the root-module alternative was rejected in `adr-20260727-root-go-module-flag-day`.

**The gate hole to avoid** [measured]: `make lint` runs `cd services && golangci-lint run ./...` only, so `clients/web-host` has no lint gate. The `Makefile` targets and the verification profile entries are part of this contract for that reason.

**Adversarial witness (risk: correctness).** Add a services dependency to the CLI module. Expected: the dependency check fails. The precedent is `verify-bridge-deps`, which already does the same judgement with `go list -deps | grep -E <forbidden>` [measured].

**Delegated.** `discretion-flag-parsing-implementation` — which flag and subcommand mechanism the entry point uses, inside verb-first dispatch, no positional selector, and usage errors exiting `2`.

<!-- anchor: contract-external-dependency-registration -->
### contract-external-dependency-registration — the external-dependency triple

Owner: `comp-enforcement`; fake: `comp-fakegateway`; seam: `comp-tty`.

**Decision rule.** The CLI has two external dependency boundaries: the gateway (REST and WebSocket) and the local terminal (raw mode, size, input queue). Each gets the repository's three-part treatment — an in-process fake, a contract test that names the invariant, and a `FakeVsReal*` T3 backstop — and each is declared in `test-harness/dependencies.json`.

[measured] The registry's existing `pty` entry is `strategy: exception` / `kind: trusted-runtime` and covers the **server-side** pty, not a client terminal, so the local terminal needs its own judgement. Making `comp-tty` an interface seam is what allows every exit path in `contract-terminal-mode-lifecycle` and `contract-detach-boundary` to be driven deterministically at T1; only the residue that requires a real terminal (echo actually disappearing and returning) stays at T3.

**Forbidden.** Weakening an assertion when the T3 backstop fails. Fix the fake [measured: the taxonomy ADR states this explicitly].

**Delegated.** `discretion-fake-gateway-internals` — whether the fake binds a real listener or wires handlers in process, and how frames are driven.

---

## E. Decisions

### E.1 Decisions closed in this phase

<!-- anchor: adr-cli-independent-go-module -->
#### adr-cli-independent-go-module

Place `clients/cli` as an independent Go module with no dependency on the services module. Alternatives: (a) inside the services module's `cmd/` — contradicts the source requirement's boundary rule and the ownership structure of `adr-20260727-services-go-module-flag-day`; (b) inside the `clients/web-host` module to reuse its `internal/` packages — pollutes a shipped client host's module for the CLI's convenience, and is superseded anyway by extracting the shared package properly; (c) a repository-root module — already rejected in `adr-20260727-root-go-module-flag-day`. Rationale in `contract-module-boundary`.

<!-- anchor: adr-cli-attach-via-lifecycle-multiplexed-ws -->
#### adr-cli-attach-via-lifecycle-multiplexed-ws

Attach over the lifecycle-multiplexed socket (`GET /ws?ticket=`) with `k:"ld"`; do not use `GET /ws?session=`. Alternative (a), `?session=` alone, is disqualified twice over — no lifecycle observability and no subscriber identity [measured]. Alternative (b), two sockets (per-session plus lifecycle), can classify departures but doubles the failure axis, so the presented reason becomes non-deterministic in which socket dropped first. Counterexamples in `contract-attach-transport-topology`.

<!-- anchor: adr-cli-raw-mode-critical-section -->
#### adr-cli-raw-mode-critical-section

Delay raw entry until after the subscription reaches a terminal accepted outcome, funnel restore through a single critical-section exit, and discard the terminal input queue before restoring on detach. Alternatives: (a) enter raw mode at start-up and restore with `defer` — enlarges the set of paths that must restore and fails UAC-016's witness; (b) implement restore separately in each departure path — a new path silently omits it. Detail in `contract-terminal-mode-lifecycle` and `contract-detach-boundary`.

<!-- anchor: adr-cli-status-display-matrix-cli-pinning -->
#### adr-cli-status-display-matrix-cli-pinning

Take labels and precedence from `protocol/session-status-display.json` and pin the CLI's in-module copy to it with a test. Alternatives: (a) copy from the TypeScript implementation by eye — TypeScript and C# are already hand-mirrored with no parity test [measured], and a third eyeball copy widens the drift; (b) add Go to the generated SDK — the quicktype pipeline emits four languages and adding one needs its own ADR [measured], and this file is a display matrix rather than a schema, so it is not a generation target.

The claim is deliberately narrow: this makes the canonical file authoritative **for one of three consumers**. The TypeScript and C# copies stay unpinned and free to drift; converting them is a separate change, recorded as `open-status-matrix-cross-language-drift`. Framing this as a repository-wide promotion to single source of truth would overstate what is actually gated.

<!-- anchor: adr-shared-go-client-config-module -->
#### adr-shared-go-client-config-module

Make `clients/config` the **canonical client configuration contract for every client host**, give each client language exactly one loader implementation of it, and prove three-language agreement with one shared fixture corpus in which every fixture carries its expected outcome.

**Why the earlier framing was wrong, and the correction is not a widening.** The earlier revision framed this as "extract the web host's Go loader so the CLI does not become a fourth copy". That framing is falsified by the repository. `clients/config/README.md:3-4` already states the contract is shared — "Web host, Windows Shell, and Workspace share the `servers.json` version 2 contract" — three languages already read it, and the SPA already consumes a projection of the result. There is no sense in which the web host owns it. Treating it as the origin picked one consumer as the definition and left the other two to drift, which is exactly what has happened: unknown-field handling, URL-credential handling and the configuration-directory branch all differ between the three [measured, tabulated under `nfr-009`]. The CLI is the fifth consumer, not the fourth copy.

**Decision.** (1) `clients/config` — the document shape, the admission rules, the canonical URL and server-identity derivations, and the per-platform directory resolution — is the normative contract, and every host is a consumer of equal rank. (2) Each client language holds exactly one loader of that document, checked structurally: Go gets one, shared by the web host and the CLI; TypeScript and C# already have one each. (3) Parity is proven by one corpus whose fixtures carry expected outcomes as data — the decision, the per-entry admission states, the canonical URL and the derived identifier — read by all three languages.

**Why parity, and not just consolidation.** Consolidating Go removes one divergence source and leaves two. The identity that `fr-019` and DP-JSON-SESSION-IDENTIFIER-SHAPE make a machine-readable value is `srv_` + SHA-256 of the *canonical* URL, and canonicalisation already differs: Go rejects a URL carrying a query string, TypeScript strips it and proceeds [measured: `config.go:143` versus `desktop-config.ts:328-332`]. The same file therefore yields two identities for one server today, and no Go-side consolidation can detect that. Only a corpus that all three read can.

**Alternatives.** (a) A fourth hand-written implementation inside the CLI with fixture pinning — rejected: it multiplies a measured divergence at the moment a display name is becoming addressable, and shape-only pinning cannot detect it anyway. (b) Extract the Go loader and stop there — rejected above: it fixes one of three languages and leaves the identity divergence in place. (c) Generate all three loaders from the schema — attractive, and genuinely stronger, but it changes the SDK generation surface for four languages and needs its own ADR; also [measured] the schema is read by no code today, so generation would be a larger step than making the contract checkable. Recorded as a follow-up. (d) Making `clients/web-host/internal/clientconfig` non-internal in place — rejected: the loader would stay inside a shipped client's module, so the CLI's dependency graph would reach a client host.

**Consequences.** A third Go module and a third set of gate entries; changed imports in the web host; and a corpus reader in each of three languages. Where the three loaders disagree today, the corpus forces one answer, so at least one host's behaviour changes deliberately — the corpus is the record of which, and that is the point rather than a side effect. A host's own runtime settings are explicitly outside this contract (`open-web-host-server-side-config`).

<!-- anchor: adr-cli-selector-kinds-reduced-to-id-and-prefix -->
#### adr-cli-selector-kinds-reduced-to-id-and-prefix

Fix the selector kinds at the full identifier and an identifier prefix. This is not an override of a product choice; it is the firing of the `invalidation_behavior` the upstream plan recorded for `assumption-stable-name-field-unknown` ("if no stable name field exists, the kinds reduce to two"). Grounded in a field-by-field reading of `proto.SessionInfo`.

<!-- anchor: adr-cli-failure-class-exit-codes -->
#### adr-cli-failure-class-exit-codes

Fix the numbering: success `0`, generic or internal fault `1`, usage error `2`, not found `3`, ambiguous selector `4`, authentication or compatibility failure `5`, transport failure `6`.

**Why design closes this.** `handoff-exit-code-numeric-assignment` is an upstream `design_handoff` with `decision_refs: [DP-EXIT-CODE-GRANULARITY]`, `source: user_consultation`, `confidence: confirmed`, and the recorded rationale that only the numbering remained. It is not a `product_decisions_remaining` item awaiting the user. Leaving distinctness as the only contracted property would make the codes a public interface that varies per implementer, defeating `fr-024`'s reproducibility.

**Alternatives.** A coarser bucketing is excluded by DP-EXIT-CODE-GRANULARITY. A number space above 100, to avoid colliding with a proxied remote exit code, was considered and rejected: the CLI never proxies a remote process's exit code, so there is nothing to collide with, and the shell-reserved range above 125 is a hazard rather than a safeguard. The assignment matches the candidate list in the source requirement's error-model section, so it introduces no new product choice.

**Consequences.** Seven values become a public contract of the client and cannot be renumbered without a breaking change; a future class takes a new number rather than reusing one.

**The capacity clause, restated after consultation.** The numbering `0`–`6` is accepted and unchanged. What changed is the capacity refusal. It still maps to `6` and still gets no number of its own — and the dedicated-code question is now **answered no** rather than escalated, because the reason for asking it has gone. The question was live only while the discriminator was a CLI-authored sentence, which a script cannot branch on reliably; with `adr-transport-close-reason-taxonomy` the discriminator is a published value that a script reads directly, so a second exit code would add a public number to say something the reason already says. What remains open is a different question that was bundled with it — whether the eight-subscription budget should be per client rather than per gateway-to-daemon connection — and that is an execution-plane decision, not an exit-code one.

<!-- anchor: adr-cli-session-terminal-state-retention-prerequisite -->
#### adr-cli-session-terminal-state-retention-prerequisite

Take the prerequisite contract change: an ended session is retained with a terminal lifecycle state and that transition is published on the lifecycle socket, on all four termination paths. **Accepted** by user consultation (`consultation-20260814-cli-terminal-client-design#session-terminal-state-retention`); design did not accept it alone.

The measured gap, the three §17 conditions with evidence for the third, the consumer set, the rollout and the rollback are in `contract-session-terminal-state-retention`. Three things follow from it that are worth naming here:

- It closes UAC-029's second half, which no CLI-side implementation can close.
- It closes UAC-007 on the three self-exit paths where the earlier design wrongly treated the case as settled.
- It supplies the discriminator `contract-exit-classification` needs for departure class (b), which removes the risk that a negative Phase 0 spike result leaves `fr-014`, UAC-018 and UAC-019 permanently unsatisfiable.

One prerequisite therefore closes three separate gaps, which is the main reason to prefer it over three local workarounds. Execution responsibility stays with the execution plane; a CLI-side store of terminated sessions is forbidden by `nfr-004` and `cc-execution-plane-ownership`.

**What acceptance did and did not settle.** It settled that the retention exists and is published on all four paths, which is what `unit-terminal-state-retention-prerequisite` builds. It did **not** settle how long a terminal row is retained or what reclaims it; that stays with the durable-snapshot owner and remains open as `open-terminate-observability`. The unit is therefore buildable — every one of its acceptance items is decidable without the window — while the window itself is written as an explicitly named owner rather than a number this design invents.

<!-- anchor: adr-server-display-name-naming-contract -->
#### adr-server-display-name-naming-contract

Land the upstream naming constraint as an enforced contract — uniqueness by exact code-point equality after trimming among enabled servers, a CLI-safe character set (non-empty after trimming, no ASCII control characters, no leading `-`) — and enforce it **per entry**: the host starts, the offending entry and its collision partners become configuration-invalid and are never connected to, and every other error class that can be blamed on an entry or a collision group moves to the same discipline. Only a document whose entry list cannot be read stops a host.

**What the consultation changed.** The earlier revision enforced the constraint by rejecting the whole configuration and stopping the host, which matched the documented behaviour [measured: `clients/config/README.md:41-43`]. The consultation decided the opposite direction and extended it: the host starts, naming errors are the server's own error, and the existing server-unit-closable errors — URL duplication above all — move per-entry with them. This is a larger change to three shipping loaders than the earlier form, and deliberately so, because a rule about one entry that costs the whole fleet has the wrong blast radius for a rule that is about to fire more often.

**The partition, and the one place it goes further than asked.** Per-entry covers everything attributable to an entry or a named collision group; file-level keeps only what leaves no entry to name. The full table is in `fr-030`. The one step beyond the literal classes named: "zero usable servers" stops being an error, because per-entry admission already makes that state reachable and keeping a *separate* fatal path to the same observable would mean the same end state stops the host on one route and not on another.

**Scope of design's authority.** Whether the constraint should exist is settled upstream (`handoff-server-display-name-naming-contract`, confirmed). The handoff assigns to design the character set, where uniqueness is enforced, and the migration of existing configurations. This ADR decides those plus the enforcement granularity the consultation directed, and plus the qualifier-resolution rule that follows from it — which was previously only prose with no requirement, no verification and no unit, and which addressed display only, leaving a duplicate display name passed to `--server` with no defined outcome.

**Alternatives.** Case-insensitive or NFC-folded uniqueness — rejected for cross-language fold parity risk with no measured need. Enforcing uniqueness only in the CLI — rejected: the CLI would reject a configuration the graphical clients accept, and the constraint exists precisely because the name is becoming an addressable identifier for every host. Auto-suffixing duplicates — rejected: it silently changes an operator-authored identifier that scripts may already use. Blaming one member of a collision — rejected: every winner rule silently redirects an existing `--server <name>` to a different machine. Keeping whole-file rejection for the *new* rules while moving only the old ones — rejected: two disciplines in one loader is worse than either.

**Consequences worth naming.** The observable "the host refuses to start" is removed for a class of configurations that trip it today, so an operator who relied on the loud failure now gets a quiet one unless the invalid entry is presented — which is why `fr-031` is a requirement of the same change and not a follow-up. `clients/config/README.md:41-43` must be rewritten, and `clients/web-host/internal/webhost/host.go:30-31`, which refuses a snapshot with no servers, must be relaxed.

**Governance.** `clients/config/` is governed by `design-windows-shell` [measured: RESP-005 / INV-009], so this ADR requires an upsert of a design whose name no longer matches its scope; the carve-out to `design-clients-config` is recorded as a promotion candidate and not decided here.

<!-- anchor: adr-transport-close-reason-taxonomy -->
#### adr-transport-close-reason-taxonomy

Publish a closed vocabulary of transport close reasons in the public contract, carried on every server-initiated socket close and in the error body of every pre-upgrade rejection, so a client can tell capacity, authentication, protocol, going-away, backend loss and an unattributed drop apart.

**Why this is a new ADR rather than a clause in the exit-code one.** The exit-code ADR is about the CLI's own public numbering, which the consultation accepted unchanged. This is a change to a server contract that four clients read, argued under source-requirement §17. Folding a prerequisite server change into an accepted client-side ADR would have hidden it from exactly the review it needs.

**The gap, in one sentence.** [measured] Three of six departure kinds — graceful shutdown, ticket rejection, network drop — are mutually indistinguishable to every shipped client, because the gateway's only close code is `StatusGoingAway`, its four reason strings appear in no schema, most departures send no frame at all, and pre-upgrade rejections answer in plain text before a socket exists. The evidence, path by path, is in `contract-transport-close-taxonomy`.

**Alternatives.** (a) Numeric close codes per cause — rejected: it needs a registry this design would have to invent and every client mirror, it creates a second vocabulary parallel to the reason string, and it cannot cover the pre-upgrade rejections where no close code exists. (b) Leave it and let the CLI infer from timing and from which operation was in flight — rejected: that is a heuristic presented as a fact, and it is unavailable for the case that matters most, a rejected credential, which looks exactly like a dropped packet. (c) Model only the `{"k":"c"}` frame and leave the close reason as-is — rejected: the frame is best-effort and is not sent on the paths that unwind into `CloseNow()`, so the cases with no information today would still have none. (d) Give the CLI a health probe to disambiguate after the fact — rejected: it answers a different question (is the server up *now*) and cannot recover why *this* connection ended.

**If this is not accepted.** `fr-014`'s transport-loss class stays a single word covering a rejected ticket, a draining server and a pulled cable, so `ac-disconnect-classes-distinct` holds only in the coarse sense that transport loss differs from detach and from session end. `fr-024`'s capacity clause falls back to a CLI-authored reason string — deterministic within one build, but a per-implementer interface of exactly the kind the same requirement forbids for the codes. Automation that retries on exit `6` keeps retrying a connection that a rejected credential will never let succeed, and `open-resource-exhausted-failure-class` reopens, because the argument for declining a dedicated capacity exit code is precisely that the discriminator lives in the public contract instead. The browser and Workspace clients keep their current position: one cause distinguishable out of six, by an application frame no schema describes.

<!-- anchor: adr-cli-sessioninfo-golden-vector-conformance -->
#### adr-cli-sessioninfo-golden-vector-conformance

Pin the session payload shape the CLI reads with a checked-in golden vector that the server's own serializer is tested against, and register it in the repository's enumerating gate.

**Why.** `SessionInfo` and `View` have no schema entry and no generated SDK; the TypeScript side is a hand mirror [measured]. The status matrix is pinned to a canonical file, but the fields taken from `view.card` and the working-directory field are pinned nowhere, and a CLI-side test proves only the CLI's own behaviour. A server-side rename would break the CLI with every gate green.

**Alternatives.** (a) Add `SessionInfo` to `protocol/*.schema.json` and to `quicktype-emit.json` — strictly better and would also cover the TypeScript mirror, but it changes the generated surface of four languages and needs its own ADR; recorded as a follow-up. (b) Import the Go types from `services` — excluded by `nfr-002`. (c) Rely on the T3 fake-versus-real backstop — it exercises frames, not the payload's field names, and would not fail on a rename that both sides of the fake agree on.

**Consequences.** One golden vector file and two tests, one of them inside `services`. A deliberate field rename now costs one extra edit, which is the intended cost.

### E.2 Delegated to the implementer

`discretion-input-read-buffering` — stdin read buffer size and coalescing granularity, inside `contract-input-passthrough`'s UTF-8 boundary rule and `contract-detach-boundary`'s cut position. Escalate if the boundary rule would have to be relaxed, if the server turns out to bound bytes per input frame, or if coalescing changes observable ordering.

`discretion-pump-goroutine-topology` — the goroutine and channel arrangement of both pumps and the lease timer, inside `contract-attach-lease-progress`'s progress bound and the single restore path. Escalate if renewal can only work by depending on output progress, if a second restore path appears, or if cancellation needs a frame-vocabulary change.

`discretion-flag-parsing-implementation` — the flag and subcommand parsing mechanism, inside verb-first dispatch, no positional selector, and usage errors exiting `2`. Escalate if the mechanism decides the usage exit code itself, treats a non-verb first argument as a target, or adds an unaccepted runtime dependency.

`discretion-table-rendering` — column widths, alignment, and colouring, with the display-name column never truncated and output content identical between TTY and non-TTY. Escalate if a name cannot be laid out without truncation, if colouring would change non-TTY content, or if a column would have to be dropped.

`discretion-fake-gateway-internals` — the fake gateway's internal construction, provided the whole admission sequence, capacity refusal and every disconnect shape stay deterministically drivable with the real gateway's frame shapes. Escalate if the fake would need a frame the real gateway does not produce, or if a fidelity test tempts an assertion change.

### E.3 Evidence to acquire before the dependent choice

`evidence-create-request-vocabulary` — the vocabulary of `parameters` in `POST /api/sessions` and how agent kind and working directory are expressed in it. The profile source itself is now measured (`GET /api/session-config` with `ID` and `Revision`), so only the parameter map remains. **[unmeasured]**

`evidence-session-end-frame-shape` — which frame a lifecycle socket receives when an attached session ends server-side. Candidates: the target disappearing from `k:"v"`, a `{"k":"c","data":"surface-unsubscribed","sessionId":…}` severance [measured: `services/server/api/push_wire.go:9-29`, gated on `proto.CmdNameSurfaceUnsubscribe`], or both. The consequence of a negative result changed after consultation: it no longer degrades the departure contract, because the accepted retention prerequisite publishes the transition regardless. The observation still shapes the implementation, so the item stays. **[unmeasured]**

`evidence-cwd-worktree-field` — which public path field the graphical row presents as the working directory. Measurement so far establishes only that `project`, `workspace` and `workspace_root` all exist, which does not cover the claim the column needs. UAC-004 requires the CLI row and the GUI row to be matchable, so the column choice waits for this. **[unmeasured]**

`evidence-windows-console-behavior` — whether raw entry on the Windows console reliably removes echo and line editing, how VT input arrives, whether VT output processing must be enabled, and how Ctrl-C arrives. **[unmeasured]**

`evidence-windows-resize-source` — the Windows mechanism for observing terminal size changes and its latency and loss characteristics. `SIGWINCH` does not exist there [inferred]. **[unmeasured]**

`evidence-detach-key-chord-collision` — whether a single `Ctrl-]` (`0x1D`) collides with Claude Code's or Codex's own key handling on real hardware. The upstream plan recorded this as explicitly unverified; the escape hatch (`cc-detach-key-configurable`) is already contracted. **[unmeasured]**

---

## F. Critique resolution

Every critique issue with `verdict: Y` is tracked here. Blockers first.

This section is the record of the **critique** pass and is deliberately left as written then. Where the later user consultation changed a decision, an identifier or a verification named below, §I is authoritative and says so explicitly — in particular `ver-departure-classes-distinct-or-declared-degraded` and `ver-duplicate-display-name-rejected-by-every-loader` have been renamed, and the "interim" framing of `fr-029` has been replaced by a permanent resolution rule.

**issue-terminate-contracts-a-known-uac029-failure (blocker).** Patch hint path (a) taken and extended. `fr-023` now states only the acknowledgement semantics and is explicitly separated from any claim about UAC-029; `contract-explicit-terminate` adds `ver-terminate-report-claims-only-acknowledgement`, which fails a report asserting an unread lifecycle state; the gap is carried by the new `fr-026` and `contract-session-terminal-state-retention` with `adr-cli-session-terminal-state-retention-prerequisite` proposed; `ac-terminate-is-explicit-only` claims only the delivered half and the new `ac-ended-session-remains-observable` carries the other half. Escalated as `open-terminate-observability`. Path (b) was not taken alone because the design can name the prerequisite precisely; both the prerequisite and the escalation are recorded.

**issue-ended-session-row-retention-unmeasured (blocker).** Patch hint taken. `fr-015` is corrected to the client-side obligation ("shall not itself remove or hide"), the four termination paths are tabulated with their reducers in `contract-session-terminal-state-retention`, and the self-exit paths are folded into the same prerequisite instead of being treated as settled by UAC-007. `ver-cli-never-drops-a-published-session` verifies the client half; `ver-self-exit-paths-leave-a-terminal-row` verifies the server half.

**issue-naming-prerequisite-interim-behaviour-not-contracted (blocker).** Patch hint taken and the corresponding graft applied. The interim rule is now `fr-029` with a decision rule and an outcome partition in `contract-server-naming-prerequisite`, a T1 verification, and a carrier unit. The constraint itself remains the upstream handoff; design fixed only what the handoff assigned it — enforcement point, character set, migration shape — plus the interim resolution rule the hint asked for. The graft's enforcement point in each loader, the shared fixture triple and the actionable migration error are all in `unit-server-naming-contract`.

**issue-display-name-truncation-delegated-against-its-own-invariant (major).** Patch hint taken. Truncation is removed from `discretion-table-rendering`'s question, "the display-name column is never truncated, abbreviated, or elided" is added as a constraint, and the escalation now trips when a name cannot be laid out rather than only when a row becomes unmatchable.

**issue-exit-code-numbering-handoff-left-open (major).** Patch hint taken and the graft applied. `adr-cli-failure-class-exit-codes` fixes 0–6 with the >100 range rejected and why; `fr-024` and `contract-exit-classification` carry the same numbers; the disposition changes from carried-unresolved to closed.

**issue-resource-exhausted-outside-the-closed-failure-partition (major).** Patch hint taken with one deliberate difference. The closed-partition claim is removed from `fr-024`, and the measured capacity path is written into `contract-attach-transport-topology`'s partition and default policy rather than only into unresolved prose. The hint's instruction not to assign a dedicated code is honoured: capacity maps to the existing transport code with a distinguishing reason, so the behaviour is deterministic and script-branchable, and only the question of a dedicated code stays open. Leaving the exit code itself undefined was rejected because an implementation must exit with something and an unstated code is exactly the per-implementer variance `fr-024` forbids.

**issue-departure-rule-satisfiability-unevidenced (major).** Patch hint taken and strengthened. The negative branch is written into `contract-exit-classification`'s default policy, and `ver-departure-classes-distinct-or-declared-degraded` is scoped to whichever branch the spike selects. The strengthening: the degraded branch does not merely record a two-way distinction, it names where the missing discriminator comes from — the same retention prerequisite — so a negative spike result does not leave UAC-018 and UAC-019 unsatisfiable in perpetuity.

**issue-section17-third-condition-unargued (major).** Patch hint taken via its first option. Condition (3) is now evidenced: the protocol schema declares `stopped`, the shared display matrix gives it a label and a precedence slot, `view/status.go` maps `StatusStopped` to it, and both the SPA and the C# panel render it — so every shipping client already presents a state the plane cannot reach on three of four paths. The condition is measured, not asserted.

**issue-config-loader-fourth-implementation (major).** Patch hint taken via its first option and the graft applied. `adr-shared-go-client-config-module` replaces the fourth-loader ADR, the both-ways framing is gone, and `unit-shared-goclient-config` moves the package with the web host migrated and its existing tests passing unchanged. The measured divergence on unknown fields, token trimming and URL credentials, plus the missing Windows directory branch, are the recorded grounds.

**issue-model-column-false-premise-not-escalated (major).** Patch hint taken. `contract-discovery-projection` states that the upstream availability premise is measured false against `view/view.go:23` and `SessionList.tsx:83`, takes no column decision either way, and routes the falsified premise back to requirements as `open-model-column-premise-falsified`.

**issue-sessioninfo-shape-has-no-cross-language-gate (major).** Patch hint taken via its second option. `contract-wire-shape-conformance`, `nfr-008` and `adr-cli-sessioninfo-golden-vector-conformance` add a golden vector that the **server's** serializer is tested against, so a rename fails on the producing side. The schema-plus-quicktype option is recorded as the stronger follow-up with its cost stated.

**issue-concurrent-attach-cap-contradicts-ungated-claim (major).** Patch hint taken. `ac-concurrent-attach` is qualified with the measured capacity counterexample and cross-referenced to the same contract fix as the failure-partition issue, so both resolve through `contract-attach-transport-topology` rather than through two separate patches.

**issue-status-matrix-ssot-claim-overstated (minor).** Patch hint taken. The ADR is renamed and narrowed to CLI-consumer pinning, and the unpinned TypeScript and C# copies are recorded in the contract text and as `open-status-matrix-cross-language-drift`.

**issue-input-discretion-crosses-its-owning-component (minor).** Patch hint taken via its first option. `contract-input-passthrough` and `contract-output-passthrough` are owned by `comp-attach`, where the delegated files live, so an escalation raised from `input.go` has one adjudicating owner. `comp-wire` keeps its own contracts (`contract-wire-shape-conformance`, `contract-identity-single-origin`) rather than being left owning nothing.

**issue-cwd-column-source-unevidenced (minor).** Patch hint taken and the graft applied. The `[measured]` tag is corrected to cover field existence only, the column choice is deferred to `evidence-cwd-worktree-field`, and that observation is an acceptance item of the Phase 0 unit.

**issue-credential-exposure-in-diagnostics-uncontracted (minor).** Patch hint taken. `fr-027` is a new invariant, `contract-exit-classification` carries the forbidden clause and the adversarial witness, `ver-credential-never-reaches-diagnostics` verifies it, and redaction is placed at the single point where a transport error becomes a reason.

### Grafts from draft 2

All seven applied: the naming prerequisite with its enforcement points, fixture triple, migration error and interim rule (`fr-028`, `fr-029`, `unit-server-naming-contract`); the exit-code assignment (`adr-cli-failure-class-exit-codes`); the shared Go config module (`adr-shared-go-client-config-module`, `unit-shared-goclient-config`); the identity and serialization single-owner guard (`nfr-007`, `contract-identity-single-origin`); the create profile source and its ordering (`contract-create-and-attach`); the cwd evidence item (`evidence-cwd-worktree-field`); and the binary-name boundary question (below).

---

## G. Unresolved — escalated to the user

**open-terminate-observability.** The prerequisite contract change was **taken** — an ended session stays observable with a terminal lifecycle state on all four termination paths and the transition is published on the lifecycle socket, so UAC-029's second half and UAC-007's three self-exit paths are reachable. What remains open is the sub-question the acceptance deliberately left alone: **how long a terminal row is retained and what reclaims it.** It is product-visible (it decides what a listing shows an hour later), it belongs with the durable-snapshot owner rather than with any client, and design does not settle it. Until it is settled, `contract-session-terminal-state-retention` requires only that the window and its reclamation have one named owner, and no client compensates with a private record.

**open-resource-exhausted-failure-class.** Half of this is now closed. The shared surface-subscribe budget of eight is consumed by graphical clients [measured], so a CLI attach can be refused for capacity; the behaviour is contracted as exit `6` carrying the reason the execution plane published, and the dedicated-exit-code question is answered **no** — a second public number would say what the published reason already says, and the reason is what a script branches on. What stays open is the other half that was bundled with it: whether the budget of eight should be raised, or made per client rather than per gateway-to-daemon IPC connection. That is an execution-plane capacity decision with its own users; design records it and does not take it.

**open-web-host-server-side-config.** The consultation confirmed that the client configuration contract is common to every host and that a host's own server-side settings, if it needs them, are a separate design. [measured] The web host takes its listen address, TLS material and configuration directory from flags only and embeds its static assets; Workspace and Windows Shell read appearance and process settings beside `servers.json`. None of that is part of the client configuration contract, none of it is implemented in more than one language, and binding three languages to it would be parity for its own sake. Recorded as a follow-up change or a promotion candidate, and explicitly out of scope here.

**open-model-column-premise-falsified.** The upstream exclusion of a model column rests on an availability claim that is measured false. Design takes no position on whether the column should be shown; the decision returns to requirements with the corrected fact. The consultation confirmed the correction is a repair of `DP-LS-DEFAULT-COLUMNS`'s candidate set rather than a design judgement, so this plan removed the "do not add a model column" boundary it had written into `unit-discovery-render` and states no column outcome anywhere.

**open-ambiguous-candidate-breadth.** How much context an ambiguous candidate row carries. Upstream placed it in both `product_decisions_remaining` (advisory) and `design_handoffs`; acceptance requires only distinguishability. Not settled here.

**open-config-contract-design-ownership.** `clients/config/` is governed by `design-windows-shell` [measured]. A CLI-motivated naming change therefore requires an upsert of that design, which is consistent as an SSOT but whose name and scope have diverged. Carving out `design-clients-config` is a promotion candidate.

**open-status-matrix-cross-language-drift.** The TypeScript and C# status copies remain hand-maintained with no parity test after this change. Pinning them is a separate change.

**handoff-cli-binary-canonical-name — recorded boundary question.** The executable's name is a packaging decision that does not change any observable, and it stays open. But an accepted design already names *the CLI* as a paired consumer of the Workspace named-pipe control endpoint [measured: `design-workspace-desktop` RESP-003 and its control-envelope compatibility policy], and no such executable exists in the repository today. Whether that named consumer is this executable or a different one is unestablished, so packaging could resolve the name into a collision with a separate accepted consumer contract. Recorded so the packaging decision is made with this in view.

---

## H. Implementation order

Phase 0 comes first because the source requirement makes contract inventory the first task, and because three of the six evidence items resolve there. The configuration prerequisites come next, in a chain of three that must run in order: `unit-shared-goclient-config` gives Go one loader, `unit-client-config-parity` makes the three languages mechanically comparable, and only then does `unit-server-naming-contract` add the naming rule and per-entry admission. That order is not cosmetic — a naming rule landing on a whole-file failure path would stop hosts that work today, and a per-entry rewrite with no expected-outcome corpus would change three loaders with nothing able to tell whether they still agree.

Then the CLI proceeds from what can be observed with the least machinery to the interactive core: skeleton, admission, reachability, codec, vocabulary, rendering, selection, terminal adapter, attach runtime, lifecycle commands, and finally platform hardening. `comp-tty` precedes `comp-attach` because the fake terminal is a precondition for testing every attach exit path.

Two units sit outside that chain because they are prerequisite contract changes to the execution plane rather than CLI work:

- `unit-terminal-state-retention-prerequisite`. Its ADR is accepted. Its absence blocks no CLI unit; it blocks `ac-ended-session-remains-observable` and the session-end departure class.
- `unit-transport-close-taxonomy-prerequisite`. Its ADR is `proposed`. Its absence blocks no CLI unit either; it blocks `ac-transport-close-is-typed`, and it degrades — rather than blocks — `ac-disconnect-classes-distinct` and the capacity clause of `ac-concurrent-attach`, which fall back to a CLI-authored reason string. The CLI's own units can be written against the published vocabulary and exercised through the fake gateway before the server side lands; what they cannot do is observe it from a real server.

---

## I. Consultation rework

The user consultation `consultation-20260814-cli-terminal-client-design` returned verdicts on all ten decision ADRs. Seven were accepted and their decisions are unchanged. Three were returned as `free_text` with a direction, and one new prerequisite ADR was requested. This section records what each rework changed, so a second review can check the change rather than re-derive it.

### I.1 ADR status after consultation

| ADR | Status | Note |
|---|---|---|
| `adr-cli-independent-go-module` | accepted | unchanged |
| `adr-cli-attach-via-lifecycle-multiplexed-ws` | accepted | unchanged; the capacity paragraph inside its contract was corrected, not the decision |
| `adr-cli-raw-mode-critical-section` | accepted | unchanged |
| `adr-cli-status-display-matrix-cli-pinning` | accepted | unchanged, CLI-consumer scope kept |
| `adr-cli-selector-kinds-reduced-to-id-and-prefix` | accepted | unchanged |
| `adr-cli-session-terminal-state-retention-prerequisite` | accepted | retention window and reclaimer stay open by the acceptance's own terms |
| `adr-cli-sessioninfo-golden-vector-conformance` | accepted | unchanged |
| `adr-server-display-name-naming-contract` | **proposed** | reworked — enforcement moves per entry |
| `adr-shared-go-client-config-module` | **proposed** | reworked — the contract, not one host's loader, and three-language parity |
| `adr-cli-failure-class-exit-codes` | **proposed** | numbering accepted and unchanged; only the capacity clause reworked |
| `adr-transport-close-reason-taxonomy` | **proposed** | new, requested by the consultation |

### I.2 Rework 1 — naming enforcement becomes per entry

Direction: the host starts; a bad name is that server's error and it is not connected to; and other server-unit-closable errors move to the same discipline.

| Changed | From | To |
|---|---|---|
| `fr-028` | reject the configuration, stop the host | start; the entry and its collision group become configuration-invalid |
| `fr-029` | fires when a qualifier matches ≥2 enabled servers | fires over all configured entries, with unusability outranking ambiguity, and the "a stable id always resolves to at most one server" claim corrected to "at most one **usable** entry" |
| new `fr-030` | — | the general per-entry admission rule and the attribution criterion, with the full class table |
| new `fr-031` | — | an unusable entry is observable, in a third vocabulary disjoint from reachability and lifecycle |
| `contract-server-naming-prerequisite` | partition over stable-id / one-name / many-names, with a load-time host stop | partition over the match set's usability, with an explicit precedence and the exit-code mapping |
| new `contract-config-entry-admission` | — | the per-entry / file-level partition, the no-winner rule, and the observability witnesses |
| `ver-duplicate-display-name-rejected-by-every-loader` | — | renamed to `…-flagged-per-entry-by-every-loader`; the old name asserted the behaviour that was just removed |

Implementation impacts recorded, not applied in this run: `clients/config/README.md:41-43` states the whole-file rule and must be rewritten; `clients/web-host/internal/webhost/host.go:30-31` refuses a snapshot with no servers and must be relaxed.

### I.3 Rework 2 — the client configuration contract is common, and parity is proven

Direction: the client configuration is common to every client; a host's own server-side config is a separate design; go as far as three-language parity.

The ADR's subject changed from "extract the web host's Go loader" to "`clients/config` is the canonical contract; one loader per language; parity by shared expected-outcome corpus", and its title changed with it. `contract-config-and-fleet-identity` moved owner from `comp-goclient-config` to `comp-config-contract`, because a rule every language must obey cannot be defined by one language's package. `ver-web-host-tests-pass-through-shared-module` was removed: "the web host's tests pass unchanged" was the right check for an extraction and is the wrong one here, because the corpus deliberately changes behaviour where the loaders disagree; it is replaced by `ver-every-go-host-loads-through-one-owner`, which checks the property that actually matters. `nfr-009` is new and is the requirement that binds the three languages. `unit-shared-goclient-config` was rewritten to stop treating the web host as the origin, and `unit-client-config-parity` is new.

The measured divergences that justify going to parity rather than stopping at Go — unknown fields, URL credentials, the Windows directory branch, and the duplicate-URL message — are tabulated under `nfr-009`. The sharpest is URL canonicalisation, because it makes the same server carry two different `srv_` identities across hosts, and that identity is a value `fr-019` puts in machine-readable output.

### I.4 Rework 3 — transport close gets a type

Direction: exit codes `0`–`6` stay as accepted; add a prerequisite contract change that defines typed close reasons in the public contract.

New ADR `adr-transport-close-reason-taxonomy` and new `contract-transport-close-taxonomy`, with `fr-032` as the requirement and `unit-transport-close-taxonomy-prerequisite` as the carrier. `contract-exit-classification` now maps each published reason to an exit class in a table instead of asserting three distinct texts; `contract-attach-transport-topology`'s capacity paragraph replaces the CLI-authored capacity sentence with the published code. `fr-024`'s numbering is untouched and only its capacity clause is restated. `open-resource-exhausted-failure-class` closes on the dedicated-code question and keeps only the budget-ownership half.

One correction fell out of the measurement: the earlier revision cited `services/server/api/gateway.go:100` for `ConnID`. There is no `ConnID` in that file — it is allocated at `services/host/runtime/ipc.go:101-108` for a daemon-side IPC connection. The conclusion the citation supported (one shared budget of eight for the whole fleet) is right; the citation was not.

### I.5 Fix — design stops deciding the model column

`unit-discovery-render`'s boundaries carried "do not add a model column", which is a column decision written as a boundary. It is removed and replaced with a symmetric boundary: the unit adds no column and removes none on design's authority, and renders whatever `DP-LS-DEFAULT-COLUMNS` resolves to once requirements has the corrected fact. The disposition `open-model-column-premise-falsified` was already correct and is unchanged.

### I.6 Fix — prose aligned with the accepted status

`adr-cli-session-terminal-state-retention-prerequisite` no longer says "status stays proposed; design does not accept it alone"; `contract-session-terminal-state-retention` no longer says its unit is gated on a verdict; `unit-terminal-state-retention-prerequisite`'s boundary no longer says "do not start before the ADR is accepted". §G's `open-terminate-observability` is rewritten to the post-acceptance shape: the prerequisite is taken, and the retention window and its reclaimer remain the open sub-question the acceptance deliberately left alone.
