---
id: task-20260728-secure-clone-authority
kind: task
title: Authenticated lease and replay-resistant clone authority
status: todo
created: '2026-07-28'
priority: critical
effort: large
files_touched:
- services/server/api/mux.go
- services/server/api/mux_*test.go
- services/server/api/repository_lease*.go
- services/server/api/mux_repository*.go
- services/server/api/auth*.go
- services/platform/lib/gitclone/credential*.go
- services/platform/lib/gitclone/broker*.go
- services/platform/lib/gitclone/process*.go
- services/platform/procgroup/
- services/cmd/server/
pr: null
tags:
- github
- security
- git-clone
- credential-lease
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
- {type: dependsOn, target: task-20260728-repository-protocol-domain}
source_paths:
- services/server/api/
- services/platform/lib/gitclone/
- services/platform/procgroup/
- services/cmd/server/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/requirements.md
- docs/changes/change-20260728-github-repository-sessions/implementation.md
- docs/adr/adr-20260728-ephemeral-github-clone-authority.md
- services/server/api/auth.go
- services/server/api/protocol_version.go
- services/platform/procgroup/
done_criteria:
- authenticated repository-bound memory-only lease admission
- peer-bound one-use askpass broker と sanitized git process
- FakeGitClone、named contracts、opt-in FakeVsRealGitClone
max_diff_loc: 1000
change: change-20260728-github-repository-sessions
summary: Admit repository authority only on secure authenticated exact-version routes
  and consume it once through a sanitized askpass broker.
---

# Authenticated lease and replay-resistant clone authority

## 責務

authenticated・exact-version・HTTPS または loopback の server route だけで repository-bound authority を受け入れ、memory-only・60 秒・one-use lease と peer/nonce-bound askpass broker を通して sanitized `git clone` process に一度だけ供給する。

## 詳細手順

1. transport/auth/version gate を lease/preparation/filesystem mutation より前に置き、repository identity、host identity、expiry、nonce、consumption state、32 lease cap を memory-only table で管理する。
2. validated owner/name から `https://github.com/<owner>/<name>.git` だけを導出し、remote URL、argv、environment value、git config、file、log、durable record に credential を置かない askpass broker を実装する。
3. replay、expiry、revoke、peer mismatch、redirect、不正 owner/name、process cancellation と bounded sanitized diagnostic を契約テストで網羅する。
4. FakeGitClone、GitCloneContract、private fixture repository を使う opt-in FakeVsRealGitClone を同じ seam に接続する。authority 未設定時は直接 `t.Skip` せず typed unavailable outcome をcontract assertionする。

## 前提

`task-20260728-repository-protocol-domain` が exact-version repository identity と typed failure surface を提供する。既存 auth middleware と procgroup の ownership/cancellation 規約を再利用する。

`max_diff_loc: 1000` は、最初のfrontier実装がscope・testsを通過し944 LOCを要した実測に基づく。route composition、lease table、one-use broker、sanitized process とsecurity contractを、credentialを露出する中間状態なしにlandする。

## スコープ外

credential の永続化、URL 埋め込み、環境変数注入、汎用 Git credential helper、SSH clone、GitHub Enterprise、clone publication transaction は扱わない。

## 受け入れ条件

- unauthenticated、version mismatch、非 secure transport が lease/preparation/filesystem side effect 前に typed rejection となる。
- lease は repository-bound、one-use、60 秒、host あたり最大 32 件で、replay・expiry・revoke・peer mismatch を拒否する。
- sentinel が URL、argv、environment、git config、filesystem、record、log、diagnostic に現れない。
- `cd services && GOCACHE=/tmp/gocache-agent-grid go test ./server/api/... ./platform/lib/gitclone/... ./platform/procgroup/... ./cmd/server/...` が成功する。
