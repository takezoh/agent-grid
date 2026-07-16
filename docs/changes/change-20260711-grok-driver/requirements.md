---
change: change-20260711-grok-driver
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260711-grok-driver
kind: spec
title: Grok Driver
status: draft
created: '2026-07-11'
tags: [grok, driver]
owners: []
functional_requirements:
- id: FR-001
  statement: システムはGrok frame が存在する間、session identity、status、model、reasoning effort と各 metadata の unset/set/cleared および source authority を観測可能にしなければならない
  priority: must
- id: FR-002
  statement: 新規 Grok session を作成したとき、システムはruntime が生成した UUID を `--session-id` に指定し、initial input を既存 PTY launch 契約で interactive `grok` TUI に渡さなければならない
  priority: must
- id: FR-003
  statement: persisted session ID を持つ stopped root frame を cold start したとき、システムは`--resume <ID>` で同じ会話を再開しなければならない
  priority: must
- id: FR-004
  statement: current directory の最新 session を明示して再開したとき、システムは値を付けない `--continue` を保持しなければならない
  priority: must
- id: FR-005
  statement: session を fork したとき、システムは`--resume <ID> --fork-session` を使用し、親 session を変更してはならない
  priority: must
- id: FR-006
  statement: もし base command の session flag と requested lifecycle が競合する場合、システムは重複 flag を生成してはならず launch 前に診断可能なエラーを返さなければならない
  priority: must
- id: FR-007
  statement: Grok process lifecycle または実機 characterization 済みの同一 process signal が変化したとき、システムはidle/running/stopped/failed と確認済み coarse status だけを表示しなければならない
  priority: must
- id: FR-008
  statement: もし terminal signal が未確認または process が失敗した場合、システムはVT 本文から identity/model/effort/rich status を推測してはならず terminal output を保持した stopped/failed status と reason を表示しなければならない
  priority: must
- id: FR-009
  statement: Driver state を persist/restore したとき、システムはsession ID、metadata tri-state、authority、Status と View を round-trip しなければならない
  priority: must
- id: FR-010
  statement: automation launch を作成したとき、システムは`--no-auto-update` を重複なく指定しなければならない
  priority: must
- id: FR-011
  statement: host または container で Grok を起動したとき、システムは同じ `GROK_HOME` session namespace を読み書き可能にし、`config.toml` を生成または上書きしてはならない
  priority: must
- id: FR-012
  statement: Grok Driver を built-in registry に登録したとき、システムは手動 allowlist なしで共通 Driver conformance の対象に含めなければならない
  priority: must
non_functional_requirements:
- id: NFR-001
  type: maintainability
  criteria: Driver.Step、argv policy、metadata precedence は I/O、goroutine、global mutation を行わない
  measurement: T0 pure tests と go test -race
- id: NFR-002
  type: reliability
  criteria: Grok CLI/process lifecycle 外部境界ごとに fake、invariant 名付き contract、FakeVsRealGrok を同一 scenario で提供する
  measurement: T1/T2 は通常 suite、T3 は明示 opt-in で実行
- id: NFR-003
  type: security
  criteria: 実機検証は isolated GROK_HOME を使い、既存 credentials、config、sessions を変更または削除しない
  measurement: filesystem mutation contract test
- id: NFR-004
  type: compatibility
  criteria: wire/persistence type は stdlib-only、three-layer import direction を維持し、argv は agentlaunch.SplitArgs で token-aware に扱う
  measurement: make lint、make vet、go test
acceptance:
- id: AC-001
  given: Grok Driver の新規 frame と固定 UUID、initial input がある
  when: launch plan を作成する
  then: PTY command は grok、--session-id UUID、--no-auto-update を一度ずつ含み initial input を保持する
  requirement_refs: [FR-002, FR-010]
- id: AC-002
  given: persisted Grok session ID を持つ stopped root frame がある
  when: cold start recovery を行う
  then: --resume ID を用い --session-id を用いず同じ GROK_HOME namespace を参照する
  requirement_refs: [FR-003, FR-011]
