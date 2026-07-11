---
id: adr-20260711-0082-frame-exec-launcher
kind: adr
title: ADR 0082 — Container-side frame sequencing (including devcontainer preExec) is owned by `bridge frame-exec`, shell composition is removed from the daemon boundary entirely
status: accepted
created: '2026-07-11'
updated: '2026-07-11'
tags:
- adr
- sandbox
- codex
- launcher
owners: []
relations:
- {type: partOf, target: plan-20260711-frame-exec-launcher}
- {type: references, target: adr-20260624-0001-multiplexed-backends-shared-routing-contract}
- {type: references, target: adr-20260624-0081-codex-frame-init-serialize}
source_paths:
- src/cmd/bridge/frame_exec.go
- src/platform/sandbox/devcontainer/manager.go
- src/client/runtime/subsystem/stream/backend.go
decision_makers:
- take.gn
summary: sandbox frame launch の sequencing (devcontainer preExec 評価 + pre-command 逐次実行 + main への syscall.Exec) を container 内の Go プロセス `bridge frame-exec` が単独所有する。daemon → container の `docker exec` argv には shell fragment を組み立てず literal `bridge frame-exec` のみ渡す。shell 呼び出しは preExec 評価のために bridge の内部で 1 回発生するが境界には露出しない
---

## Context

Commit `28ad8999` ("Trust container Codex workspaces before session attach") が sandbox 経路の `LaunchPlan.Command` に `X && exec Y` の compound を注入した結果、`platform/sandbox/devcontainer/manager.go:517` の envelope が `<preExec>; exec <Command>` を組み立てる際に POSIX shell の `exec` semantics (`exec CMD1 && CMD2` は CMD1 が execve に成功した時点で shell を置換し、`&& CMD2` は絶対に走らない) が発火し、**codex attach が到達しない** 回帰が起きた (2026-07-11 の 3 セッション連続再現)。

hotfix 案 B'' (`PreConditions []string` field + shell fragment 合成 + `validateExecCommand` boundary 検査) は症状を塞ぐが、**daemon 側で shell を組み立てる責務が残っている** 限り、異なる pre-step が追加されたときや shell の別の corner case で同型の invariant 破壊が再発する余地が残る。

sequencing の owner を「shell」「bridge」の 2 者ハイブリッドにするのではなく、**shell 解釈を境界から排除して bridge が単一 owner** になるのが本 ADR の方針。特に devcontainer.json 由来の `preExecCommand` (mise trust など tool 側 setup) 評価も bridge の内部責務に含めることで、docker exec argv には shell の文字を 1 文字も残さない。

adr-20260624-0001 (Multiplexed backends shared routing contract) と adr-20260624-0081 (codex frame init serialize) は frame lifecycle owner が stream backend であることを前提にしているため、bridge frame-exec の責務が backend と衝突しないよう境界を明示分離する必要がある (下記 Consequences)。

## Decision

sandbox frame launch の container 側逐次実行を `bridge frame-exec` が **単独所有** する。具体的には:

1. daemon は `FrameSpec` を JSON にまとめ、`docker exec -e AG_FRAME_SPEC=<json>` として container に渡す。FrameSpec は:
   - `pre_exec` (string) — devcontainer.json 由来の shell fragment (mise trust など)
   - `login_shell` (string) — user の login shell 絶対パス。host 側で `getent passwd | cut -d: -f7` の等価を daemon が resolve して渡す
   - `pre_commands` ([]argv) — trust step 等の argv 列
   - `main_command` (argv) — codex 等の final argv
   - `pre_command_timeout` (duration) — 各 pre-command の deadline (default 30s)

2. daemon の docker exec argv は **`bridge frame-exec` のみ** (login shell の resolution も includes-shell wrap も含めない):
   ```
   docker exec -it -u <user> -w <workdir> -e AG_FRAME_SPEC=<json> -e <他 env>... <cont> bridge frame-exec
   ```
   ← ここに `sh`, `-c`, `-lc`, `&&`, `;`, `exec ` の文字は 1 つも含まれない

3. `frame-exec` 実装は shared package `platform/framelaunch` に一本化する。container 側の `bridge` binary と host 側の daemon binary (`agent-grid-server`) が同じ `framelaunch.Run()` を subcommand として dispatch する。sandbox 機構 (docker exec / 直接 spawn) のみが caller 側で異なり、sequencing 実装は完全共通 (FR-005)。

4. `framelaunch.Run()` は次を順に行う:
   1. `AG_FRAME_SPEC` を parse。missing / invalid は非ゼロ exit
   2. `spec.LoginShell` が空文字なら `/etc/passwd` の該当 user の SHELL 列を読んで自己解決 (現行 envelope の `getent passwd | cut -d: -f7` と等価)。解決失敗時は `/bin/sh` fallback
   3. `pre_exec` が非空なら `<login_shell> -lc '<pre_exec> && env -0'` を subprocess として実行、stdout の NUL 区切り env dump を parse して自プロセスの `os.Environ()` を上書き。この shell 呼び出しは launcher の内部実装で、境界には露出しない
   4. `pre_commands` を argv 経由で逐次実行 (Go の `os/exec`)。exit == 0 → 次、非ゼロ → 実行を止めて自プロセスをその exit code で終了 (main は起動しない)
   5. `pre_command_timeout` (default 10s) 経過で pre-command を SIGTERM → 5s 後 SIGKILL。timeout も非ゼロ exit と同扱い
   6. 全 pre-command 成功後、`syscall.Exec(main_command[0], main_command, os.Environ())` で自プロセスを main に置換

5. frame lifecycle owner は stream backend のまま。frame-exec の exit は backend から見て通常の `runtime: frame exited` と同型。

