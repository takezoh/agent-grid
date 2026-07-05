---
id: adr-20260705-driver-conformance-registry-suite
kind: adr
title: Driver conformance suite enforced via registry iteration
status: accepted
created: '2026-07-05'
decision_makers:
- Takehito Gondo
tags:
- testing
- driver
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: references, target: adr-20260705-metadata-source-priority}
- {type: referencedBy, target: note-20260624-agent-testing}
- {type: referencedBy, target: note-20260624-technical-code-enforcement}
- {type: references, target: note-20260624-agent-testing}
- {type: references, target: note-20260624-technical-code-enforcement}
source_paths:
- src/client/state/driver_iface.go
- src/client/driver/
summary: state.Register 済み全 driver に共通契約 (Step 純粋性・DriverEvent totality・Persist/Restore・metadata
  source priority) を registry 走査テストで自動強制する
updated: '2026-07-05'
---

# Driver conformance suite enforced via registry iteration

## Context

{% context %}
`state.Driver` interface の doc は「`Step` は純粋関数でなければならない (I/O / goroutine / global 禁止)」
と宣言し、各 driver (claude / codex / gemini / shell / generic) は個別に厚いテストを持つ。しかし
「driver として満たすべき共通契約」を検証する仕組みは無く、契約は interface コメントと個別テストの慣習に
分散している。新 driver 追加時にどのテストを書くべきかは暗黙で、gemini のような後発 driver が契約の一部
(たとえば Persist/Restore round-trip) を欠いても機械的には検出されない。

adr-20260705-metadata-source-priority は「authoritative source 確定後は fallback が上書き不可」
「tri-state (unset/set/cleared)」という cross-driver 不変条件を新設したが、その検証は claude / codex の
個別テストに書かれており、契約としての共通化はされていない。
{% /context %}

## Decision

{% decision %}
`client/driver/drivertest` package に `Conformance(t *testing.T, drv state.Driver)` suite を新設し、
**registry 走査テスト** (`state.GetDriver` に登録された全 driver を 1 テストで列挙して suite を適用) で
新 driver の自動加入を強制することにする。suite の契約:

1. **Step 純粋性** — 同一入力での 2 回呼び出しが同一出力を返し、`prev DriverState` を mutate しない
   (呼び出し前後の deep-equal)。
2. **DriverEvent totality** — 全 `DEv*` 種 (10 種) の zero-value 入力で panic しない。
3. **Persist/Restore round-trip** — `NewState → Step 数回 → Persist → Restore` 後の `View` /
   `Status` が一致する。
4. **View/Status totality** — 到達可能な各 state で `View` / `Status` が panic しない。
5. **metadata source priority** — driver ごとに authoritative source (claude: hook / codex:
   `thread/settings/updated`) を差し替えるパラメタライズド契約として、「authoritative 確定後は
   transcript / launch-parse fallback が model / effort を上書きできない」「clear は明示状態として
   復元後も維持される」を検証する。

driver 固有の挙動テストは従来どおり個別テストに残す。suite は「全 driver に共通の最低保証」のみを持つ。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
新 driver は registry 登録した瞬間に共通契約の検証対象になる (加入手作業ゼロ、NFR-001)。interface コメント
でしか表明されていなかった純粋性契約が実行可能な検証になる。
{% /consequence %}

{% consequence kind="negative" %}
契約が厳格化されるため、既存 driver が暗黙に破っている契約 (もしあれば) の修正が suite 導入の前提作業になる。
{% /consequence %}

{% consequence kind="neutral" %}
suite は T0 (pure) tier に属し、fake も I/O も使わない。
{% /consequence %}

## Alternatives

- **interface コメント + レビューによる担保 (現状維持)** — 却下。新 driver / 新 DriverEvent 追加時の
  契約漏れを機械検出できない。
- **各 driver テストに契約テストを複製する** — 却下。契約の変更 (metadata priority のような追加) が
  N driver への手動反映になり drift する。suite の一元化が変更点を 1 箇所に保つ。
- **lint (ruleguard) で Step 内の I/O を静的検出する** — 部分採用の余地はあるが、純粋性の完全な静的証明は
  できず (間接呼び出し)、round-trip / priority のような動的契約は表現できない。suite と排他ではないため、
  将来の強化候補として却下ではなく保留。


{% transition from="proposed" to="accepted" date="2026-07-05" %}
drivertest.Conformance + registry 走査が全 driver で green
{% /transition %}
