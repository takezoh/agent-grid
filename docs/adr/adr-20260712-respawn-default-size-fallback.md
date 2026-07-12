---
id: adr-20260712-respawn-default-size-fallback
kind: adr
title: Keep RespawnFrame at 80x24 default and document explicitly
status: proposed
created: '2026-07-12'
decision_makers:
- Takehito Gondo
tags:
- runtime
- respawn
- simplification
owners: []
relations:
- {type: partOf, target: plan-20260712-frame-size-ownership}
- {type: references, target: spec-20260712-frame-size-ownership}
source_paths:
- src/client/runtime/pty_backend.go
summary: 'RespawnFrame は production caller 0 のため hint 保存機構を増やさず default 80x24 fallback
  を仕様として明示する。

  Consequences 三極は本文タグに記載 (docs schema が spec-detail v1 frontmatter フィールド未サポートのため本文のみ)。

  '
---

## Context

{% context %}
`PtyBackend.RespawnFrame` (`pty_backend.go:91-111`) は現状 termvt.Spec に cols/rows を渡さないため 80×24 で再 spawn する。issue の grep 調査で production caller が 0 件であることが確認されている (spawn hint 未配線と同じ調査で得た事実)。ここに直近 hint 保存 (frameID → (cols,rows) の内部 map) や signature 拡張 (cols/rows uint16 を追加) を導入する誘惑があるが、それは (a) 実際の消費者ゼロの状態で状態管理を増やす (b) multi-viewer last-writer-wins との整合 (直近 spawn 時 hint と直近 resize hint のどちらを再利用するか) を再考する追加設計が発生する、という二重の speculative_generality を生む。本 issue のスコープは 歪み 1 (spawn hint 未配線) + 歪み 2 (FrameSize dead API) の 2 点で、cold-start restore 経路は out-of-scope として critique が確定している。RespawnFrame の挙動を本 issue で「拡張しない」と明示的に決めておかないと、実装者が spawn 経路の変更に流されて RespawnFrame にも size 経路を通してしまう risk がある。
{% /context %}

## Decision

{% decision %}
RespawnFrame の実装挙動は変更しない。現状の「size なしで termvt.Spec を作り normalizeSize 経由で 80×24 に fallback する」挙動を仕様として明示する doc comment を `RespawnFrame` の宣言直上に追加する。「production caller 0 のため hint 保存機構は追加せず、将来 respawn 消費者が発生した時点で本 ADR を supersede する専用 ADR を起こす」の 2 文を含める。signature 変更 / 内部 map 導入は行わない。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
本 issue の変更範囲が「spawn 経路 + boundary validation + FrameSize 削除 + doc comment」の 4 点に閉じ、状態管理を増やさない。design-quality §12 speculative_generality 排除の要求 trace 攻撃を pass する。
{% /consequence %}

{% consequence kind="positive" %}
RespawnFrame の default 80×24 fallback が implementation detail から仕様上の invariant に格上げされ、将来「なぜ respawn で 80×24 に戻るのか」を code 読解でしか知れなかった曖昧さが解消される (Failure Modes 節の respawn-size-loss と 1:1 対応)。
{% /consequence %}

{% consequence kind="negative" %}
将来 RespawnFrame が production 経路に載る (例: 端末 crash 後の自動 respawn を実装する) 場合、その時点で size hint 継承機構を新設する追加作業が発生する。ただし現時点で仕様が固定されるため、その追加作業の開始時に「今の default を維持する / 直近 spawn を再利用する / 直近 resize を再利用する」の 3 択が明示的に立てられる。
{% /consequence %}

{% consequence kind="negative" %}
Failure Modes 節の respawn-size-loss は recovery=escalate (仕様として documented limitation) に分類されるため、将来ユーザーが「respawn 後に画面が小さくなった」と報告した場合、この doc comment / Failure Modes 節を参照する必要があり、運用側での認知コストが 1 段発生する。
{% /consequence %}

{% consequence kind="neutral" %}
現状の 80×24 挙動は変わらないため、実装への影響は doc comment 追加のみ。RespawnFrame 呼び出し 0 件のため regression 影響も 0。
{% /consequence %}

## Alternatives

- **B. PtyBackend 内で直近 spawn の cols/rows を frameID → (cols,rows) の内部 map で保持し、RespawnFrame で再利用** — 却下。production caller 0 の経路のために PtyBackend に状態を追加するのは §12 speculative_generality の cut-point 不在攻撃で fail する (同 map を使う caller が 0 件)。加えて map と直近 resize (termvt.Session が持つ) の 2 SoT が生まれ SoT invariant に反する。
- **C. RespawnFrame signature に cols/rows を追加し呼び出し側 (現状 production 0 件) が渡す** — 却下。API surface の破壊的変更を「0 caller のため作業量は小」を根拠に行うのは、interface 変更の立証責任 (§12 立証責任攻撃) を満たさない。将来 respawn が production 経路に載った時点で ADR を supersede すれば同じ変更を要件付きで実施できる。
