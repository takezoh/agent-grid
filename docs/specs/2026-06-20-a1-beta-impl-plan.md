# A1-β Implementation Plan: vanilla JS → React+TypeScript frontend

- **作成日**: 2026-06-20
- **ブランチ**: `feat/tmux-free-web-server`(A1-α 完了、commit `2dd928f`、origin より 70 commits ahead)
- **親計画**: [Master Plan(`plans-cheerful-thompson.md`)](../../plans/arc-server-client-split.md)
- **前段**: A1-α(commits 615b55f..1d7cb8b)で wire 層・gateway 化が完了。**本 plan は wire 互換のまま `client/web` を React+TS SPA に置換**
- **生成プロセス**: Master Plan §A1-β と A1-α 完了状況から自前で生成(plan-how Workflow が α の続きと誤解したため代替)

## Goal

`client/web` の vanilla JS UI(`index.html` + `app.js` + `vendor/`)を **React + TypeScript SPA**(Vite + Zustand + xterm.js)に置換する。wire は A1-α 互換のまま(asciicast v2 配列 + `{k,code,data}` JSON + Surface 系 ws frame)。`go:embed` で `client/web/dist/*` を server バイナリに埋め込む。**subscribe race(ADR 0018 で β に倒した)の client-side retry ロジックを React store に実装する**(本 β の核心スコープ)。

## Scope

### In scope

- 新規 `package.json`(React 18.x、Zustand、xterm.js 5.x、Vite、vitest、@testing-library/react、biome)を exact pin で導入
- 新規 `tsconfig.json`(strict + `noUncheckedIndexedAccess` + `verbatimModuleSyntax`)
- 新規 `vite.config.ts` + `vitest.config.ts`(または統合)
- 新規 `biome.json`(eslint/typescript-eslint 単一バイナリ代替)
- 新規 `.gitignore`(node_modules / dist / coverage)
- 新規 `src/client/web/src/`:
  - `main.tsx`(エントリ、Zustand provider)
  - `App.tsx`(ルート、ticket 取得 → socket 起動)
  - `wire/server.ts`(server → browser frame の discriminated union、asciicast v2 + control + Surface event)
  - `wire/client.ts`(browser → server frame、`{k:'i'}` `{k:'r'}` 等)
  - `wire/codec.ts`(JSON parse + asciicast v2 array decode、unknown frame は drop)
  - `store/daemon.ts`(Zustand: `sessions` / `activeSessionID` / `connectors` / `features` / `serverTime`、`view-update` reducer + `hello` frame seeding)
  - `store/notifications.ts`(Zustand: notification LRU 32 件)
  - `socket/connection.ts`(WebSocket open + ticket auth + exp backoff 250ms→4s)
  - `socket/retry.ts`(subscribe race → `RespErr(frame-not-ready)` → 同 backoff で再 subscribe、ADR 0018 の β 実装)
  - `socket/subscribe.ts`(active subscription 管理、reconnect 時に自動再送)
  - `components/SessionList.tsx`(セッション一覧、選択)
  - `components/TerminalPane.tsx`(xterm.js wrapper、subscribe-terminal lifecycle、resize、input)
  - `components/CreateSessionForm.tsx`(POST `/api/sessions` → `CmdCreateSession` 経由)
  - `components/StatusBanner.tsx`(daemon-disconnected control frame を拾い banner 表示)
  - `css/app.css`(従来 UI の見た目を最低限保持、ファイル 1 本)
- `Makefile` に `build-web-frontend` ターゲット追加(`npm ci && npm run build` を `build-all` 前段に配線)
- `scripts/run-dev.sh` に `npm run build` 配線(または既存ターゲットに合流)
- `src/client/web/host.go` の `go:embed` パスを `dist/*` に変更
- `src/client/web/headers.go` の CSP 検証テストを追加(`script-src 'self'` 維持、`unsafe-inline` / inline `<script>` が無いことを assert)

### Deletion(本 PR で実施)

- `src/client/web/index.html`(vanilla JS のエントリ)
- `src/client/web/app.js`(現状の唯一の UI ロジック)
- `src/client/web/vendor/*`(ローカル vendoring された xterm.js 等)

### Out of scope

