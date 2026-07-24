---
change: change-20260724-server-lan-exposure
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Content

### REQ-1: opt-in で -no-auth を non-loopback bind に対して許可する

- Given: 運用者が systemd unit で `-no-auth` を指定している
- When: `-addr 0.0.0.0:<port>` (または他の non-loopback) と `-allow-non-loopback-no-auth` を同時に渡す
- Then: daemon は起動を拒否せず、bearer token を発行せずに 0.0.0.0 で listen する

### REQ-2: opt-in なしの non-loopback + -no-auth は従来通り拒否される

- Given: 運用者が `-no-auth` と `-addr 0.0.0.0:<port>` を指定した
- When: `-allow-non-loopback-no-auth` が渡されていない
- Then: daemon は起動時に error を返して停止する (guardrail invariant)

### REQ-3: 起動 log で non-loopback bind を目立たせる

- Given: `-no-auth` + non-loopback bind で起動した
- When: gateway が listen 開始する
- Then: 従来の loopback WARN とは別文言で "NON-LOOPBACK bind — reachable from the network" を含む WARN を emit する

### REQ-4: opt-in flag 単独では auth を無効化しない

- Given: `-allow-non-loopback-no-auth` だけを渡した (`-no-auth` なし)
- When: daemon が起動する
- Then: token resolution は通常 precedence (-token > -token-file > random) に従い、無認証 mount にはならない
