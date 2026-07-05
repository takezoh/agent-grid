---
id: adr-20260705-eventsink-seam-tap-relay-contracts
kind: adr
title: EventSink seam and contract pins for the pty-tap and surface-relay paths
status: accepted
created: '2026-07-05'
decision_makers:
- Takehito Gondo
tags:
- testing
- runtime
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: references, target: adr-20260624-0003-termvt-fanout-isolation}
- {type: referencedBy, target: note-20260624-agent-testing}
- {type: referencedBy, target: note-20260624-technical-code-enforcement}
- {type: references, target: note-20260624-technical-code-enforcement}
source_paths:
- src/client/runtime/tap_manager.go
- src/client/runtime/pty_tap.go
- src/client/runtime/terminal_relay.go
- src/client/runtime/subsystem/stream/backend.go
summary: stream backend の RuntimeHook パターンを tap/relay へ複製し、pty→OSC routing と relay
  severance を real-pty contract で pin する
updated: '2026-07-05'
---

# EventSink seam and contract pins for the pty-tap and surface-relay paths

## Context

{% context %}
イベント伝搬 5 経路のうち stream 経路のみが、`RuntimeHook` interface (`subsystem/stream/backend.go`) +
`recordingRuntime` + 高忠実 fake app-server により「端から端まで」を 1 テストで駆動できる。対照的に:

- **経路 A (pty → `PtyFrameTap.forwardEvents` → `readTap` → VT emulator → `EvFrameOsc`)** は
  goroutine 連鎖の途中に seam が無く、`fakeFrameTap` で連鎖が途切れる。OSC event の FrameID routing と
  emulator panic 封じ込め (`feedSafe`) は unit test で直接駆動できない。
- **surface relay (`TerminalRelay.fanOut`)** は `state.Reduce` を通らない internal 経路で、slow
  subscriber の sever 分岐は channel 飽和を要し決定的に駆動できない。

いずれも termvt / stream と同じ「1 source → 多 subscriber」形状であり、cross-talk / blocking が
安全性欠陥となる点も同じだが、対応する contract pin が無い。
{% /context %}

## Decision

{% decision %}
1. **EventSink seam** — `tap_manager` の enqueue 先を `EventSink` interface (`Enqueue(state.Event)`
   のみ、stream backend の `RuntimeHook` と同型) に seam 化する。production では `*Runtime` を、テストでは
   recording 実装を渡す。
2. **tap contract (T2)** — real pty 上の frame に OSC 0/2/9/133 シーケンスを書き込み、`-race` 下で
   次を pin する: (a) frame F 由来の OSC event は `FrameID == F` のみ (routing 正しさ)、(b) 不正 /
   破壊的シーケンスでも emulator panic は封じ込められ event 発行が継続する (liveness)。termvt の
   `fanout_contract_test.go` と同じ real-pty 流儀。
3. **relay severance contract (T2)** — `TerminalRelay` の subscriber channel 容量をコンストラクタ
   注入化 (production 既定値は現状維持、テストでは 1) し、「追従できない subscriber は sever され、他
   subscriber と他 session の配送順序は維持される」を決定的に pin する。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
「1 source → 多 subscriber」形状の全インスタンス (stream / termvt / tap / relay) が同じ保証構造
(seam + contract) を持ち、enforcement note の test-pin 節に同列で記載できる。
{% /consequence %}

{% consequence kind="negative" %}
production コードに 2 箇所の seam 変更 (interface 導入・容量注入) が入る。挙動は不変だが、コンストラクタ
シグネチャの変更が既存呼び出し箇所へ波及する。
{% /consequence %}

## Alternatives

- **fakeFrameTap の拡充で代替する** — 却下。fake tap は連鎖の入口を置換するため、`readTap` 以降
  (VT feed / OSC parse / enqueue) が対象外のまま残る。検証したいのはまさにその連鎖。
- **termvt に fake を導入して tap を fake pty で駆動する** — 却下。real pty は安価かつ hermetic であり
  (adr-20260705-test-tier-taxonomy の pty 例外)、fake の導入は忠実性保証という新たな負債を作る。
- **relay の sever を統計的に (大量データ投入で) 駆動する** — 却下。flaky の温床。容量注入により決定的に
  再現するのが正道。


{% transition from="proposed" to="accepted" date="2026-07-05" %}
tap/relay contract 実装済み・severance contract は scheduling 非依存化 (453c8803) 済み
{% /transition %}
