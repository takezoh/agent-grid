---
id: adr-20260712-remove-framesize-dead-api
kind: adr
title: Remove FrameInspect.FrameSize dead API
status: proposed
created: '2026-07-12'
decision_makers:
- Takehito Gondo
tags:
- runtime
- simplification
- cleanup
- backend
owners: []
relations:
- {type: partOf, target: plan-20260712-frame-size-ownership}
- {type: references, target: spec-20260712-frame-size-ownership}
source_paths:
- src/client/runtime/backends.go
- src/client/runtime/pty_backend.go
- src/client/runtime/pty_backend_test.go
summary: 'production caller 0 の FrameSize API を interface / PtyBackend / fake / noop
  の 4 面で削除、test は termvt.Session.Size() で観測する。

  Consequences 三極は本文タグに記載 (docs schema が spec-detail v1 frontmatter フィールド未サポートのため本文のみ)。

  '
---

## Context

{% context %}
`src/client/runtime/backends.go` の `FrameInspect` interface に `FrameSize(frameID) (cols, rows uint16, ok bool)` が宣言され、`pty_backend.go:176`, `noopBackend`, `fakeBackend`, `blockingBackend` に実装がある。issue の grep 調査で確認されたとおり、この API に対する production caller は 0 件で、唯一の caller は `pty_backend_test.go` の `TestResizeSurface` (resize 後観測のため)。CaptureFrame (`interpret.go:219,403`) は size ではなく画面テキストを返すので同 role ではない。この状態を維持する根拠 (design/PRINCIPLES.md §12 speculative_generality の要件 trace 攻撃) を確認したが、この API を要求する FR / NFR は存在せず、「将来の headless debugger」等の仮想の consumer が唯一の根拠 — YAGNI に反する。dead API は interface の理解負荷 (「FrameInspect って何を inspect する interface?」) と抽象の drift 源 (実装 4 面と interface 1 面の同期コスト) を継続的に発生させる。
{% /context %}

## Decision

{% decision %}
FrameInspect.FrameSize を interface method から削除し、4 実装 (PtyBackend, noopBackend, fakeBackend, blockingBackend) 上の FrameSize メソッドを併せて削除する。テストで唯一使っている `pty_backend_test.go TestResizeSurface` は観測経路を `termvt.Session.Size()` 直接 (Manager 経由の access が必要なら私設 accessor を pty_backend_test.go 内に置く) に切り替える。`FrameInspect` interface が FrameSize しか持たない場合は interface 自体も削除し、caller (現状 0 件) の import を刈る。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
呼び出し 0 の query surface が消え、interface の意味論 (FrameInspect が何を提供するのか) が明確になる。抽象と実装が 5 面 (interface + 4 実装) 同期する drift コストが恒久的にゼロになる。
{% /consequence %}

{% consequence kind="positive" %}
`pty_backend_test.go TestResizeSurface` の観測経路が termvt.Session.Size() 直接になることで、resize の effect が backend 経由の「間接観測」から termvt の「一次観測」に短縮され、test の因果関係が明確になる。
{% /consequence %}

{% consequence kind="negative" %}
将来 orchestrator や headless debugger が frame の cols/rows を production 経路で読みたくなった場合、この API を再導入する追加作業が発生する (interface method 追加 + 4 実装 + role wiring)。ただしその時点でこそ本当の consumer が存在するので、要件 trace は成立する — YAGNI 排除の常道。
{% /consequence %}

{% consequence kind="negative" %}
FrameSize 削除に伴う test 経路変更で、pty_backend_test.go の import surface が Manager / termvt.Session のより深い部分に依存する可能性がある (Manager accessor を追加する場合は accessor 自体が新規 API surface)。実装フェーズで最短経路 (termvt.Session.Size() 直接) が取れることを verify する。
{% /consequence %}

{% consequence kind="neutral" %}
production 挙動には影響しない (dead code の削除)。wire-format / persist schema / driver interface のいずれにも波及しない。
{% /consequence %}

## Alternatives

- **保持 + 呼び出し 0 件でも invariant contract test を書く** — 却下。「使われていない API を pin するテスト」を書くことは speculative_generality の教条化で、design/PRINCIPLES.md §12 の要件 trace 攻撃で fail する (この API を要求する FR が無い)。
- **FrameInspect interface から外し PtyBackend の非公開 method に降ろす** — 却下。「呼び出し 0 の method が private で残る」だけで実質的な状況は変わらず、interface と実体の乖離を追加で生む。将来 consumer が発生したら (a) 再 export or (b) 再度 interface 化 が必要で、いずれにせよ削除案と同じ作業。
