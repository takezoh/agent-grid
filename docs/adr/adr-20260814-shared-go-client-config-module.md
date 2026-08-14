---
id: adr-20260814-shared-go-client-config-module
kind: adr
title: Make clients/config the canonical client configuration contract with one loader
  per language and a shared expected-outcome corpus
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
- {type: references, target: adr-20260727-host-neutral-fleet-provider}
- {type: references, target: adr-20260727-single-user-server-auth}
- {type: references, target: adr-20260727-server-qualified-identity-partial-failure}
- {type: references, target: change-20260814-cli-prerequisite-contracts}
source_paths:
- clients/config
- clients/goclient
- clients/web-host/internal/clientconfig
- clients/workspace/src/main/desktop-config.ts
- clients/windows-shell/AgentGrid.Shell.Core/Configuration/DesktopConfig.cs
consequences:
  positive:
  - The same configuration document yields the same decision, the same admission states,
    the same canonical URL and the same server identifier in all three client languages,
    proven rather than asserted.
  - The two Go client hosts derive server identity from one implementation, and the
    missing Windows configuration-directory branch is fixed once instead of reproduced.
  negative:
  - Where the three loaders disagree today the corpus forces one answer, so at least
    one shipping host changes behaviour deliberately.
  - A third Go module, a third set of gate entries, and a corpus reader in each of
    three languages.
  neutral:
  - The SPA is unaffected — it consumes a credential-free projection rather than the
    file, so it inherits parity through whichever host serves it.
  - A host's own runtime settings stay outside this contract; only the shared client
    configuration document is bound.
confirmation: One shared corpus in which every fixture carries its expected outcome
  is read by the Go, TypeScript and C# loaders and asserted field by field; a structural
  check fails when a client language holds a second parser of the document; and the
  three measured divergences are closed in the direction the contract declares.
summary: Treat clients/config as the normative client configuration contract for every
  client host, give each client language exactly one loader of it, and prove three-language
  agreement with a shared fixture corpus whose fixtures carry their expected outcomes.
updated: '2026-08-14'
---

# Make clients/config the canonical client configuration contract with one loader per language and a shared expected-outcome corpus

## Context

The CLI needs to read `servers.json`. The obvious framing — "extract the Go loader out of the web host so the CLI does not become a fourth copy" — is what an earlier revision of this decision proposed, and the repository falsifies it.

`clients/config/README.md:3-4` already states the contract is shared: "Web host, Windows Shell, and Workspace share the `servers.json` version 2 contract." Three loaders read it — Go (`clients/web-host/internal/clientconfig/config.go`), TypeScript (`clients/workspace/src/main/desktop-config.ts`) and C# (`clients/windows-shell/.../DesktopConfig.cs`) — and the SPA consumes a credential-free projection of the result, over `GET /host/config` in the browser (`clients/web-host/internal/webhost/host.go:41-46`) and over preload IPC in Workspace (`clients/workspace/src/preload/electron-preload.cjs:28-74`). There is no sense in which the web host owns the contract. Naming it as the origin picks one consumer as the definition and leaves the others to drift.

They have drifted, measurably:

| Behaviour | Go | TypeScript | C# |
|---|---|---|---|
| unknown field on an entry | silently dropped (`encoding/json` default) | rejected, `"unsupported field '…'"` (`desktop-config.ts:276-282`) | rejected, `UnmappedMemberHandling.Disallow` (`DesktopConfig.cs:66`) |
| credentials / query / fragment in a URL | rejected (`config.go:143`) | silently stripped (`desktop-config.ts:328-332`) | rejected (`DesktopConfig.cs:159-162`) |
| configuration directory on Windows | no branch — `~/.config/agent-grid` (`config.go:40-49`) | `%APPDATA%\agent-grid\config` (`desktop-config.ts:82-89`) | `%APPDATA%\agent-grid\config` (`DesktopConfig.cs:69-75`) |
| duplicate-URL error names the URL | yes (`config.go:91-93`) | yes (`desktop-config.ts:302-304`) | no — `"server ids must be unique"` (`DesktopConfig.cs:132-133`) |

The second row is the one that matters most. The server identity is `srv_` plus the SHA-256 of the **canonical** URL, and canonicalisation differs, so the same `servers.json` yields a different server identity in Workspace than in the web host. `adr-20260727-server-qualified-identity-partial-failure` makes identity `(serverId, localId)` end to end, and the upstream experience contract makes that `serverId` a machine-readable value a script hands straight back as a server qualifier.

Nothing today can catch this. `clients/config/servers.schema.json` is read by no code at all. Three fixture tests exist and every one is shape-only: Go reads **one** of the four fixtures and asserts `Load` succeeds (`config_test.go:10-35`); TypeScript asserts a bare `.not.toThrow()` / `.toThrow()` (`desktop-config.test.ts:339-365`); C# asserts `Assert.Throws<DesktopConfigException>` (`DesktopConfigTests.cs:215-243`). None compares *which* rule fired, so three loaders can reject the same document for three different reasons and the suite stays green. And no fixture covers duplicate URLs, multiple default-execution entries, zero enabled entries, or an unparseable document — the four cases the README's own sentence promises.

The source requirement's extraction rule says to extract once "another Go client or automation consumer actually needs it". That trigger is met. But extraction alone answers only the Go third of the problem.

## Decision

**1. `clients/config` is the canonical client configuration contract.** The document shape, the admission rules, the canonical URL and server-identity derivations, and the per-platform configuration-directory resolution are defined there, and every client host — CLI, web host, Workspace, Windows Shell — is a consumer of equal rank. No host is the reference implementation.

