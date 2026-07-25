---
id: adr-20260726-server-owned-session-card-title
kind: adr
title: セッションカード title は server wire 境界が完全値を保証する
status: accepted
created: '2026-07-26'
decision_makers:
- agent-grid-maintainers
tags:
- host
- wire
- clients
owners:
- agent-grid-maintainers
relations:
- {type: references, target: adr-20260624-0079-session-card-title-fallback-chain}
source_paths:
- src/host/runtime/proto_bridge.go
- src/host/state/view/view.go
- clients/ui/src/components/SessionList.tsx
- clients/windows-shell/AgentGrid.Shell.Core/SupervisionState/PanelPresentation.cs
summary: server が title を trim し空値を New Session に正規化し、クライアントは受信 title をそのまま表示する
consequences:
  positive:
  - Web と Windows Shell で placeholder と trim の挙動が分岐しない
  negative:
  - server wire 境界は全 session view の title 正規化を必ず通す必要がある
  neutral:
  - driver の title fallback chain 自体は ADR-0079 のまま維持する
confirmation: TestWireCardTitleOwnsPlaceholderAndTrimming と各 client の titleText/SessionLabel
  テスト
updated: '2026-07-26'
---

# セッションカード title は server wire 境界が完全値を保証する

## Context

ADR-0079 は driver の title fallback chain の末尾を空文字とし、Web client が
`New Session` を補完していた。Windows Shell の追加により同じ補完と trim が
複数クライアントへ重複し、表示内容が分岐した。

## Decision

`Runtime.buildOneSessionInfo` を server wire の title owner とする。driver view の
`Card.Title` を trim し、空なら `stateview.DefaultCardTitle` (`New Session`) を設定して
から `proto.SessionInfo` を生成する。Web と Windows Shell は受信した title を加工せず
表示し、project、command、session ID、client-side placeholder を fallback に使わない。

## Consequences

frontmatter `consequences` が正本。

## Alternatives

- client ごとに `New Session` を補完する: 表示規則が分散するため却下。
- gateway JSON encoder で補完する: IPC/API の他 consumer へ空 title が残るため却下。
