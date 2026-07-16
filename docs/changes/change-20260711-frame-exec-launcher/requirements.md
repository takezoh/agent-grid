---
change: change-20260711-frame-exec-launcher
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260711-frame-exec-launcher
kind: spec
title: Frame launch sequencing owned by an in-container Go launcher
status: draft
created: '2026-07-11'
tags:
- sandbox
- codex
- launch
- regression-pin
owners: []
functional_requirements:
- id: FR-001
  statement: system は sandbox frame の pre-command / main command / devcontainer preExec
    の実行を、container 内 Go プロセス (`bridge frame-exec`) の single-owner sequencing で行い、daemon
    → container の docker exec argv に shell fragment (`sh -c` / `sh -lc` / `<login_shell>
    -lc` / `&&` / `;` / `exec ` 等) を組み立ててはならない
  priority: must
  rationale: shell の compound semantics (`&&` / `;` / `exec`) を daemon 側で組み立てる限り commit
    28ad8999 と同型の invariant 破壊 (`exec X && exec Y` が Y を落とす POSIX 仕様に起因する回帰) が再発する余地が残る。境界に
    shell の文字を 1 つも残さないことで shell 解釈を daemon 側から完全に排除する
- id: FR-002
  statement: WHEN sandbox frame の pre-command 列を実行するとき system はそれらを逐次実行し、いずれかが 非ゼロ
    exit のとき system は main command を実行してはならない
  priority: must
  rationale: '`codex-trust-project` が失敗した状態で `codex` attach を続けると untrust のまま起動する。gate
    semantics は spec レベルで保証する必要がある'
- id: FR-003
  statement: WHILE main command が起動している間 pty ownership は main プロセスに継承され、中間 launcher
    (bridge frame-exec) は pty を保持したまま残ってはならない
  priority: must
  rationale: codex TUI が pty を所有する必要がある。中間 launcher が残るとエスケープ処理・signal 転送・resize が二重になる。syscall.Exec
    で置換することで PID・pty 両方を継承しつつ launcher プロセスは消える
- id: FR-004
  statement: WHEN launcher が frame spec を container に渡すとき system は shell 経由の argv
    埋め込みではなく、構造化 (JSON) transport を用いなければならない
  priority: must
  rationale: docker exec 引数として shell 経由で argv を組み立てると、pre-command / preExec 内の quote/space
    処理で FR-001 と同型のバグ余地が残る。JSON は shell 解釈を通らない
- id: FR-005
  statement: sandbox 経路 (container / devcontainer) と host 経路 (`DirectLauncher`) は同一の
    frame-exec sequencing 実装を共有しなければならず、sandbox 機構 (docker exec vs 直接 spawn) 以外の launch
    semantics (preExec 評価 / pre-command 逐次実行 / main への syscall.Exec / timeout / LoginShell
    resolution) を分岐実装してはならない
  priority: must
  rationale: 実装乖離は同型 invariant 破壊の温床。sandbox / host で sequencing コードを分けると、片側の bug
    fix が他側に伝搬しない / 両側で微妙に違う semantics が生まれる。sequencing 実装は shared package (`platform/framelaunch`)
    に一本化し、caller (container 側 bridge / host 側 daemon) は subcommand として dispatch するだけ
- id: FR-006
  statement: '`LaunchPlan.Argv` が非空の frame launch について system は Argv を単一プロセスの argv[0]
    + args として直接 exec 可能な shape として扱い、`Argv[0]` 以外に shell を挟んではならない'
  priority: must
  rationale: string 表現だと caller が `&&` / `;` を混入できてしまい FR-001 の invariant を型で表明できない。argv
    であれば単一プロセスであることが型で示せる
- id: FR-007
  statement: WHEN pre-command が deadline (default 30s) を超えたとき launcher は SIGTERM →
    5s 後 SIGKILL の順で子プロセスを終了させ、その pre-command の exit code を非ゼロとみなして main command を実行してはならない
  priority: must
  rationale: 単一 pre-command が deadlock した場合の launcher の behavior を規定しないと、frame が pty
    を握ったまま無限待ちになる。deadline は spec.PreCommandTimeout で override 可能