**2. Exactly one loader per client language.** Go gets one, shared by the web host and the CLI: the existing package is moved out of `clients/web-host/internal/clientconfig` into an importable module, carrying the canonical URL and `srv_` digest derivations byte-identically, with the missing Windows directory branch added as a declared change. TypeScript and C# already have one each. That a language holds no second parser of the document is checked structurally, because a second parser is invisible in the diff that adds it.

**3. Parity is proven by one shared corpus.** Every rule the contract declares is a fixture, and every fixture carries its **expected outcome as data**: the accept-or-reject decision, the admission state of each entry, the canonical URL, and the derived server identifier. All three loaders read the same corpus and assert against the same record. Adding a rule to the contract means adding a fixture; it must not mean editing three test bodies that can disagree about what they assert.

**4. The three measured divergences are closed** in the direction the contract declares, each pinned by a corpus entry.

**5. A host's own runtime settings are out of scope.** The web host takes its listen address, TLS material and configuration directory from flags and embeds its static assets; the desktop hosts read appearance and process settings beside `servers.json`. Those are per-host concerns that happen to share a directory. Binding three languages to settings only one of them has would be parity for its own sake, and a host that needs server-side configuration gets its own design.

## Consequences

{% consequence kind="positive" %}
The same document produces the same decision, the same per-entry admission states, the same canonical URLs and the same server identifiers in Go, TypeScript and C# — proven against one corpus rather than asserted in a README. The identity divergence that today gives one server two `srv_` values across hosts is closed, and that identity is exactly the value automation passes back as a server qualifier. The two Go hosts derive it from one implementation, and the missing Windows directory branch is fixed once instead of reproduced into a fifth consumer. Adding a rule to the contract becomes a fixture, so the next rule cannot land in one language and not the others.
{% /consequence %}

{% consequence kind="negative" %}
Where the three loaders disagree today the corpus forces one answer, so at least one shipping host changes behaviour deliberately — and the corpus is the record of which, which is the point rather than a side effect. The cost is a third Go module, a third set of gate entries, changed imports inside a shipped client host, and a corpus reader in each of three languages. The move must be behaviour-preserving apart from the declared changes, which limits how much incidental cleanup can ride along.
{% /consequence %}

{% consequence kind="neutral" %}
The SPA is untouched: it never reads the file, only the credential-free projection (`projection.go:3-13`), so it inherits parity through whichever host serves it and needs no fourth loader. `servers.json` schema v2, the read-once-no-reload rule, the `none` XOR `bearer` authentication shape and the deprecated `id` field are all carried across unchanged. `clients/config/servers.schema.json`, which no code reads today, is either wired into the loaders or explicitly declared documentation-only, so the document and the behaviour cannot disagree silently.
{% /consequence %}

## Alternatives

- **A fourth hand-written implementation inside the CLI, pinned with the existing fixtures** — rejected: it multiplies a measured divergence at the moment a display name is becoming an addressable identifier, and shape-only fixture pinning cannot detect the divergence anyway, as the three existing tests demonstrate by passing while the loaders disagree.
- **Extract the Go loader and stop there** (the earlier form of this decision) — rejected: it fixes one of three languages, leaves the URL-canonicalisation divergence that splits server identity between Workspace and the web host, and encodes the false premise that the web host owns a contract three clients share.
- **Generate all three loaders from `servers.schema.json`** — genuinely stronger, and rejected only for scope: it changes the SDK generation surface for four languages and needs its own ADR, and the schema is read by no code today, so generation is a larger step than making the contract checkable. Recorded as a follow-up.
- **Vendoring the Go package by copy** — rejected: same divergence, and no shared test surface at all.
- **Making `clients/web-host/internal/clientconfig` non-internal in place** — rejected: the loader would stay inside a shipped client's module, so the CLI's dependency graph would reach a client host. The move is barely larger and leaves a cleaner owner.
- **Including each host's own runtime settings in the parity obligation** — rejected: only one host has most of them, so parity would be defined over a set two of the three languages do not implement.

### If this is not accepted

The CLI becomes a fifth consumer of a contract three implementations already read three different ways, and the `srv_` identity it prints for a server with a query string in its URL depends on which host the operator compares it against. The four failure classes the README promises stay untested in every language, and the naming and per-entry-admission rules of `adr-20260814-server-display-name-naming-contract` land in three loaders with nothing able to tell whether they still agree afterwards.

Confirmation: `ver-expected-outcome-corpus-agrees-in-three-languages` (shared corpus through the pull-request schema profile), `TestExpectedOutcomeCorpus` and `TestExpectedOutcomeCorpusCoversEveryDeclaredRule` in the Go loader, `TestUnknownFieldIsRejected` / `TestUrlCredentialsAreRejectedNotStripped` for the closed divergences, `TestClientConfigHasExactlyOneLoaderPerLanguage` for the structural check, and `TestServerIdMatchesCrossLanguageGoldenVector` / `TestConfigDirectoryMatchesExistingHostsPerPlatform` / `TestConfigDirectoryIsNeverWritten` for identity and directory resolution.

## Related

- requirements: `NFR-009`, `NFR-003`, `FR-003`, `FR-019`, `FR-021`, `FR-022`
- contracts: `contract-client-config-parity`, `contract-config-and-fleet-identity`
- references: `adr-20260727-host-neutral-fleet-provider`, `adr-20260727-single-user-server-auth`, `adr-20260727-server-qualified-identity-partial-failure` (the identity this contract keeps single-valued)
- open: `open-web-host-server-side-config` (a host's own runtime settings, deliberately out of scope)
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md` anchor `adr-shared-go-client-config-module`


{% transition from="proposed" to="accepted" date="2026-08-14" %}
consultation-20260814-cli-terminal-client-design-final
{% /transition %}