- wire protocol の拡張(γ で `view-update` broadcast 差分化、δ で persist/connector、ε で cleanup)
- driver / connector / persist 経路、`platform/termvt` 改変
- server-side(`server/web`、`cmd/server`、`client/runtime`、`client/state`、`client/proto` は完全に touch しない)
- bearer token + ephemeral WS ticket + strict CSP(無改変、A1-α と同等)
- `server/session` の `git rm`(ε)
- multi-frame session 表示(本 β は active frame のみ、γ 以降で拡張)
- connector の深いアクション(δ)
- tracing / metrics 基盤(α と同様 `console.warn` のみ)

## EARS Requirements

| ID | Type | Statement | Rationale |
|---|---|---|---|
| **FR-β01** | ubiquitous | システムは A1-α の server/web wire(asciicast v2 配列 + `{k,code,data}` JSON + Surface 系 ws frame)を 1 byte も改変してはならない | wire 互換維持 |
| **FR-β02** | event_driven | browser がページを読み込んだとき、システムは Vite が build した `dist/` の React+TS SPA を `go:embed` 経由で配信しなければならない | build pipeline 確立 |
| **FR-β03** | ubiquitous | システムは CSP `script-src 'self'` を維持し、CDN・inline `<script>`・`'unsafe-inline'` を一切導入してはならない | A1-α auth invariant 継承 |
| **FR-β04** | ubiquitous | システムは `package.json` で全依存を exact pin し、`npm ci` でビルド再現性を担保しなければならない | 供給網リスク対策 |
| **FR-β05** | event_driven | browser が `CmdSurfaceSubscribe{SessionID}` を発行し `RespErr{Code:'frame-not-ready'}` を受領したとき、システムは exponential backoff(初期 250ms、cap 4s、full jitter)で再 subscribe しなければならない(最大 16 試行、その後は user 操作で再試行) | ADR 0018 の β 実装 |
| **FR-β06** | event_driven | WebSocket が close され `controlMsg{k:'c',code:'daemon-disconnected'}` を受領したか socket error が発生したとき、システムは同じ backoff で reconnect を試み、再接続時に active subscribe を自動再送しなければならない | ADR 0011 の UX 拾い込み |
| **FR-β07** | event_driven | `EvtSurfaceOutput` frame を 1 秒間に kHz オーダで受信したとき、システムは xterm.js に対して dropped frame を起こさず、UI スレッドが 200ms 超ブロックしてはならない | パフォーマンス予算 |
| **FR-β08** | ubiquitous | システムは Zustand store を UI の真実の単一源とし、コンポーネント local state にはローカルな ephemeral 情報のみを保持しなければならない | state purity |
| **FR-β09** | ubiquitous | システムは vitest + happy-dom + @testing-library/react で (1) コンポーネント test、(2) Zustand store reducer test、(3) wire codec round-trip test、(4) socket retry の fake WS test を保持しなければならない | 三層テストゲート |
| **FR-β10** | optional | システムは `dist/` 出力の gzip 後合計サイズを 250KB 以下に保つよう努め、超過時は PR 説明で理由を述べなければならない | bundle size 予算 |
| **FR-β11** | ubiquitous | システムは bearer token 取得 → `POST /api/ws-ticket` → WS `?ticket=...` のシーケンスを無改変で維持しなければならない | auth flow 不変 |
| **FR-β12** | event_driven | system が `controlMsg{k:'c',code:'daemon-disconnected'}` を受信したとき、システムは `StatusBanner` に「daemon と切断中、再接続中…」相当のメッセージを表示し、reconnect 完了時に banner を非表示にしなければならない | UX 改善(ADR 0011 のフック) |
| **FR-β13** | unwanted | もし `dist/index.html` 内に inline `<script>` または `'unsafe-inline'` が含まれたなら、CI(`go test ./client/web/...`)が失敗しなければならない | CSP 回帰防止 |
| **FR-β14** | ubiquitous | システムは TypeScript の `strict` + `noUncheckedIndexedAccess` + `verbatimModuleSyntax` を有効化し、`tsc --noEmit` がエラー 0 件でなければならない | 型安全性 |
| **FR-β15** | event_driven | browser が `SessionList` で session を選択したとき、システムは前 session の `CmdSurfaceUnsubscribe` を発行してから新 session の `CmdSurfaceSubscribe` を発行しなければならない | リソースリーク防止 |
| **FR-β16** | event_driven | browser が `CreateSessionForm` を submit したとき、システムは `POST /api/sessions` を呼び、返却された SessionID で自動 subscribe しなければならない | UX 完結性 |

