---
id: adr-20260711-extend-sever-not-drop-shared-ipc-hops
kind: adr
title: Extend sever-not-drop contract to daemon-side shared IPC hops
status: accepted
created: '2026-07-11'
decision_makers:
- Takehito Gondo
tags:
- runtime
- ipc
- terminal
- reliability
owners: []
relations:
- {type: partOf, target: change-20260711-terminal-output-backpressure}
- {type: references, target: adr-20260705-eventsink-seam-tap-relay-contracts}
- {type: references, target: change-20260711-terminal-output-backpressure}
source_paths:
- src/client/runtime/ipc.go
- src/client/runtime/terminal_relay.go
- src/server/web/daemon_client.go
summary: per-subscription backlog attribution で internalCh/ipcConn.outbox/DaemonClient
  per-tab fanout に adr-20260705 の sever-not-drop を一貫適用する。Consequences は三極 (positive/negative/neutral)
  を本文 Consequences 節に記載 (このリポジトリの docs schema は spec-detail v1 の consequences/confirmation
  frontmatter フィールドを未サポートのため本文のみ)。
updated: '2026-07-11'
---

## Context

{% context %}
`adr-20260705-eventsink-seam-tap-relay-contracts` は「1 source → 多 subscriber」形状 (termvt の `fanout()`, `TerminalRelay.fanOut`) において、追従できない subscriber は sever し、他 subscriber / 他 session の配送順序は維持する、という契約を確立した。しかし agent-grid の TERMINAL 打鍵エコー消失バグを調査した結果、同じ「1 source → 多 subscriber」形状でありながらこの契約が及んでいないホップが daemon 側にさらに 2 つ存在することが判明した: `client/runtime/ipc.go` の `r.internalCh` (cap 64、TerminalRelay の全購読の fanOut goroutine + 内部ライフサイクルイベントが共有) と `ipcConn.outbox` (cap 64、daemon↔gateway 間の唯一の物理 IPC コネクションの outbox、全 browser タブ・全セッションが共有)。両ホップは `select { default: drop }` で輻輳時に無帰責にイベントを破棄しており、Codex CLI の `/resume` のような大量再描画バーストがこれを溢れさせると、無関係な購読を含めて surface-output が間引かれる。gateway 側の `DaemonClient.broadcastEvent` (per-tab, cap 64) も同型だが、こちらは `SubscribeEvents` 呼び出しごとに専用チャンネルを持つため既に購読単位の隔離があり、sever 化のコストは低い。
{% /context %}

## Decision

{% decision %}
`adr-20260705` の sever-not-drop 契約を、`internalCh`・`ipcConn.outbox`・`DaemonClient.broadcastEvent` の全3ホップへ一貫適用することにする。ただし帰責困難な `internalCh` / `ipcConn.outbox` では、単純な drop カウントによるヒューリスティック (Option A) ではなく、`TerminalRelay` 内に (ConnID, SessionID, SubscriberID) 単位の専用 backlog attribution を新設してから severance 判定を行う (Option B) を採用する — 共有チャンネルの輻輳観測だけでは無関係な購読を誤って victim 化するリスク (FR-009) を構造的に排除するためであり、実装コストの高さよりも帰責の正確性を優先する。severance 判定ポリシー (閾値・帰責方法) は `TerminalRelay` 内の1機構に集約し、他ホップはこれを呼び出す (FR-008)。閾値は `WithTerminalRelaySubscriberBuffer` と同型のコンストラクタ注入 seam として提供する (adr-20260705 と同じテスト可能性の流儀)。severance が発生した際に既存の `internalSurfaceClosed → EvCmdSurfaceUnsubscribe` 合成経路を trigger 元として再利用する (browser への通知は `adr-20260711-server-initiated-severance-signal` が別途規定する)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
internalCh・ipcConn.outbox・DaemonClient 側 fan-out の3ホップが termvt fanout() と同じ「輻輳した購読だけが被害を受け、他は無傷」という保証を持つようになり、系全体で backpressure の意味論が統一される。
{% /consequence %}

{% consequence kind="positive" %}
severance 判定が1機構 (TerminalRelay 内の per-subscription backlog attribution) に集約されるため、将来同種の 1→N ホップが追加されても同じ機構を再利用できる (FR-008)。
{% /consequence %}

{% consequence kind="negative" %}
internalCh / ipcConn.outbox は購読単位の帰責を持たない共有チャンネルであるため、per-subscription backlog counter という新しい状態をホットパス (毎チャンク) に追加する必要があり、実装・レビューコストが増える。
{% /consequence %}

{% consequence kind="negative" %}
既存の3箇所の select/default drop 分岐を新しい severance helper へ置き換える変更は、対象ファイルの既存呼び出し規約に波及する。
{% /consequence %}

{% consequence kind="neutral" %}
帰責困難な共有ホップでの severance は、ヒューリスティックな drop カウント方式より高コストな「購読単位の専用バッファ」方式 (Option B) を採用する。コスト最小化ではなく構造的正しさを優先するという明示的なトレードオフの結果である。
{% /consequence %}

## Alternatives

- **Option A (ヒューリスティック): 各購読ごとに consecutive drop 回数をカウントし閾値超過した購読を victim とする** — 却下。共有チャンネル輻輳時に無関係な購読を誤って victim 化するリスク (FR-009) を運用時まで残す。実装コストは最低だが、ユーザーが明示した「構造的修正」の要求水準を満たさない。
- **Option C (無帰責な全体再構築): 輻輳時は internalCh なら内部イベント全体、ipcConn.outbox なら唯一の IPC コネクション全体を re-sync 前提で再構築する** — 却下。ブラスト半径が最大化し、FR-005 (無差別な全体巻き添え禁止) に直接抵触する。
- **DaemonClient.broadcastEvent を専用の新規機構として個別実装する** — 却下。FR-008 (severance 判定の一箇所集約) に反する。DaemonClient は既に購読単位で隔離されているため、`internalCh`/`ipcConn.outbox` 用に新設する機構と同じ判定ロジックを呼び出す形で十分。
