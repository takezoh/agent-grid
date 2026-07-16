---
id: adr-20260711-priority-lane-interactive-vs-bulk
kind: adr
title: Separate interactive surface-output from bulk telemetry into priority lanes
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
- {type: references, target: adr-20260711-extend-sever-not-drop-shared-ipc-hops}
- {type: references, target: change-20260711-terminal-output-backpressure}
source_paths:
- src/client/runtime/ipc.go
- src/client/runtime/proto_bridge.go
- src/client/runtime/proto_bridge_surface.go
summary: 共有ホップを対話的レーンとbulkレーンに分離し相互飢餓を防ぐ。Consequences は三極を本文 Consequences 節に記載 (このリポジトリの
  docs schema は spec-detail v1 の consequences/confirmation frontmatter フィールドを未サポートのため本文のみ)。
updated: '2026-07-11'
---

## Context

{% context %}
`internalCh` と `ipcConn.outbox` は、対話的な surface-output (打鍵エコー、Codex `/resume` のような大量再描画バーストを含む) と、bulk な control-plane telemetry (`sessions-changed` / `agent-notification` / `session-file-line`、後者はセッションIDで絞られず daemon 全体分が届く) を、優先度の区別なく同じ FIFO で同居させている。`adr-20260711-extend-sever-not-drop-shared-ipc-hops` の per-subscription severance だけでは、同一購読内で対話的チャンクと bulk チャンクが競合するケース (例: 同じセッションの surface-output 自体が大量再描画チャンクと個々の打鍵エコーを両方含む) や、無関係なセッション宛の bulk イベントが他セッションの対話的バーストによって飢餓するケースを解決できない。ユーザーの元指示は「sever-not-drop の一貫化、および/または優先度分離」を明示的に or/both としており、FR-006/FR-007 (対話的・bulk の相互飢餓禁止を対称に固定する invariant FR) を満たすには severance だけでは不十分と判断した。
{% /context %}

## Decision

{% decision %}
`internalCh` と `ipcConn.outbox` を、surface-output 用の interactive レーンと `sessions-changed`/`agent-notification`/`session-file-line` 用の bulk レーンの2レーンに分離することにする。両ホップの drain ループは priority-select (interactive を優先的にドレインし、bulk は interactive に空きがある間のみ処理する) で運用し、一方のバーストが他方を無制限に飢餓させることを防ぐ。severance 判定 (どの購読を sever するか) は `adr-20260711-extend-sever-not-drop-shared-ipc-hops` の機構をレーン内で呼び出す — レーン分離と severance は独立した軸の判断であり、両方を組み合わせて初めて FR-006/FR-007 と FR-009 の両方を満たす。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
対話的 surface-output と bulk telemetry が相互に飢餓させ合わないことが構造的に保証される (FR-006/FR-007 の対称 invariant を満たす)。
{% /consequence %}

{% consequence kind="positive" %}
per-subscription severance と組み合わさることで、severance 判定は「どのレーンで」「どの購読が」輻輳したかを明確に区別できる。
{% /consequence %}

{% consequence kind="negative" %}
internalCh・ipcConn.outbox の drain ループが単一 select から priority-select (2レーン) に変わり、実装・デバッグの複雑さがわずかに増える。
{% /consequence %}

{% consequence kind="negative" %}
bulk レーン専用の空き容量が常に確保されるため、対話的トラフィックが皆無の定常状態でも2レーン分のメモリを保持する。
{% /consequence %}

{% consequence kind="neutral" %}
レーン分離は per-subscription attribution と独立した軸の判断であり、どちらか一方だけでは FR-006/FR-007 と FR-009 の両方を同時に満たせない。
{% /consequence %}

## Alternatives

- **レーン分離なし (per-subscription severance のみで十分とする)** — 却下。同一購読内の順序は termvt fanout() の FIFO で保たれるが、無関係セッション宛の bulk イベントが対話的バーストに飢餓させられるケース (FR-006) を解決できない。
- **surface-output 内部のみレーン分離し control-plane 全体は分離しない** — 却下。元バグの直接原因 (`/resume` の大量再描画自体が対話的 surface-output として分類される) を見落とす — 対話的トラフィック同士の内部競合よりも、対話的トラフィックが無関係な bulk トラフィックを飢餓させるケースの方が本バグの実害に近い。
- **優先度を動的に変える (adaptive priority)** — 却下。立証責任攻撃 (design/PRINCIPLES.md §12) — 固定 priority-select で FR-006/FR-007 を満たせることが確認できており、動的優先度の追加複雑性を正当化する要件が無い (speculative generality)。
