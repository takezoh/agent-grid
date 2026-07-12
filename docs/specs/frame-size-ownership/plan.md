---
id: plan-20260712-frame-size-ownership
kind: plan
title: Complete size hint plumbing and clean up dead FrameSize API
status: draft
created: '2026-07-12'
goal: state.LaunchOptions.Cols/Rows を termvt.Spec.Cols/Rows まで貫通させ、新規セッションの初期表示を作成元 device の cols/rows で開始する。 併せて死 API FrameSize を除去し、HTTP/WS 境界で narrow uint16 変換前の値域検証と境界外 resize の可観測な拒否を導入する。
scope_in:
- src/client/state/driver_iface.go (LaunchOptions.Cols/Rows — 既存)
- src/client/runtime/backends.go (FrameLifecycle.SpawnFrame signature 拡張 / FrameSize 削除)
- src/client/runtime/pty_backend.go (SpawnFrame / RespawnFrame の size 転記・SoT 照会)
- src/client/runtime/interpret_spawn.go (e.Options.Cols/Rows を backend へ渡す + slog 観察点)
- src/server/web/mux.go (apiCreateReq の int→uint16 変換前検証)
- src/server/web/gateway.go (lifecycle WS {k:'r'} の境界外 error frame 拒否)
scope_out:
- platform/termvt の VT emulator / Session actor 自体
- multi-viewer の size reconcile ポリシー変更 (current の last-writer-wins を保持)
- session env / persistence への size 保存 / reattach 時の前回 size 復元
- attach 経路 (readInbound の resize) の error frame 化 (response 契約を持たないため silent drop 維持)
- tap の 1×1 emulator 撤去 / Emulator interface 分割 (別 issue)
milestones:
- id: c1-signature-plumbing
  title: backend.SpawnFrame signature 拡張 + spawn 観察ログ
  status: todo
- id: c2-respawn-sot
  title: RespawnFrame の size SoT 照会
  status: todo
- id: c3-web-boundary
  title: web layer 境界検証 (mux / gateway 両方)
  status: todo
- id: c4-framesize-cleanup
  title: FrameSize backend API の削除
  status: todo
contracts:
- pty_backend_test.go の fakeEmulator/fakePTY による size 反映契約
- mux_test.go の HTTP 境界値契約 (0/1/maxDim/maxDim+1/65536)
- gateway_test.go の lifecycle WS {k:'r'} 境界外拒否契約
- 既存の termvt.Session.Size() API (RespawnFrame の SoT 照会先)
tags:
- runtime
- terminal
- pty
- size-ownership
owners:
- take.gn@gmail.com
relations:
- {type: implements, target: spec-20260712-frame-size-ownership}
- {type: hasPart, target: adr-20260712-size-hint-wiring-completion}
- {type: hasPart, target: adr-20260712-ws-resize-bound-error-frame}
source_paths:
- src/client/state/driver_iface.go
- src/client/runtime/backends.go
- src/client/runtime/pty_backend.go
- src/client/runtime/interpret_spawn.go
- src/server/web/mux.go
- src/server/web/gateway.go
summary: LaunchOptions.Cols/Rows を backend.SpawnFrame signature 拡張で termvt.Spec まで貫通させ、RespawnFrame は termvt.Session.Size() 照会で SoT を単一化、web 境界で値域検証と WS error frame 拒否を導入し、死 API FrameSize を削除する 4 chunk 計画。
updated: '2026-07-12'
---

## Goal

`state.LaunchOptions.Cols/Rows` は β scope として定義されたまま消費点が repository 内に存在せず、全 frame が 80×24 で spawn される。 本計画はこの β scope 配線を完成させ、`backend.SpawnFrame` の signature を size 受領可能な形へ拡張することで size hint を `termvt.Spec` まで貫通させる。 併せて `RespawnFrame` は `termvt.Session.Size()` 照会で SoT を単一化し、web 境界で `int → uint16` narrow conversion の前の値域検証と、lifecycle WS の境界外 resize を error frame で拒否する契約を導入する。 死 API `FrameSize` は削除する。 詳細な要件根拠は `spec.md`、各設計判断の Why は個別 ADR を参照 — ここでは述べない。

