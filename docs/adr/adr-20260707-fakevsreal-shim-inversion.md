---
id: adr-20260707-fakevsreal-shim-inversion
kind: adr
title: FakeVsReal は fakecodex 側の real-cli e2e に shim 経由の driving 反転 subtest を追加する
status: accepted
created: '2026-07-07'
tags:
- adr
- codex
- shim
- e2e
owners: []
decision_makers:
- unknown
relations:
- {type: partOf, target: plan-20260707-codexclient-jsonrpc-id-opaque}
- {type: references, target: spec-20260707-codexclient-jsonrpc-id-opaque}
- {type: references, target: adr-20260624-0002-optin-appserver-e2e-validates-fakes}
- {type: references, target: adr-20260704-cli-fake-validated-by-real-cli-e2e}
source_paths:
- src/platform/agent/fakecodex/codex_real_cli_e2e_test.go
- src/cmd/bridge/codex_app_server_shim.go
summary: 既存 `//go:build e2e` + AG_E2E_CODEX_BIN gate に相乗りし、fakecodex real-cli e2e
  に shim を挟んで real codex-cli を driving する反転方向 subtest を追加する。adr-0002 の supersede ではなく
  extends
updated: '2026-07-07'
---

# FakeVsReal は fakecodex 側の real-cli e2e に shim 経由の driving 反転 subtest を追加する

## Context

{% context %}
`spec-20260707-codexclient-jsonrpc-id-opaque` は「real codex-cli 0.142.5 が shim を app-server 役として driving し、shim が upstream の real codex-app-server と橋渡しする」bug 方向で string id が silent drop されていた事案である (前段 `/debug` で MITM 復号確認済み)。この方向を pin する FakeVsReal が必要 (spec `AC-005`)。

既存の real-binary e2e harness は 2 系統ある:

1. `src/client/runtime/subsystem/stream/routing_e2e_test.go` — `//go:build e2e` + `AG_E2E_CODEX_BIN` / `AG_E2E_APPSERVER_BIN` gate。agent-grid が client、real binary が app-server の方向で routing isolation を pin (`adr-20260624-0002` の対象)
2. `src/platform/agent/fakecodex/codex_real_cli_e2e_test.go` — `//go:build e2e` + `AG_E2E_CODEX_BIN` gate。agent-grid の client 層 (fake app-server + real codex-cli の子プロセス) の cli fake fidelity を pin (`adr-20260704-cli-fake-validated-by-real-cli-e2e` の対象)

否定役指摘 [assumption_gap (target: assumptions[5] / DP-d7 / DP-d8)] にあるように、これらの harness はいずれも「agent-grid client 側の routing / fake fidelity」を対象としており、**shim を app-server 役として real codex-cli に driving される反転方向** はカバー範囲外だった。

DP-d7 選択肢の「既存 `appserver_e2e` に相乗り」は tag 名が誤り (実測は `//go:build e2e`)。DP-d8 の「新規 ADR を立てる」ことは既存 ADR との scope 差 (どちらが real 起動対象か) を明示するために必要。
{% /context %}

## Decision

{% decision %}
`src/platform/agent/fakecodex/codex_real_cli_e2e_test.go` (既存 `//go:build e2e` + `AG_E2E_CODEX_BIN` gate) に **shim を挟んで real codex-cli 0.142.5 を driving する subtest を追加**する。

- **build tag**: 既存 `//go:build e2e` に相乗り (新規タグを切らない — 否定役指摘 [adr_conflict, DP-d7 の option A の tag 名誤りを訂正])
- **env gate**: 既存 `AG_E2E_CODEX_BIN` に相乗り (semantics は「real codex-cli バイナリの絶対パス」で共通)
- **harness 構造**: agent-grid の shim を app-server 役として起動 (`codex_app_server_shim.go` の `codexShimServer`)、shim の upstream に fakecodex.Server (in-process) を接続、shim の downstream に real codex-cli を子プロセスとして起動して driving する。CLI が `{"id":"initialize","method":"initialize",...}` を string id で送出、fake upstream が initialize reply を返し、reply の id が入力と bytes-preserving に一致することを assert
- **placement**: 反転方向 harness の setup helper は `fakecodex/` に置く (既存 real-cli e2e の spawn / initialize プリミティブを再利用できる)。真に置き場所を分けたい場合のみ後続 PR で `platform/agent/shime2e/` 等に移す (現在は最小差分優先)

