# forks/ — patched vendored forks

上流バグの fix を当てたローカル fork。`src/go.mod` の `replace` directive からここを指す。
背景と分析は `issues/2026-07-02-vt-emulator-insertlinearea-panic.md`。

上流に PR を提出し、merge されたら該当 fork を削除して pin を上流バージョンに戻す。

## x-vt

- Module: `github.com/charmbracelet/x/vt`
- Base: `v0.0.0-20260615091924-bb3af1bbe712` (module cache からコピー)
- Patch (producer 側 invariant):
  - `screen.go` — `setVerticalMargins` / `setHorizontalMargins` が margin を
    buffer bounds に clamp し、clamp 後に退化した region は無視する。
    resize が PTY を通過する間に child が stale なサイズ前提で
    DECSTBM / DECSLRM を送っても、scroll region が buffer を超えなくなる
  - `margins_test.go` — 追加 (production crash 経路の再現テスト含む)
- 受理判定 (`top >= bottom` / `left >= right`) は handler 側で生値のまま行う。
  clamp を handler 側で行うと上流既存テスト
  (`TestTerminal/CUP_Relative_to_Origin` 系) が要求する挙動と衝突する

## ultraviolet

- Module: `github.com/charmbracelet/ultraviolet`
- Base: `v0.0.0-20260303162955-0b88c25f3fff` (module cache からコピー)
- Patch (consumer 側 defense-in-depth):
  - `buffer.go` — `InsertLineArea` / `DeleteLineArea` が area を
    `b.Bounds()` に Intersect してから操作する (旧: `area.Max.Y > b.Height()`
    のとき `b.Lines[i][x]` が index out of range で panic)
  - `buffer_area_bounds_test.go` — 追加

## 検証メモ

追加テストはどちらも上流 HEAD (2026-07-05 時点: x `2cc9a8f`, ultraviolet
`f5a850f`) に対して fail する (= 上流未修正、本物のバグを捉えている) ことを
確認済み。
