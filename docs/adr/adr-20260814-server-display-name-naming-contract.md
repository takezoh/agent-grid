---
id: adr-20260814-server-display-name-naming-contract
kind: adr
title: Enforce unique CLI-safe server display names per entry, keeping the client
  host running
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- native-clients
- config
- protocol
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: design-windows-shell}
- {type: references, target: adr-20260814-shared-go-client-config-module}
- {type: references, target: change-20260814-cli-prerequisite-contracts}
source_paths:
- clients/config/servers.schema.json
- clients/config/README.md
- clients/config/fixtures
- clients/web-host/internal/clientconfig/config.go
- clients/web-host/internal/webhost/host.go
- clients/workspace/src/main/desktop-config.ts
- clients/windows-shell/AgentGrid.Shell.Core/Configuration/DesktopConfig.cs
consequences:
  positive:
  - A display name becomes an addressable identifier with one meaning, enforced identically
    in Go, TypeScript and C# against shared fixtures.
  - One bad entry costs one server instead of the whole fleet on every host, so a
    typo in an unused server stops taking down the ones in use.
  negative:
  - Three shipping loaders change their failure model from whole-file rejection to
    per-entry admission, which is a larger change than adding the naming rule alone.
  - The loud "the host refuses to start" signal is removed for a class of configurations,
    so the unusable entry must be presented or the failure becomes silent.
  neutral:
  - Uniqueness is exact code-point equality after trimming, so no locale-dependent
    case or Unicode folding is introduced into any loader.
  - Only a document whose entry list cannot be read still stops a host, which keeps
    one file-level rule instead of a list of exceptions.
confirmation: Shared fixtures are admitted identically by the Go, TypeScript and C#
  loaders; a configuration with a duplicate or ill-formed display name starts the
  host with exactly the offending entries marked configuration-invalid and named rather
  than renamed; and a truncated document still stops the host with an error that names
  no entry.
summary: Make the operator-configured server display name unique and CLI-safe, enforced
  at configuration load in all three client hosts as a per-entry admission failure
  that leaves the host running, with an actionable report naming the offending entry.
updated: '2026-08-14'
---

# Enforce unique CLI-safe server display names per entry, keeping the client host running

## Context

The upstream experience contract settled that `--server` accepts the operator-configured display name as its value, and recorded the consequence as a confirmed handoff: the name must be unique and restricted to characters usable as a CLI argument. The handoff explicitly assigns to design the concrete shape — which characters are allowed, where uniqueness is enforced, and how existing configurations migrate.

Measurement shows the constraint has no enforcement point anywhere today. `clients/config/servers.schema.json:22` constrains a display name only with `minLength: 1` — and no code reads that schema at all; all three loaders hand-roll their validation. The uniqueness check in every loader is keyed on the **URL digest**, not the name: `clients/web-host/internal/clientconfig/config.go:91-93` (`"duplicate server url %q"`), `clients/workspace/src/main/desktop-config.ts:302-304`, `clients/windows-shell/.../DesktopConfig.cs:132-133` (`"servers.json: server ids must be unique"`). The shared SPA even contains disambiguation logic that presumes name collisions occur.

So a display name passed to `--server` today can match two configured servers, and nothing in the system defines what should happen.

### The second question the constraint forces

Adding a rule requires deciding what a violation costs. The documented behaviour today is whole-file rejection: `clients/config/README.md:41-43` — "Configuration is loaded once at process start; malformed files, unsupported versions, duplicate server URLs, and invalid values stop startup with a file-specific error" — and all three loaders implement it. Go returns an error from `Load` for every failure; TypeScript throws out of `loadOrCreateDesktopConfig`; C# throws `DesktopConfigException`.

That is the wrong blast radius for a rule about one entry. A typo in the display name of a server the operator is not currently using would take away every other server, on every host, at once — and the rule is being added precisely because display names are becoming addressable, so the class of configurations that can trip it is about to grow. The user consultation chose the per-entry direction and extended it to the error classes that can already be closed at a single server: URL duplication above all.

### Why this is a permitted server-side contract change

