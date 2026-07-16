---
id: adr-20260711-0084-frame-spec-transport
kind: adr
title: ADR 0084 — Frame launch spec transport is env var (JSON); file-based transport
  reserved for secret-carrying evolutions
status: proposed
created: '2026-07-11'
tags:
- adr
- sandbox
- security
- launcher
owners: []
relations:
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
- {type: references, target: change-20260711-frame-exec-launcher}
source_paths:
- src/platform/sandbox/devcontainer/manager.go
- src/cmd/bridge/frame_exec.go
decision_makers:
- take.gn
summary: '`bridge frame-exec` に渡す frame spec (pre-commands + main argv) を `AG_FRAME_SPEC`
  env var (JSON) で転送する。現行 spec 内容 (workspace path・argv・timeout) は非機密のため env transport
  で十分。将来 spec が secret を含むよう拡張された場合は per-frame 0600 spec file 経由に切り替える (evolvability
  を予約)'
---

## Context

adr-20260711-0082 で `bridge frame-exec` を container 内 launcher として導入。daemon (host) → bridge frame-exec (container) に **pre-command argv 列 + main command argv + timeout** を渡す必要がある。transport の候補:

- (a) env var `AG_FRAME_SPEC` に JSON をそのまま乗せて `docker exec -e` で渡す
- (b) host が per-frame の spec file (`<runDir>/frame-spec-<frameID>.json`) を書き、container 内は path で参照
- (c) argv の repeat フラグ (`--pre '<cmd>' --pre '<cmd>' -- <main-argv>`)

secret 露出面: env var は `ps auxwwe` / `/proc/<pid>/environ` から同 UID の他プロセスに読める。file は permission 0600 で protect できる。argv は `ps aux` の COMMAND 列に出るため env より露出面が広い。

現行 spec 内容 (adr-20260711-0082 更新後):

- `pre_exec`: `~/.dotfiles/modules/devcontainer/scripts/pre-exec.sh` 相当の shell fragment (現行 repo では 4 行、`mise trust --quiet`)
- `login_shell`: `/bin/sh` or user resolved (現状 fallback 想定)
- `pre_commands`: `[["bridge", "codex-trust-project"]]` (workspace path も含まない)
- `main_command`: `codex --model X --remote unix://sock -C /path`
- `pre_command_timeout`: `30s`

これらは全て非機密。JSON size 見積り: 上記フィールドで ~500-800 bytes (pre_exec が inline shell script の場合最大でも数 KB を想定)。ただし将来 pre-command / pre_exec に AG_SOCKET_TOKEN 相当の secret を含める設計 (agent identity 系) が入る可能性は否定できない。

## Decision

**現行 scope では env var 転送 (`AG_FRAME_SPEC`) を採用**。理由:

- 現行 spec 内容は非機密 (workspace path・argv のみ)
- 実装最小 (host は encode するだけ、container は `os.Getenv` するだけ、file lifecycle 管理不要)
- container 内の他 process への露出リスクは、そもそも container 内 UID が単一 (`ubuntu`) で分離が既に無いため、file の 0600 でも大差ない
- argv (c) は `ps auxwwe` 露出が広く、また shell escape が必要 (adr-0082 と矛盾)

**Evolvability**: spec に secret を含める拡張が入った時点で file transport に切り替える。切替の trigger:

- `FrameSpec` struct に `Env map[string]string` フィールドを足す case
- pre_commands の argv に credential token を含める case

その場合の実装形:

- host: `<runDir>/frame-spec-<frameID>.json` (mode 0600) に spec を write、frame 終了時 `defer` で削除
- container: `bridge frame-exec --spec-file <path>` で読み込み。読み込み直後に unlink して他 process からの inspect を防ぐ
- FrameSpec の interface は同一 (env / file の switch は bridge 側 flag だけの差分)

## Consequences

**Positive (現行)**

- 実装最小 (spec JSON encode + env で完結)
- file lifecycle 管理 (write / cleanup / race) 不要
- crash / kill 時に stale file が残らない (env は process と共に消える)

**Negative (現行)**

- env は同 UID 他 process に読める (現状の container は user 分離無しなので実害無し、doc に明記)
- linux MAX_ARG_STRLEN (128 KiB) が spec size 上限。現行 (~500 bytes) では十分だが、pre-command 数が多い / argv が長い場合に将来詰まる可能性 → 監視で拾う

**Positive (future evolvability)**

- FrameSpec struct は transport 中立に設計するため、env → file の切替が bridge 側 flag だけの差分で済む
- file 切替時に env 経路を廃止するか coexist するかは future ADR で決定

## Alternatives

### Alternative A — 最初から file transport で実装

**却下理由**: file lifecycle (write / cleanup / race / crash-time stale) の管理コストが大きい。現行 spec 内容が非機密であることを踏まえると overengineering。secret を含める設計変更が入るタイミングで file transport を敷けば十分。

### Alternative B — stdin JSON push (`docker exec -i` + `bridge frame-exec` が stdin から spec を読む)

**却下理由**: `docker exec -it` で pty attach する frame launch のとき、stdin は user input 経路として main command が持つ必要がある。bridge frame-exec が stdin を先に読んで close すると main command が stdin を失う。pty ownership の syscall.Exec 継承 (adr-0082 FR-003) と衝突。

### Alternative C — argv フラグ経由 (`--pre '<cmd>' -- <main>`)

**却下理由**: `ps auxwwe` に spec 全体が露出する。将来 secret を含めるようになった時点で全面書き換えが要る。env と比べても secret 露出面が広い。

## References

- adr-20260711-0082-frame-exec-launcher — bridge frame-exec の設計
- Linux MAX_ARG_STRLEN — env / argv の 128 KiB 上限
- `docker exec -e KEY=VAL` — env 転送の官約 API
