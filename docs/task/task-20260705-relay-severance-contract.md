---
id: task-20260705-relay-severance-contract
kind: task
title: TerminalRelay severance contract
status: done
created: '2026-07-05'
priority: normal
effort: small
files_touched:
- src/client/runtime/terminal_relay.go
- src/client/runtime/terminal_relay_test.go
pr: null
tags:
- testing
owners: []
relations:
- {type: partOf, target: plan-20260705-test-harness}
- {type: dependencyOf, target: task-20260705-docs-llm-constraints}
source_paths:
- src/client/runtime/terminal_relay.go
- docs/component/component-20260624-platform-termvt-multiplexer-testing.md
summary: subscriber channel 容量をコンストラクタ注入化し、slow subscriber の sever が他 subscriber/session
  を阻害しないことを contract で pin
updated: '2026-07-05'
---

# TerminalRelay severance contract

## 責務

surface fan-out (`TerminalRelay.fanOut`) の severance 分岐を決定的に駆動可能にし、termvt の fan-out
isolation と対になる relay 層の不変条件を T2 contract として pin する (spec FR-004 / AC-006)。

## 詳細手順

1. `TerminalRelay` の subscriber channel 容量をコンストラクタ引数 (または Option) に昇格する。production
   の既定値は現行値を維持し、挙動変更ゼロであることを既存テストで確認する。
2. contract test を新設する (容量 1 を注入):
   - 受信を停止した subscriber が sever される (`internalSurfaceClosed` 経路)
   - sever 中も他 subscriber は全 event を順序どおり受信する
   - 別 session の配送に影響しない (cross-session 阻害なし)
   - sever 後の再 subscribe が正常に機能する
3. `-race` 下で green を確認する。

## 前提

なし (独立して着手可能)。task-20260705-runtimetest-harness と並行可。

## スコープ外

- relay を `state.Reduce` 経由に再設計すること (internal 経路の設計は現状維持、
  adr-20260705-eventsink-seam-tap-relay-contracts 参照)

## 受け入れ条件

- 新 contract が `-race` で決定的に green (sleep 依存・データ量依存の駆動をしない)
- 既存 relay テストが無修正で green (production 既定値の非変更)
- `make lint` green


{% transition from="todo" to="in_progress" date="2026-07-05" %}
Started implementation of TerminalRelay subscriber-buffer injection and severance contract test.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-05" %}
Implemented TerminalRelay subscriber-buffer injection and severance contract test; go test -race ./client/runtime -run TestTerminalRelay and make lint green.
{% /transition %}
