---
id: spec-20260712-frame-size-ownership
kind: spec
title: Frame size ownership — spawn hint plumbing and dead API cleanup
status: draft
created: '2026-07-12'
methodology: sdd
tags:
- runtime
- terminal
- pty
- size-ownership
owners:
- take.gn@gmail.com
functional_requirements:
- id: FR-001
  statement: セッションが起動中である間、システムは emulator grid と kernel pty winsize が同じ (cols, rows) 値を保持していなければならない。
  priority: must
  rationale: size の SoT は termvt session actor 内の kernel pty winsize と emulator grid の 2 owner のみ、という既存 architecture 決定 (adr-20260624-0004) の invariant を明示化する。
- id: FR-002
  statement: システムは、frame spawn 時に有効な (cols, rows) を 1 以上 maxDim (2000) 以下の範囲で必ず確定していなければならない。
  priority: must
  rationale: hint 経路のどこかで値が失われても termvt.normalizeSize の 80×24 fallback が最終保証を担う。「常に有効な size」invariant を SoT 化する (§13 (i) 意味論再定義)。
- id: FR-003
  statement: POST /api/sessions が cols>0 かつ rows>0 を含むとき、システムはそれらの値を kernel pty winsize と emulator grid の spawn 初期値に反映しなければならない。
  priority: must
  rationale: 本 issue の主目的 (β scope FR-022 の完成)。 client-supplied hint の消費点を明示する。
- id: FR-004
  statement: もし POST /api/sessions の cols または rows のいずれかが 0 (もしくは欠落) であるならば、システムは cols=80 かつ rows=24 の fallback で spawn しなければならない (片方だけ hint を反映してはならない)。
  priority: must
  rationale: 片方だけ hint を反映すると size 判別性が壊れる。 判別を『両方>0=hint 有り / それ以外=両方 fallback』の 2 状態に閉じる。
- id: FR-005
  statement: RespawnFrame が発生したとき、システムは既存 termvt session の Size() が返す現在の (cols, rows) を新 session の初期値としなければならない。既存 termvt session が不在の場合は cols=80 かつ rows=24 の fallback を用いなければならない。
  priority: must
  rationale: size SoT の一意化 (§SSOT)。 PtyBackend に in-process cache を作らず、kernel/emulator を SoT に保つ。
- id: FR-006
  statement: もし POST /api/sessions の cols または rows が負数、または maxDim (2000) 超過であるならば、システムは int→uint16 変換を行わず HTTP 400 (invalid_dim) で拒否しなければならない。
  priority: must
  rationale: narrow uint16 conversion で 65536 以上が 0 に wrap し fallback を誤発火する構造欠陥 (issue の指摘 5) を境界で除去する。
- id: FR-007
  statement: もし lifecycle WebSocket の resize フレーム ({k:'r'}) の cols または rows が 0 以下、または maxDim (2000) 超過であるならば、システムは既存 winsize/grid を維持し、reqId 付きの error frame ({k:'e', reason:'invalid_dim'}) で拒否しなければならない (silent drop してはならない)。
  priority: must
  rationale: 握り潰し前提 API の禁止 (§13) と観察可能性の保存 (§0)。 client が原因を知れる形で拒否する。
- id: FR-008
  statement: frame spawn 処理が実行中である間、システムは (hint_cols, hint_rows, effective_cols, effective_rows) を必ず slog に出力しなければならない。 hint と effective が乖離した (fallback が発火した) 場合は追加で slog.Warn を出さなければならない。
  priority: must
  rationale: silent fallback を可観測化することで、hint が経路のどこかで失われた regression を prod ログから即断できる (§0 観察可能性の保存)。
