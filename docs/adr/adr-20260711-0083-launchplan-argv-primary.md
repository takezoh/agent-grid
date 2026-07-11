---
id: adr-20260711-0083-launchplan-argv-primary
kind: adr
title: ADR 0083 — `LaunchPlan.Argv` + `PreCommands` は frame launch の primary 表現、`Command
  string` は legacy
status: proposed
created: '2026-07-11'
tags:
- adr
- launch
- api-contract
owners: []
relations:
- {type: references, target: plan-20260711-frame-exec-launcher}
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
- {type: referencedBy, target: spec-20260711-frame-exec-launcher}
source_paths:
- src/client/state/driver_iface.go
- src/platform/agentlaunch/types.go
- src/platform/sandbox/manager.go
decision_makers:
- take.gn
summary: '`LaunchPlan.Argv []string` と `LaunchPlan.PreCommands [][]string` を primary
  shape として型で invariant を表明する (Argv[0] は単一プロセスの exec 対象、単純コマンドに限定)。既存の `Command string`
  は claude / gemini / shell driver 用の legacy 経路として残し、Argv が非空なら frame-exec 経路、空なら
  Command 経路に disjoint に分岐する'
---

## Context

commit 28ad8999 の回帰の根本原因は `LaunchPlan.Command string` が「単純コマンド」invariant を **doc コメントにしか持たない** 契約の暗黙化 (debug session R2 判定 `契約=Y`)。string 型のため caller が `&&` / `;` / `exec` を書けてしまい、compile time にも run time にも検出されない。

`agentlaunch.LaunchPlan` (`src/platform/agentlaunch/types.go:11`) にはすでに `Command string` と `Argv []string` が並存する pattern が入っており、doc は "callers choose which to use" と書いている。`state.LaunchPlan` (`src/client/state/driver_iface.go:374`) はまだ `Command string` のみ。

adr-20260711-0082 で `bridge frame-exec` が argv-based sequencing を担う設計を採る以上、caller が argv を直接持てる shape を型で表明する必要がある。同時に既存の string-based driver (claude / gemini / shell) を破壊しない compatibility (spec NFR-003) も要る。

## Decision

`state.LaunchPlan` / `agentlaunch.LaunchPlan` / `sandbox.LaunchSpec` の 3 型に以下を追加する:

- `Argv []string` — 非空なら「frame-exec 経路」。`Argv[0]` は単一プロセスの exec 対象、`Argv[1:]` は args。**shell を挟まない** ことが型で示される (`Argv[0] = "sh"` は formally 可能だが、それを caller が意図的に書く時点で明示的な shell 使用を宣言することになる)
- `PreCommands [][]string` — 各要素は argv。frame-exec 経路でのみ意味を持つ。空でも可。**Argv が非空のときだけ有効**
- `PreExec string` — devcontainer.json 由来の shell fragment (mise trust など)。frame-exec 経路のとき launcher が内部で shell を呼んで env を吸い上げる。**Argv が非空のときだけ有効** (adr-20260711-0082 FR-008 参照)
- `LoginShell string` — **optional override**。空文字のとき launcher が `/etc/passwd` から self-resolve (現行 envelope の `getent passwd` と等価、解決失敗時は `/bin/sh` fallback)。caller が明示的に shell を強制したい場合 (test 等) のみ埋める。daemon は基本空文字を送る
- `PreCommandTimeout time.Duration` — pre-command 1 本ごと + preExec 評価の deadline (**default 10s**、`framelaunch.DefaultTimeout` 定数)。Argv が非空のときのみ有効。caller が本質的に長い pre-command (network fetch 等) を導入したときのみ override

`Command string` はそのまま残す (legacy)。仕様:

- **disjoint 契約**: `Argv` 非空 → frame-exec 経路、`Argv` 空 → legacy string 経路 (`Command` を消費)
- **Argv 非空時に `Command` に値が入っている** ケースは (`LaunchSpec` boundary で validation error として reject する) — 契約違反として明示検出
- **`PreCommands` は Argv 非空時のみ有効**。Argv が空で PreCommands が非空 → validation error

これにより:

- 型上で「frame-exec 経路は shell 解釈を挟まない」が表明される
- claude / gemini / shell driver は変更不要 (`Command string` そのまま)
- 新規 driver は argv 経路が primary となる (docs で誘導)

## Consequences

**Positive**

- caller が `Argv []string` を使えば `&&` / `;` を **書けない** (中間要素として shell を経由しないため)。契約が型で表明される
- 既存 driver の破壊が無い (NFR-003)
- `agentlaunch.LaunchPlan` の既存 Argv field と一貫し、layer 間の重複が減る
- 将来 `Command string` を廃止する道を残しつつ、hotfix 効果を先に得られる

**Negative**

- 3 つの表現 (`Command string`, `Argv []string`, `PreCommands [][]string`) が並存する期間が発生
- LaunchPlan の frontier が広がる (LaunchSpec boundary の validation を怠ると Argv 空 + PreCommands 非空のような矛盾状態が起きうる)

**Boundary validation** (implementation detail):

- `sandbox.LaunchSpec` の consumer 境界 (`BuildLaunchCommand` 冒頭) で:
  - `len(Argv) > 0 && Command != ""` → error ("mutually exclusive")
  - `len(Argv) == 0 && len(PreCommands) > 0` → error ("PreCommands requires Argv")
  - `len(Argv) == 0 && PreExec != ""` → error ("PreExec is only meaningful in frame-exec path; devcontainer preExec 経由の legacy 経路は spec.PreExec 側で扱う")
  - `len(Argv) > 0` かつ Argv 経路の未実装 backend (現状 devcontainer 以外) → error ("frame-exec launcher unavailable")

## Alternatives

### Alternative A — `Command string` を全 driver で `Argv []string` に置き換える (一括移行)

**却下理由**: claude / gemini / shell driver の string form (`claudecli.ForkCommand(command, cs.ForkParentID)` など既存 API) の全面書き換えが必要。scope が本 spec を超え、本来のバグ修正が遅れる。段階的移行 (本 ADR で argv 経路を正 → 別 PR で legacy 廃止) の方が safe。

### Alternative B — `Command string` を newtype `SimpleCommand` として型化する (`type SimpleCommand string`)

**却下理由**: Go の型システムは refinement type を持たないため、`SimpleCommand("X && exec Y")` は compile time に検出できない。runtime validation は現行 `validateExecCommand` と同等で、型による表明にならない。argv 化が本質的に強い invariant を表明できる。

### Alternative C — `LaunchPlan` に `Sandboxed bool` を持たせて sandboxed のみ Argv 経路

**却下理由**: sandboxed 判定と launch shape が結合し、future host-side frame-exec (spec FR-005) が入りにくくなる。Argv/Command の分岐と sandboxed の分岐は直交させておくべき。

## References

- adr-20260711-0082-frame-exec-launcher — 消費者 (bridge frame-exec) の設計
- 現行 `agentlaunch.LaunchPlan` の Command + Argv 並存パターン (`src/platform/agentlaunch/types.go:11`)
