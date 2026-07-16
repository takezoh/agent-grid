---
change: change-20260712-frame-size-ownership
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260712-frame-size-ownership
kind: plan
title: Frame size ownership implementation plan
status: draft
created: '2026-07-12'
goal: spawn 時 size hint を web API → LaunchOptions → EffSpawnFrame → PtyBackend → termvt.Spec
  の 1 直線で運び、HTTP/WS 境界で narrow 前 validation を敷き、FrameInspect.FrameSize dead API を削除する
scope_in:
- 'HTTP 境界 (mux.go apiCreateReq) の int -> uint16 narrow conversion 前 validation: 1..maxDim
  (=2000) を許容、0/0 は未指定として通過、asymmetric は 400 reject'
- 'WebSocket 境界 (gateway.go の {k:"r"} 2 か所) の narrow 前 validation: >maxDim は warn
  log + drop、既存の非正値 drop は維持'
- state.LaunchOptions.Cols/Rows を LaunchPlan/CreateLaunch を素通しで EffSpawnFrame.Options
  まで運ぶ既存経路の verify + 写し漏れ driver への pass-through 追加
- FrameLifecycle.SpawnFrame signature 拡張 (cols/rows uint16 を末尾追加) と 4 実装 (PtyBackend
  / noopBackend / fakeBackend / blockingBackend) への追随
- PtyBackend.SpawnFrame で cols/rows を termvt.Spec に転記 (0/0 は normalizeSize (80,24)
  fallback を仕様として維持)
- FrameInspect.FrameSize + PtyBackend / noopBackend / fakeBackend / blockingBackend
  の 4 実装 + interface 定義 の削除。test 観測経路 (pty_backend_test.go TestResizeSurface) を termvt.Session.Size()
  直接 (or Manager accessor) へ切替
- RespawnFrame の default 80×24 fallback を doc comment で明示 (adr-20260712-respawn-default-size-fallback)
- termvt.MaxDim の export と共通 validation helper (state package の pure) の追加
- T0 pure (境界 validation helper) / T1 wired (spawn 時 hint 反映 + FrameSize 削除後の observation)
  / e2e (HTTP 400 body / WS drop 経路) のテスト追加
scope_out:
- tap 1×1 emulator 撤去 / Emulator interface 分割
- forks/x/vt bounds bug 対応
- multi-viewer size reconcile policy の変更
- state 層 reducer が size を保持するモデルへの変更
- driver interface (LaunchPreparer) への size 責務追加
- cold-start restore における size 復元機構の追加
- RespawnFrame への size hint 継承経路の追加
- session-env / persist スキーマの拡張
milestones:
- id: m1
  title: 境界検証 helper + termvt.MaxDim export + HTTP/WS wire-up
  status: todo
- id: m2
  title: spawn size hint 配線 (LaunchOptions → SpawnFrame signature → termvt.Spec)
  status: todo
- id: m3
  title: FrameInspect.FrameSize 削除 + test 観測経路切替
  status: todo
- id: m4
  title: RespawnFrame default fallback の doc 化 + Failure Modes 節反映
  status: todo
contracts:
- FR-001, FR-002 (spawn hint 反映 / default 80×24 invariant) を pty_backend_test.go の
  T1 wired で pin する
