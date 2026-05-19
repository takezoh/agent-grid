# 001: roost を client/ に、共有を platform/ に物理移動

- **Phase**: P0a ([plans/04-phases.md#p0a-物理移動](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: —
- **Blocks**: 002, 003, 004 (および後続全 Phase)

## Background

[plans/02-layout.md](../plans/02-layout.md) のディレクトリ構造に揃える物理移動。**挙動変更ゼロ** が絶対条件。後続全 Phase の前提となるため最優先で実施する。

src/ 直下に並んでいるパッケージを以下に振り分ける:

- 共有基盤 → `src/platform/<同名>/`
- roost 専用 → `src/client/<同名>/`
- バイナリエントリ → `src/cmd/roost/main.go`

## Tasks

### A. ファイル移動

- [ ] `src/main.go` → `src/cmd/roost/main.go`
- [ ] 共有基盤を `src/platform/` 配下に移動:
  - [ ] `src/sandbox/` → `src/platform/sandbox/`
  - [ ] `src/hostexec/` → `src/platform/hostexec/`
  - [ ] `src/mcpproxy/` → `src/platform/mcpproxy/`
  - [ ] `src/lib/pathmap/` → `src/platform/pathmap/`
  - [ ] `src/lib/git/` → `src/platform/lib/git/`
  - [ ] `src/lib/github/` → `src/platform/lib/github/`
  - [ ] `src/lib/claude/` → `src/platform/lib/claude/`
  - [ ] (他 lib/<tool>/ があれば同様)
  - [ ] `src/logger/` → `src/platform/logger/`
  - [ ] `src/features/` → `src/platform/features/`
- [ ] `src/config/` を分割:
  - [ ] 共有部 (SandboxResolver, DataDir 系) → `src/platform/config/`
  - [ ] roost 専用部 (TUI 設定 / driver 設定 / connector 設定 等) → `src/client/config/`
- [ ] roost 専用を `src/client/` 配下に移動:
  - [ ] `src/state/` → `src/client/state/`
  - [ ] `src/runtime/` → `src/client/runtime/`
  - [ ] `src/proto/` → `src/client/proto/`
  - [ ] `src/tui/` → `src/client/tui/`
  - [ ] `src/tools/` → `src/client/tools/`
  - [ ] `src/tmux/` → `src/client/tmux/`
  - [ ] `src/driver/` → `src/client/driver/`
  - [ ] `src/connector/` → `src/client/connector/`
  - [ ] `src/event/` → `src/client/event/`
  - [ ] `src/uiproc/` → `src/client/uiproc/`
  - [ ] `src/cli/` → `src/client/cli/`
  - [ ] `src/procio/` → `src/client/procio/`
  - [ ] `src/winexec/` → `src/client/winexec/`

### B. import path 一括更新

- [ ] 全 `.go` ファイルの import を新パスに更新 (gopls か `goimports` + sed で機械的に)
- [ ] `go.mod` の module path は据え置き、subpackage path のみ書き換え

### C. Makefile / ビルド調整

- [ ] `make build` の対象を `cmd/roost/` に変更
- [ ] `make vet` `make lint` が全 packages を対象に変更
- [ ] `make test` 相当が `go test ./...` で通ることを確認

### D. depguard / lint ルール更新

- [ ] `.golangci.yml` の import boundary を更新:
  - `platform/*` は `client/*` `orchestrator/*` を import 禁止
  - `client/*` は `orchestrator/*` を import 禁止 (逆も)
  - `cmd/<name>/main.go` のみが各層を自由に wiring 可能
- [ ] 既存の `runtime/isolation_test.go` 相当の test を新構造に追随

### E. ドキュメント追随

- [ ] `ARCHITECTURE.md` 内の "Layer Structure" を新構成 (`platform/` `client/`) に書き換え
- [ ] `AGENTS.md` 内の build コマンドが影響を受ければ更新
- [ ] `docs/interfaces.md` 内のパス参照を更新

## Acceptance Criteria

- `make build` `make vet` `make lint` がすべて通る
- `cd src && go test ./...` が緑
- 既存 roost binary の挙動は変わらない (warm start / cold start / palette / IPC 全て)
- import boundary 違反が depguard で検出可能になっている
- `git mv` で rename detection が効き、blame が保たれている

## Notes

- 1 PR で全てやるか、(1) ファイル移動のみ (2) import path 更新 に分けるかは規模次第。**git rename detection を効かせるため、移動と内容変更は分離する**
- 移動時 PR には **他の変更を一切混ぜない** (review 容易性のため)
- 一時的に build が壊れる中間 commit を許す場合は CI を skip するブランチで作業

## References

- [plans/02-layout.md](../plans/02-layout.md) — ターゲットの完成形
- [plans/04-phases.md#p0a-物理移動](../plans/04-phases.md)
- [ARCHITECTURE.md](../ARCHITECTURE.md) — 現状の層構造
