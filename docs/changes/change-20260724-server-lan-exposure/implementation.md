---
change: change-20260724-server-lan-exposure
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Content

### daemon_flags.go

- `daemonFlagSet` に `allowNoAuthNonLoop bool` を追加
- `parseDaemonArgs` で `-allow-non-loopback-no-auth` を登録。help text は「DANGEROUS — exposes the unauthenticated REST/WS surface to the network. Only intended for isolated dev networks.」と明示

### gateway.go

- `resolveAuth` の signature に `allowNoAuthNonLoop bool` を追加。既存の loopback 判定を `if !isLoopbackAddr(addr) && !allowNoAuthNonLoop` に変更。error message は opt-in flag を提案する文言に更新
- `startGateway` から `df.allowNoAuthNonLoop` を渡すよう配線
- `logStartup` の `noAuth` 分岐を `isLoopbackAddr(addr)` で split。non-loopback 側は distinct WARN を emit する

### resolve_auth_test.go

- 既存 `TestResolveAuth_NoAuthRejectsNonLoopback` は温存 (default guardrail の invariant)
- `TestResolveAuth_NoAuthAllowsNonLoopbackWithOptIn` を追加 (REQ-1)
- `TestResolveAuth_AllowNonLoopbackWithoutNoAuthIsNoop` を追加 (REQ-4)
- 既存 caller は全て新 signature に合わせて `false` を渡すよう更新

### systemd 運用ガイド

- `docs/note/note-20260624-user-systemd.md` に opt-in flag の存在、危険性、revert 手順を追記
