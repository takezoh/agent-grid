---
id: adr-20260705-view-update-sessions-only
kind: adr
title: view-update broadcast carries sessions only (no activeSessionID)
status: accepted
created: '2026-07-05'
decision_makers:
- Takehito Gondo
tags:
- web
- wire
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: supersedes, target: adr-20260624-0023-view-update-broadcast-shape}
source_paths:
- src/server/web/wire.go
- src/client/web/src/wire/server.ts
summary: 実装済みの現実 (viewUpdate は activeSessionID を mirror しない) を正本化し ADR 0023 を supersede
  する
updated: '2026-07-05'
---

# view-update broadcast carries sessions only (no activeSessionID)

## Context

{% context %}
adr-20260624-0023 は view-update broadcast を「`EvtSessionsChanged` の 1:1 mirror
(`activeSessionID` を含む)」と規定した。しかし実装 (`server/web/wire.go` の `viewUpdateFrame`) は
「`ActiveSessionID` is deliberately NOT mirrored」と明記して sessions のみを送っており、frontend の
fixtures にも activeSessionID は存在しない。active session はデーモン側の概念 (IPC client 単位) であり、
ブラウザの複数タブ / 複数デバイスがそれぞれ独立の選択状態を持つ web UI では、サーバ由来の active を mirror
すると全クライアントの選択が引きずられるため、実装時に意図的に落とされた。ADR 0023 は accepted のまま実装と
矛盾しており、docs の正本性を損ねている。
{% /context %}

## Decision

{% decision %}
実装済みの現実を正本とすることにする: **view-update broadcast frame は sessions 配列のみを運び、
`activeSessionID` を mirror しない**。クライアント側の選択状態 (active session) は各ブラウザクライアントの
ローカル state であり、wire 契約に含めない。adr-20260624-0023 は本 ADR で supersede する (broadcast の
存在・トリガ・全 subscriber への配布という核心は本 ADR が引き継ぐ)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
docs と実装の矛盾が解消され、wire fixtures (adr-20260705-wire-fixtures-pipeline) が pin する契約と
ADR の記述が一致する。
{% /consequence %}

{% consequence kind="neutral" %}
コード変更は無い。既存挙動の文書化のみ。
{% /consequence %}

## Alternatives

- **実装を ADR 0023 に合わせて activeSessionID を mirror する** — 却下。複数クライアントの選択状態が
  サーバの単一 active に引きずられる UX 欠陥を再導入する。
- **ADR 0023 を編集して記述だけ直す** — 却下。ADR は不変レコードであり、判断の変更は supersede で表現する
  (docs 方法論)。


{% transition from="proposed" to="accepted" date="2026-07-05" %}
The implemented wire contract already omits activeSessionID on view-update frames.
{% /transition %}