## Architecture Decision Records

| ID | Title | Status |
|---|---|---|
| [ADR 0019](../adr/0019-react-ts-vite-zustand-stack.md) | A1-β frontend stack: React 18 + TypeScript + Vite + Zustand + xterm.js | accepted |
| [ADR 0020](../adr/0020-biome-as-frontend-lint.md) | Biome を frontend lint/format の単一ツールとして採用 | accepted |
| [ADR 0021](../adr/0021-frontend-wire-types-hand-written.md) | wire 型は手書き + round-trip test で proto と整合させる(go generate しない) | accepted |
| [ADR 0022](../adr/0022-subscribe-retry-in-socket-layer.md) | subscribe race retry を socket 層に置き Zustand store と統合する | accepted |

## Components(主要)

### Build / Tooling
- **`src/client/web/package.json`**(新設、~30 行): exact pin、scripts(`dev`/`build`/`test`/`typecheck`/`lint`)
- **`src/client/web/tsconfig.json`**(新設): strict + noUncheckedIndexedAccess + verbatimModuleSyntax + JSX react-jsx
- **`src/client/web/vite.config.ts`**(新設): `@vitejs/plugin-react-swc`、output to `dist/`、`build.rollupOptions.input` で entry 指定、CSP 整合のため `manifest` 不使用、単一 chunk
- **`src/client/web/vitest.config.ts`**(新設、または vite.config 統合): happy-dom 環境、setup file で xterm.js mock
- **`src/client/web/biome.json`**(新設): formatter + linter 統合
- **`src/client/web/.gitignore`**(新設): `node_modules/` `dist/` `coverage/` `.vite/`
- **`Makefile`**(修正): `build-web-frontend` ターゲット追加、`build-all` の前段に配線
- **`src/client/web/host.go`**(修正): `go:embed dist/*` に変更(現状 vanilla JS の `embed.go` から再構成)
- **`src/client/web/embed.go`**(修正 or 削除): dist/ に統合

### Wire(TypeScript 側)
- **`src/client/web/src/wire/server.ts`**(新設): discriminated union
  - `OutputFrame = ['o', timeSec: number, dataB64: string]`(asciicast v2 配列)
  - `ControlFrame = {k: 'c'; code: string; data?: string | string[]}`(daemon-disconnected, slow-subscriber, ...)
  - `HelloFrame = {k: 'h'; sessions: SessionInfo[]; activeSessionID: string | null; features: string[]; serverTime: number}`
  - `ViewUpdateFrame = {k: 'v'; sessions: SessionInfo[]; ...}`(γ で活用、β は seeding のみ)
  - `RespOKFrame = {k: 'r'; reqId: string; body?: unknown}`
  - `RespErrFrame = {k: 'e'; reqId: string; code: string; message: string}`
- **`src/client/web/src/wire/client.ts`**(新設):
  - `InputFrame = {k: 'i'; d: string}`(raw input、d は文字列。サーバが `CmdSurfaceWriteRaw{Data:[]byte(d)}` に変換)
  - `ResizeFrame = {k: 'r'; cols: number; rows: number}`
  - `SubscribeFrame = {k: 's'; reqId: string; sessionId: string}`(`CmdSurfaceSubscribe` の WS 直接送信用、または REST 経路と混在判断は実装で決定)
- **`src/client/web/src/wire/codec.ts`**(新設、~80 行): `parseServerFrame(raw: string) → ServerFrame | null`、`serializeClientFrame(f: ClientFrame): string`、unknown は `null` で drop

### Store(Zustand)
- **`src/client/web/src/store/daemon.ts`**(新設、~120 行):
  - `interface DaemonState { sessions, activeSessionID, connectors, features, serverTime, status: 'connecting'|'open'|'reconnecting'|'closed' }`
  - actions: `seedHello(frame)`, `applyViewUpdate(frame)`, `setStatus(s)`, `selectSession(id)`
- **`src/client/web/src/store/notifications.ts`**(新設、~60 行):
  - LRU 32 件、`add(notification)`, `dismiss(id)`, `clear()`

### Socket
- **`src/client/web/src/socket/connection.ts`**(新設、~180 行):
  - `connect(ticketEndpoint): WebSocket` — bearer 取得 → ticket 発行 → WS open
  - reconnect with exp backoff(250ms → 4s、full jitter、cap 16 試行)
  - `onopen` / `onclose` / `onerror` / `onmessage` を Zustand store に流す
  - `controlMsg{k:'c',code:'daemon-disconnected'}` を `StatusBanner` 用に store に反映
