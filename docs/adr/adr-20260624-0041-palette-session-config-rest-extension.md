---
id: adr-20260624-0041-palette-session-config-rest-extension
kind: adr
title: ADR 0041 — push_commands と project metadata は GET /api/session-config の REST 拡張で配信する
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations: []
source_paths: []
decision_makers:
- unknown
---

<!-- migrated_from: docs/adr/0041-palette-session-config-rest-extension.md -->

# ADR 0041 — push_commands と project metadata は GET /api/session-config の REST 拡張で配信する

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-command-palette/spec.md), [plan](../specs/2026-06-24-web-ui-command-palette/plan.md)
Related requirements: FR-013, FR-014, FR-015, FR-027

## Context

現状 handleSessionConfig は cfg.Session.Commands と projects (path 配列) のみ返す。push_commands と projectIsGit / projectIsSandboxed の判定源を web に届ける経路が存在しない。hello フレームや view-update に乗せると WS が静的構成情報を運ぶことになり、view-update は activeSessionID をドロップする規約 (ADR-0023) と意味論的に揃わない。

## Decision

GET /api/session-config の apiSessionConfig レスポンスに push_commands ([]string) と projects ([{path:string, isGit:bool, isSandboxed:bool}]) を追加する。WS フレーム (hello / view-update) には何も足さない。Web は palette open 時 (or App 起動時) にこの REST を 1 回 fetch し store/daemon に格納する。

## Consequences

- **positive**: WS は I/O 専用の制約を守れる (ADR-0030 親和)
- **positive**: 構成情報は REST、状態は WS の関心分離が明示される
- **positive**: 既存 CreateSessionForm が叩いていた REST 経路をそのまま拡張するだけ
- **negative**: isGit / isSandboxed は config 変更で値が変わるが、palette open 毎に再 fetch する必要性は要観察

## Alternatives Considered

### hello frame に push_commands と project metadata を載せる

却下: WS で静的構成情報を運ぶことになり関心分離が崩れる

### 別エンドポイント GET /api/push-commands と GET /api/projects/meta を新設

却下: round-trip が増え、CreateSessionForm 撤去後に session-config と分かれる意味が乏しい
