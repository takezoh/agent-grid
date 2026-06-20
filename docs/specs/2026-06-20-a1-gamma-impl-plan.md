# A1-γ Implementation Plan: view-update broadcast(run-state / driver view / tool log)

- **作成日**: 2026-06-20
- **ブランチ**: `feat/tmux-free-web-server`(A1-α + A1-β 完了、`make build-all` 緑、`go test ./... -race` 76/76、TS typecheck/lint/test 49/49)
- **親計画**: [Master Plan(`plans-cheerful-thompson.md`)](../../plans/arc-server-client-split.md)
- **前段**: A1-β で React+TS SPA、Zustand store、socket retry/reconnect が稼働

## Goal

`EvtSessionsChanged.Sessions[].View`(`Card` / `Tags` / `StatusLine` / `Status` / `LogTabs` / `InfoExtras`)を WebSocket の `view-update` frame として React に流し、`RunStateBadge` / `DriverViewPanel` / `LogTabSelector` で render する。`running ↔ idle` 等の driver run-state 遷移と `StatusLine` の elapsed が browser でリアルタイム反映される。

A1-β で既に `wire/server.ts` に `ViewUpdateFrame = {k: 'v'; sessions: SessionInfo[]; ...}` を定義済み(ADR 0021、frontend は前方互換型として用意)。本 γ で **server/web 側の発火経路と React 側の render パスを接続** し、未実装だった driver view の broadcast を完成させる。

## Scope

### In scope
- **`src/client/state/proto_session_info.go` 周辺**(既存): `state.SessionInfo` に `View` field が含まれているか確認、無ければ追加(`view.View` を `omitempty` で)。
- **`src/client/runtime/proto_bridge.go`**(修正): `EvtSessionsChanged` を送出する際、各 Session の現在の View(`state.Sessions[sid].View` または `driver.View()` の結果)を含めて送る。
- **`src/server/web/gateway.go`**(修正): daemon から受信した `EvtSessionsChanged` を WebSocket に `view-update` frame として変換・送出。`subscribeLifecycle` で `CmdSubscribe(filters=['sessions-changed'])` を発行し、初回フレームを `hello` frame として、以降を `view-update` として送る。
- **`src/server/web/wire.go`**(修正): `EvtSessionsChanged` → `viewUpdateFrame{k:'v', sessions, activeSessionID, ...}` のエンコード関数追加。
- **`src/client/web/src/store/daemon.ts`**(修正): `applyViewUpdate` を `sessions[]` 受け入れに拡張(現状の sessions 状態を新 `View` 込みで置き換え)。
- **`src/client/web/src/wire/server.ts`**(修正): `SessionInfo` 型に `view?: View` field を追加、`View` 型を新規定義(`card`, `status`, `status_line`, `log_tabs`, `info_extras` 等を Go の `view.View` と整合)。
- **`src/client/web/src/components/RunStateBadge.tsx`**(新設): `Status` 値(`running` / `idle` / `error` / …)を色付き badge で表示。
- **`src/client/web/src/components/DriverViewPanel.tsx`**(新設): `Card` の title / subtitle / tags / border 系を 1 つのパネルに描画。`StatusLine` を elapsed 付き(秒進行は 1s タイマー)で footer に。
- **`src/client/web/src/components/LogTabSelector.tsx`**(新設): `LogTabs` を tab 列として表示。タブ選択時の content 表示は δ スコープ(本 γ は selector のみ)。
- **`src/client/web/src/App.tsx`**(修正): `SessionList` + `TerminalPane` の隣に `DriverViewPanel` と `LogTabSelector` を組み込む。
- **テスト**:
  - `src/server/web/wire_test.go` に `EvtSessionsChanged → viewUpdateFrame` の round-trip 追加。
  - `src/server/web/gateway_terminal_test.go` または新 `gateway_view_update_test.go` で broadcast 経路試験。
  - `src/client/web/src/store/daemon.test.ts` に `applyViewUpdate(view あり)` ケース追加。
  - `src/client/web/src/wire/codec.test.ts` に `ViewUpdateFrame(View 込み)` round-trip 追加。
  - 各新コンポーネントに testing-library test。

### Deletion(本 PR では無し)
- 既存 vanilla JS UI は β で削除済み。
- view-update broadcast の dead-code 経路があれば整理(α/β の足跡を確認)。

### Out of scope
- Log tab の内容表示(δ で transcript-tail / event-log-tail を実装)。
- Connector view の表示(δ)。
- 永続化(`transcript` / `event-log` ファイル読み取り、δ)。
- driver の新規追加 / `View()` の実装変更(本 γ は既存 driver の output を素通しするだけ)。
- React Concurrent Features の活用拡張(本 γ は state update のみ)。
- `EvtAgentNotification`(α で実装済み)の改修。
- `prompt-event` の driver 発火(本 γ は経路のみ、δ 以降で接続)。