The configuration contract is shared by every client host, so this satisfies the source requirement's conditions for a prerequisite change: the capability (a name guaranteed to identify one server) does not exist; it cannot be expressed by any single client, since a client that enforced it alone would reject a configuration its siblings accept; and every host benefits, because a name that identifies one server is what the SPA's disambiguation logic is currently working around, and because per-entry admission means an operator error stops costing every host its whole fleet.

**Execution responsibility is not moved into the CLI.** The CLI does not normalise, deduplicate or repair configuration. It validates nothing the other hosts do not validate; it only consumes the guarantee.

## Decision

**Uniqueness.** Among enabled servers, display names are unique by exact code-point equality after trimming leading and trailing whitespace.

**Character set.** After trimming, a display name is non-empty, contains no ASCII control character (U+0000–U+001F, U+007F), and does not begin with `-`.

**Enforcement is per entry.** Validation happens at configuration load in all three loaders. A violating entry — and every entry it collides with — is admitted in the `configuration-invalid` state: the host starts, those entries are never probed and never connected to, and the loader never renames, normalises, appends a suffix, disables an entry, or picks an arbitrary winner. Each such entry is reported with its position in the entry list, its display name as written, and the rule it violates.

**No winner is chosen in a collision.** A duplicate is a property of a pair, so blaming one member requires choosing which, and every choice rule — first wins, last wins, enabled wins, lexical — silently redirects an operator's existing `--server <name>` to a different machine. Marking the whole collision group invalid is the only rule decidable without a choice.

**The partition of error classes.** An error is per-entry exactly when it can be attributed: decidable from one entry alone, or from a relation among a named set of entries. It is file-level exactly when no entry can be named.

