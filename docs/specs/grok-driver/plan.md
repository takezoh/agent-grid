---
id: plan-20260711-grok-driver
kind: plan
title: Grok Driver implementation
status: draft
created: '2026-07-11'
goal: 公式 Grok Build CLI を既存 Driver/runtime 契約へ追加し、PTY TUI と process lifecycle 観測を再現可能に統合する
scope_in:
- Grok Driver state、launch/session policy、persist/view/status
- PTY/CLI subsystem の process lifecycle と確認済み same-process signal
- GROK_HOME host/container wiring
- fake、contract、FakeVsRealGrok
scope_out:
- Grok CLI、config、session file の変更
- Grok ACP sidecar または ACP primary UI transport
- Grok plugin/dashboard/worktree UI
milestones:
- {id: m1, title: 公式 CLI と terminal signal を characterization する, status: todo}
- {id: m2, title: 純粋 Grok Driver と launch policy を実装する, status: todo}
- {id: m3, title: PTY process lifecycle と runtime wiring を実装する, status: todo}
- {id: m4, title: fidelity suite と統合 gate を完成する, status: todo}
contracts:
- fresh=--session-id UUID; continue=--continue; resume=--resume ID; fork=--resume ID --fork-session
- automation=--no-auto-update
- persisted authoritative > launch seed; 確認済み same-process signal のみ status を更新する
- external dependency triple=fake + invariant contract + FakeVsRealGrok
tags: [grok, driver]
owners: []
relations:
- {type: implements, target: spec-20260711-grok-driver}
- {type: hasPart, target: adr-20260711-grok-observation-transport}
- {type: hasPart, target: adr-20260711-grok-session-launch-contract}
- {type: hasPart, target: adr-20260711-grok-metadata-authority}
- {type: hasPart, target: adr-20260711-grok-home-automation-policy}
source_paths: []
methodology: sdd
summary: 純粋 Driver、PTY process lifecycle、GROK_HOME wiring、三段階 fidelity test を依存順に実装する
---

## Goal

spec の観測契約を、既存 `state.Driver` と PTY runtime の process event を再利用して実装する。Grok 固有 I/O は runtime seam に隔離し、純粋 reducer と launch policy は固定入力で検証可能にする。

## Implementation Sequence

### m1

{% milestone id="m1" %}
isolated `GROK_HOME` と固定 workspace で installed v0.2.93 の help、fresh/continue/resume/fork、`--no-auto-update`、process lifecycle、同一 process の OSC/window-title signal の有無を記録する。明示契約が得られなければ status は idle/running/stopped/failed に限定し、非公開 session file は読まない。

Task units: (1) `src/platform/lib/grok/` に CLI/process/terminal-signal probe と sanitized recording を追加し、credentials/config/sessions 非変更を検査する（最大 250 LOC）。(2) fake executable/process fixture と invariant contract を同じ scenario API で追加する（最大 300 LOC、1 に依存）。
{% /milestone %}

### m2

{% milestone id="m2" %}
`GrokState`/`GrokDriver`、token-aware `GrokLaunchPolicy`、persist/view/status/metadata precedence を実装する。UUID の生成と session collision lookup は runtime seam が所有し、純粋 policy は注入済み UUID の形式・flag 競合・argv のみ決定する。

Task units: (1) `src/platform/lib/grok/argv.go` と tests に session/automation/model/effort argv 契約を実装（最大 300 LOC、m1 に依存）。(2) `src/client/driver/grok*.go` と tests に reducer/view/persist を実装（最大 500 LOC、unit 1 に依存）。(3) registry/conformance/MetadataSourcePriority に Grok scenario を追加（最大 200 LOC、unit 2 に依存）。各 unit は実装と自身の test を同時に含む。
{% /milestone %}

### m3

{% milestone id="m3" %}
既存 PTY/CLI subsystem で Grok TUI を起動し、process lifecycle と characterization 済み same-process signal だけを Driver event にする。host/container の `GROK_HOME` resolution/mount は既存 runtime launch plan seam へ結線し、config は read-only user-owned asset として扱う。ACP factory/process/bind は追加しない。

Task units: (1) PTY process lifecycle/status adapter と T1 wired tests（最大 300 LOC、m1/m2 に依存）。(2) GROK_HOME host/container wiring と mutation contract tests（最大 300 LOC、m2 に依存）。ACP が起動されないことも runtime test で固定する。
{% /milestone %}

### m4

{% milestone id="m4" %}
fake と real に同一 scenario を適用し、CLI/process lifecycle drift、failure mapping、config non-mutation、ACP 非起動を検証する。real mismatch は fake の修正対象とし assertion は緩めない。build/vet/lint/full tests を再走する。

Task units: (1) `FakeVsRealGrok` opt-in e2e と invariant contract の完成（最大 300 LOC、m1-m3 に依存）。(2) cross-package integration/race/quality gate と docs source path 更新（最大 200 LOC、unit 1 に依存）。
{% /milestone %}

## Targets

- `src/platform/lib/grok/`: argv parser/builder、process/terminal-signal fixture、fake/real scenario。純粋核は stdlib-only で `platform/agentlaunch.SplitArgs` を再利用する。
- `src/client/driver/grok*.go`: state owner、Step、View/Status、Persist/Restore。外部 I/O を import しない。
- 既存 `src/client/runtime` PTY/CLI subsystem: Grok process lifecycle と確認済み same-process signal を受ける。Grok 専用 ACP subsystem は作らない。
- `src/client/driver/register.go`、`conformance_test.go`: built-in 登録と registry-driven test。
- runtime launch/container wiring: `UUIDSource`、`SessionLookup`、`GrokHomeResolver`/mount planner を seam とし、global HOME、filesystem、乱数を Driver から直接取得しない。
- 外部依存 seam: CLI process=`CommandRunner`; terminal signal=`TerminalSignalSource`; UUID=`UUIDSource`; session collision=`SessionLookup`; filesystem/home=`GrokHomeResolver`; time=`Clock`。fake は同じ narrow interface を実装する。
- 構造規則: `platform/*` は client を import しない、外部 CLI の persistence type は stdlib-only、PTY bytes は identity metadata の source にしない。

## Verification

| profile | Tier | command | 判定基準 / milestone DoD |
|---|---|---|---|
| launch-policy | T0 pure | `cd src && go test ./platform/lib/grok ./client/driver` | session flag matrix、UUID validation、metadata clear/priority、persist round-trip が通る / m2 |
| driver-wired | T1 wired | `cd src && go test ./client/runtime ./client/driver` | fake process lifecycle、ACP 非起動、failure mapping、GROK_HOME wiring が通る / m3 |
| grok-contract | T2 contract | `cd src && go test -run 'TestGrok.*Contract|TestDriver.*Conformance|TestDriverMetadataSourcePriority' ./...` | invariant 名付き fake contract と registry conformance が通る / m1-m4 |
| fake-vs-real | T3 fidelity | `cd src && AGENT_GRID_REAL_GROK=1 go test -run FakeVsRealGrok ./platform/lib/grok/...` | installed CLI と fake が同一 invariant を満たし user config/session mutation が 0 / m4 |
| architecture | T0/T1 | `make vet && make lint` | depguard、stdlib persistence type、staticcheck が zero error |
| full-suite | T0-T2 | `cd src && go test -race ./...` | race と全 regression が zero failure |

Failure triage: argv duplicate は正規化で消す (i)。pure policy が不可能な lifecycle combination を返した場合は fail fast (ii)。binary/version/process/filesystem は型付き failure と stopped/failed reason で回復または停止する (iii)。terminal signal は characterization 済みのものだけ採用し、未確認 signal から rich status を作らない。
