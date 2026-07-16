---
id: adr-20260714-editor-atomicity-tmp-rename
kind: adr
title: Save atomicity uses same-directory tmp file + os.Rename with inode verification
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
summary: Save atomicity uses same-directory tmp file + os.Rename with inode verification
updated: '2026-07-14'
---

## Context

decision-point 'atomicity 機構' の 3 option (tmp+rename / direct WriteFile / write+fsync) の中から、reader が中途書き込み状態を観測しない invariant を守るもの、かつ syscall 失敗の 4 partition (success/failure/unknown/inconclusive) を閉じるものが必要。

## Decision

**tmp file (同一ディレクトリ) + os.Rename** を確定し、rename 成功後に inode の size を verify してから 200 を返す。syscall outcome partition は success / failure(ENOSPC 等) / unknown(EIO) / inconclusive(rename ok but size mismatch) の 4 種で閉じ、成功以外は typed 5xx を返し disk を変更しない。fsync は本 ADR では追加しない (crash-durability は現段階の scope 外)。

## Consequences

- reader は常に旧 bytes か新 bytes の完全な形を観測する (contract-write-atomicity)。
- unknown/inconclusive も含めた typed 5xx により silent success が構造的に排除される。
- fsync 未採用のため crash-across-durability は将来 ADR で扱う。

## Alternatives

- **却下: direct os.WriteFile** — 並行 reader が中途書き込みを観測できる非-atomic path。
- **却下: write+fsync のみ** — 同一ディレクトリ tmp+rename が持つ visible-file always fully-old-or-fully-new invariant を単独では満たさない。

## Trace

- Requirements: FR-106
- Implementation contracts: contract-write-atomicity
