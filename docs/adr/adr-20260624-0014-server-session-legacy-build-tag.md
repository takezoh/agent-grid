---
id: adr-20260624-0014-server-session-legacy-build-tag
kind: adr
title: ADR 0014 — Quarantine `server/session/` behind build tag `legacy_session`
status: superseded
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations: []
source_paths:
- src/cmd/server/
decision_makers:
- unknown
---

<!-- migrated_from: docs/adr/0014-server-session-legacy-build-tag.md -->

# ADR 0014 — Quarantine `server/session/` behind build tag `legacy_session`

Status: Superseded by removal in A1-ε (2026-06-20)

## Context

A1-α removes `cmd/server`'s dependency on `server/session.Service`. Three
options for handling the now-unused package:

1. **Empty out the file** — minimise diff but the existing
   `service_test.go` and `mux_test.go` (which reference `session.NewService`,
   `session.Spec`, `session.Info`) stop compiling. Tests must be deleted in
   the same PR, ballooning the diff.
2. **Delete the directory** — same problem plus loses the clean ε deletion
   boundary.
3. **Leave it untouched** — depguard / lint flag the package as unused, and
   any new contributor reads it as still-active code.

## Decision

Add `//go:build legacy_session` to every file under `server/session/` plus
the existing tests that depend on it (`service_test.go`, the relevant parts
of `mux_test.go`). Normal builds and `go test ./...` ignore the package.

In PR ε, the cleanup commit becomes `git rm -r server/session/` plus the
matching test file deletions — a single mechanical change.

## Consequences

- A1-α's diff stays focused on the gateway transformation, with no test-file
  noise.
- ε becomes a pure-deletion commit, trivial to review and revert.
- A short-lived `legacy_session` tag exists between α and ε; build failures
  on the regular build path catch any forgotten tag immediately.
- Documentation must note the tag is transient (removed in ε).
- **2026-06-20 (A1-ε)**: 削除完了。`src/server/session/` ディレクトリと
  `legacy_session` build tag を持つ test 3 ファイル(`gateway_test.go` / `inbound_test.go`
  / `mux_test.go`)を `git rm` で削除。本 ADR の隔離戦略は当初の計画どおり寿命を終えた。

## Alternatives

- **Empty the file in α** — rejected; breaks tests, bloats α PR.
- **Delete in α** — rejected; collapses the α/ε boundary.
- **Leave untouched** — rejected; lints would flag the unused code and
  reviewers waste time on dead paths.

## Related requirements

(none directly)