**adr-20260624-0002 との関係**: `supersede` ではなく **extends / complements**。adr-0002 が定義する「real app-server + agent-grid client」の方向は継続し、本 ADR は「real client (codex-cli) + agent-grid shim (app-server 役)」の反転方向を追加するだけで、`routing_e2e_test.go` の isolation 契約には触れない。`relations[]` は `references` で adr-0002 に後方リンクし、`supersededBy` は張らない。

**adr-20260704-cli-fake-validated-by-real-cli-e2e との関係**: 同じ real-cli を起動する harness を再利用するが、対象は「fakecodex の CLI fake fidelity」ではなく「shim の string id echo」なので、対象 assertion が独立している。同じ file 内に別 subtest として同居させて harness を再利用する。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
- 既存 harness (real codex-cli を子プロセス化して stdio pipe に繋ぐ setup) を再利用でき、新規 opt-in tag / 新規 env var を増やさない。CI マトリクス増大を避けられる
- 反転方向を明示的に pin することで、本 bug (shim 経由 string id) と同種の envelope 変種が real CLI との組み合わせで再発したときも 1 テストで検出できる
- adr-0002 (routing isolation) と本 ADR (shim id echo) が別 scope として整理され、後段レビュアーが adr-0002 を supersede と誤認する余地がない
{% /consequence %}

{% consequence kind="negative" %}
- fakecodex パッケージが 「client 側 fake fidelity」と「shim 側 id echo」の 2 責務を同居させることになる。将来 `platform/agent/shime2e/` に切り出す動機が生まれた時点で file 移動を検討する (本 PR では最小差分を優先)
- opt-in であるため CI 定期実行では走らない。手動 or nightly の別枠 CI ジョブが必要 (これも adr-0002 と同じ posture であり本 ADR で新設しない)
{% /consequence %}

{% consequence kind="neutral" %}
- adr-0002 / adr-20260704 の gate 設計 (build tag `e2e` + env presence) は本 ADR の subtest でも継承される。設計変更なし
- FakeVsReal が失敗したら fake を修正する (AGENTS.md 規約) 原則は本 subtest でも適用される
{% /consequence %}

## Alternatives

- **新規 build tag `codex_appserver_id_e2e` を切る** — 却下 (DP-d7 Option B)。CI マトリクスが増える割に、既存 tag と gate が対等な運用契約 (adr-0002) を確立しているため、scope を tag 分離で表す価値が薄い。テスト subtest 名で scope を分ければ十分。
- **通常テストに含める (codex-cli をローカルに要求)** — 却下 (DP-d7 Option C)。real binary + model access が要求される test を通常 CI に入れると flaky / slow で merge blocker 化する。opt-in real integration の broader posture (adr-0002) に反する。
- **routing_e2e_test.go 側 (stream backend の e2e) に相乗り** — 却下。routing_e2e_test.go は「agent-grid が client として real app-server を driving する」方向であり、shim が app-server 役として real client (codex-cli) に driving される反転方向を混ぜると、harness の setup path が交差して読解負荷が上がる。fakecodex 側に置いた方が pipe 方向が naturally 対応する。
- **adr-0002 の下に注記して新規 ADR を立てない** — 却下 (DP-d8 Option B)。id 型は codexclient の外部契約なので supersede-independent な判断として個別 ADR で扱うのが SDD 的に妥当。粒度は「独立に supersede される単位」の原則に従う (Principle 2)。


{% transition from="proposed" to="accepted" date="2026-07-07" %}
user G1 承認
{% /transition %}
