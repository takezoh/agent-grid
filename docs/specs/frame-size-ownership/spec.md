---
id: spec-20260712-frame-size-ownership
kind: spec
title: 'Frame size ownership: spawn size hint wiring + dead FrameSize API cleanup'
status: draft
created: '2026-07-12'
tags:
- runtime
- termvt
- server
- validation
owners: []
functional_requirements:
- id: FR-001
  statement: When a session-create request supplies Cols and Rows hint values that
    are both in the inclusive range 1..maxDim (2000), the system shall initialize
    the pty winsize and the VT emulator grid to those exact values on spawn.
  priority: must
  rationale: '歪み 1 (spawn size hint 未配線) の解消。issue の TL;DR §修正スコープ A に対応。

    happy path invariant として browser 初回 fit の resize dance を消す。

    '
- id: FR-002
  statement: The system shall treat 80x24 as the explicit contractual default size
    (not a silent fallback) whenever a session-create request omits size hints (both
    Cols and Rows are 0).
  priority: must
  rationale: 'critique graft #1 (default fallback を暗黙 fallback ではなく明示された既定値として仕様化)。

    debug 時に「hint 未送信 vs hint 無視」を区別する契約。

    '
- id: FR-003
  statement: If a session-create HTTP request supplies Cols or Rows outside the valid
    range (negative, above maxDim=2000, or asymmetric where only one dimension is
    non-zero), then the system shall reject the request with HTTP 400 body '{"code":"invalid_cols_rows","message":"cols
    must be 0 or 1..2000; got <n>"}' before any int-to-uint16 conversion.
  priority: must
  rationale: 'critique graft #2 (400 reject body 具体形)。

    silent wrap-around を仕組みで防ぐ boundary hardening。

    '
- id: FR-004
  statement: If a WebSocket resize frame ({k:"r"}) supplies Cols or Rows outside the
    valid range (non-positive or above maxDim=2000), then the system shall drop the
    resize and emit a structured warn log (fields cols, rows, reason) before any int-to-uint16
    conversion.
  priority: must
  rationale: 'critique graft #2 の WS 側 asymmetric branch (400 vs silent drop 峻別)。

    既存の非正値 drop 慣行と対称に維持しつつ maxDim 超過を observable にする。

    '
- id: FR-005
  statement: While a size hint value is being carried across the web boundary from
    int to uint16, the system shall have already applied maxDim validation so uint16
    wrap-around cannot bypass termvt clamping.
  priority: must
  rationale: 'critique pass2 improvement #2 の一般化 (narrow conversion 前検証を invariant
    化)。

    HTTP と WS の 3 箇所すべてで narrow 前に validate される契約。

    '
- id: FR-006
  statement: The system shall preserve LaunchOptions.Cols/Rows verbatim from the web
    API through the state layer (LaunchPlan.Options / CreateLaunch.Options / EffSpawnFrame.Options)
    to the runtime PtyBackend without any driver being required to read or forward
    the values.
  priority: must
  rationale: 'critique pass2 improvement #3 (DP-d1 C 削除で FR-d4 と両立)。

    size の SoT を termvt に閉じ、driver に size 干渉権を与えない SoT invariant。

    '
- id: FR-007
  statement: The system shall not expose FrameInspect.FrameSize as a public interface
    method, and shall provide equivalent test observations only via termvt.Session.Size()
    or an internal Manager accessor.
  priority: must
  rationale: 'critique pass2 improvement #4 (FR-d6 の EARS 単一検証対象化)。

    歪み 2 (FrameSize dead API) の削除 invariant を単一 shall で pin する。

    '
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: 'narrow conversion (int -> uint16) の wrap-around が発生する経路が存在しない: 全ての境界
    (HTTP mux.go apiCreateReq / WS gateway.go の {k:"r"} 2 箇所) で validation が narrow
    conversion より statically 前段に位置する。'
  measurement: T0 test で 65536 / -1 / >maxDim 入力に対して validation 関数が invalid を返すこと、および
    static analysis / grep で narrow 済み uint16 上での validation 呼び出しが存在しないこと。
- id: NFR-002
  type: maintainability
  criteria: size hint 反映は fakePTY / fakeEmulator を用いた T1 (wired) test で観測可能。新たな test
    infra は追加しない。
  measurement: pty_backend_test.go に spawn 時 hint 反映テストが追加され、fakeEmulator.Size() /
    fakePTY.Winsize が指定値を返す。
- id: NFR-003
  type: reliability
  criteria: 境界検査の reject / drop は必ず observable (HTTP 400 body / WS warn log の一方)。silent
    分岐を残さない。
  measurement: HTTP e2e test で 400 の body が仕様の JSON 形式を含み、WS test で warn log 出力が観測される。