- FR-003, FR-004 (境界検査) を helper の T0 + HTTP e2e + WS test で pin する
- FR-006 (driver 素通し) を driver 実装 grep 検証 + T0 test で pin する
- FR-007 (FrameInspect.FrameSize 不在) を grep confirmation で pin する
- NFR-005 (reducer が size を持たない) を reduce_surface.go の diff で pin する
tags:
- runtime
- termvt
- server
- validation
owners: []
relations:
- {type: implements, target: spec-20260712-frame-size-ownership}
- {type: hasPart, target: adr-20260712-launch-size-pass-through}
- {type: hasPart, target: adr-20260712-size-hint-boundary-validation}
- {type: hasPart, target: adr-20260712-remove-framesize-dead-api}
- {type: hasPart, target: adr-20260712-respawn-default-size-fallback}
- {type: hasPart, target: adr-20260712-spawnframe-inline-size-pair}
source_paths:
- src/server/web/mux.go
- src/server/web/gateway.go
- src/server/web/wire.go
- src/client/state/driver_iface.go
- src/client/state/effect.go
- src/client/runtime/interpret_spawn.go
- src/client/runtime/pty_backend.go
- src/client/runtime/backends.go
- src/client/runtime/pty_backend_test.go
- src/platform/termvt/session.go
summary: 4 milestone (境界検証 / spawn 配線 / FrameSize 削除 / RespawnFrame doc) で spec-20260712-frame-size-ownership
  を実装する
---

## Goal

spec-20260712-frame-size-ownership の 7 FR / 6 NFR / 6 AC を実装する。size ownership の SoT を termvt に閉じ、driver / reducer / state 層は size 運搬役以上の責務を持たない構造を確立し、HTTP/WS 境界で narrow 前 validation を敷いて silent wrap-around を仕組みで防ぐ。dead API (FrameInspect.FrameSize) を削除して抽象と実装の drift 源を除く。設計判断の詳細は 5 ADR (launch-size-pass-through / size-hint-boundary-validation / remove-framesize-dead-api / respawn-default-size-fallback / spawnframe-inline-size-pair) に集約されている。

## Implementation Sequence

{% milestone id="m1" %}
**境界検証 helper + termvt.MaxDim export + HTTP/WS wire-up** (adr-20260712-size-hint-boundary-validation)

作業単位:
- `src/platform/termvt/session.go` の `maxDim = 2000` を `MaxDim` に export する (SoT 定数)
- `src/client/state/` 配下に共通 validation helper (仮称 `ValidateSizeHint(cols, rows int) error`) を追加 (state pure). 3 条件を判定: (i) 両方 0 は通過、(ii) 片方だけ 0 は asymmetric として reject、(iii) 1..MaxDim 範囲外は reject。narrow は行わない
- `src/server/web/mux.go` apiCreateReq の decode 後・LaunchOptions 生成前に helper を呼び、invalid は `respondError(400, "invalid_cols_rows", "cols must be 0 or 1..2000; got <n>")` で応答する
- `src/server/web/gateway.go` / `wire.go` の WS `{k:"r"}` 経路 2 か所 (applyInboundProto と readLifecycleInbound 相当) で helper を呼び、invalid は warn log (session_id / cols / rows / reason) を残して drop
- helper と 3 caller の T0 pure test (負値 / 65536 wrap / >maxDim / asymmetric / 両方 0 通過 / 正常値通過 の 6 パターン)

Members:
- component: `size hint validation helper`, `mux.go apiCreateReq`, `WS resize inbound (gateway.go / wire.go)`
- req: FR-003, FR-004, FR-005
- adr: adr-20260712-size-hint-boundary-validation
{% /milestone %}

{% milestone id="m2" %}
**spawn size hint 配線** (adr-20260712-launch-size-pass-through + adr-20260712-spawnframe-inline-size-pair)

作業単位:
- 5 driver (generic / codex / claude / shell / gemini) の `PrepareLaunch` / `PrepareCreate` 実装を grep verify し、LaunchOptions を LaunchPlan/CreateLaunch.Options に verbatim 転写しているかを確認。写し漏れがあった driver に対して 1 行の pass-through を追加
- `FrameLifecycle.SpawnFrame` interface signature を `SpawnFrame(frameID, name, command, startDir string, env map[string]string, cols, rows uint16) error` に拡張
- 4 実装 (PtyBackend / noopBackend / fakeBackend / blockingBackend) の SpawnFrame を signature 追随。PtyBackend のみ受けた cols/rows を termvt.Spec.Cols/Rows に転記 (cols=0/rows=0 は Spec に 0 のまま渡し normalizeSize (80,24) fallback に委ねる)
- `src/client/runtime/interpret_spawn.go` の spawnFrameWindow で `e.Options.Cols/Rows` を SpawnFrame 呼び出しに渡す
- spawn ログ (slog.Info) にフィールド (frame_id, cols, rows, source="hint"|"default") を追加
- T1 wired test を `pty_backend_test.go` に追加: SpawnFrame(cols=120, rows=40, ...) → fakeEmulator の cols/rows / fakePTY の Winsize が 120x40 であること、および cols=0, rows=0 で 80x24 fallback を観測