**preExec 評価に関する contract**:
- bridge は preExec の **環境変数 side-effect のみ** を pre-commands / main に伝搬する。`alias` / `function` / `shopt` などの shell-only state は継承されない
- 本 repo 現行の `~/.dotfiles/modules/devcontainer/scripts/pre-exec.sh` (4 行、`mise trust --quiet` のみ、env 非変更) は当契約に完全準拠。将来 preExec が env 以外の state に依存する場合は当契約違反として拒絶する

**shell への内部依存**:
- bridge は preExec 評価と `env -0` のために `<login_shell> -lc ...` を 1 回 subprocess として起動する
- この shell 呼び出しは bridge の内部実装で、境界 (docker exec argv / daemon → container の interface) には露出しない
- shell 依存を完全排除するには preExec を shell fragment から Go-native な env delta (map[string]string) に契約変更する必要があるが、devcontainer.json の `preExecCommand` 契約と互換性が壊れるため本 ADR の scope 外

## Consequences

**Positive**

- **境界に shell が 1 文字も残らない**: daemon → container の docker exec argv は literal `bridge frame-exec`。commit 28ad8999 と同型の shell semantics 起因回帰は構造的に発生不可能
- **単一 owner**: sequencing (preExec / pre-command / main) の責務が bridge 1 プロセスに集約。分業ハイブリッドの曖昧さが無くなる
- pre-command / preExec の失敗は Go の error として structured logging (slog) で残る (現状は shell 内で silent)
- pre-command timeout / retry / env passing の拡張が Go code で自然に書ける (shell 上より test 容易)
- daemon 側の shell fragment 生成コード (`envelope.go`, `buildEnvelopeFragment`, `validateExecCommand`) が全廃可能

**Negative**

- container 内 bridge binary に新機能を足す必要 (~200 行 + test)
- daemon → container の spec transport (env var) が全 sandbox launch に追加される。他 container tool (kubectl / podman) 移植時に同型 API 対応要
- bridge が内部で shell を 1 回起動する (`<login_shell> -lc '<preExec> && env -0'`)。shell 依存自体は完全消滅しない (preExec が shell fragment である以上不可避)
- preExec の伝搬対象が env のみに限定される制約。alias / function に依存する preExec は当契約違反 (現行 repo の preExec は問題無し)

**Boundary — bridge と backend の責務分業**

- **bridge frame-exec**: preExec 評価 (内部 shell 経由 env 吸い上げ)、pre-command 逐次実行、成功判定、timeout、main への syscall.Exec 置換。frame の存在 / 消滅は関知しない
- **stream backend** (adr-20260624-0001 / 0081): frame lifecycle (spawn / attach / thread routing / kill / reap) の owner。bridge の exit code は backend から見て frame 終了と同型
- 境界の doc は `docs/component/component-*-client-stream-backend*` に referencedBy relation を追加して pointer 化する (別 PR)

## Alternatives

### Alternative A — 案 B'' (`PreConditions []string` + shell fragment 合成 + `validateExecCommand`) を land

**却下理由**: shell composition が daemon 側に残る限り、pre-step が増える / 複雑化した際に同型 invariant 破壊が再発する余地が残る。boundary validation は防御的措置に留まり、契約を型 / 構造で表明できていない。今回の唯一の pre-command で症状は塞げるが、根本原因 (契約の暗黙化 + shell 責務の daemon 分散) の解消にならない。

### Alternative B — bridge frame-exec で preExec を扱わず、外側で `sh -lc '<preExec>; exec bridge frame-exec'` の 2 段構えにする

**却下理由**: 境界に shell fragment が残り、preExec 評価責務が daemon 側 (shell 生成) と bridge 側 (pre-command 逐次実行) の 2 者にまたがるハイブリッドになる。sequencing owner が完全に単一にならず、責務境界の曖昧さが再発リスクとして残る。今回の 28ad8999 回帰そのものが「daemon 側 shell composition が壊れた」事例なので、その面を残す設計は根本策にならない。

### Alternative C — preExec 契約を shell fragment から env delta (map[string]string) に変更し、shell を完全排除

**却下理由**: devcontainer.json の `preExecCommand` 契約と互換性が壊れる (現行 `mise trust --quiet` のように「shell script を実行する」semantic を前提)。scope が本 spec を大幅に超え、devcontainer 側の migration が必要になる。将来 preExec が env 以外の shell state (alias / function) を要求するユースケースが顕在化してから検討する。

### Alternative D — pty expect 相当 (daemon が pty stdin/stdout に command を書き込み、PS1 sniffing で exit code を読む)

**却下理由**: 実装コストが大きい (500-800 行 + fake shell test 基盤)。PS1 が user 側 .bashrc / .zshrc で上書きされる corner case、raw モード切替タイミング、multibyte 境界などの fragility を daemon 側で全て抱え込む必要。今回の pre-command 数 (=1) では過剰。

### Alternative E — multi-exec (pre-command 毎に `docker exec` を回す)

**却下理由**: docker exec 起動 overhead (~50-100ms/回) が pre-step 数に比例。**env が pre-step 間で persist しない** (`export FOO=bar` が次の exec で見えない) 制約が入り、将来 pre-command が env を渡す必要が出た場合に破綻する。preExec の env 伝搬もこの経路では不可能。

## References

- Commit `28ad8999be09ad92dc3e0404b048add7d0057c38` — 回帰の起因
- 2026-07-11 debug session (server.log セッション 7ba71b71 / 6940641650 / 21de3b7 で 3 連続再現)
- POSIX `exec` semantics: replaced shell process, `&&` に到達しない
- 現行 `~/.dotfiles/modules/devcontainer/scripts/pre-exec.sh` — env 非変更、当 ADR の契約に完全準拠