- **`src/client/web/src/socket/retry.ts`**(新設、~80 行):
  - `subscribeWithRetry(sessionId, sendFn): Promise<void>` — `RespErr(frame-not-ready)` を捕捉して exp backoff で再試行(最大 16 試行)
  - subscribe state machine: `requested → confirmed | failed | retrying`
- **`src/client/web/src/socket/subscribe.ts`**(新設、~60 行):
  - active subscription set を保持し、reconnect 時に全件再送

### Components
- **`src/client/web/src/main.tsx`**(新設、~30 行): React 18 root、Zustand provider、global error boundary
- **`src/client/web/src/App.tsx`**(新設、~80 行): ticket 取得 → socket 起動 → レイアウト(SessionList / TerminalPane / StatusBanner)
- **`src/client/web/src/components/SessionList.tsx`**(新設、~80 行): 一覧表示、選択時に `selectSession` action
- **`src/client/web/src/components/TerminalPane.tsx`**(新設、~150 行): `useEffect` で xterm.js 生成、`onMessage` で `OutputFrame` を `term.write(base64Decode(d))`、`onResize` で resize frame 送信、`onData` で input frame 送信
- **`src/client/web/src/components/CreateSessionForm.tsx`**(新設、~80 行): form → `POST /api/sessions` → 成功時 `selectSession`
- **`src/client/web/src/components/StatusBanner.tsx`**(新設、~50 行): `status: 'reconnecting'|'closed'` のとき banner 表示
- **`src/client/web/src/css/app.css`**(新設、~80 行): minimal 既存風スタイル

### Tests(vitest + happy-dom + @testing-library/react)
- **`src/client/web/src/wire/codec.test.ts`**: 全 frame の parse / serialize round-trip
- **`src/client/web/src/store/daemon.test.ts`**: seedHello / applyViewUpdate / selectSession の reducer test
- **`src/client/web/src/store/notifications.test.ts`**: LRU 動作確認
- **`src/client/web/src/socket/retry.test.ts`**: fake WS で `RespErr(frame-not-ready)` → backoff → 成功シナリオ
- **`src/client/web/src/socket/connection.test.ts`**: fake WS で reconnect + 自動再 subscribe
- **`src/client/web/src/components/SessionList.test.tsx`**
- **`src/client/web/src/components/TerminalPane.test.tsx`**(xterm.js は mock)
- **`src/client/web/src/components/CreateSessionForm.test.tsx`**
- **`src/client/web/headers_test.go`**(Go 側、CSP 検証): `script-src 'self'` 維持、`unsafe-inline` / inline `<script>` が無いことを assert

## Verification

```sh
# Frontend(初回)
cd src/client/web && npm ci
cd src/client/web && npm run typecheck
cd src/client/web && npm run lint
cd src/client/web && npm run test -- --run
cd src/client/web && npm run build

# Go 側(CSP / embed 検証)
cd src && go test ./client/web/... -race
cd src && go vet ./client/web/...
cd src && go tool golangci-lint run ./client/web/...

# Full
make build-all
cd src && go test ./... -race

# Manual smoke
make run-dev
# arc daemon を別ターミナル起動
# browser: http://127.0.0.1:8080/#token=<printed>
# (1) session 作成、(2) xterm に出力、(3) input 反映、(4) daemon kill → banner 表示 → daemon 再起動 → 自動 reconnect
```

## Open Questions(実装直前に決める、minor)

1. xterm.js の addon(fit / web-links / search)を bundle に含めるか — 最低 fit は欲しい。これも exact pin。
2. dev server を Vite HMR(`npm run dev`)で動かす vs build 後のみで触る — α と同様 build 後アクセスを default(CSP との整合が単純)
3. CSS 単一 `app.css` でなく CSS Modules にするか — β は単一 css で開始、肥大化したら γ で分割

## Traceability

- **親 plan**: [Master Plan(`plans-cheerful-thompson.md`)](../../plans/arc-server-client-split.md)
- **前段 PR**: A1-α(commits 615b55f..2dd928f)で wire 層・gateway・server backend 完了
- **次の作業**: A1-γ(view-update broadcast)→ A1-δ(persist + connector)→ A1-ε(cleanup, `server/session` 完全削除)→ C(tmux 実装削除)