- id: NFR-004
  type: compatibility
  criteria: cols/rows omitted の既存 client (curl 手動 create / browser の後方互換経路) は挙動が変わらない
    (80x24 spawn → 初回 resize dance を維持)。
  measurement: FR-002 の default fallback パスに対する回帰 test。
- id: NFR-005
  type: maintainability
  criteria: state 層 reducer (reduce_surface.go) は size を保持せず、size は effect (EffSpawnFrame.Options)
    と backend (termvt.Spec) の pass-through のみ。
  measurement: 'reduce_surface.go の diff が pass-through 契約 (adr-20260712-launch-size-pass-through
    の Decision) を破らない: grep で reducer 内に size field 保持がないこと。'
- id: NFR-006
  type: maintainability
  criteria: FrameBackend.SpawnFrame の signature 拡張は既存 fake / noop / blocking 3 実装への
    signature 追随のみに閉じる (driver 層および conformance test は影響ゼロ)。
  measurement: diff scope が backends.go / pty_backend.go / noop_backend / fake_backend
    に閉じ、driver package および conformance test に変更なし。
acceptance:
- id: AC-001
  given: browser が POST /api/sessions body に cols=203, rows=47 を含めて送る
  when: session-create request が処理され spawn 完了する
  then: termvt.Session の内部 size が 203x47 で初期化されており、pty.Winsize と emulator grid の両方が
    203x47 を保持している
  requirement_refs:
  - FR-001
  - FR-006
- id: AC-002
  given: browser が POST /api/sessions body に cols/rows を含めない (0 相当)
  when: session-create request が処理され spawn 完了する
  then: termvt.Session の内部 size が 80x24 で初期化されており、spawn ログには source="default" が明示される
  requirement_refs:
  - FR-002
- id: AC-003
  given: HTTP client が POST /api/sessions body に cols=99999 を含めて送る
  when: mux.go apiCreateReq が decode したのち validation を実行する
  then: gateway は 400 応答 body '{"code":"invalid_cols_rows","message":"cols must be
    0 or 1..2000; got 99999"}' を返し、daemon には CreateSession コマンドが送られない
  requirement_refs:
  - FR-003
  - FR-005
- id: AC-004
  given: HTTP client が POST /api/sessions body に cols=120, rows=0 (asymmetric) を含めて送る
  when: mux.go apiCreateReq が decode したのち validation を実行する
  then: gateway は 400 応答を返し、asymmetric hint が daemon に伝播しない
  requirement_refs:
  - FR-003
