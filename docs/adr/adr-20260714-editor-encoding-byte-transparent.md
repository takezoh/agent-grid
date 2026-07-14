---
id: adr-20260714-editor-encoding-byte-transparent
kind: adr
title: Editor save preserves encoding and line endings byte-transparently
status: accepted
created: '2026-07-14'
decision_makers:
- Takehito Gondo
tags:
- workspace-editor
- design
owners: []
relations:
- {type: partOf, target: plan-20260714-agent-workspace-editor}
source_paths: []
summary: Editor save preserves encoding and line endings byte-transparently
updated: '2026-07-14'
---

## Context

decision-point 'non-UTF-8 / CRLF file' は observable が根本的に異なる (byte-transparent / normalize / reject)。触れていない領域の silent corruption を回避する必要があり、CodeMirror 6 の既定改行 normalize を明示的に無効化する implementation constraint が伴う。

## Decision

**byte-transparent round-trip** を確定する。CodeMirror EditorState.lineSeparator を open 時のファイル内容から検出して明示的に設定し、CRLF/LF/mixed を engine の default normalize から守る。非 UTF-8 バイト sequences は engine が保持できない場合 editor mode を拒否して read-only viewer に fallback する (typed unsupported_encoding banner)。

## Consequences

- 触れていない領域のバイト diff が支援対象 fixture 全体でゼロになる (contract-encoding-newline-fidelity)。
- normalize-on-save や reject-non-UTF-8 の別 option は排除される。
- read-only viewer への fallback path が非 UTF-8 で発火する明示的な degradation経路として存在する。

## Alternatives

- **却下: normalize-with-notice** — operator の意図しない非可逆変換を発生させる — 触れていない領域の破損リスクを design で内包することになる。
- **却下: reject-non-UTF-8** — 適用範囲が read-only viewer に比べ狭まり、UTF-8 だが CRLF/mixed のファイルまで巻き添えで排除される。

## Trace

- Requirements: FR-110
- Implementation contracts: contract-encoding-newline-fidelity
