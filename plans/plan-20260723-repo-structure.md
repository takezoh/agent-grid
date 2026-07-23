# リポジトリ構造整理 (native clients / multi-host gateway 見越し)

- **作成日**: 2026-07-23
- **更新日**: 2026-07-23 (rev 2 — 命名提案を採用して M0/M1 相当を実行済みに更新。`server/api` / `clients/ui` / `src/uihost` 確定)
- **ブランチ**: `claude/native-clients-plan-review-2sjfs5`
- **ステータス**: M0 + 命名修正 (M1 相当) 実施済み / M2 (client→host rename) 未着手
- **関連**: [plan-20260723-native-clients.md](./plan-20260723-native-clients.md) / [plan-20260723-windows-shell-design.md](./plan-20260723-windows-shell-design.md) / [multi-host-gateway.md](./multi-host-gateway.md)

## 1. 動機

これから増えるものが、従来の構造に置き場所を持たなかった:

1. **ユーザー向けクライアント群** (windows-shell C#、workspace Electron、apple、android)。非 Go であり `src/` (Go module) の中に置くべきでない。
2. **契約層** (`protocol/` スキーマ + `contracts/` 振る舞い契約) — native-clients plan Phase 0-1 の成果物。
3. **multi-host gateway** — 新規 `gateway/` layer + `cmd/gateway/` + `platform/transport/` (multi-host plan §冒頭)。

さらに既存の名前が実体とずれていた:

- 「**web**」1 語に 3 つの別物が詰まっていた: SPA 本体 / ブラウザ配信 (`cmd/web`) / gateway (`server/web`)。Electron workspace の導入で SPA はブラウザ専用でなくなり、「web = SPA の名前」は不正確になった。
- `server/web` は**全クライアントが使う API façade** であってブラウザ専用面ではない。
- `src/client/` は**デーモン**(実行プレーン)だが、これから作るのは**ユーザー向け client**。multi-host plan はこの実体を **Host** と呼んでいる。
- 公開網 relay の名前 `gateway` を予約しておかないと将来衝突する。

## 2. 語彙 (確定)

| 語 | 意味 | 実体 |
|---|---|---|
| `ui` | 共有 SPA (React/TS/xterm.js)。単体ではクライアントでなく、全 GUI クライアントが使う UI 資産 | `clients/ui/` |
| `uihost` | ui の配信機 (Go: dist embed + /api,/ws リバースプロキシ)。ブラウザと Electron workspace の両方がこの origin をロードする | `src/uihost/` + `src/cmd/uihost/` (成果物バイナリ名・systemd unit は歴史的に `web` / `agent-grid-web` を維持) |
| ブラウザクライアント | `uihost` の origin をブラウザで直接開く**配信モード**。固有コードを持たないため専用ディレクトリなし | — |
| `api` (server 層内) | 全クライアント向け HTTP/WS gateway (旧 `server/web`) | `src/server/api/` |
| `gateway` (最上位・予約) | multi-host plan の公開 relay/tunnel/authorizer | `src/gateway/` (未作成・予約) |
| `host` | 実行プレーンのデーモン層 (旧 `client/`、M2 で rename 予定) | `src/host/` (予定) |
| `clients` | ユーザー向けクライアント群 | `clients/` |

## 3. 現在の構造 (M0 + 命名修正 実施後)

```text
agent-grid/
├── src/                      # Go module = 実行プレーン — 核は不変
│   ├── platform/             #   共有基盤 (multi-host M3 で transport/ が増える予定地)
│   ├── client/               #   デーモン層 (M2 で host/ に rename 予定)
│   ├── server/api/           #   ✅ 全クライアント向け HTTP/WS gateway (旧 server/web)
│   ├── uihost/               #   ✅ 共有 UI の配信機 (旧 client/web の Go 側: embed + Handler)
│   ├── cmd/uihost/           #   ✅ 旧 cmd/web
│   ├── gateway/              #   (予約: 公開 relay。中身は M3)
│   ├── orchestrator/ · internal/ · gorules/ · cmd/
│
├── clients/                  # ユーザー向けクライアント (非 Go 中心)
│   ├── ui/                   #   ✅ 共有 SPA (旧 src/client/web の npm プロジェクト)
│   ├── windows-shell/        #   C# / WinUI 3 (Phase 2 S1 で着工)
│   ├── workspace/            #   Electron / TS (Phase 2 S4 で着工)
│   ├── apple/ · android/     #   (Phase 3+)
│
├── protocol/                 # ✅ スキーマ正本 (中身は Phase 1)
├── contracts/                # ✅ 振る舞い契約の正本 (中身は Phase 0-1)
└── docs/ · plans/ · scripts/ · deploy/ · test-harness/ · playground/
```

ビルド配線: `clients/ui` で `npm run build` → Makefile (`build-web-frontend`) が dist を `src/uihost/dist/` へ同期 → `//go:embed all:dist`。`src/uihost/dist/.gitkeep` が未ビルド tree のコンパイルを担保する。

原則:

- **`src/` = Go module = 実行プレーン**に純化する。ユーザー向けクライアントは言語を問わず `clients/` 配下で、各自のビルド/テストツールチェーンを持つ。
- **`protocol/` と `contracts/` が正本**。生成コードは各消費者の木に置く (Go は `src/` 内、TS は `clients/ui/` 内、C# は `clients/windows-shell/` 内、…)。正本→生成の一方向のみ。
- Go コードを `clients/` に出さない (単一 module の depguard/forbidigo 強制の中に留める)。`uihost` が `src/` にあるのはこのため。

## 4. 実施記録

### 実施済み (本ブランチ)

1. **M0**: `clients/` `protocol/` `contracts/` を所有権 README 付きで新設。
2. **`src/server/web` → `src/server/api`** (package `web` → `api`)。参照更新: `cmd/server`、`.golangci.yml`、test-harness、coverage floors。
3. **SPA 移設**: npm プロジェクトを `clients/ui/` へ、Go 側 (embed + `Handler` + proxy) を `src/uihost/` へ分割。`cmd/web` → `cmd/uihost`。参照更新: Makefile (dist 同期ステップ新設)、CI workflows、scripts、test-harness、`wire_fixtures_test` の出力先、gorules/harnesspolicy テスト、生きているドキュメント一式。**成果物バイナリ名 `web` と systemd unit 名 `agent-grid-web` は維持** (稼働環境と deploy/ を壊さない。ops 語彙の改名は配布開始時の課題)。
4. **強制機構の修理 (棚卸しでの発見)**: `.golangci.yml` の depguard/forbidigo が旧モジュールパス `agent-reactor` を参照しており、**層境界強制の大半が無効化されていた**。`agent-grid` へ修正して復活。復活で表面化した違反 2 件を処置:
   - `client/runtime/workspace_root.go` — driver import を消費側ローカルの structural interface に置換 (正攻法で解消)。
   - `client/runtime/frame_messaging_store.go` — driver 名定数の参照。能力ベース判定への置換が本筋のため、**文書化した逸脱**として単一ファイル exclusion を追加 (弱体化ではなく明示)。
   - あわせて no-mutex forbidigo をテストのフェイク実装に適用しない exclusion を追加 (invariant は本番コアの production code が対象)。

### M2 — Rename: `src/client/` → `src/host/` (未着手)

multi-host plan の語彙 (Host) と一致させ、「client = ユーザー向け」の衝突を解消する。

- import path 一括書換 (`github.com/takezoh/agent-grid/client/…` → `…/host/…`)。パッケージ名は原則維持 (`state`, `runtime`, `proto`, …)。
- `src/.golangci.yml` のルール名 (`client-no-orchestrator` → `host-no-orchestrator` 等) とパス更新。
- ARCHITECTURE.md / AGENTS.md / docs/design/design-client.md (→ design-host.md) の語彙更新。ADR・docs/changes は歴史文書として書き換えない。
- Gate: `go test ./...` + `make lint` + `make test-e2e` + CI green。diff は巨大だが全て機械的。**open PR が捌けたタイミングで単独 PR として実施**。

### M3 — multi-host / 契約層の着地 (該当プラン側で実施)

- multi-host plan 着工時: `src/gateway/` + `src/cmd/gateway/` + `src/platform/transport/` を予約名どおりに新設。depguard に `gateway-layer` ルール追加 (gateway は platform のみ import 可、no-domain 原則の機械的裏付け)。
- native-clients plan Phase 1 着工時: `protocol/` にスキーマ、`contracts/` に契約文書、生成物を各消費者へ。
- Phase 2 着工時: `clients/windows-shell/` `clients/workspace/` に実体。

## 5. やらないこと

- `src/server/` 層自体の改名 — host-local façade として実体と一致 (`api` subpackage への改名で十分)。
- `orchestrator/` `platform/` の再配置 — 境界は健全で動機がない。
- 成果物バイナリ / systemd unit の改名 (`web` / `agent-grid-web`) — 稼働環境に対する破壊的変更。配布開始時に扱う。
- モノレポツール (nx/turbo 等) の導入 — clients/ 配下は各自のツールチェーンで独立ビルド。必要が実証されてから。
- ADR・docs/changes・docs/specs の遡及書き換え (歴史文書)。

## 6. リスク

- **M2 の open PR 衝突**: rename は全ファイルに触れる。実施タイミングは open PR が捌けた瞬間を選び、1 PR で完結させる。
- **dist 同期の入れ忘れ**: `build-web-frontend` を経ずに `go build` すると古い UI を配信し続ける。uihost に「dist の build hash を /healthz に出す」改善余地あり (未実施)。
- **git 履歴追跡**: 実施済みの移動は git rename 検出が効く形 (move + 参照更新を同一コミットに同居させたが、`git log --follow` は機能する)。M2 も同様に。