## Implementation Sequence

{% milestone id="c1-signature-plumbing" %}
**backend.SpawnFrame signature 拡張 + spawn 観察ログ** (`adr-20260712-size-hint-wiring-completion`)

`FrameLifecycle.SpawnFrame` の signature を `(frameID, name, command, startDir string, env map[string]string, cols, rows int)` へ拡張し、`PtyBackend` / `noopBackend` / 全 test fake が新 signature に追随する。 `spawnFrameWindow` は `e.Options.Cols/Rows` を `int` にキャストして backend へ渡し、`slog.Info` に `(hint_cols, hint_rows, effective_cols, effective_rows)` を必ず出力する (fallback 発火時は `slog.Warn` も併記)。 `PtyBackend.SpawnFrame` は受け取った cols/rows を `termvt.Spec.Cols/Rows` に転記する。 hint 欠落 (少なくとも一方が 0) は既存 `termvt.normalizeSize` の 80×24 fallback をそのまま SoT として利用する。

**units** (task-grade 分解 — decompose の fast path 対象):

- **backend.SpawnFrame signature 拡張** — Objective: `FrameLifecycle` interface の SpawnFrame に (cols, rows int) を追加、全実装と全 test fake を追随。 files: `src/client/runtime/backends.go`, `pty_backend.go`, `interpret_spawn.go`, `pty_backend_test.go`, `interpret_spawn_test.go`. Boundaries: termvt 側 API は触らない、respawn は本 unit 外。 Acceptance: go build/vet pass、fake で size 転記が観測、interpret_spawn が e.Options.Cols/Rows を渡す。
- **spawn 時の hint→effective observation ログ** — Objective: `slog.Info` で pair を必ず出す (fallback 時は `slog.Warn` 併記)。 files: `src/client/runtime/interpret_spawn.go`, `interpret_spawn_test.go`. Boundaries: event 追加は行わず slog のみ、fallback 判定は `cols<=0 || rows<=0`。 Acceptance: hint=(100,40)/(0,0)/(100,0) の 3 case で slog capture が期待通り。
{% /milestone %}

{% milestone id="c2-respawn-sot" %}
**RespawnFrame の size SoT 照会** (`adr-20260712-size-hint-wiring-completion`)

`PtyBackend.RespawnFrame` を new session 作成前に `termvt.Session.Size()` で現在の cols/rows を照会し、`termvt.Spec.Cols/Rows` に転記する形へ書き換える。 既存 termvt session が teardown 済み (真の cold-start) の場合は 80×24 fallback。 `RespawnFrame` の呼び出し側 signature は変えない (LaunchOptions を渡さない現行契約を維持)。 termvt 側 API (`Session.Size()`) は既存 — 追加しない。

**units**:

- **RespawnFrame の size SoT 照会** — Objective: `termvt.Manager.Get(target).Size()` を `mgr.Remove` の直前で呼び、`termvt.Spec.Cols/Rows` へ転記。 files: `src/client/runtime/pty_backend.go`, `pty_backend_test.go`. Boundaries: RespawnFrame signature 変更なし、termvt 側 API 変更なし。 Acceptance: respawn 前後で Size() が同値、既存 session 不在時は 80×24 fallback。
{% /milestone %}

{% milestone id="c3-web-boundary" %}
**web layer 境界検証 (mux / gateway 両方)** (`adr-20260712-ws-resize-bound-error-frame`)

`mux.go` の `apiCreateReq` ハンドラで `req.Cols/Rows` を `int` のまま受け、`(cols<0 || rows<0 || cols>maxSpawnDim || rows>maxSpawnDim)` を HTTP 400 (`invalid_dim`) で拒否する。 `(0, 0)` と `(cols>0 && rows>0)` の 2 状態のみ通過させ、片方だけ 0 は 400 で拒否する (FR-004 の判別性を境界で守る)。 `maxSpawnDim = 2000` は `src/server/web/` 内の定数として持ち、`src/platform/termvt/session.go:264` (`maxDim`) との整合はコメントで担保する (SoT drift は integration/e2e で検出可能)。