- id: AC-003
  given: current directory latest resume または fork が要求される
  when: launch plan を作成する
  then: 前者は値なし --continue、後者は --resume ID --fork-session になる
  requirement_refs: [FR-004, FR-005]
- id: AC-004
  given: interactive Grok TUI を起動する
  when: runtime が subsystem を選択して process lifecycle を Driver に通知する
  then: "`grok agent stdio` ACP process/bind は一切起動せず、未確認 signal や VT 本文から rich status、identity、model、effort を捏造しない"
  requirement_refs: [FR-001, FR-007, FR-008]
- id: AC-005
  given: isolated fake と installed grok がある
  when: 同じ launch/process lifecycle contract scenario を実行する
  then: invariant が一致し、不一致時は assertion を緩めず fake drift として検出する
  requirement_refs: [FR-010, FR-012, NFR-002]
relations:
- {type: implementedBy, target: plan-20260711-grok-driver}
source_paths: []
methodology: sdd
summary: 公式 Grok Build CLI を PTY TUI と process lifecycle 観測で安全に起動・再開・永続化する Driver 契約
---

## Overview

公式 Grok Build CLI (`grok`) を組み込み Driver として扱う。interactive TUI は既存 PTY/CLI subsystem で起動し、status は process lifecycle と実機確認済みの同一 process OSC/window-title signal（存在する場合）だけから得る。model/effort は launch seed と persisted state を扱う。fresh、current-directory continue、ID resume、fork を別契約として扱い、ユーザーの `GROK_HOME/config.toml` は変更しない。

## Requirements

{% req id="FR-001" %}Grok frame の存続中、identity、status、model、effort、値の有無と authority を常時表示できる。{% /req %}
{% req id="FR-002" %}fresh launch は runtime 注入 UUID、PTY、initial input を使う。{% /req %}
{% req id="FR-003" %}cold start は persisted ID を `--resume` する。{% /req %}
{% req id="FR-004" %}current directory latest は値なし `--continue` を使う。{% /req %}
{% req id="FR-005" %}fork は `--resume ID --fork-session` を使う。{% /req %}
{% req id="FR-006" %}競合 session flags は launch 前に拒否する。{% /req %}
{% req id="FR-007" %}process lifecycle と確認済み same-process signal だけを status に使う。{% /req %}
{% req id="FR-008" %}外部 failure を成功扱いせず、VT 本文からの metadata/rich status 推測を禁止する。{% /req %}
{% req id="FR-009" %}persist/restore で表示契約を round-trip する。{% /req %}
{% req id="FR-010" %}automation は `--no-auto-update` を使う。{% /req %}
{% req id="FR-011" %}`GROK_HOME` namespace を維持し config を変更しない。{% /req %}
{% req id="FR-012" %}registry conformance に自動加入する。{% /req %}

Failure modes は、外部 CLI 不在/unsupported version、argv conflict、process exit、session identity mismatch、GROK_HOME inaccessible を区別する。外部由来は stopped/failed と reason を公開し、内部の不可能な lifecycle 組合せは fail fast、既存 `--no-auto-update` の重複は argv 正規化でエラー自体を消す。

## Acceptance Criteria

{% acceptance id="AC-001" %}Given fresh frame, when launch, then UUID/auto-update/initial input が保持される。{% /acceptance %}
{% acceptance id="AC-002" %}Given persisted ID, when cold start, then ID resume と同じ home を使う。{% /acceptance %}
{% acceptance id="AC-003" %}Given continue/fork, when launch, then公式 flag 組合せになる。{% /acceptance %}
{% acceptance id="AC-004" %}Given interactive TUI, when launch/status update, then ACP を起動せず未確認 signal から rich state を作らない。{% /acceptance %}
{% acceptance id="AC-005" %}Given fake/real, when contract, then同一 invariant を満たす。{% /acceptance %}

## Non-Goals

Grok CLI/API の再実装、session file の非公開 format parsing、plugin/leader/dashboard/memory/worktree の UI 化、ユーザー config の migration は行わない。`grok agent stdio` ACP は別 agent process なので初回 Driver では起動しない。ACP を primary UI transport として TUI を置換する場合は別設計とする。

````
