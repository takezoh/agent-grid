---
id: change-20260812-cli-terminal-client
kind: change
title: Terminal CLI client for Agent Grid sessions
status: active
created: '2026-08-12'
profile: sdd@1
intent: server が所有する既存 session へローカル terminal から attach / detach / reattach できる terminal-first
  client を、公開 wire contract の consumer として追加する。terminal を持たない client host
  だけが利用できていた session 監督を、shell の中から同じ意味論で行えるようにする。
outcomes:
- clients/cli は services module への dependency edge を持たない 5 番目の Go module として、build
  / test / vet / lint / verification profile / protocol-version projection の全 entry point
  に登録された状態で存在する。
- discovery / attach / create / terminate が、既存 client host と同じ server-qualified identity、partial
  failure、status vocabulary で観察できる。
- raw mode は established な lifecycle outcome の後にだけ入り、8 つの exit path すべてで terminal がちょうど一度復元される。
- 失敗は 6 つの class と固定 exit code に解決され、published close reason を持たない離脱は推測されず unattributed
  として報告される。
- CLI が読む session payload の形は server 側 serializer が byte 単位で再現する golden vector で固定され、rename
  は rename した側で落ちる。
scope:
- clients/cli の新規 Go module と 13 unit のうち 12 unit（module skeleton、transport admission、wire
  codec、fleet、status vocabulary、discovery render、selector、tty adapter、fake gateway、attach
  runtime、lifecycle commands、phase0 contract pin）
- protocol/fixtures の SessionInfo golden vector と、その対称的な consumer registry gate
- test-harness の dependency triple / e2e suite / profile 登録と、e2e suite registry への module
  field 追加
- Makefile・scripts・docs の module enumeration 更新
non_goals:
- services module への require / replace / internal import
- PTY 所有、ownership / takeover / 排他制御
- replay・transcript・scrollback の保持
- CLI 専用 endpoint の追加
- user configuration の書き込み
- Windows console / BSD の入力キュー破棄と size 変更観測（unit-platform-hardening-spikes として未取得）
change_classes:
- capability
- boundary
- dependency
- invariant
governance:
  gate: auto
  reasons:
  - 既存 client host の観察を変えず、公開 contract の consumer を 1 つ追加する。boundary は module graph
    と gate で機械的に強制される。
members:
- role: requirements
  path: changes/change-20260812-cli-terminal-client/requirements.md
  required: true
- role: implementation
  path: changes/change-20260812-cli-terminal-client/implementation.md
  required: true
- role: verification
  path: changes/change-20260812-cli-terminal-client/verification.md
  required: true
- role: ux
  path: changes/change-20260812-cli-terminal-client/ux.md
  required: false
promotion:
- target: design-clients-cli
  section: capabilities
  action: upsert
  item:
    id: cap:terminal-session-client
    statement: clients/cli は公開 wire contract の terminal client として discovery / attach
      / create / terminate を提供する独立 Go module である。
  reason: 新しい恒久 client-host capability を記録する。
- target: design-clients-cli
  section: boundaries.forbidden
  action: upsert
  item:
    id: boundary-cli-no-services-edge
    statement: clients/cli は services module への依存、PTY 所有、ownership/takeover、replay store、CLI
      専用 endpoint のいずれも持たない。
  reason: 元要件の boundary rule を将来変更が緩められない形で固定する。
- target: design-clients-cli
  section: invariants
  action: upsert
  item:
    id: invariant-single-owner-exit-and-identity
    statement: exit code 割当、server-qualified identity の構築、outbound serialization、terminate
      の emission site はいずれもちょうど 1 箇所が所有し、2 箇所目の出現は gate が落とす。
  reason: review 規約ではなく機械的検査で保つ責務単一性を記録する。
- target: none
  section: none
  action: none
  item: {}
  reason: design-clients-ui と design-workspace-desktop は本 change で変更されない。CLI は host kind
    による分岐ではなく同じ contract の別 transport であり、両 design の invariant はそのまま成り立つ。design-workspace-desktop
    RESP-003 が名指す "the CLI" は adr-20260814-cli-canonical-binary-name により agent-grid
    と特定できるようになったが、control endpoint の consumer 実装は本 change の scope 外。
promotion_applied_at: '2026-08-14T00:00:00Z'
unresolved_decisions:
- 'open-ambiguous-candidate-breadth: 曖昧候補行に併記する情報の広さ（所属 server と display name を併記するか ID
  だけか）。利用者へ escalate 済みで未回答。現行実装は --id / --server / display name / cwd を併記する。UAC-031
  は候補同士の相互判別可能性だけを要求しており広さには依存しないため、実装は要件を満たしたまま回答を待てる。'
tags:
- native-clients
- cli
- go-module
- terminal
owners: []
relations:
- {type: dependsOn, target: change-20260814-cli-prerequisite-contracts}
- {type: conformsTo, target: design-clients-cli}
- {type: modifies, target: design-clients-cli}
source_paths:
- clients/cli
- protocol/fixtures
- services/gorules
- services/host/state/view
- services/server/api
- services/internal/harnesspolicy
- test-harness
- scripts
- Makefile
- AGENTS.md
summary: 既存 server-side の Claude Code / Codex session へローカル terminal から attach / detach
  / reattach する terminal-first CLI client の explore requirements
updated: '2026-08-14'
---

## Summary

`clients/cli` is a fifth Go module and a client of the public wire contract:
no `require`/`replace` on `services`, no PTY, no ownership or takeover concept,
no CLI-only endpoint. It presents discovery, attach, create and terminate with
the same server-qualified identity, partial-failure and status semantics the
existing client hosts use, and it holds a single owner for exit classification,
for identity construction, for outbound serialization and for the terminate
emission site — each backed by a scan that fails when a second site appears.

Governing design: `design-clients-cli`, created when the promotion manifest was
applied. `design-host` (id `design-client`) does not govern clients in general;
its `source_paths` are the server tree and the web host.

## Closure Notes

Outstanding before `closing`:

- `unit-platform-hardening-spikes` — the three platform observations need a real
  Windows console and a real agent session. The Windows size-change source and
  input-queue discard stay explicitly unimplemented until they exist.
- `open-ambiguous-candidate-breadth` is still with the user.
- `TestE2E_GatewayScenarioWorkspaceFileWrite` and
  `TestE2E_GatewayScenarioFakeCodexSurfaceAndSessionState` in
  `services/server/api` fail deterministically on a host that permits unix
  sockets. They fail identically at `bcf6d844`, before any CLI work, so they are
  pre-existing rather than caused here — but they are red, and this change
  should not close while claiming a green repository.

Both fidelity backstops have now been observed passing.
`cli-gateway` found three defects on its first real run — a wrong scheme in the
harness, a production transport asymmetry now recorded as
`design-clients-cli` INV-011, and three keys missing from the fake's hello
frame. That is the backstop doing its job, and it is why "registered" and
"observed passing" are tracked as separate claims.

`handoff-cli-binary-canonical-name` is closed:
`adr-20260814-cli-canonical-binary-name` fixes the installed name at
`agent-grid` and declines to claim `ag`.


{% transition from="draft" to="ready" date="2026-08-14" %}
design closure passed; 11 ADR accepted via consultation-20260814-cli-terminal-client-design-final
{% /transition %}


{% transition from="ready" to="active" date="2026-08-14" %}
main session implementation; twelve of the thirteen units landed. unit-platform-hardening-spikes is outstanding: its three observations need a real Windows console and a real agent session, and the Windows paths are explicitly unimplemented rather than guessed (verification.md "Not yet acquired").
{% /transition %}
