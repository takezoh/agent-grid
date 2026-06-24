# 031: client/runtime — FileRelay sweep が broadcast drop 時に dirty/offset を戻さない

- **Phase**: client-runtime follow-up（2026-06-22 web-gateway-isolation インシデント由来。Symphony SPEC 範囲外）
- **Status**: Open
- **Depends on**: なし（[030](030-runtime-internalch-saturation-diagnosis.md) と独立に着手可）
- **Blocks**: なし

## Background

`src/client/runtime/filerelay.go:185-207` の `sweep()` は次の順で動く:

1. lock 下で全 `relayFile.dirty` を `false` に落とし、対象を snapshot
2. lock を解放して `readFrom(f.path, f.offset)` でファイルを読む
3. lock 下で `f.offset = newOffset` を**前進**させる
4. `fr.broadcast(f, content)` で `internalBroadcastWire` を enqueue

`broadcast` の中（`filerelay.go:237-253`）は `fr.send(internalBroadcastWire{...})`
を呼ぶだけ。`fr.send` は `rt.enqueueInternal` に bind されており
（`filerelay.go:58`）、`internalCh` が満杯なら**黙って drop**される
（`ipc.go:233-239`）。

問題: dirty を落としオフセットを進めた**後**で drop が起きると、

- `f.dirty == false`（次の write が来るまで sweep は再走らない）
- `f.offset` は読んだ末尾まで前進している
- broadcast は失敗、client には届かない

→ **読まれたが届かなかったログ行は永久に失われる**。

実害: Web UI の Log タブで 100ms 単位の行抜け、`EvtSessionFileLine`
（claude/codex transcript tail）の単発欠落。daemon は死なないが UX 劣化。

ループ自体（FileRelay drop → arc.log への Warn → 再 drop の自己増幅）は
[memory: web_gateway_isolation] の `slog.Warn`→`Debug` 降格で塞いだが、
1 行単発の行抜けは現在も再現する。

## Tasks

- [ ] `fr.send` を failable signature に変更するか、FileRelay 専用のブロッキング
      送信経路を用意する。次のいずれかを選択:
      1. `enqueueInternal` を `(delivered bool)` 返し型に変える → `broadcast`
         が delivered=false なら呼び出し元 sweep に伝播
      2. `enqueueInternal` の counterpart として `enqueueInternalBlocking(ev, done)`
         を用意し（`sendSpawnComplete` と同形）、FileRelay の broadcast はこちらを
         使う（FileRelay はリアルタイム性が要らないので blocking 許容）
      3. FileRelay 用に別 channel + 別 consumer goroutine を立て、internalCh
         とは独立に back-pressure を持たせる
- [ ] sweep 側: 採用した経路で drop（or block timeout）が起きた場合に
      `f.dirty = true` を戻し `f.offset` を巻き戻す。複数 file が一度の sweep
      で混在する場合の挙動も決める（片方だけ失敗したらその file のみ戻す）。
- [ ] test: fake `send` を inject して常に drop する場合、次の sweep tick で
      同じ content が再読・再送されること。`f.dirty` と `f.offset` のロールバックを
      観測する。

## Acceptance Criteria

- internalCh saturation 下でも FileRelay が読んだログ行は最終的に必ず client に
  届く（drop による永久ロスがない）。
- 採用経路が「broadcast 失敗時に sweep が retry できる」契約を満たすこと
  （signature, doc comment 両方を更新）。
- 通常負荷では sweep の lock 競合が悪化しないこと（既存ベンチがあれば
  regression なし、なければ簡易マイクロベンチを足す）。
- `go test ./client/runtime/...` 緑（`-race` 含む）。

## References

- roost client runtime — Symphony SPEC 範囲外。source of truth は
  [ARCHITECTURE.md](../ARCHITECTURE.md) "Single-writer event loop"。
- `src/client/runtime/filerelay.go:185-253` — `sweep`, `broadcast`
- `src/client/runtime/ipc.go:233-278` — `enqueueInternal`（drop 経路）と
  `sendSpawnComplete`（blocking counterpart 既存例）
- 関連 follow-up: [030](030-runtime-internalch-saturation-diagnosis.md) —
  そもそもなぜ saturate するかの診断
- 由来: 2026-06-22 web-gateway-isolation インシデント（memory:
  `web_gateway_isolation`）の未着手 follow-up 2/2。