- id: AC-005
  given: browser が WebSocket resize frame '{"k":"r","cols":99999,"rows":47}' を送る
  when: gateway.go applyInboundProto が受信し validation を実行する
  then: resize は drop され、warn log にフィールド (session_id, cols=99999, rows=47, reason="cols
    out of range")が記録され、termvt.Session.Resize は呼ばれない
  requirement_refs:
  - FR-004
  - FR-005
- id: AC-006
  given: FrameInspect.FrameSize が削除されている
  when: repo 全体を grep する
  then: FrameSize method 宣言 / 呼び出しが production / interface 経路に存在せず、pty_backend_test.go
    TestResizeSurface は termvt.Session.Size() 直接 (もしくは Manager accessor) 経由で観測している
  requirement_refs:
  - FR-007
relations:
- {type: implementedBy, target: plan-20260712-frame-size-ownership}
- {type: referencedBy, target: adr-20260712-launch-size-pass-through}
- {type: referencedBy, target: adr-20260712-size-hint-boundary-validation}
- {type: referencedBy, target: adr-20260712-remove-framesize-dead-api}
- {type: referencedBy, target: adr-20260712-respawn-default-size-fallback}
- {type: referencedBy, target: adr-20260712-spawnframe-inline-size-pair}
source_paths:
- src/server/web/mux.go
- src/server/web/gateway.go
- src/server/web/wire.go
- src/client/state/driver_iface.go
- src/client/state/effect.go
- src/client/runtime/interpret_spawn.go
- src/client/runtime/pty_backend.go
- src/client/runtime/backends.go
- src/platform/termvt/session.go
summary: spawn 時 size hint を web API → LaunchOptions → EffSpawnFrame → PtyBackend
  → termvt.Spec の 1 直線で運び、HTTP/WS 境界で narrow 前 validation を敷き、production caller 0
  の FrameInspect.FrameSize を削除する
---

## Overview

本 spec は 2 つの構造的歪みを解消する。**歪み 1**: `LaunchOptions.Cols/Rows` は API から state 層まで運ばれているが、`PtyBackend.SpawnFrame` の signature が size を受けないため `termvt.Spec` 生成箇所 (`pty_backend.go:61`) で落ちる。結果として全 frame が 80×24 で spawn し、browser 初回 fit で即 resize される dance が発生する。**歪み 2**: `FrameInspect.FrameSize` API に production caller が 0 件で、interface / 4 実装 / test 用途のみが残っている。同時に、int→uint16 narrow conversion の pre-validation を HTTP と WS の 3 境界で敷き、silent wrap-around を仕組みで防ぐ。size の SoT (per-pty state) は termvt に閉じ、driver / reducer / state 層は size 運搬役以上の責務を持たない (詳細は adr-20260712-launch-size-pass-through)。

スコープは歪み 1 + 歪み 2 の 2 点。cold-start restore path / RespawnFrame の hint 継承機構 / multi-viewer size reconcile policy 変更 / tap 1×1 emulator 撤去 / VT bounds bug 対応 は out-of-scope。

## Requirements

### Functional

{% req id="FR-001" %}
When a session-create request supplies Cols and Rows hint values that are both in the inclusive range 1..maxDim (2000), the system shall initialize the pty winsize and the VT emulator grid to those exact values on spawn.
{% /req %}

{% req id="FR-002" %}
The system shall treat 80x24 as the explicit contractual default size (not a silent fallback) whenever a session-create request omits size hints (both Cols and Rows are 0).
{% /req %}

{% req id="FR-003" %}
If a session-create HTTP request supplies Cols or Rows outside the valid range (negative, above maxDim=2000, or asymmetric where only one dimension is non-zero), then the system shall reject the request with HTTP 400 body `{"code":"invalid_cols_rows","message":"cols must be 0 or 1..2000; got <n>"}` before any int-to-uint16 conversion.
{% /req %}

{% req id="FR-004" %}
If a WebSocket resize frame (`{k:"r"}`) supplies Cols or Rows outside the valid range (non-positive or above maxDim=2000), then the system shall drop the resize and emit a structured warn log (fields cols, rows, reason) before any int-to-uint16 conversion.
{% /req %}

{% req id="FR-005" %}
While a size hint value is being carried across the web boundary from int to uint16, the system shall have already applied maxDim validation so uint16 wrap-around cannot bypass termvt clamping.
{% /req %}

{% req id="FR-006" %}
The system shall preserve LaunchOptions.Cols/Rows verbatim from the web API through the state layer (LaunchPlan.Options / CreateLaunch.Options / EffSpawnFrame.Options) to the runtime PtyBackend without any driver being required to read or forward the values.
{% /req %}

{% req id="FR-007" %}
The system shall not expose FrameInspect.FrameSize as a public interface method, and shall provide equivalent test observations only via termvt.Session.Size() or an internal Manager accessor.
{% /req %}

### Non-Functional

- **NFR-001 (reliability)**: narrow conversion (int→uint16) の wrap-around が発生する経路が存在しない。全ての境界 (HTTP mux.go / WS gateway.go 2 箇所) で validation が narrow より前段に位置する。
- **NFR-002 (maintainability)**: size hint 反映は fakePTY / fakeEmulator を用いた T1 wired test で観測可能。新規 test infra は追加しない。
- **NFR-003 (reliability)**: 境界検査の reject / drop は必ず observable (HTTP 400 body / WS warn log の一方)。silent 分岐なし。
- **NFR-004 (compatibility)**: cols/rows omitted の既存 client は挙動不変 (80×24 spawn → 初回 resize dance を維持)。
- **NFR-005 (maintainability)**: state 層 reducer は size を保持しない。size は effect / backend の pass-through のみ。
- **NFR-006 (maintainability)**: FrameBackend.SpawnFrame の signature 拡張は既存 fake / noop / blocking 3 実装への追随のみに閉じる。

## Acceptance Criteria

{% acceptance id="AC-001" %}
- **Given**: browser が POST /api/sessions body に cols=203, rows=47 を含めて送る
- **When**: session-create request が処理され spawn 完了する
- **Then**: termvt.Session の内部 size が 203x47 で初期化されており、pty.Winsize と emulator grid の両方が 203x47 を保持している
- **Refs**: FR-001, FR-006
{% /acceptance %}

{% acceptance id="AC-002" %}
- **Given**: browser が POST /api/sessions body に cols/rows を含めない (0 相当)
- **When**: session-create request が処理され spawn 完了する
- **Then**: termvt.Session の内部 size が 80x24 で初期化されており、spawn ログには source="default" が明示される
- **Refs**: FR-002
{% /acceptance %}

{% acceptance id="AC-003" %}
- **Given**: HTTP client が POST /api/sessions body に cols=99999 を含めて送る
- **When**: mux.go apiCreateReq が decode したのち validation を実行する
- **Then**: gateway は 400 応答 body `{"code":"invalid_cols_rows","message":"cols must be 0 or 1..2000; got 99999"}` を返し、daemon には CreateSession コマンドが送られない
- **Refs**: FR-003, FR-005
{% /acceptance %}

{% acceptance id="AC-004" %}
- **Given**: HTTP client が POST /api/sessions body に cols=120, rows=0 (asymmetric) を含めて送る
- **When**: mux.go apiCreateReq が decode したのち validation を実行する
- **Then**: gateway は 400 応答を返し、asymmetric hint が daemon に伝播しない
- **Refs**: FR-003
{% /acceptance %}

{% acceptance id="AC-005" %}
- **Given**: browser が WebSocket resize frame `{"k":"r","cols":99999,"rows":47}` を送る
- **When**: gateway.go applyInboundProto が受信し validation を実行する
- **Then**: resize は drop され、warn log にフィールド (session_id, cols=99999, rows=47, reason="cols out of range") が記録され、termvt.Session.Resize は呼ばれない
- **Refs**: FR-004, FR-005
{% /acceptance %}

{% acceptance id="AC-006" %}
- **Given**: FrameInspect.FrameSize が削除されている
- **When**: repo 全体を grep する
- **Then**: FrameSize method 宣言 / 呼び出しが production / interface 経路に存在せず、pty_backend_test.go TestResizeSurface は termvt.Session.Size() 直接 (もしくは Manager accessor) 経由で観測している
- **Refs**: FR-007
{% /acceptance %}

## Non-Goals

本 spec は size ownership の整理として 2 つの限定的な歪みを解消するが、以下は明示的にスコープ外とする (design/PRINCIPLES.md §20 の 2 段構造)。

**must_not (絶対にやらない)**:
- **tap 1×1 emulator 撤去 / Emulator interface 分割**: VT issue 第 2 段の別 issue で扱う (混ぜると要件 trace 破綻)
- **上流 forks/x/vt bounds bug 対応**: 上流 fork 側で修正済み。forks/ に触らない
- **multi-viewer size reconcile ポリシーの変更**: current の last-writer-wins を維持する (per-viewer ownership / max-size 集約は導入しない)
- **state 層 reducer が size を保持するモデルへの変更**: reduce_surface.go の pass-through 契約は不動
- **driver interface (LaunchPreparer) に Cols/Rows 責務を追加**: adr-20260712-launch-size-pass-through Decision により driver を素通す

**should_not (原則やらない)**:
- **cold-start restore で size を復元する機構の追加**: bootstrap_coldstart.go spawnWrapped の現行契約 (browser 初回 fit で resize) を維持する。例外 (実装フェーズで round-trip の regression が発覚 等) が生じた場合は別 ADR で判断
- **RespawnFrame への size hint 継承経路の追加**: production caller 0 のため adr-20260712-respawn-default-size-fallback で doc 化のみに留める
- **session-env / persist スキーマの拡張**: Cols/Rows は既に uint16 で LaunchOptions に乗っている (persist schema 互換維持)

## Failure Modes

3 者 SoT (unwanted FR / Failure Modes / error 三分法 ADR) 表 (design/PRINCIPLES.md §16) — unwanted FR は FR-003 / FR-004、error 三分法の仕分けは各 ADR Consequences に記録。

| class | detection | recovery | operator_action | related_fr |
|---|---|---|---|---|
| `narrow-conv-overflow` | HTTP mux.go / WS gateway.go の boundary validation が 65535 以上 / 負値 / >maxDim を検出 | `fail_fast` (HTTP 400 invalid_cols_rows / WS warn log + drop) | 監視でこの 400 / log を追跡し、送信元 UI / script の bug として修正指示 | FR-003, FR-004 |
| `termvt-spec-reject` | termvt.NewSession 内で Spec.Cols/Rows が maxDim を超過するケース (validation 通過後は起きないはず) | `fail_fast` (assertion / structured error) — 内部契約違反として default 継続しない (error 三分法 (ii)) | 発生した場合は validation の抜けを意味するため、境界 helper の pre-check を見直す | FR-005 |
| `respawn-size-loss` | RespawnFrame が呼ばれた場合、hint 継承機構がないため 80×24 で再 spawn される | `escalate` (documented limitation) — adr-20260712-respawn-default-size-fallback で仕様固定 | 「respawn 後に画面が小さくなった」報告が発生した場合、ADR を supersede する形で hint 継承機構を新設する検討を開始 | (n/a, out-of-scope from unwanted FR) |

## Open Questions

- **RespawnFrame が production 経路に載る時期**: 現状 caller 0 のため本 spec では default 80×24 fallback を仕様固定するが、将来端末 crash 自動 respawn 等の要件が浮上した場合、adr-20260712-respawn-default-size-fallback を supersede する新 ADR で hint 継承 policy を決める必要がある。決定は本 issue 外。
