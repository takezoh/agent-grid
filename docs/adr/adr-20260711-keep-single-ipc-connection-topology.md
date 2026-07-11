---
id: adr-20260711-keep-single-ipc-connection-topology
kind: adr
title: Keep the single physical daemon-gateway IPC connection topology
status: accepted
created: '2026-07-11'
decision_makers:
- Takehito Gondo
tags:
- runtime
- ipc
- reliability
owners: []
relations:
- {type: references, target: adr-20260711-extend-sever-not-drop-shared-ipc-hops}
- {type: references, target: spec-20260711-terminal-output-backpressure}
- {type: partOf, target: plan-20260711-terminal-output-backpressure}
source_paths:
- src/server/web/daemon_client.go
- src/cmd/server/gateway.go
summary: 物理輸送層は分割せず論理的な購読隔離で対処する。Consequences は三極を本文 Consequences 節に 記載 (このリポジトリの
  docs schema は spec-detail v1 の consequences/confirmation frontmatter フィールドを未サポートのため本文のみ)。
updated: '2026-07-11'
---

## Context

{% context %}
agent-grid の TERMINAL 打鍵エコー消失バグの調査で、daemon↔gateway 間には唯一の物理 IPC コネクション (`server/web/daemon_client.go` の `DaemonClient`、`cmd/server/gateway.go:72` で1回だけ生成) があり、全 browser タブ・全セッションの購読がこの1本の上で `proto.SubscriberID` によって論理的に多重化されていることを確認した。この物理トポロジー自体を、購読単位 (browser タブ単位、または (session, subscriber) 単位) で複数コネクション化するかどうかは、本バグ修正のスコープに直接関わる判断点である。
{% /context %}

## Decision

{% decision %}
既存の単一物理 IPC コネクションのトポロジーを維持することにする。購読単位の隔離・優先度分離は `adr-20260711-extend-sever-not-drop-shared-ipc-hops` (per-subscription severance) と `adr-20260711-priority-lane-interactive-vs-bulk` (priority lane) の論理層の仕組みで担保し、物理輸送層 (Unix socket 接続そのもの) の分割・複数化は今回は行わない。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
daemon↔gateway 間の再接続機構 (DaemonClient の supervise / 指数バックオフ) を一切変更せずに済み、本修正のスコープが小さく保たれる。
{% /consequence %}

{% consequence kind="positive" %}
既存の proto.Client / DaemonClient のテスト (net.Pipe ベースの dialer 注入) をそのまま再利用できる。
{% /consequence %}

{% consequence kind="negative" %}
物理層では引き続き全 browser タブ・全セッションが1本の IPC コネクションを共有するため、将来さらにセッション数・タブ数が増えた場合、論理層の隔離 (severance + priority lane) だけでは吸収しきれない負荷が生じる可能性が残る。
{% /consequence %}

{% consequence kind="neutral" %}
購読単位の物理コネクション複数化は将来の north-star として残すが、今回は着手しない — 必要になった時点で本 ADR を supersede する。
{% /consequence %}

## Alternatives

- **購読単位 (browser タブ or (session, subscriber)) で物理コネクション/ストリームを複数化する** — 却下 (今回は不採用)。帰責・隔離が構造的に保証される最も正しい解だが、輸送層・プロトコル・接続管理の全面変更を伴う大工事であり、ユーザーが明示的にスコープ外とした「daemon↔gateway 輸送層の全面再設計」に該当する。将来の負債認識として本 ADR の Alternatives に記録し、north-star として残す。