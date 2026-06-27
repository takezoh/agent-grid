# 030: client/runtime — internalCh の初回 saturation 発生源を診断する

- **Phase**: client-runtime follow-up（2026-06-22 web-gateway-isolation インシデント由来。Symphony SPEC 範囲外）
- **Status**: Done (55321ad0, 2026-06-27)
- **Depends on**: なし
- **Blocks**: なし（実害は単発 event ロスのみ。daemon は wedge しない）

## Background

2026-06-22 の wedge インシデント（`web_gateway_isolation` memory 参照）は
`enqueueInternal` の drop 時に出していた `slog.Warn` が `arc.log` に流れ、
FileRelay が読み戻して再度 `internalCh` に enqueue する自己増幅ループだった。
直接の対症として `src/client/runtime/ipc.go:233-239` で `slog.Warn` →
`slog.Debug` に降格してループは止めた。

しかし**そもそも何が最初に `internalCh` (cap 64) を満たしたのか**は未診断。
ループは止まったが drop された event は失われたままで、UI 上で:

- `internalBroadcastSurface` drop → web terminal で pty 出力の瞬間的な抜け
- `internalBroadcastWire` drop → session card / log tab の更新欠落
- `connOpen` / `connClose` drop → reconnect notification の欠落

といった単発の劣化として残り得る。

容疑が濃い producer（`internalEventName` の列挙より）:

| 種別 | 起源 | 性質 |
|---|---|---|
| `internalBroadcastSurface` | `TerminalRelay` の pty 出力 fan-out（A1-α 導入） | コンパイラ出力や大量 stdout で容易に burst |
| `internalBroadcastWire` | FileRelay sweep / state broadcast / wire fan-out | reconnect 時に複数 frame 分が同時に発火 |
| `connOpen` / `connClose` | web client の reconnect 連打 | exp backoff retry の境界で集中し得る |

drop は現状 Debug ログにしか痕跡が残らず、saturation の頻度・主因・spike
形状を観測できない。

> **2026-06-27 解消** (commit 55321ad0): `enqueueInternal` を failable 化
> (bool 返却) し、drop 時に pre-populated `map[name]*atomic.Uint64` で
> per-event-type にカウント。`Runtime.InternalDropStats()` public 経由で
> snapshot を取得可能。spike 発生時の主犯特定が可能になった。
> tests: `TestInternalDropCounter_{incPerType,unknownBucket,concurrent}` /
> `TestEnqueueInternal_dropsCountAndReturnsFalse`.

## Tasks

- [ ] `enqueueInternal` に drop カウンタを追加（`internalEventName` で type 別、
      atomic / per-event-type で集計）。`/debug/varz` か既存の metrics 経路で
      露出。
- [ ] daemon 起動から測った `internalCh` の最大 occupancy を観測できるよう
      sample する（ring buffer or quantile）。常時計装でなくても、`-debug-internal-ch`
      flag で opt-in する形で十分。
- [ ] 実機で web 接続から `make test` 級の負荷を流し、最初の drop までの caller
      内訳を計測。
- [ ] 結果に基づき、最も saturate しやすい producer に coalesce / batching を
      入れる、または `internalCh` の cap を引き上げる。drop が起きた時点で
      event を失わない設計上の選択（ブロッキング化）も検討する。

## Acceptance Criteria

- 通常負荷（web client 2–3 本 + 数 session）で internalCh occupancy が定常的に
  50% 未満。
- spike 発生時の主犯（producer type）が metrics から特定できる。
- saturation 発生時の event loss が UI 観測可能な欠落として現れないこと
  （UI スモークテストで terminal scroll / session badge / log tab の抜けが
  起きない）。
- `go test ./client/runtime/...` 緑（`-race` 含む）。

## References

- roost client runtime — Symphony SPEC 範囲外。source of truth は
  [ARCHITECTURE.md](../ARCHITECTURE.md) "Single-writer event loop"。
- `src/client/runtime/ipc.go:221-265` — `enqueueInternal`, `internalEventName`,
  `sendSpawnComplete`（drop しない counterpart）
- `src/client/runtime/terminal_relay.go` — `internalBroadcastSurface` producer
  （A1-α）
- `src/client/runtime/filerelay.go` — `internalBroadcastWire` producer
  （broadcast→send→enqueueInternal）
- 関連 follow-up: [031](031-filerelay-drop-dirty-restore.md) — drop 時の
  FileRelay 側 dirty/offset 戻し
- 由来: 2026-06-22 web-gateway-isolation インシデント（memory:
  `web_gateway_isolation`）の未着手 follow-up 1/2。