Members:
- component: `state.LaunchOptions` (既存), `LaunchPlan.Options / CreateLaunch.Options` (既存), `EffSpawnFrame.Options` (既存), `runtime.spawnFrameWindow`, `FrameLifecycle.SpawnFrame`, `PtyBackend.SpawnFrame`, `noopBackend / fakeBackend / blockingBackend`, `termvt.Spec / termvt.NewSession` (既存, 消費側)
- req: FR-001, FR-002, FR-006
- adr: adr-20260712-launch-size-pass-through, adr-20260712-spawnframe-inline-size-pair
{% /milestone %}

{% milestone id="m3" %}
**FrameInspect.FrameSize 削除 + test 観測経路切替** (adr-20260712-remove-framesize-dead-api)

作業単位:
- `src/client/runtime/backends.go` の `FrameInspect` interface から `FrameSize` を削除。interface が FrameSize のみ持つ場合は interface 自体も削除
- 4 実装 (PtyBackend / noopBackend / fakeBackend / blockingBackend) の `FrameSize` method を削除
- `pty_backend_test.go TestResizeSurface` の観測経路を `termvt.Session.Size()` 直接に切替。Manager 経由の accessor が必要なら pty_backend_test.go 内で私設 accessor を用意 (公開 API surface を増やさない)
- grep confirmation で `FrameSize(` の残存が test を除きゼロであることを verify

Members:
- component: `FrameInspect.FrameSize`, `PtyBackend.FrameSize`, `noopBackend/fake/blocking の FrameSize`, `pty_backend_test.go TestResizeSurface`
- req: FR-007
- adr: adr-20260712-remove-framesize-dead-api
{% /milestone %}

{% milestone id="m4" %}
**RespawnFrame default fallback の doc 化 + Failure Modes 節反映** (adr-20260712-respawn-default-size-fallback)

作業単位:
- `PtyBackend.RespawnFrame` (`pty_backend.go:91-111`) の宣言直上に doc comment を追加: (1) 現状 size なしで termvt.Spec を作り 80×24 に fallback する挙動、(2) production caller 0 のため hint 保存機構は追加しない、(3) 将来 respawn 消費者が発生した時点で adr-20260712-respawn-default-size-fallback を supersede する ADR を起こす、の 3 点を明記
- spec の Failure Modes 節 (respawn-size-loss) との 1:1 対応を doc 化 (spec の該当行への参照 or ADR 番号を doc comment 内に含める)

Members:
- component: `PtyBackend.RespawnFrame`
- req: (仕様上の invariant を doc 化する対象; unwanted FR には対応しない - Failure Modes 節 respawn-size-loss と対応)
- adr: adr-20260712-respawn-default-size-fallback
{% /milestone %}

## Targets

