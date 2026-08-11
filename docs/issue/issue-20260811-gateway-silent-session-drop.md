---
id: issue-20260811-gateway-silent-session-drop
kind: issue
title: Gateway silently drops WS frames whose session id cannot be resolved
status: open
created: '2026-08-11'
issue_type: bug
tags:
- gateway
- observability
- fleet
owners: []
relations:
- {type: originatedFrom, target: change-20260811-fleet-session-id-ssot}
subject_paths:
- services/server/api/gateway_lifecycle_inbound.go
- services/server/api/gateway.go
summary: The lifecycle WS handler logs and discards frames with an unresolvable session
  id without answering the client, which is the recurrence condition that hid the
  fleet-key leak through two prior fixes.
---

## 症状 / 機会

gateway が解決できない session id を持つ WS frame を、client へ何も返さずに捨てる。
client からは「入力したのに何も起きない」としか見えず、frame が届かなかったのか agent が沈黙しているのかを区別できない。

## 証拠

- `services/server/api/gateway_lifecycle_inbound.go:124-140` — `{k:"i"}` は `msg.SessionID` をそのまま
  `sess.WriteRaw` に渡し、失敗時は `slog.Warn("server/api: lifecycle write raw", ...)` だけで応答 frame を返さない。
  `{k:"r"}` も同じ経路 (`tryResize` は非正値と範囲外を silently drop / warn する)。
- `services/server/api/gateway.go:538-551` — per-session `AttachWS` の `applyInboundProto` も同じ形で、
  doc comment に "Unknown kinds are silently dropped" と明記されている。
- この無音性が実際に欠陥を隠した観測: debug run `debug-run-workspace-electron-terminal-dead` は、
  SPA が compound fleet key を `{k:"i"}` に載せていた欠陥が **2 度の修正機会 (9ef53ddc / 525b3091) の
  どちらの症状集合にも入らなかった**ことを RCA として確定した。理由は、他の交差点は可視な症状 (再生された
  status bell 等) を出したのに対し、この交差点だけ gateway が無音で捨てたため。
- 実スタック観測 (`clients/ui/e2e/real-stack/terminal-input.spec.ts`): `ld` は bare id で成功する一方、
  `i` / `r` は compound key のまま送られ、PTY に到達せず、client 側には何のエラーも現れない。

## 着手に必要な文脈

- 発見元: change-20260811-fleet-session-id-ssot (この change は client 側の所有権を直すが、
  gateway の無音性そのものは non_goals として明示的に切り出した)。
- design 側の位置づけ: 本 issue は「A を直しても同種の次の欠陥がまた無音になる」という再発条件であり、
  client 側の SSOT 統合とは独立に閉じる必要がある。
- 制約として既知: wire protocol (frame 形状・kind) の変更は change-20260811 の non_goals に入っている。
  応答 frame を足すのか、既存の `c` (control) / `e` (error) 系に載せるのか、metrics だけにするのかは未決。
- 関連: `docs/note/note-20260624-technical-web-gateway.md`。
