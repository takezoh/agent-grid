---
id: task-20260705-tap-osc-contract
kind: task
title: pty→OSC tap contract test + FuzzParseOsc
status: done
created: '2026-07-05'
priority: high
effort: medium
files_touched:
- src/client/runtime/tap_contract_test.go
- src/client/runtime/tap_manager.go
- src/client/runtime/pty_tap.go
pr: null
tags:
- testing
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependsOn, target: task-20260705-runtimetest-harness}
- {type: dependencyOf, target: task-20260705-docs-llm-constraints}
source_paths:
- src/client/runtime/tap_manager.go
- src/client/runtime/pty_tap.go
- src/platform/termvt/fanout_contract_test.go
summary: real pty に OSC 列を書き込み EvFrameOsc/EvFramePrompt の FrameID routing と emulator
  panic 封じ込めを pin する contract、parseOscNotification の fuzz
updated: '2026-07-05'
---

# pty→OSC tap contract test + FuzzParseOsc

## 責務

経路 A (pty → `PtyFrameTap.forwardEvents` → `readTap` → VT emulator → `EvFrameOsc`/`EvFramePrompt`)
を端から端まで 1 テストで駆動し、routing と liveness を T2 contract として pin する
(spec FR-003 / AC-004)。

## 詳細手順

1. `EventSink` seam (task-20260705-runtimetest-harness で導入済み) に recording 実装を渡し、real pty
   (termvt 経由で spawn した frame) に対して次のシーケンスを書き込む contract test を新設する:
   - OSC 0 / 2 (title) → `EvFrameOsc` が **書き込んだ frame の FrameID** で届く
   - OSC 133 (prompt phase) → `EvFramePrompt` が正しい phase 変換 (`vtPromptPhase`) で届く
   - OSC 9 / 99 / 777 (notification) → notification 系 event が届く
   - 2 frame 並行で書き込み、cross-talk が無い (frame F の OSC が frame G の event にならない)
2. emulator panic 封じ込めの pin: 既知の panic を誘発する不正シーケンス (docs/issues の VT emulator
   panic レポート参照) を書き込み、`feedSafe` の recover 後もイベント発行が継続することを assert する。
   これは既知 panic の再発検知であり、recover の温存を正当化するものではない (root fix は別 issue)。
3. `FuzzParseOsc` — `parseOscNotification` / `vtPromptPhase` (純関数) への stdlib fuzz を追加する。
4. contract は `-race` 下で走ることを確認し、`make test-race` の対象 subtree であることを維持する。

## 前提

- task-20260705-runtimetest-harness (EventSink seam)

## スコープ外

- VT emulator panic 自体の root fix (別 issue で追跡)
- TerminalRelay (task-20260705-relay-severance-contract)

## 受け入れ条件

- 新 contract が `-race` で決定的に green
- FuzzParseOsc が CI fuzz job に追加され 30s で green
- `make lint` green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
Reconciling implemented tap contract with docs lifecycle.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
Implemented and verified tap OSC/prompt routing, feedSafe boundary handling, and FuzzParseOsc.
{% /transition %}