**変更対象ファイル**:
- `src/server/web/mux.go` — apiCreateReq (m1)
- `src/server/web/gateway.go` — WS 'r' 経路 (m1)
- `src/server/web/wire.go` — WS 'r' 経路 (m1)
- `src/client/state/driver_iface.go` — LaunchOptions のコメント整理 (m2 / adr-20260712-launch-size-pass-through 反映)
- `src/client/state/` 配下の新規または既存 helper file — `ValidateSizeHint` (m1)
- `src/client/runtime/interpret_spawn.go` — spawnFrameWindow (m2)
- `src/client/runtime/backends.go` — FrameLifecycle / FrameInspect interfaces (m2 signature 拡張 / m3 削除)
- `src/client/runtime/pty_backend.go` — SpawnFrame / RespawnFrame / FrameSize (m2 転記, m3 削除, m4 doc)
- `src/client/runtime/pty_backend_test.go` — TestResizeSurface 観測経路切替 + spawn hint T1 wired test 追加 (m2, m3)
- `src/platform/termvt/session.go` — MaxDim export (m1)
- 各 driver package (generic / codex / claude* / shell / gemini) の Prepare* — pass-through 確認 (m2 grep verify、必要なら 1 行追加)

**再利用する既存資産 (発明せず命名)**:
- `state.LaunchOptions.Cols/Rows` (uint16 field): 既に存在する transport slot、そのまま流用
- `state.EffSpawnFrame.Options`: 既に LaunchOptions を verbatim で運ぶ effect payload
- `termvt.Spec.Cols/Rows` + `termvt.normalizeSize` (80x24 fallback): 既存の受け皿と default 挙動を流用
- `termvt.Session.Size()`: FrameSize 削除後の test 観測用 (既存 accessor)
- `slog.Info` (spawn ログ): 既存の logging 経路にフィールド追加のみ
- `respondError(400, ..., ...)` 相当の HTTP error 応答 helper (mux.go に既存の pattern があれば追随)
- `fakeEmulator` / `fakePTY` (test 資産): T1 wired test の観測点として既存 pattern を再利用

**外部依存の seam** (design/PRINCIPLES.md §11):
- **kernel pty** (creack/pty): `pty.StartWithSize` → PtyBackend のみが触る。test は `fakePTY` で seam 済み (既存)
- **VT emulator** (forks/x/vt): `emulatorFor(cols, rows)` → PtyBackend のみが触る。test は `fakeEmulator` で seam 済み (既存)
- **HTTP transport** (net/http): mux.go の apiCreateReq 経由。境界 validation は helper に集約 (seam 化)
- **WebSocket transport** (gorilla/websocket 相当): gateway.go / wire.go 経由。境界 validation は同じ helper に集約 (seam 化)

新規 seam 追加はなし (既存 fake / helper 資産の再利用のみ)。

**純粋核**:
- `ValidateSizeHint(cols, rows int) error` — state package の pure function、I/O 依存なし、T0 でカバー

## Verification

各 milestone の完了確認手段。real 依存でしか検証できない部分は明示する。

| profile | milestone | Tier | 実行コマンド (verbatim) | 判定基準 |
|---|---|---|---|---|
| `helper_pure` | m1 | T0 pure | `cd src && go test ./client/state/... -run ValidateSizeHint` | 6 パターン (負値 / 65536 wrap / >maxDim / asymmetric / 両方 0 / 正常値) 全 pass |
| `http_boundary` | m1 | T1 wired | `cd src && go test ./server/web/... -run TestApiCreateSessionInvalidCols` | apiCreateReq が 400 応答を返し body JSON が仕様形式に一致 |
| `ws_boundary` | m1 | T1 wired | `cd src && go test ./server/web/... -run TestWsResizeInvalidCols` | resize drop + warn log (fields = session_id, cols, rows, reason) が観測される |
| `spawn_hint` | m2 | T1 wired | `cd src && go test ./client/runtime/... -run TestSpawnFrameWithSizeHint` | fakeEmulator の Size() と fakePTY の Winsize が指定した cols/rows と一致 |
| `spawn_default` | m2 | T1 wired | `cd src && go test ./client/runtime/... -run TestSpawnFrameDefaultsToNormalizeSize` | cols=0/rows=0 で spawn 時に 80x24 が観測され spawn log に source="default" が含まれる |
| `driver_pass_through_grep` | m2 | T0 pure | `cd src && grep -RnE 'PrepareLaunch\|PrepareCreate' ./client/drivers` (人手 review) | 全 driver 実装が LaunchOptions を LaunchPlan/CreateLaunch.Options に verbatim 転写 |
| `framesize_removed` | m3 | T0 pure | `cd src && grep -RnE 'FrameSize\(' . \| grep -v _test.go` | production 経路 (非 _test.go) からの FrameSize 呼び出しが 0 件 |
| `resize_observation_switched` | m3 | T1 wired | `cd src && go test ./client/runtime/... -run TestResizeSurface` | 既存 test が termvt.Session.Size() 直接 (or Manager accessor) 経由で pass |
| `respawn_doc` | m4 | T0 pure | `cd src && grep -n 'RespawnFrame' ./client/runtime/pty_backend.go` (人手 review) | doc comment に (1) 80×24 fallback、(2) production caller 0、(3) supersede 予定 ADR ref の 3 点が含まれる |
| `reducer_size_invariant` | 全体 | T0 pure | `cd src && grep -RnE 'Cols\|Rows' ./client/state/reduce_surface.go` | reducer 内に size field 保持がゼロ (NFR-005 の pin) |
| `structural` | 全体 | T0 pure | `make vet && make lint` (repo root) | depguard / funlen / staticcheck が zero error |

