---
id: change-20260814-cli-prerequisite-contracts
kind: change
title: Execution plane and client config prerequisites for the terminal CLI
status: active
created: '2026-08-14'
profile: sdd@1
intent: terminal CLI が依存するが CLI では実装できない 3 つの前提 contract — 終了 session の terminal state
  保持、公開された transport close reason 語彙、全 client host 共通の configuration 契約 — を、CLI
  より先に、CLI に依存しない形で確定させる。client が推測で埋めていた事実を、公開契約として観測可能にする。
outcomes:
- 終了した session は消えるのではなく terminal な lifecycle state を持つ行として観測でき、その遷移が lifecycle socket
  へ publish される。client は行の消失ではなく published transition から離脱理由を導ける。
- server 起点の close と upgrade 前の拒否応答が、閉じた公開語彙の reason を必ず 1 つ運ぶ。client 側で理由を creative
  に命名する必要がなくなる。
- clients/config の servers.json は全 client host の契約であり、Go projection は clients/goclient
  という独立 module 1 つだけが持つ。言語ごとに loader は 1 つで、2 つ目は gate が落とす。
- 3 言語の loader の一致が README の主張ではなく expected-outcome corpus で担保される。
- admission が per-entry になり、不正な 1 entry が fleet 全体を落とさなくなる。server display name は一意かつ
  CLI で扱える文字種に限定される。
scope:
- group A — services/host/state/ と services/server/api/gateway.go の terminal state 保持と
  lifecycle publication
- group B — protocol/ の transport-close 語彙 schema と services/server/api/ の close / 拒否経路
- group C — clients/config 契約、新規 clients/goclient module、既存 3 loader の per-entry admission
  と server naming rule、expected-outcome corpus と parity script
non_goals:
- CLI binary の build、import、設定。CLI 側の observable behaviour は change-20260812-cli-terminal-client
  が持つ
- clients/ui・workspace・windows-shell が published close reason を採用すること（各 client 自身の change）
- clients/config の design ownership 確定（open-config-contract-design-ownership として未決）
- runtime config reload、credential の UI 投影
change_classes:
- behavior
- boundary
- invariant
- capability
governance:
  gate: auto
  reasons:
  - 既存 client の観察を壊さない追加が中心であり、per-entry admission への変更も whole-file 失敗より厳しくない方向にしか動かない。3
    言語の一致は corpus で機械的に担保される。
members:
- role: requirements
  path: changes/change-20260814-cli-prerequisite-contracts/requirements.md
  required: true
- role: implementation
  path: changes/change-20260814-cli-prerequisite-contracts/implementation.md
  required: true
- role: verification
  path: changes/change-20260814-cli-prerequisite-contracts/verification.md
  required: true
promotion: []
unresolved_decisions:
- 'open-config-contract-design-ownership: clients/config の design ownership。現在 design-windows-shell
  が RESP-005 / INV-009 として持つが、本 change はその契約を全 client host の canonical な契約にしたため、名前と範囲の乖離が広がった。design-clients-config
  を切り出すかは候補であって決定ではない。実装は影響を受けないが、implementation.md の promotion manifest はこの決着を待って
  closure 時に適用する。'
tags:
- execution-plane
- client-config
- transport
- go-module
owners: []
relations:
- {type: conformsTo, target: design-client}
- {type: references, target: design-windows-shell}
- {type: references, target: change-20260727-web-host-client-boundary}
source_paths:
- services/host/state
- services/server/api
- protocol
- clients/config
- clients/goclient
- clients/web-host
- clients/workspace
- clients/windows-shell
- scripts
summary: 終了 session の terminal state 保持、transport close の型付き reason、client config
  契約の全 client 共通化と server display name の per-entry admission — terminal CLI が依存する
  3 つの前提 contract change
updated: '2026-08-14'
---

## Summary

## Closure Notes


{% transition from="draft" to="ready" date="2026-08-14" %}
members derived from the shared accepted design plan (change-20260812-cli-terminal-client/design-plan); 4 ADR accepted
{% /transition %}


{% transition from="ready" to="active" date="2026-08-14" %}
main session implementation
{% /transition %}