- id: FR-009
  statement: もし web 層で cols/rows の値域検証が narrow uint16 conversion の後で行われるならば、システムはその設計を採用してはならない (検証は必ず int 値のまま先に行わなければならない)。
  priority: must
  rationale: shotgun parsing / 変換順序の逆転を禁止する規範 (§13 parse, don't validate)。
- id: FR-010
  statement: backend の FrameInspect interface が FrameSize メソッドを提供しない場合であっても、システムは spawn / resize / respawn の全経路で FR-001〜FR-005 の invariant を維持しなければならない。
  priority: must
  rationale: FrameSize 削除 (production 呼び出しゼロの死 API) が他 invariant を壊さないことの明示宣言。
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: hint 経路の各段は int/uint16 値域安全を境界検証 (T1 mux/gateway テスト) または境界 unit test でカバーし、65536 wrap による誤 fallback が発生しないこと。
  measurement: 境界値テスト (0, 1, maxDim, maxDim+1, 65536) が全て期待通り分岐する。
- id: NFR-002
  type: maintainability
  criteria: size を持つ backend interface (FrameLifecycle.SpawnFrame) は seam として明示され、fake 実装が signature 追随で成立すること。
  measurement: PtyBackend / noopBackend / 全 test fake が新 signature で compile。
- id: NFR-003
  type: usability
  criteria: 新規セッションの初期表示は browser 側 fit.fit() 直後の mid-flight resize を通らず、作成元 device の cols/rows で表示されること。
  measurement: T1 で hint 供与時に spawn 直後の frame.Size() が hint と一致 (fake 観測)。
acceptance:
- id: AC-001
  given: POST /api/sessions を cols=100, rows=40 で叩く
  when: セッションが spawn する
  then: termvt.Session.Size() が (100, 40) を返し、slog に hint=(100,40) effective=(100,40) が出る
  requirement_refs:
  - FR-001
  - FR-003
  - FR-008
- id: AC-002
  given: POST /api/sessions を cols=0, rows=0 (もしくは cols/rows 欠落) で叩く
  when: セッションが spawn する
  then: termvt.Session.Size() が (80, 24) を返し、slog に hint=(0,0) effective=(80,24) と Warn が出る
  requirement_refs:
  - FR-002
  - FR-004
  - FR-008
- id: AC-003
  given: POST /api/sessions を cols=100, rows=0 で叩く
  when: web 層の validator が受け付ける
  then: HTTP 400 invalid_dim で拒否され、spawn は発生しない
  requirement_refs:
  - FR-004
  - FR-006
  - FR-009
- id: AC-004
  given: POST /api/sessions を cols=2001, rows=40 (または cols=65536) で叩く
  when: web 層の validator が受け付ける
  then: HTTP 400 invalid_dim で拒否され、uint16 wrap は発生しない
  requirement_refs:
  - FR-006
  - FR-009
- id: AC-005
  given: spawn 後の RespawnFrame が発火する
  when: 既存 termvt session が Size() = (120, 50) を持つ
  then: 新 session の termvt.Spec.Cols/Rows は (120, 50) となり、spawn 経路とは独立に SoT が保たれる
  requirement_refs:
  - FR-001
  - FR-005
- id: AC-006
  given: lifecycle WS に {k:'r', sessionId:sid, cols:2001, rows:40, reqId:'x'} を送る
  when: gateway.go の validator が受け付ける
  then: 既存 winsize/grid が変わらず、{k:'e', reqId:'x', reason:'invalid_dim'} が返る
  requirement_refs:
  - FR-007
- id: AC-007
  given: 本 spec 完了後の repository
  when: grep -rn 'FrameSize(' src/ を実行する
  then: 0 件 (FrameInspect.FrameSize が削除され、production/テスト 呼び出しが残っていない)
  requirement_refs:
  - FR-010
relations:
- {type: implementedBy, target: plan-20260712-frame-size-ownership}
- {type: referencedBy, target: adr-20260712-size-hint-wiring-completion}
- {type: referencedBy, target: adr-20260712-ws-resize-bound-error-frame}
source_paths:
- src/client/state/driver_iface.go
- src/client/runtime/pty_backend.go
- src/client/runtime/backends.go
- src/client/runtime/interpret_spawn.go
- src/server/web/mux.go
- src/server/web/gateway.go
- src/platform/termvt/session.go
summary: β scope として定義されていた LaunchOptions.Cols/Rows を termvt.Spec まで貫通させて新規セッションの初期表示を作成元 device のサイズで開始し、死 API FrameSize を除去、HTTP/WS 境界で narrow uint16 変換前の値域検証と境界外 resize の可観測な拒否を導入する。
updated: '2026-07-12'
---

## Overview

`state.LaunchOptions.Cols/Rows` は「The runtime bridges these to termvt.Spec on session launch (β scope)」というコメントを伴って定義されているが、リポジトリ内に `.Cols` / `.Rows` を読むコードが存在しない。 このため全 frame は 80×24 で spawn され、browser 初回の `fit.fit()` が発火する `CmdSurfaceResize` によって即座に resize が走る。 併せて `FrameInspect.FrameSize` は production 呼び出しがゼロで、テスト依存だけが残った死 API になっている。

本 spec は 3 つの構造的な穴を同時に埋める: (a) `LaunchOptions.Cols/Rows` を `backend.SpawnFrame` の signature 拡張を経由して `termvt.Spec` まで貫通させ、hint 有無・fallback の判別性を仕様レベルで固定する。 (b) `int→uint16` 変換の前に web 層で値域検証を行い、`65536` wrap による誤 fallback と境界越え要求の silent drop を排除する。 (c) 死 API `FrameSize` を削除する。 `size` の SoT は既存の設計どおり **kernel pty winsize + emulator grid** の 2 owner のみに保ち、PtyBackend / state 層に in-process cache を作らない。

## Requirements

{% req id="FR-001" %}
セッションが起動中である間、システムは emulator grid と kernel pty winsize が同じ (cols, rows) 値を保持していなければならない。
{% /req %}

{% req id="FR-002" %}
システムは、frame spawn 時に有効な (cols, rows) を 1 以上 maxDim (2000) 以下の範囲で必ず確定していなければならない。
{% /req %}

{% req id="FR-003" %}
POST /api/sessions が cols>0 かつ rows>0 を含むとき、システムはそれらの値を kernel pty winsize と emulator grid の spawn 初期値に反映しなければならない。
{% /req %}

{% req id="FR-004" %}
もし POST /api/sessions の cols または rows のいずれかが 0 (もしくは欠落) であるならば、システムは cols=80 かつ rows=24 の fallback で spawn しなければならない (片方だけ hint を反映してはならない)。
{% /req %}

{% req id="FR-005" %}
RespawnFrame が発生したとき、システムは既存 termvt session の Size() が返す現在の (cols, rows) を新 session の初期値としなければならない。 既存 termvt session が不在の場合は cols=80 かつ rows=24 の fallback を用いなければならない。
{% /req %}

{% req id="FR-006" %}
もし POST /api/sessions の cols または rows が負数、または maxDim (2000) 超過であるならば、システムは int→uint16 変換を行わず HTTP 400 (invalid_dim) で拒否しなければならない。
{% /req %}

{% req id="FR-007" %}
もし lifecycle WebSocket の resize フレーム ({k:'r'}) の cols または rows が 0 以下、または maxDim (2000) 超過であるならば、システムは既存 winsize/grid を維持し、reqId 付きの error frame ({k:'e', reason:'invalid_dim'}) で拒否しなければならない (silent drop してはならない)。
{% /req %}

{% req id="FR-008" %}
frame spawn 処理が実行中である間、システムは (hint_cols, hint_rows, effective_cols, effective_rows) を必ず slog に出力しなければならない。 hint と effective が乖離した (fallback が発火した) 場合は追加で slog.Warn を出さなければならない。
{% /req %}

{% req id="FR-009" %}
もし web 層で cols/rows の値域検証が narrow uint16 conversion の後で行われるならば、システムはその設計を採用してはならない (検証は必ず int 値のまま先に行わなければならない)。
{% /req %}

{% req id="FR-010" %}
backend の FrameInspect interface が FrameSize メソッドを提供しない場合であっても、システムは spawn / resize / respawn の全経路で FR-001〜FR-005 の invariant を維持しなければならない。
{% /req %}

## Acceptance Criteria

{% acceptance id="AC-001" %}
**Given** POST /api/sessions を cols=100, rows=40 で叩く
**When** セッションが spawn する
**Then** termvt.Session.Size() が (100, 40) を返し、slog に hint=(100,40) effective=(100,40) が出る
Refs: FR-001, FR-003, FR-008
{% /acceptance %}

{% acceptance id="AC-002" %}
**Given** POST /api/sessions を cols=0, rows=0 (もしくは cols/rows 欠落) で叩く
**When** セッションが spawn する
**Then** termvt.Session.Size() が (80, 24) を返し、slog に hint=(0,0) effective=(80,24) と Warn が出る
Refs: FR-002, FR-004, FR-008
{% /acceptance %}

{% acceptance id="AC-003" %}
**Given** POST /api/sessions を cols=100, rows=0 で叩く
**When** web 層の validator が受け付ける
**Then** HTTP 400 invalid_dim で拒否され、spawn は発生しない
Refs: FR-004, FR-006, FR-009
{% /acceptance %}

{% acceptance id="AC-004" %}
**Given** POST /api/sessions を cols=2001, rows=40 (または cols=65536) で叩く
**When** web 層の validator が受け付ける
**Then** HTTP 400 invalid_dim で拒否され、uint16 wrap は発生しない
Refs: FR-006, FR-009
{% /acceptance %}

{% acceptance id="AC-005" %}
**Given** spawn 後の RespawnFrame が発火する
**When** 既存 termvt session が Size() = (120, 50) を持つ
**Then** 新 session の termvt.Spec.Cols/Rows は (120, 50) となり、spawn 経路とは独立に SoT が保たれる
Refs: FR-001, FR-005
{% /acceptance %}

{% acceptance id="AC-006" %}
**Given** lifecycle WS に {k:'r', sessionId:sid, cols:2001, rows:40, reqId:'x'} を送る
**When** gateway.go の validator が受け付ける
**Then** 既存 winsize/grid が変わらず、{k:'e', reqId:'x', reason:'invalid_dim'} が返る
Refs: FR-007
{% /acceptance %}

{% acceptance id="AC-007" %}
**Given** 本 spec 完了後の repository
**When** grep -rn 'FrameSize(' src/ を実行する
**Then** 0 件 (FrameInspect.FrameSize が削除され、production/テスト 呼び出しが残っていない)
Refs: FR-010
{% /acceptance %}

## Open Questions

- 本 spec 実装完了後に browser 側 (TerminalPane.tsx) が実際に POST /api/sessions body に cols/rows を載せているかの現状確認 (issue 本文は 131,180 の fit / onResize 起点として言及があるが、create 時の body 組み立てコードのパスは実装 unit で確認する必要がある)。