| Class | Today | Under this decision |
|---|---|---|
| bytes unreadable, content not JSON | file-level stop | file-level stop (unchanged) |
| `schema_version` absent or not 2 | file-level stop | file-level stop (unchanged) |
| `servers` absent or not an array | file-level stop | file-level stop (unchanged) |
| display name blank / outside charset | unenforced | per-entry |
| display name duplicated | unenforced | per-entry, whole collision group |
| URL duplicated | file-level stop | per-entry, whole collision group |
| URL invalid (scheme, credentials, host) | file-level stop | per-entry |
| auth shape invalid | file-level stop | per-entry |
| unknown field on an entry | file-level stop (TS/C#), silently ignored (Go) | per-entry, and the divergence is closed |
| `enabled` / `default_execution` not boolean | file-level stop | per-entry |
| more than one `default_execution` | file-level stop | per-entry, whole marked group |
| `servers` empty, or zero enabled | file-level stop | not an error — an empty usable fleet, fully observable |

The last row goes one step beyond the classes the consultation named, and deliberately. Once entries can individually become unusable, "no usable server" is already a reachable state the host must survive and present; keeping a *separate* fatal path to the same observable would mean the same end state stops the host on one route and not on another. One rule — the entry list is readable or the host stops — is what per-entry admission actually costs, and stating it as a list of exceptions would leave the incoherence in place.

**Visibility is part of the decision, not a follow-up.** A configuration-invalid entry appears in the client's discovery output, in a vocabulary disjoint from both server reachability and session lifecycle state. Without that, per-entry admission would be strictly worse than the whole-file stop it replaces: the host would start, a server would silently be missing, and the operator would get no signal. The stop was at least loud.

**Qualifier resolution.** Resolution runs over every configured entry, not only the usable ones, so a qualifier naming a configured server never reports not-found. In order: every match unusable → usage error naming each entry and its violated rule; two or more usable matches → ambiguous, listing them distinguishably and selecting none; exactly one usable match → resolved; zero matches → not-found, or transport if a server could not be read. Unusability outranks ambiguity because when no candidate is choosable, the only next action is editing the configuration.

**Fixtures.** The shared corpus introduced by `adr-20260814-shared-go-client-config-module` gains a fixture for every per-entry and file-level class above, each carrying its expected admission outcome, and all three loaders assert against it.

## Consequences

{% consequence kind="positive" %}
A display name becomes an addressable identifier with exactly one meaning, enforced identically in three languages against shared fixtures rather than asserted in a document. The CLI's `--server` value stops being ambiguous by construction, and the SPA's collision disambiguation stops being load-bearing. Separately and more broadly: one malformed entry now costs one server instead of the entire fleet on every host, which is the behaviour an operator would expect from a fleet client and is not what any of the three hosts does today.
{% /consequence %}

{% consequence kind="negative" %}
Three shipping loaders change their failure model, not just their rule set — that is a larger change than adding the naming constraint alone, and it touches paths (duplicate URL, invalid auth, unknown field) that have nothing to do with display names. The loud "the host refuses to start" signal disappears for a class of configurations that trip it today, so the failure becomes silent unless the invalid entry is presented; that is why visibility is part of this decision rather than a follow-up. Two concrete edits fall out: `clients/config/README.md:41-43` states the whole-file rule and must be rewritten, and `clients/web-host/internal/webhost/host.go:30-31`, which refuses a snapshot with no servers, must be relaxed.
{% /consequence %}

{% consequence kind="neutral" %}
Uniqueness is exact code-point equality after trimming, so no locale-dependent case folding or Unicode normalisation enters any loader. Configurations that already satisfy the constraint are unaffected, and rollback is reverting the loader change with no data change. The credential-free host projection keeps its current shape; widening it so the SPA can show the new state is a separate change, since the CLI reads the loader directly.
{% /consequence %}

## Alternatives

- **Reject the whole configuration and stop the host, as the README documents today** — rejected by the consultation, and on its merits: a rule about one entry that costs every host its whole fleet has the wrong blast radius, and the rule is being added at the moment the space of configurations that can trip it grows.
- **Case-insensitive or NFC-folded uniqueness** — rejected: it requires Go, TypeScript and C# to agree exactly on a fold, which is a new cross-language divergence source in the very code path this change exists to make consistent, and there is no measured need for it.
- **Enforce uniqueness only in the CLI** — rejected: the CLI would reject a configuration the graphical clients accept, which is a worse inconsistency than the one being fixed, and the constraint exists precisely because the name is becoming addressable for every host.
- **Auto-suffix or auto-rename duplicates** — rejected: it silently changes an operator-authored identifier, and any script or bookmark referring to the old name breaks without a message.
- **Blame one member of a collision and keep the other usable** — rejected: every winner rule silently redirects an existing `--server <name>` to a different machine.
- **Let the CLI pick the first match** — rejected: it contradicts the settled decision that ambiguity is reported rather than resolved silently.
- **Move the new naming rules per-entry but leave the existing classes file-level** — rejected: two failure disciplines inside one loader is worse than either alone, and it would leave a duplicate URL costing the fleet while a duplicate name costs one server, for no principled reason.

### If this is not accepted

The naming constraint stays unenforced and the display-name paths of two upstream acceptance scenarios hold only for fleets that happen to have unique names; addressing a server by stable identifier keeps working in every case. Separately, whole-file rejection stays, so any invalid entry keeps costing every host its entire fleet — including the duplicate-URL case that exists today and is unrelated to this change's motivation.

Confirmation: `ver-duplicate-display-name-flagged-per-entry-by-every-loader` (shared corpus through the pull-request schema profile), `TestDisplayNameCharsetRejection`, `TestNamingViolationReportsOffendingEntryWithoutRenaming`, `TestHostStartsAndOnlyOffendingEntriesAreInvalid`, `TestUnreadableEntryListIsAFileLevelStop`, and `TestUnusableMatchesExitUsageAndAmbiguousOnlyAppliesToUsableOnes` for qualifier precedence.

## Related

- requirements: `FR-028`, `FR-029`, `FR-030`, `FR-031`, `FR-021`
- contracts: `contract-server-naming-prerequisite`, `contract-config-entry-admission`, `contract-config-and-fleet-identity`
- references: `adr-20260814-shared-go-client-config-module` (the corpus this decision's fixtures live in)
- governance: `clients/config/` is currently owned by `design-windows-shell` (RESP-005 / INV-009), so this requires an upsert of that design; carving out a `design-clients-config` is recorded as a promotion candidate, not decided here (`open-config-contract-design-ownership`)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-server-display-name-naming-contract`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
