---
id: adr-20260714-editor-write-endpoint-shape
kind: adr
title: Editor save wire contract is PUT /workspace/file with raw body and If-Unmodified-Since
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
summary: Editor save wire contract is PUT /workspace/file with raw body and If-Unmodified-Since
updated: '2026-07-14'
---

## Context

operator save の endpoint shape は decision-input-write-endpoint-shape の 3 option (PUT raw body / POST multipart / WS write frame) の中から選ぶ必要がある。read-side は既に GET /workspace/file?path=... で raw content を返しており、対称形が最小差分。既存 ViewUpdate WS は server→client broadcast 専用であり request/response 相関を持たない。

## Decision

Editor save の wire contract を **PUT /api/sessions/{id}/workspace/file?path=<workspace-relative>** with **raw body bytes** and **If-Unmodified-Since header** (open 時 mtime を precondition として反映) に確定する。response は **200 + JSON {updated_mtime}** on success、**412 precondition_failed + JSON {current_mtime}** on optimistic-lock mismatch、**409 handle_stale** on frame push mismatch、**413 oversize_body** on cap exceed、**401** on missing auth、**404 workspace_path_not_found** on containment failure。

## Consequences

- client (api/workspace.ts) は既存 getFile() と対称な save() を追加でき、他 endpoint は変わらない。
- server は既存 mux.go の GET-only workspace 群と隣接する PUT ハンドラを 1 つ追加するだけで、GET 呼び出し側の互換性は完全に保たれる。
- concurrency policy は wire に直接乗る (If-Unmodified-Since) ため、conflict detector と自然に組み合わされる。

## Alternatives

- **却下: POST multipart** — 単一テキストファイル保存に対して multipart parsing コストと 3 boundary 分の余分な構造化が正当化されない。
- **却下: 既存 ViewUpdate WS への write frame 追加** — WS channel は現在 server→client broadcast 専用であり方向を反転させ request/response 相関の別実装が必要になる — HTTP + 標準 REST 慣習からの逸脱が大きい。

## Trace

- Requirements: FR-102
- Implementation contracts: contract-write-persistence-save
