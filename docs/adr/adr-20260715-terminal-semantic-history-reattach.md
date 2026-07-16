---
id: adr-20260715-terminal-semantic-history-reattach
kind: adr
title: Own semantic terminal history and reattach ANSI in the VT fork
status: accepted
created: '2026-07-15'
decision_makers:
- unknown
tags:
- terminal
- scrollback
- reflow
- vt
owners: []
relations:
- {type: partOf, target: change-20260715-terminal-scrollback-reflow}
- {type: references, target: change-20260715-terminal-scrollback-reflow}
- {type: supersedes, target: adr-20260624-0066-terminal-scrollback-via-vt-buffer}
- {type: referencedBy, target: adr-20260716-vt-semantic-buffer-owner}
- {type: referencedBy, target: adr-20260716-vt-screen-specific-resize}
- {type: referencedBy, target: adr-20260716-vt-snapshot-failure-and-locking}
- {type: referencedBy, target: adr-20260716-vt-reflow-pr-migration}
source_paths:
- src/platform/termvt
- src/go.mod
summary: Preserve hard/soft boundary provenance in the existing x/vt fork and emit
  width-independent xterm-compatible reattach snapshots.
updated: '2026-07-15'
---

## Context

ADR 0066 は server-side VT scrollback を採用したが、`uv.Lines(...).Render()` が物理 row を newline で連結することを正しい seed contract とした。xterm は newline を hard break として保持するため、狭い幅で保存された row を広い viewer へ attach しても連結できない。「resize crossing は構造上存在しない」という前提は late join と reconnect で反証された。

wrap provenance を旧 row 長や trailing blank から推測できない。overwrite、erase、scroll、exact-column pending wrap、wide/combining cell、cap eviction が同じ外形を異なる terminal semantics から作るためである。agent-grid 側に parallel model/renderer を置けば VT semantics の決定権が分散する。

## Decision

既存 `github.com/takezoh/x/vt` fork を semantic terminal history の唯一の owner とする。fork は hard break / soft continuation、head-truncated、exact-column pending wrap、cell/style/cursor を保持し、target geometry から xterm-compatible opaque ANSI `ReattachSnapshot` を生成する。

provenance は write/overwrite、erase、insert/delete char、insert/delete line、LF/index、region/full scroll、resize、cap eviction、primary/alternate transition を含む全 row-producing/removing mutation の閉じた保存則とする。新 mutation が分類されていなければ invariant contract を失敗させる。agent-grid の Emulator seam は opaque snapshot bytes と typed failure だけを受け渡し、`SemanticLine` / `CellRun` adapter、wrap inference、terminal renderer を持たない。

この ADR は ADR 0066 を supersede する。server-side bounded in-memory history、raw PTY replay rejection、alternate-screen exclusion は継承し、newline physical-row serializer、browser unchanged、resize crossing absent を置換する。fork change は既存 replace target の release/tag として提供し、`src/go.mod` の pin を更新する。`SerializeScrollback` と旧二段 seed は consumer 移行後に削除し、fallback として残さない。

## Consequences

### Positive

{% consequence kind="positive" %}terminal semantics と ANSI 再構成が一つの owner に閉じ、late join と再 resize の correctness を row mutation invariants と xterm observable matrix で検証できる。bounded final-state snapshot と公開 xterm API を維持できる。{% /consequence %}

### Negative

{% consequence kind="negative" %}agent-grid repo だけでは完結せず、既存 fork の release、pin 更新、drift backstop が必要になる。full row mutation closure と exact-column/style/cursor fidelity の fixture は実装量が大きい。内部 invariant 違反時は誤表示へ degrade せず session を失う。{% /consequence %}

### Neutral

{% consequence kind="neutral" %}history は in-memory bounded のまま、alternate-screen は primary history に入らず、outbound output frame は ANSI bytes のままである。multi-viewer size arbitration と persistence はこの判断で変えない。{% /consequence %}

## Alternatives

**agent-grid に semantic snapshot model と renderer を置く。** fork と adapter の両方が terminal semantics を解釈して drift するため却下する。

**旧 row の長さや空白から soft wrap を推測する。** exact-column hard break、erase、wide cell で反例があり、silent corruption を契約化するため却下する。

**raw PTY transcript を replay する。** bounded final-state history、side-effect safety、attach latency の既存判断を失うため却下する。

**structured cells を xterm private buffer API へ注入する。** browser implementation detail と wire schemaへ semantic ownershipを分散するため却下する。

## Confirmation

fork の invariant-naming contract と FakeVsReal、ANSI fidelity matrix の Playwright、`rg 'SerializeScrollback' src` が移行完了時に 0 件であることを fitness function とする。


{% transition from="proposed" to="accepted" date="2026-07-15" %}
User approved implementation of the designed contract
{% /transition %}
