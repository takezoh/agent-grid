# server: subscribe 時の replay が pending approvals/questions を再送しない

- status: open
- created: 2026-07-25
- area: services/host (state replay) / services/server/api (lifecycle WS)
- origin: debug-run-shell-zero-sessions の horizontal scan (`~/.dev-skills/debug/shell-zero-sessions/investigation.json`)

## 症状 (潜在)

lifecycle WS の subscribe 時 replay はセッション一覧のみを hello (`k:"h"`) で再送し、
**pending 中の approvals / questions を再送しない** (`services/host/state/reduce_conn.go:39-51`)。
クライアント (Shell / Workspace / browser UI) が「承認要求が発生した後」に接続・再接続すると、
その承認/質問はどのクライアントにも表示されず、resolve もタイムアウトもされないまま
エージェントがブロックし続ける可能性がある。

## 根拠

- `reduce_conn.go` の subscribe replay は `EvtSessionsChanged` 相当のみを合成する
- approvals/questions は発生時の broadcast (`k:"ar"` / `k:"qr"`) でしか流れない
  (ADR: approval-broadcast-coexists-view-update)
- Shell の再接続 (full-jitter backoff) は日常的に発生する — 再接続直後の窓で取りこぼす

## 修正方向 (案)

subscribe 時に sessions と同様、未解決の approvals/questions を `ar`/`qr` として
replay する (サーバー側 SSOT は既に保持している)。クライアント側 reducer は同一
id の再受信に冪等 (SupervisionReducer は id keyed) なので互換。

## 検証案

- Go gateway scenario test (T1): 承認発生 → 新規 WS attach → `ar` が届くことを assert
- windows-shell 側は既存 reducer テストで冪等性を pin 済み
