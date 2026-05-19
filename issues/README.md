# Issues

[plans/](../plans/) で定めた計画を実行可能な単位に分解した issue 群。

## 表記規約

各 issue ファイルは以下のセクションを持つ:

```markdown
# <ID>: <タイトル>

- **Phase**: P0a / P0b / ... ([plans/04-phases.md](../plans/04-phases.md))
- **Status**: Open / In Progress / Blocked / Done
- **Depends on**: 他 issue ID または PR
- **Blocks**: 他 issue ID

## Background
## Tasks
## Acceptance Criteria
## References
```

## 直近 issue 一覧 (P0 batch)

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [001](001-p0a-physical-move.md) | roost を client/ に、共有を platform/ に物理移動 | P0a | Open | — |
| [002](002-p0b-agentlaunch.md) | agentlaunch を runtime/ から platform/ へ抽出 | P0b | Open | 001 |
| [003](003-p0c-codexclient.md) | codexclient と codexschema を stream/ から platform/agent/ へ抽出 | P0c | Open | 001 |
| [004](004-p0d-cmd-scaffolding.md) | cmd/orchestrator/ と cmd/claude-app-server/ の雛形 + Makefile | P0d | Open | 001 |

## 依存関係グラフ

```
            001 (P0a)
              │
   ┌──────────┼──────────┐
   ▼          ▼          ▼
  002        003        004
 (P0b)      (P0c)      (P0d)
```

- **001 (P0a)** が全てのルート。ディレクトリ構造を確定させる必要があるため
- **002 / 003 / 004** は 001 完了後に並行可能
- 001 は 1 PR で完結させるのが望ましいが規模次第で move-only と import-update に分割可

## 次の batch (P1 以降)

P0 完了後に作成する issue 群:

- P1: WORKFLOW.md loader + wfconfig + preflight + stub scheduler
- P2: Linear adapter + workspace + 4 hooks
- P3: scheduler core (poll/dispatch/retry/reconcile)
- ...

詳細は [plans/04-phases.md](../plans/04-phases.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