- id: FR-008
  statement: WHEN `FrameSpec.PreExec` が非空のとき system は `<login_shell> -lc '<pre_exec>
    && env -0'` を subprocess として起動し、その stdout の NUL 区切り env dump を parse して frame-exec
    自プロセスの環境変数として上書きした後、pre-command 列と main command を実行しなければならない。`FrameSpec.LoginShell`
    が空文字のとき launcher は `/etc/passwd` の該当 user の SHELL 列を読んで自己解決し、解決失敗時は `/bin/sh`
    を fallback として用いなければならない
  priority: must
  rationale: devcontainer.json 由来の preExec (mise trust など tool 側 setup) の env side-effect
    を main / pre-commands に伝搬させる必要がある。この shell 呼び出しは launcher の内部実装で、境界には shell fragment
    を露出させない (FR-001 の invariant を維持)。login shell は user の dotfile (mise activation
    等) を source するために user 実 shell (zsh / bash) が望ましく、`/etc/passwd` 自己解決が現行 envelope
    の dynamic resolution と等価
- id: FR-009
  statement: '`FrameSpec.PreExec` の評価によって伝搬される shell state は環境変数のみに限定されなければならず、`alias`
    / `function` / `shopt` などの shell-only state を pre-command / main が継承することを保証してはならない'
  priority: must
  rationale: '`env -0` dump で伝搬できるのは環境変数のみ。alias / function に依存する preExec 契約を許すと bridge
    実装で必ず対応漏れが発生する。この制約を spec レベルで明示することで、将来の preExec 変更時に契約違反として検出可能にする'
non_functional_requirements:
- id: NFR-001
  type: performance
  criteria: frame launch の overhead は現行 (案 B'' 適用前) 比 +200ms 以内 (bridge frame-exec
    の起動 + JSON parse + preExec 用 shell subprocess 1 回)
- id: NFR-002
  type: maintainability
  criteria: pre-command sequencing の contract (FR-002 / FR-007) は Go unit test (Tier
    T0) で verify 可能でなければならない。preExec env 吸い上げ (FR-008) は fake shell script を用いた Tier
    T2 contract test で verify する
- id: NFR-003
  type: compatibility
  criteria: 既存の string-based `LaunchPlan.Command` を使う driver (claude / gemini / shell)
    は変更なしに動作しなければならない (legacy 経路は残す)
- id: NFR-004
  type: reliability
  criteria: pre-command 失敗 / preExec 失敗 / timeout 発火時、`server.log` には失敗した pre-command
    の argv と exit code、または preExec の評価結果 (exit code + stderr 先頭) が構造化 (slog) で残らなければならない
- id: NFR-005
  type: security
  criteria: pre-command spec (`AG_FRAME_SPEC`) は現行では workspace path / login shell
    path / preExec fragment 等の非機密のみを含む。secret を pre-command / preExec に含める拡張が入った場合は
    env transport から file transport (`0600` の per-frame spec file) に切り替えなければならない
relations:
- {type: references, target: adr-20260624-0001-multiplexed-backends-shared-routing-contract}
- {type: references, target: adr-20260624-0081-codex-frame-init-serialize}
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
- {type: references, target: adr-20260711-0083-launchplan-argv-primary}
- {type: references, target: adr-20260711-0084-frame-spec-transport}
- {type: implementedBy, target: plan-20260711-frame-exec-launcher}
summary: bridge frame-exec が frame launch の逐次実行を単独所有し、daemon 境界から shell 合成を排除する仕様。
---

## 背景

Commit `28ad8999` ("Trust container Codex workspaces before session attach") で sandbox 経路の `LaunchPlan.Command` に

```
/opt/agent-grid/run/bridge codex-trust-project && exec codex …
```

を注入した結果、`platform/sandbox/devcontainer/manager.go:517` の envelope

