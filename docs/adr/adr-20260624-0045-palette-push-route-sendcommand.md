---
id: adr-20260624-0045-palette-push-route-sendcommand
kind: adr
title: ADR 0045 — POST /api/sessions/{id}/push は handleCreateSession 同形の SendCommand
  + proto.CmdEvent パターンで実装する
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: change-20260624-2026-06-24-web-ui-command-palette}
source_paths: []
decision_makers:
- unknown
summary: web gateway の daemon_client.go は SendCommand(ctx, proto.Command) を共通 RPC
  エントリとして使う設計で、handleCreateSession は EventCreateSession を SendCommand 経由で発行する。PushDriver
  専用 RPC ラッパを追加すると 2 経路化する。
---

<!-- migrated_from: docs/adr/0045-palette-push-route-sendcommand.md -->

# ADR 0045 — POST /api/sessions/{id}/push は handleCreateSession 同形の SendCommand + proto.CmdEvent パターンで実装する

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-command-palette/spec.md), [plan](../specs/2026-06-24-web-ui-command-palette/plan.md)
Related requirements: FR-025

## Context

web gateway の daemon_client.go は SendCommand(ctx, proto.Command) を共通 RPC エントリとして使う設計で、handleCreateSession は EventCreateSession を SendCommand 経由で発行する。PushDriver 専用 RPC ラッパを追加すると 2 経路化する。

## Decision

POST /api/sessions/{id}/push の Go 側ハンドラは SendCommand(ctx, proto.CmdEvent{Event: state.EventPushDriver, Payload: state.PushDriverParams{SessionID: id, Command: command}}) を発行する。daemon_client.go に PushDriver 専用ラッパは追加しない。

## Consequences

- **positive**: 既存パターン (handleCreateSession) と完全に揃う
- **positive**: daemon_client.go に分岐を増やさない
- neutral: PushDriver の入力検証 (空 command / 巨大 command) は mux.go 側で行う

## Alternatives Considered

### daemon_client.go に PushDriver(ctx, id, cmd) wrapper を追加

却下: SendCommand に統一されている現状を分裂させる