`gateway.go` の lifecycle WS `{k:'r'}` ハンドラは、既存の silent `continue` を `writeRespErrFrame` による `{k:'e', reqId, reason:'invalid_dim'}` 返答に置き換える。 境界外検出時は `sess.Resize` を呼ばず既存 winsize/grid を維持する。 attach 経路 (`readInbound at gateway.go:539`) の resize は reqId (response 契約) を持たないため既存の silent drop を維持する。

**units**:

- **apiCreateReq の値域検証 (create-session)** — Objective: `int` のまま validator を通し、境界外は 400。 files: `src/server/web/mux.go`, `mux_test.go`. Boundaries: gateway.go (resize 経路) は本 unit 外。 Acceptance: AC-001 相当が 202 で hint 反映、AC-003 の (100,0) と AC-004 の 2001/65536 が 400 invalid_dim。
- **lifecycle WS resize の境界外 error frame 拒否** — Objective: `writeRespErrFrame` で `{k:'e', reason:'invalid_dim'}` を返す形へ置換。 files: `src/server/web/gateway.go`, `gateway_test.go`. Boundaries: attach 経路の resize は response 契約なしのため対象外。 Acceptance: AC-006 相当が error frame で拒否、境界内は既存動作維持、`sess.Resize` が境界外時に呼ばれない (fake 観測)。
{% /milestone %}

{% milestone id="c4-framesize-cleanup" %}
**FrameSize backend API の削除** (`adr-20260712-size-hint-wiring-completion`)

`FrameInspect` interface から `FrameSize` を削除し、`PtyBackend` / `noopBackend` / 全 test fake の実装を剥がす。 production 呼び出しゼロが `grep -rn 'FrameSize(' src/` で 0 件になることを確認する。 テスト依存があった場合はそのテストを `termvt.Session.Size()` 直接呼び出しに書き換える (別 SoT で置換)。

**units**:

- **FrameSize backend API の削除** — Objective: interface / 実装 / test fake / test 呼び出しから FrameSize を除去。 files: `src/client/runtime/backends.go`, `pty_backend.go`, `pty_backend_test.go`, その他 fake. Boundaries: `termvt.Session.Size()` は残す (本 unit は `FrameInspect.FrameSize` のみ)。 Acceptance: `grep -rn 'FrameSize(' src/` が 0 件、build/vet/test all pass。
{% /milestone %}

## Targets

**変更対象ファイル**:

- `src/client/runtime/backends.go` — `FrameLifecycle.SpawnFrame` signature 拡張、`FrameInspect.FrameSize` 削除、`noopBackend` 追随
- `src/client/runtime/pty_backend.go` — `SpawnFrame` (size 転記)、`RespawnFrame` (`termvt.Session.Size()` 照会)、`FrameSize` 実装削除
- `src/client/runtime/interpret_spawn.go` — `e.Options.Cols/Rows` を backend へ渡す、`slog` 観察点追加
- `src/client/runtime/pty_backend_test.go` / `interpret_spawn_test.go` — fake が新 signature に追随、size 転記と slog capture の観測
- `src/server/web/mux.go` — `apiCreateReq` の `int` のままの validator、`maxSpawnDim = 2000` 定数
- `src/server/web/mux_test.go` — 境界値テスト
- `src/server/web/gateway.go` — lifecycle WS `{k:'r'}` の error frame 返答へ置換
- `src/server/web/gateway_test.go` — WS 境界外拒否テスト

**外部依存ごとの seam (テスト可能性)**:

