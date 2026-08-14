---
change: change-20260814-cli-prerequisite-contracts
role: requirements
functional_requirements:
- id: fr-026
  statement: The execution plane shall keep a session observable with a terminal lifecycle
    state after it ends by explicit termination, by clean or signalled self-exit,
    or by its backend frame vanishing, and shall publish that transition on the lifecycle
    socket; the CLI shall never reconstruct that observation from local memory.
  priority: must
- id: fr-028
  statement: When a client host loads a configuration entry whose display name duplicates
    another entry's or falls outside the declared CLI-safe character set, it shall
    start, shall place that entry and every entry it collides with in the configuration-invalid
    admission state, shall not connect to any of them, and shall not rename, normalise,
    deduplicate, or silently drop any of them.
  priority: must
- id: fr-030
  statement: When a client host loads a client configuration whose entry list is readable,
    it shall start and admit every entry independently, placing each entry that violates
    a per-entry rule — and every member of a colliding group — in the configuration-invalid
    admission state rather than refusing the whole configuration; it shall stop at
    load only when the entry list itself cannot be read.
  priority: must
- id: fr-032
  statement: Every server-initiated close of a client socket shall carry a reason drawn
    from a closed published vocabulary, and the CLI shall derive both its transport
    failure classification and its presented reason from that value rather than from
    the close code alone or from a client-authored string.
  priority: must
- id: nfr-009
  statement: The client configuration contract shall have exactly one loader implementation
    per client language, and the accept-or-reject decision, the per-entry admission
    state, the canonical URL and the derived server identifier produced by the Go,
    TypeScript and C# loaders shall be pinned to one shared fixture corpus in which
    every fixture carries its own expected outcome.
  priority: must
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Content

Canonical plan: `../change-20260812-cli-terminal-client/design-plan/design.md` + `../change-20260812-cli-terminal-client/design-plan/spine.yaml`. That plan is **shared** by this change and by `change-20260812-cli-terminal-client`; it is neither split nor copied. This member selects the requirements this change is responsible for satisfying and points at the plan anchors that argue them. Upstream experience contract: `../change-20260812-cli-terminal-client/requirements-plan/ux.md`.

### How the shared plan's requirements were divided

A requirement belongs to the change whose deliverable **makes it true**, not to the change whose contracts happen to reference it.

- Requirements satisfied by the execution plane, by the public protocol surface, or by the client configuration loaders → **this change**.
- Requirements satisfied by the observable behaviour of the `clients/cli` executable → `change-20260812-cli-terminal-client`.

Five requirements land here. Nothing else in the shared plan is restated in this member.

### Functional requirements

| ID | Type | Statement | Plan anchor |
|---|---|---|---|
| FR-026 | invariant | The execution plane shall keep a session observable with a terminal lifecycle state after it ends by explicit termination, by clean or signalled self-exit, or by its backend frame vanishing, and shall publish that transition on the lifecycle socket; the CLI shall never reconstruct that observation from local memory. | `design.md` `fr-026` |
| FR-028 | event | When a client host loads a configuration entry whose display name duplicates another entry's or falls outside the declared CLI-safe character set, it shall start, shall place that entry and every entry it collides with in the configuration-invalid admission state, shall not connect to any of them, and shall not rename, normalise, deduplicate, or silently drop any of them. | `design.md` `fr-028` |
| FR-030 | event | When a client host loads a client configuration whose entry list is readable, it shall start and admit every entry independently, placing each entry that violates a per-entry rule — and every member of a colliding group — in the configuration-invalid admission state rather than refusing the whole configuration; it shall stop at load only when the entry list itself cannot be read. | `design.md` `fr-030` |
| FR-032 | invariant | Every server-initiated close of a client socket shall carry a reason drawn from a closed published vocabulary, and the CLI shall derive both its transport failure classification and its presented reason from that value rather than from the close code alone or from a client-authored string. | `design.md` `fr-032` |

### Non-functional requirements

| ID | Type | Statement | Plan anchor |
|---|---|---|---|
| NFR-009 | compatibility | The client configuration contract shall have exactly one loader implementation per client language, and the accept-or-reject decision, the per-entry admission state, the canonical URL and the derived server identifier produced by the Go, TypeScript and C# loaders shall be pinned to one shared fixture corpus in which every fixture carries its own expected outcome. | `design.md` `nfr-009` |

### The per-entry admission partition (FR-030)

An error is per-entry exactly when it can be attributed — decidable from one entry alone, or from a relation among a named set of entries. It is file-level exactly when no entry can be named: unreadable bytes, content that is not JSON, an absent or non-conforming `schema_version`, and a `servers` value that is absent or not an array. Everything else moves per-entry, including the duplicate-URL and invalid-value classes that stop startup today. An empty or fully-disabled entry list ceases to be an error, because per-entry admission already makes "no usable server" a reachable state the host must survive and present. The full table is in `design.md` under the `fr-030` anchor.

### Clauses of these requirements discharged in the CLI change

Three of the five requirements carry a trailing clause that binds a client rather than the plane. The clause is **not** restated as a CLI requirement — it is discharged inside `change-20260812-cli-terminal-client` under the contract named here, so each sentence has exactly one owner.

| Requirement | Clause bound to the CLI | Discharged by |
|---|---|---|
| FR-026 | "the CLI shall never reconstruct that observation from local memory" | `contract-explicit-terminate` and `unit-lifecycle-commands` task boundary, under NFR-004 |
| FR-032 | "the CLI shall derive both its transport failure classification and its presented reason from that value" | `contract-exit-classification`, verified by `ver-cli-maps-each-close-reason-to-its-exit-class` |
| NFR-009 / FR-028 / FR-030 | the CLI's presentation of a configuration-invalid entry and of a qualifier that cannot be honoured | CLI-owned FR-029 and FR-031 |

