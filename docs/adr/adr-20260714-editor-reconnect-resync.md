---
id: adr-20260714-editor-reconnect-resync
kind: adr
title: WS reconnect triggers a forced conflict-check re-fetch for every dirty buffer
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-editor
- design
owners: []
relations:
- {type: partOf, target: change-20260714-agent-workspace-editor}
source_paths: []
summary: WS reconnect triggers a forced conflict-check re-fetch for every dirty buffer
updated: '2026-07-14'
---

## Context

issue-dirty-touch-signal-drop-window-open が指摘した通り、WS reconnect の gap で touch event を取りこぼした場合、reconnect 直後の :w が silent overwrite になり得る。decision-conflict-detection-resync として明示決定が必要。

## Decision

WS reconnect イベントで、conflict detector は **dirty buffer を保持する全 path に対して強制的な mtime/ETag re-fetch** (現在のサーバ側 mtime vs open 時 snapshot 比較) を trigger する。差分がある場合は conflict banner (reconnect_mtime_differs class) を出し、fetch 自体が失敗する場合は connectivity-degraded banner で save を disable する。gap を silent overwrite window として残さない。

## Consequences

- reconnect 直後の :w が silent overwrite になる window が構造的に閉じる。
- connectivity 障害後も conflict-detection invariant が成立する。
- reconnect 頻度が高い環境で若干の HTTP overhead を伴うが、cap は dirty buffer 数に線形。

## Alternatives

- **却下: reconnect 時に何もしない** — silent overwrite window が閉じない — contract-write-conflict-detection の invariant が transport 障害後に成立しない。

## Trace

- Requirements: FR-112
- Implementation contracts: contract-write-conflict-detection