**real 依存でしか検証できない部分**:
- HTTP 400 body / WS warn log の end-to-end 観測は最終的に `make build-server` 済みの server に curl / websocket client で叩く手動確認で締める (T2 相当)
- kernel pty (creack/pty) と VT emulator (forks/x/vt) の real 挙動は既存の `FakeVsReal*` test で担保済み — 本 issue では size hint が Spec に届くところまでを T1 wired で pin し、その先の real 挙動は既存 harness を信頼する

**構造規則 → 検証手段** (fitness function):
- depguard: `platform/* は client を import しない` → `make lint` (機械)
- SoT invariant (size は termvt が SoT / reducer は size を持たない) → NFR-005 grep + adr-20260712-launch-size-pass-through Alternatives (規範 review)
- narrow 前 validation (helper が narrow より statically 前) → helper の signature が int を受けて error を返し narrow を含まない (機械) + T0 test で invalid が uint16 に化ける前に reject されることを確認

## Reference Algorithms

### alg-validate-size-hint

```
func ValidateSizeHint(cols, rows int) error {
    // Both zero -> unspecified (accepted, downstream will use 80x24)
    if cols == 0 && rows == 0 {
        return nil
    }
    // Asymmetric single-dimension -> reject
    if (cols == 0) != (rows == 0) {
        return errors.New("asymmetric size hint: both cols and rows must be specified")
    }
    // Range check (1..MaxDim)
    if cols < 1 || cols > termvt.MaxDim {
        return fmt.Errorf("cols must be 0 or 1..%d; got %d", termvt.MaxDim, cols)
    }
    if rows < 1 || rows > termvt.MaxDim {
        return fmt.Errorf("rows must be 0 or 1..%d; got %d", termvt.MaxDim, rows)
    }
    return nil
}
```

HTTP caller は error を 400 body に mapping (code=invalid_cols_rows, message=error.String())、WS caller は warn log にフィールド化して drop。

### alg-spawn-frame-with-hint

```
// In interpret_spawn.spawnFrameWindow(e EffSpawnFrame):
cols := e.Options.Cols
rows := e.Options.Rows
err := backend.SpawnFrame(e.FrameID, e.Name, e.Command, e.StartDir, e.Env, cols, rows)

// In PtyBackend.SpawnFrame:
spec := termvt.Spec{
    // ... existing fields ...
    Cols: int(cols),  // 0 -> normalizeSize (80,24)
    Rows: int(rows),
}
source := "hint"
if cols == 0 && rows == 0 {
    source = "default"
}
slog.Info("frame spawn size", "frame_id", frameID, "cols", spec.Cols, "rows", spec.Rows, "source", source)
sess, err := termvt.NewSession(ctx, spec)
// ...
```

````
