---
id: change-20260724-server-lan-exposure
kind: change
title: Server LAN exposure escape hatch
status: active
created: '2026-07-24'
profile: sdd@1
intent: 隔離された dev ネットワークで agent-grid-server を LAN に公開できる明示 opt-in を用意する。既存の loopback
  guard は default で残し、-no-auth と 0.0.0.0 bind を同時に有効化する にはもう一段の flag を必要とすることで、意図しない露出を防ぎつつ運用者が
  local 検証時に 他ホストから到達できるようにする。
outcomes:
- 運用者は -allow-non-loopback-no-auth を明示的に渡した場合に限り 0.0.0.0 bind + -no-auth の 組み合わせを起動できる。opt-in
  なしでは従来通り fail-closed で拒否される。
- 起動 log に non-loopback bind 時の distinct WARN が出て、後から journal / server.log を 見る運用者が露出状態を認識できる。
- systemd 運用ガイドに opt-in の存在と危険性、revert 手順が記載され、次にこの unit を 触る運用者が LAN 公開状態であることに気付ける。
scope:
- daemon flag surface (parseDaemonArgs) と resolveAuth の guard 分岐
- 起動 log の non-loopback WARN 分岐
- systemd 運用ガイド (docs/note/note-20260624-user-systemd.md) の bind / auth 節
non_goals:
- TLS 追加 (別 change)
- token-based auth を有効化した状態での 0.0.0.0 bind (既に既存 flag で可能)
- 認証機構そのものの再設計
change_classes:
- capability
governance:
  gate: soft
  reasons:
  - flag 追加は既存 default 挙動を変えず、guardrail を temper せず opt-in を追加するだけ
members:
- role: requirements
  path: changes/change-20260724-server-lan-exposure/requirements.md
  required: true
- role: implementation
  path: changes/change-20260724-server-lan-exposure/implementation.md
  required: true
- role: verification
  path: changes/change-20260724-server-lan-exposure/verification.md
  required: true
promotion: []
unresolved_decisions: []
tags:
- server
- auth
- dev-experience
owners:
- cmd/server
relations: []
source_paths:
- src/cmd/server
- docs/note/note-20260624-user-systemd.md
summary: Add -allow-non-loopback-no-auth opt-in flag so operators can deliberately
  expose the daemon on 0.0.0.0 with -no-auth for isolated dev networks; default loopback
  guard remains intact.
updated: '2026-07-24'
---

## Summary

`-no-auth` は元来 loopback bind でのみ許される guardrail を持つ。他ホストの
ブラウザから同一 dev network の agent-grid にアクセスしたいというユースケースに
対し、guardrail を弱めるのではなく明示 opt-in flag `-allow-non-loopback-no-auth`
を追加した。default 挙動 (`-no-auth` + non-loopback = 起動拒否) は不変で、opt-in
がなければ既存 test `TestResolveAuth_NoAuthRejectsNonLoopback` はそのまま通る。

## Closure Notes