```
sh -lc '<preExec>; exec <Command>'
```

が `<Command>` に `X && exec Y` を渡す形になり、POSIX の `exec` semantics (`exec CMD1 && CMD2` は CMD1 が execve に成功した時点で shell を置換し、`&& CMD2` は実行されない) により **codex attach が到達しない** 回帰が発生した (2026-07-11 の debug session `~/.local/state/agent-grid/server.log` で 3 セッション連続再現)。

案 B'' (`PreConditions []string` field + shell fragment 合成 + `validateExecCommand` boundary 検査) で症状は完全に塞げるが、**daemon 側に shell composition が残る限り** 同種の invariant 破壊が異なる pre-step 追加時に再発する余地が残る。本 spec は sequencing 責務を **container 内の Go 単一 owner (`bridge frame-exec`)** に移し、shell 解釈を境界から排除することで invariant を構造的に保証する。特に devcontainer.json 由来の `preExecCommand` 評価も bridge の内部責務に含めることで、docker exec argv に shell の文字を 1 文字も残さない。

## Counterexample (spec が防ぐ誤実装)

- **誤実装 1**: pre-command と main を daemon 側で `X && exec Y` に組み立てる → shell の exec semantics で Y が落ちる (28ad8999 回帰)。FR-001 で禁止される
- **誤実装 2**: `Argv []string` の代わりに `Command string` を新しい経路に使い、caller が `Argv[0] = "sh"`, `Argv[1] = "-c"`, `Argv[2] = "X && exec Y"` を組み立てる。FR-006 の "shell を挟んではならない" に反する (Argv[0] が shell)
- **誤実装 3**: pre-command が hang したまま launcher が待ち続ける → frame が pty を握ったまま消えない。FR-007 で timeout 契約により防ぐ
- **誤実装 4**: launcher が `main` を子プロセスとして起動し、`main` の終了を待って launcher が exit する (fork 型) → pty ownership が中間 launcher に残り、resize / signal 転送が二重化する。FR-003 で syscall.Exec による置換を必須化
- **誤実装 5**: daemon が preExec を `sh -lc '<preExec>; exec bridge frame-exec'` の形で外側 wrap 評価する → docker exec argv に shell fragment が残り、境界における invariant が壊れる。FR-001 で禁止される。preExec 評価は bridge 内部の subprocess で完結させる (FR-008)
- **誤実装 6**: preExec 内の `alias` / `function` / `shopt` に依存する pre-command を書く → `env -0` dump で伝搬できないため container 内 pre-command が preExec の shell state を継承できず、silent failure。FR-009 で契約違反として明示される
- **誤実装 7**: bridge が preExec の env delta を map で受け取り os.Setenv するが、shell 経由で評価しない → devcontainer.json の `preExecCommand` 契約と互換性が壊れる (mise trust など shell script 実行を前提とする preExec が動かない)。FR-008 で shell 経由の評価を必須化

## Legacy context (置換対象)

commit `28ad8999` で導入され、案 B'' hotfix (working tree に置いた PreConditions + shell fragment 合成) で暫定塞ぎ中の以下を、本 spec の実装 (案 B''') 完了時に **全廃**:

- `src/platform/sandbox/devcontainer/envelope.go` — 削除
- `src/platform/sandbox/devcontainer/envelope.go` の `buildEnvelopeFragment` (shell fragment 合成) — 削除
- `src/platform/sandbox/devcontainer/envelope.go` の `validateExecCommand` (Command の `&&` / `;` / `||` 検出) — 削除 (argv 経路では不要)
- `src/platform/sandbox/devcontainer/manager.go:517` の `<preExec>; <preConds> && exec <command>` 経路 — 削除、`shell -lc` の wrap も削除

`Command string` フィールド自体は claude / gemini / shell driver で利用中のため残す (NFR-003)。ただし sandbox 経路 (`sandbox.LaunchSpec.Argv` 非空) では使用不可 (ADR-0083 の disjoint 契約)。

````