### Requirements of the CLI change that depend on this one

These stay in `change-20260812-cli-terminal-client/requirements.md`. They are listed here only so the dependency edge is visible from both sides; their statements are not duplicated.

| CLI requirement | Depends on |
|---|---|
| FR-014 — the three departure classes are told apart after restore | FR-026 (the session-end class) and FR-032 (the transport-loss reason) |
| FR-015 — the CLI removes nothing the server publishes, including ended sessions | FR-026 |
| FR-019 / FR-021 — owning server on every row; qualifier by display name or stable id | NFR-009 (one derived server identifier) and FR-028 (a display name that can be addressed) |
| FR-024 — fixed exit codes, with transport failure separated by its stated reason | FR-032 |
| FR-029 — a qualifier that cannot be honoured says why | FR-028 and FR-030 (the admission states it reports) |
| FR-031 — an unusable entry stays visible in its own vocabulary | FR-030 |
| FR-003 / NFR-003 — configuration is never written; directory matches the existing hosts | NFR-009 (the single Go loader that resolves and reads it) |
| FR-022 — creation requires an explicit target server | NFR-009 (`default_execution` is loaded but never read as a target) |

### Acceptance criteria

Criteria whose primary requirement is owned here. Where a criterion is only partly discharged by this change, the CLI-side half is named.

| ID | Criteria | Requirements | Cross-change half |
|---|---|---|---|
| AC-ended-session-remains-observable | A session that ended by explicit termination, by clean or signalled self-exit, or by a vanished backend frame is still listed with a terminal state rather than disappearing, and is distinguishable from an identifier that never existed without any client-side memory. | FR-026, FR-015, FR-023 | — |
| AC-transport-close-is-typed | A capacity refusal, a going-away shutdown, a backend loss, a rejected ticket, a protocol mismatch and an unexplained network drop are told apart from the public contract alone — each server-initiated departure carrying one reason from a published closed vocabulary, each pre-upgrade rejection carrying the same vocabulary in its error body, and only a departure that carries nothing being presented as unattributed. The CLI's exit class and printed reason follow that value and are never invented locally. | FR-032, FR-014, FR-024 | CLI: `ver-cli-maps-each-close-reason-to-its-exit-class` |
| AC-config-parity-across-languages | The same configuration document produces the same accept-or-reject decision, the same per-entry admission states, the same canonical URLs and the same derived server identifiers in Go, TypeScript and C#, proven against one shared corpus in which every fixture carries its expected outcome; and no client language holds a second loader of that document. | NFR-009, NFR-003, FR-021 | — |
| AC-config-entry-admission | One bad entry costs one server rather than the whole client — the host starts, exactly the offending entry and its collision partners are unusable and unprobed, a duplicated URL behaves the same way as a duplicated display name, and only a document whose entry list cannot be read at all stops the host. A configuration-invalid entry is visible in both the table and the machine-readable output, in a vocabulary that shares no value with reachability and none with session lifecycle state. | FR-030, FR-031, FR-028 | CLI: `ver-invalid-entry-is-a-third-vocabulary-in-discovery`, `ver-invalid-entry-is-never-connected-to` |
| AC-server-naming-and-qualifier-outcomes | A configuration with duplicate or ill-formed display names lets every host start while marking exactly the offending entries configuration-invalid, identically in all three loaders and with the entry named rather than repaired; a qualifier matching two usable entries exits as ambiguous instead of selecting one; and a qualifier whose every match is configuration-invalid exits as a usage error naming each entry and its violated rule. | FR-028, FR-029, FR-021 | CLI: `ver-qualifier-precedence-unusable-outranks-ambiguous`, `ver-qualifier-narrows-cross-server-collision` |

The remaining seventeen acceptance criteria of the shared plan belong to `change-20260812-cli-terminal-client`. One of them, `AC-disconnect-classes-distinct`, is discharged jointly: this change contributes `ver-terminal-transition-is-published-on-lifecycle`. `AC-boundary-and-gates` likewise takes `ver-config-never-written-and-directory-per-platform` from here.

### Why these requirements exist

Each of the three prerequisite groups came from a measured falsification of an upstream experience assumption, not from a new design idea. The measurements are in the shared plan; the summary is:

- **Ended sessions (UAC-007, UAC-029).** DP-LIST-LIFECYCLE-SCOPE settled that ended sessions appear in the listing. The execution plane deletes the session record on three of the four termination paths, so no client-side implementation can satisfy the observation. FR-026 is the prerequisite; the CLI's own obligation was narrowed to FR-015. The retention window and what reclaims a terminal row stay open (`open-terminate-observability`).
- **Transport loss as an informative class (UAC-018, UAC-019).** The public contract carries no typed reason for a connection ending: one close code, four unmodelled strings, most departures sending nothing, and ticket rejections answered in plain text before the socket exists. FR-032 makes the class carry information instead of one word covering a rejected credential, a draining server and a pulled cable.
- **`--server` by display name (UAC-032, UAC-025).** The upstream Given assumes a naming prerequisite that does not hold. FR-028 gives it an enforcement point. Consultation directed that the enforcement be per entry rather than per file, which produced FR-030.
- **Configuration parity across client languages.** The three loaders of the shared `servers.json` contract disagree on unknown fields, on URL credentials — which splits the derived server identity between hosts — and on the Windows configuration directory, with only shape-only fixture tests to catch it. NFR-009 makes agreement mechanical so the identity FR-019 puts in machine-readable output has one value across hosts.
