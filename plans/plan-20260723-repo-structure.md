# リポジトリ構造整理 (native clients / multi-host gateway 見越し)

- **作成日**: 2026-07-23
- **ブランチ**: `claude/native-clients-plan-review-2sjfs5`
- **ステータス**: draft (M0 のみ実施済み、M1/M2 は未着手)
- **関連**: [plan-20260723-native-clients.md](./plan-20260723-native-clients.md) / [plan-20260723-windows-shell-design.md](./plan-20260723-windows-shell-design.md) / [multi-host-gateway.md](./multi-host-gateway.md)

## 1. 動機

これから増えるものが、現在の構造に置き場所を持たない:

1. **ユーザー向けクライアント群** (windows-shell C#、workspace Electron、apple、android)。非 Go であり `src/` (Go module) の中に置くべきでない。
2. **契約層** (`protocol/` スキーマ + `contracts/` 振る舞い契約) — native-clients plan Phase 0-1 の成果物。
3. **multi-host gateway** — 新規 `gateway/` layer + `cmd/gateway/` + `platform/transport/` (multi-host plan §冒頭)。

さらに既存の名前が新概念と衝突する:

- `src/client/` は**デーモン**(実行プレーン)だが、これから作るのは**ユーザー向け client**。同じ語が逆の意味で使われる。multi-host plan は既にこの実体を **Host** と呼んでいる (「Host: 1 物理 PC ≒ 1 server プロセス」)。
- 現在の `src/server/` は **host-local の HTTP/WS façade** だが、multi-host plan の **Gateway は公開網の relay/tunnel/authorizer** で別物。`gateway` の名を先に予約しておかないと衝突する。

## 2. 目標構造

```text
agent-grid/
├── src/                      # Go module = 実行プレーン (Host 側) — 核は不変
│   ├── platform/             #   共有基盤 (multi-host M3 で transport/ が増える予定地)
│   ├── host/                 # ← 旧 client/ (M2 で rename): state/runtime/driver/proto/sandbox
│   │   └── webhost/          #   ← 旧 client/web の Go 側 (SPA embed + Handler) だけ残す
│   ├── server/               #   host-local HTTP/WS façade (名前維持)
│   ├── gateway/              #   (予約: multi-host plan の公開 relay/tunnel/authorizer。中身は M3)
│   ├── orchestrator/
│   ├── internal/ · gorules/
│   └── cmd/                  #   server / web / bridge / orchestrator / claude-app-server / (将来 gateway)
│
├── clients/                  # ユーザー向けクライアント (非 Go 中心; 各自のツールチェーン)
│   ├── web/                  # ← 旧 src/client/web の npm プロジェクト (M1 で移設)
│   ├── windows-shell/        # C# / WinUI 3 (Phase 2 S1 で着工)
│   ├── workspace/            # Electron / TS (Phase 2 S4 で着工)
│   ├── apple/                # Swift package + iOS/macOS (Phase 3/4)
│   └── android/              # (Phase 4/5)
│
├── protocol/                 # スキーマ正本 (openapi.yaml, events/commands/… .schema.json)
├── contracts/                # 振る舞い契約 (approval-contract.md, reconnect-contract.md, …)
│
├── docs/ · plans/ · scripts/ · deploy/ · test-harness/ · playground/
```

原則:

- **`src/` = Go module = 実行プレーン**に純化する。ユーザー向けクライアントは言語を問わず `clients/` 配下で、各自のビルド/テストツールチェーンを持つ。
- **`protocol/` と `contracts/` が正本**。生成コードは各消費者の木に置く (Go は `src/` 内、TS は `clients/web/` 内、C# は `clients/windows-shell/` 内、…)。正本→生成の一方向のみ。
- **名前の予約**: `gateway` = 公開 relay (multi-host)、`host` = 実行プレーンのデーモン層、`clients` = ユーザー向け。docs/コード/プラン全部でこの語彙に揃える。

## 3. 移行ステージ

一括移行はしない。各ステージは独立に green (test/lint/CI) で完結する。

### M0 — 置き場所の確保 (本ブランチで実施済み)

`clients/` `protocol/` `contracts/` を README 付きで新設。所有権と「何を置くか/置かないか」を README に明記。コード移動なし、既存への影響ゼロ。

### M1 — SPA の移設: `src/client/web` → `clients/web`

`src/client/web` は **npm プロジェクトと Go パッケージの二重身分**である点に注意:
`cmd/web` バイナリが `client/web` パッケージ (`clientweb.Handler`) 経由で**ビルド済み SPA を go:embed で配信し、/api と /ws を cmd/server へリバースプロキシ**している (CSP と WS origin check をこの origin で成立させるため)。

分割方針:

- **npm プロジェクト** (src/, e2e/, package.json, vite/playwright 設定) → `clients/web/` へ移動。
- **Go 側** (embed + `Handler` + proxy) → `src/host/webhost/` (M2 前は `src/client/webhost/`) として残す。ビルドは「`clients/web` で `npm run build` → dist を webhost の embed 対象へコピー」を Makefile が仲介 (現行 `build-web-frontend` の出力先変更)。

機械的更新が必要な参照 (棚卸し済み、~40 ファイル):

- `Makefile` (`WEB_DIR := src/client/web`)、`.github/workflows/ci.yml` (cache-dependency-path ×3)、`e2e-nightly.yml`
- `scripts/run-dev.sh` `run-mutation-pilot.sh` `repeat-changed-tests.sh` `coverage-floors.txt`
- `test-harness/*.json` (profiles/protected/skips/mutants のパス)
- `src/gorules/static_enforcement_test.go`、`src/internal/harnesspolicy/*_test.go` のパス前提
- docs (AGENTS.md のコマンド例、design-client.md、note-*)

Gate: `npm run test:web` + `go test ./...` + Playwright e2e + CI green。

### M2 — Rename: `src/client/` → `src/host/`

multi-host plan の語彙 (Host) と一致させ、「client = ユーザー向け」の衝突を解消する。native-clients plan Phase 0 の terminology 項の実装でもある。

- import path 一括書換 (`github.com/takezoh/agent-grid/client/…` → `…/host/…`)、パッケージ名は原則維持 (`state`, `runtime`, `proto`, …) なので参照式は不変。
- `src/.golangci.yml` depguard ルール改名 (`client-no-orchestrator` → `host-no-orchestrator` 等) とパス更新。forbidigo の対象パス (`client/state`) も同時に。
- ARCHITECTURE.md / AGENTS.md / docs/design/design-client.md (→ design-host.md) の語彙更新。ADR は歴史文書として書き換えない (冒頭に用語注記を 1 行足すのみ)。
- Gate: `go test ./...` + `make lint` + `make test-e2e` + CI green。diff は巨大だが全て機械的。**M1 完了後に実施** (SPA を巻き込まないため)。

### M3 — multi-host / 契約層の着地 (該当プラン側で実施)

- multi-host plan 着工時: `src/gateway/` + `src/cmd/gateway/` + `src/platform/transport/` を予約名どおりに新設。depguard に `gateway-layer` ルール追加 (gateway は platform のみ import 可、no-domain 原則の機械的裏付け)。
- native-clients plan Phase 1 着工時: `protocol/` にスキーマ、`contracts/` に契約文書、生成物を各消費者へ。
- Phase 2 着工時: `clients/windows-shell/` `clients/workspace/` に実体。

## 4. やらないこと

- `src/server/` の改名 — host-local façade として実体と名前が一致しており、公開 relay と衝突しない (`gateway` を予約したため)。
- `orchestrator/` `platform/` の再配置 — 境界は健全で動機がない。
- モノレポツール (nx/turbo 等) の導入 — clients/ 配下は各自のツールチェーンで独立ビルドし、統合は Makefile ターゲットと CI job 分割で足りる。必要が実証されてから。
- ADR の遡及書き換え。

## 5. リスク

- **M1 の embed 経路切断**: dist コピーの仲介を Makefile に入れ忘れると `cmd/web` が古い UI を配信し続ける。webhost に「dist の build hash を /healthz に出す」を足して検知可能にする。
- **M2 の open PR 衝突**: rename は全ファイルに触るため、進行中ブランチと衝突する。実施タイミングは open PR が捌けた瞬間を選び、1 PR で完結させる。
- **git 履歴追跡**: `git log --follow` で追えるよう、M1/M2 とも純粋な move commit (内容変更なし) と参照更新 commit を分ける。