- **kernel pty**: `termvt.NewSession` 経由 (既存)。 `pty_backend_test.go` の `fakePTY` (io.Pipe pair) が seam。 本計画で新規追加なし
- **VT emulator**: `termvt.NewSessionWithDeps` 経由 (既存 `NewSessionWithDeps` は fake `Emulator` を受ける)。 `fakeEmulator` が seam
- **backend interface (FrameLifecycle)**: `runtime` 側の spawn effect と backend 実装の境界 seam。 本計画で signature を拡張し、全 fake が新契約に追随
- **web HTTP境界**: `httptest.Server` で mux をラップして border 検証を回す (既存 pattern)
- **lifecycle WS**: `gateway_test.go` の既存 WS fake connection (`writeRespErrFrame` 観察) が seam

**再利用する既存パターン / 既存関数**:

- `termvt.normalizeSize` (80×24 fallback + maxDim clamp — SoT を termvt に集中維持)
- `termvt.Session.Size()` (RespawnFrame の照会先)
- lifecycle WS の `writeRespErrFrame` helper (境界外 resize の error 返答に再利用)
- 既存 `gatewayError` パターン (HTTP 400 レスポンス構造)
- `runtimetest.Harness` (interpret_spawn テストのセットアップ)

## Verification

**構造規則 → 検証手段**:

- **層規則** (server/web が platform/termvt を import しない): `src/.golangci.yml` の depguard で fitness function 化済 (既存)。 `golangci-lint run` で保証
- **size SoT の一意性** (kernel + emulator 以外に owner を作らない): 機械検証は困難 — レビュー規範として plan.md に明記 + `pty_backend.go` に in-process size cache を追加していないことを PR diff で確認
- **narrow conversion 順序** (validator を int で先に、uint16 変換は後): `mux_test.go` の境界値テスト (65536 を 400 で拒否) が fitness function
- **死 API 復活防止**: `grep -rn 'FrameSize(' src/` を CI に組み込むかは future work (本計画では acceptance テストのみ)

**検証プロファイル**:

| profile | milestone | tier | command | criterion | milestone DoD |
|---|---|---|---|---|---|
| signature-plumbing | c1-signature-plumbing | T1 | `cd src && go test ./client/runtime/...` | `PtyBackend.SpawnFrame` が `termvt.Spec` に size を転記 (fake 観測) | 新 signature で全 fake pass + slog capture が hint/effective を出力 |
| respawn-sot | c2-respawn-sot | T1 | `cd src && go test -run TestRespawn ./client/runtime/...` | RespawnFrame 前後で `termvt.Session.Size()` が同値、session 不在時は 80×24 | respawn テストで size 継承と fallback 両方 pass |
| web-validators | c3-web-boundary | T1 | `cd src && go test ./server/web/...` | 境界値 (0/1/maxDim/maxDim+1/65536) で HTTP status と WS error frame が期待通り | AC-003 / AC-004 / AC-006 全て pass |
| framesize-cleanup | c4-framesize-cleanup | T0 | `grep -rn 'FrameSize(' src/ \| wc -l` | 出力が 0 | grep 0 件 + `go build`/`vet`/`test` 全 pass |
| structural-fitness | all | T0 | `cd src && golangci-lint run` | 層規則違反 (web が platform/termvt を import する等) がゼロ | 既存 depguard rule が pass、web パッケージが自前 `maxSpawnDim` 定数を保持 |

**手動シナリオ**:

- 実 browser で新規 session を大画面 (e.g. 200×60) で作成し、spawn 直後の frame が 80×24 → resize の 2 phase を経ずに直接 hint size で描画されることを目視確認 (NFR-003)

## Open Questions

- browser 側 (`src/client/web/src/components/TerminalPane.tsx`) が現状で POST /api/sessions body に cols/rows を実際に載せているかは実装 unit (c1 or c3) で確認する。 issue 本文は `TerminalPane.tsx:131,180` を browser 側 fit / onResize 起点として言及しているが、create 時に body 組み立てへ流し込むコードのパスは未確認。 現状 hint を送っていなければ本計画の hint 配線が発火しないため、追加で web frontend 側の 1 chunk が要る可能性あり。
- `adr-20260712-size-hint-wiring-completion` と `adr-20260712-ws-resize-bound-error-frame` を accepted へ遷移するタイミング (spec approve と同時 or 実装完了後) は host が決める (draft のままで実装着手可能)。