## EARS Requirements

| ID | Type | Statement | Rationale |
|---|---|---|---|
| **FR-γ01** | event_driven | daemon が `EvtSessionsChanged` を発火したとき、システムは各 Session の `View` を含めて WebSocket subscriber に `view-update` frame(`{k:'v', sessions:[{id, view, ...}], activeSessionID, ...}`)を送出しなければならない | core view-update 流路 |
| **FR-γ02** | ubiquitous | システムは `view.View` 構造(Card / DisplayName / LogTabs / InfoExtras / SuppressInfo / StatusLine / Status / StatusChangedAt)を JSON で wire に流し、TypeScript 側の `View` 型と1対1で対応させなければならない | 型 / wire 整合 |
| **FR-γ03** | event_driven | React 側が `view-update` frame を受信したとき、システムは Zustand store の `sessions[]` を新 view 込みで更新し、影響を受けるコンポーネントだけが再描画されなければならない | 部分再描画(parts of FR-β07) |
| **FR-γ04** | ubiquitous | `RunStateBadge` は `View.Status` 値(`running` / `idle` / `error` 等の文字列定数)を color-coded badge で表示しなければならない | UX |
| **FR-γ05** | ubiquitous | `DriverViewPanel` は `Card.Title` / `Subtitle` / `Tags` / `BorderTitle` / `BorderTitleSecondary` / `BorderBadge` を視認可能なレイアウトで描画しなければならない | UX |
| **FR-γ06** | ubiquitous | `StatusLine` を表示する箇所では、`StatusChangedAt` からの elapsed を 1 秒刻みで更新表示しなければならない(`x sec ago` または `running for x` 形式) | TUI と同等の elapsed 表示 |
| **FR-γ07** | ubiquitous | `LogTabSelector` は `LogTabs[]` の各 entry を tab として並べ、初期選択は最初の tab。tab 内容表示は本 γ スコープ外なので "(coming in δ)" placeholder で良い | δ 拡張点の明示 |
| **FR-γ08** | unwanted | もし `View.SuppressInfo == true` なら、システムは `InfoExtras` / `LogTabs` の表示を抑制しなければならない | spec 通り(driver 都合) |
| **FR-γ09** | ubiquitous | wire / store / コンポーネントの各層に round-trip / reducer / render テストが追加され、`go test ./... -race` + `npm run test` が緑でなければならない | 三層テストゲート(β を継承) |
| **FR-γ10** | ubiquitous | 既存 `subscribeLifecycle` の流路は変更せず、`CmdSurfaceSubscribe` で取得していた asciicast 流路と `view-update` 流路は同じ WebSocket 上で衝突しないように multiplex されなければならない | β の wire 互換維持 |

## ADR(本 γ で追加)

| ID | Title | Status |
|---|---|---|
| [ADR 0023](../adr/0023-view-update-broadcast-shape.md) | view-update broadcast は `EvtSessionsChanged` を 1:1 で WebSocket frame に変換する | accepted |
| [ADR 0024](../adr/0024-elapsed-clock-in-driver-view-panel.md) | StatusLine elapsed は client-side 1Hz タイマで更新(daemon push を 1Hz に増やさない) | accepted |

## Verification

```sh
# Go
cd src && go test ./... -race -count=1
cd src && go vet ./...
cd src && go tool golangci-lint run ./...

# Frontend
cd src/client/web && npm ci
cd src/client/web && npm run typecheck
cd src/client/web && npm run lint
cd src/client/web && npx vitest --run
cd src/client/web && npm run build

# Full
make build-all

# Manual smoke
make run-dev
# 別ターミナルで arc daemon + claude/codex session を作る
# browser: http://127.0.0.1:8080/#token=...
# (1) running → idle 遷移が badge に反映、(2) StatusLine の elapsed が秒進行、
# (3) LogTabSelector に tab が並ぶ(内容は placeholder)、
# (4) Card の title / subtitle / tags が表示される
```

## Open Questions(実装直前に決める、minor)

1. proto wire の `EvtSessionsChanged` に既に `View` が含まれているか実装直前に確認。無ければ追加(`SessionInfo.View *view.View` のような pointer + `omitempty`)。
2. tab 選択状態は store に保持するか component local state か。本 γ では component local で開始、γ 後半で store 移行する余地。
3. elapsed 計算は `StatusChangedAt`(absolute time)を使うが、daemon と client の時刻ずれ補正(`hello.serverTime` ベース)をどこまで真剣にやるか。本 γ は naive(client wall clock)で開始。

## Traceability

- **親 plan**: Master Plan(`plans-cheerful-thompson.md`)
- **前段**: A1-α(wire + gateway)、A1-β(React+TS + Zustand + retry)
- **次の作業**: A1-δ(persist + connector)→ A1-ε(cleanup)→ C(tmux 実装削除)
