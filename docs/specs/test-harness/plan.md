---
id: plan-20260705-test-harness
kind: plan
title: Test harness implementation plan
status: done
created: '2026-07-05'
goal: 4 tier テスト体系 (T0-T3) を正式化し、pty→OSC・surface relay・driver conformance・docker・Go↔TS
  wire の欠落象限を既存パターン (seam + fake + contract + fidelity backstop) の複製で埋め、規範を lint/CI/docs
  に pin する
scope_in:
- src/client/runtime (runtimetest harness, EventSink seam, tap/relay contract)
- src/client/driver + src/client/state (drivertest conformance, FuzzReduce)
- src/platform/agent/fakecodex + src/client/runtime/subsystem/stream/fake (settings/updated)
- src/platform/sandbox/devcontainer (fakedocker)
- src/server/web + src/client/web/src/wire (fixtures pipeline, gateway scenario e2e)
- src/gorules + scripts + .github/workflows (static enforcement, nightly e2e)
- AGENTS.md / docs/note (規範の反映)
scope_out:
- behavioral eval (T4)
- termvt への fake 導入 (real pty posture を維持)
- orchestrator/scheduler 系の再設計
- visual regression / 実機 browser fidelity
milestones:
- id: m1
  title: runtimetest loop harness + EventSink seam
  status: done
- id: m2
  title: pty→OSC tap contract + FuzzParseOsc
  status: done
- id: m3
  title: TerminalRelay severance contract
  status: done
- id: m4
  title: drivertest.Conformance + registry 走査
  status: done
- id: m5
  title: fakecodex/stream fake settings-updated preset
  status: done
- id: m6
  title: Go 生成 wire fixtures + vitest 消費 + CI gate
  status: done
- id: m7
  title: gateway scenario e2e (fake CLI 貫通)
  status: done
- id: m8
  title: fakedocker + real-docker backstop
  status: done
- id: m9
  title: 静的 enforcement 一式 + nightly e2e
  status: done
- id: m10
  title: FuzzReduce
  status: done
- id: m11
  title: 録音駆動 fake (record/replay)
  status: done
- id: m12
  title: AGENTS.md / note 群への規範反映 (最終統合)
  status: done
contracts:
- 'tap OSC routing: frame F の pty 出力由来 OSC event は FrameID==F のみ'
- 'relay severance: slow subscriber は sever、他 subscriber/session の配送順序維持'
- 'driver conformance: Step 純粋性 / DriverEvent totality / Persist-Restore / metadata
  source priority'
- 'wire fixtures: Go 生成 fixture と TS codec 消費の byte 一致 (CI gate)'
- 'fakedocker fidelity: canned 出力 ↔ real docker 出力の形一致 (opt-in)'
tags:
- testing
- harness
owners: []
relations:
- {type: implements, target: spec-20260705-test-harness}
- {type: hasPart, target: adr-20260705-test-tier-taxonomy}
- {type: hasPart, target: adr-20260705-driver-conformance-registry-suite}
- {type: hasPart, target: adr-20260705-eventsink-seam-tap-relay-contracts}
- {type: hasPart, target: adr-20260705-fakedocker-path-injection}
- {type: hasPart, target: adr-20260705-wire-fixtures-pipeline}
- {type: hasPart, target: adr-20260705-view-update-sessions-only}
- {type: hasPart, target: task-20260705-runtimetest-harness}
- {type: hasPart, target: task-20260705-tap-osc-contract}
- {type: hasPart, target: task-20260705-relay-severance-contract}
- {type: hasPart, target: task-20260705-driver-conformance-suite}
- {type: hasPart, target: task-20260705-fakecodex-settings-updated}
- {type: hasPart, target: task-20260705-wire-fixtures-pipeline}
- {type: hasPart, target: task-20260705-gateway-scenario-e2e}
- {type: hasPart, target: task-20260705-fakedocker}
- {type: hasPart, target: task-20260705-static-enforcement}
- {type: hasPart, target: task-20260705-fuzz-reduce}
- {type: hasPart, target: task-20260705-recorded-fake-fixtures}
- {type: hasPart, target: task-20260705-docs-llm-constraints}
source_paths:
- src/client/runtime/
- src/client/driver/
- src/platform/agent/fakecodex/
- src/platform/sandbox/devcontainer/
- src/server/web/
- src/client/web/src/wire/
- src/gorules/
- .github/workflows/ci.yml
summary: 'テストハーネス強化の実装計画: runtimetest 基盤 → 経路別 contract → conformance suite → wire
  fixtures → fakedocker → enforcement の依存順'
updated: '2026-07-05'
---

# Plan — Test harness implementation

## Goal

spec-20260705-test-harness の要件を、**セッション単位で独立に実装可能な 12 task** に分解して実現する。設計判断の
Why は各 ADR (adr-20260705-test-tier-taxonomy ほか 5 本) に、要件は spec に分離済み。本 plan は How / When
のみを扱う。

各 milestone は `docs/task/task-20260705-*.md` の task doc と 1:1 対応し、依存は task frontmatter の
`dependsOn` relation が正本。**依存が無い milestone 同士は別セッションで並行実装できる。**

この plan の browser posture は historical record として保持する。後続で追加された
`client/web` の Playwright browser smoke は別系統の browser wiring harness であり、ここで扱う
Go 側の gateway scenario e2e を置き換えるものではなく補完関係にある。

## Implementation Sequence

依存グラフ (→ は dependsOn):

```
m1 → m2                     (EventSink seam が tap contract の前提)
m6 → m7                     (fixture が gateway e2e の assert 素材)
m5 → m11                    (fake preset 拡張が録音照合の前提)
m1..m9 → m12                (最終ドキュメント統合は実装完了後)
m3, m4, m5, m6, m8, m9, m10 は互いに独立 (並行可)
```

{% milestone id="m1" %}
**runtimetest loop harness + EventSink seam** — `client/runtime/runtimetest` に全 fake backend
注入済み Runtime を `Run(ctx)` で実起動し `Enqueue` / `WaitFor(func(state.State) bool)` / `Quiesce`
を提供する harness を新設。`tap_manager` の enqueue 先を `EventSink` interface (stream backend の
`RuntimeHook` と同型) に seam 化。shutdown timeout / eventCh 満杯 drop の 2 シナリオを新規追加。詳細:
{% task-ref id="task-20260705-runtimetest-harness" /%}
{% /milestone %}

{% milestone id="m2" %}
**pty→OSC tap contract + FuzzParseOsc** — real pty に OSC 0/2/9/133 を書き込み、`EvFrameOsc` /
`EvFramePrompt` の FrameID routing と emulator panic 封じ込めを `-race` 下で pin。`parseOscNotification`
/ `vtPromptPhase` に stdlib fuzz。詳細: {% task-ref id="task-20260705-tap-osc-contract" /%}
{% /milestone %}

{% milestone id="m3" %}
**TerminalRelay severance contract** — subscriber channel 容量をコンストラクタ注入化 (テスト時 1) し、slow
subscriber の sever が他 subscriber / 他 session を阻害しないことを pin。詳細:
{% task-ref id="task-20260705-relay-severance-contract" /%}
{% /milestone %}

{% milestone id="m4" %}
**drivertest.Conformance + registry 走査** — `client/driver/drivertest` に共通契約 suite を新設し、登録済み全
driver へ自動適用。metadata source priority (adr-20260705-metadata-source-priority) を driver
ごとに authoritative source を差し替えるパラメタライズド契約として実装。詳細:
{% task-ref id="task-20260705-driver-conformance-suite" /%}
{% /milestone %}

{% milestone id="m5" %}
**fakecodex/stream fake settings-updated preset** — `thread/settings/updated` を stdio (fakecodex) /
WS (stream/fake) 両系統の preset に追加し、FakeVsReal e2e で real codex の emit と照合。merge 71d05a4
で顕在化した fake drift の解消。詳細: {% task-ref id="task-20260705-fakecodex-settings-updated" /%}
{% /milestone %}

{% milestone id="m6" %}
**wire fixtures pipeline** — `server/web` の Go テストが viewUpdate / output / control の canonical
JSON fixture を `client/web/src/wire/testdata/` に生成、`codec.test.ts` が同一ファイルを消費。CI に再生成 +
`git diff --exit-code` gate を追加し、手書き `fixtures.ts` を置換。詳細:
{% task-ref id="task-20260705-wire-fixtures-pipeline" /%}
{% /milestone %}

{% milestone id="m7" %}
**gateway scenario e2e** — `mux_e2e_test.go` の方式を拡張し、fakeclaude / fakecodex を agent CLI とした
real server binary に対して session create → WS viewUpdate 受信を assert。詳細:
{% task-ref id="task-20260705-gateway-scenario-e2e" /%}
{% /milestone %}

{% milestone id="m8" %}
**fakedocker + real-docker backstop** — PATH 注入 fake docker CLI で devcontainer lifecycle を
T1 検証し、`REACTOR_E2E_DOCKER_BIN` opt-in の FakeVsRealDocker を追加。詳細:
{% task-ref id="task-20260705-fakedocker" /%}
{% /milestone %}

{% milestone id="m9" %}
**静的 enforcement + nightly e2e** — ruleguard による real CLI exec の隔離、`//go:build e2e` の sibling
fake テスト存在 check script、nightly fidelity workflow、coverage-floors の fakecodex 行補修。詳細:
{% task-ref id="task-20260705-static-enforcement" /%}
{% /milestone %}

{% milestone id="m10" %}
**FuzzReduce** — ランダム event 列を `state.Reduce` に fold し大域不変条件を assert する stdlib fuzz。詳細:
{% task-ref id="task-20260705-fuzz-reduce" /%}
{% /milestone %}

{% milestone id="m11" %}
**録音駆動 fake** — T3 実行時に real CLI のイベント列を testdata へ録音し、fake preset との照合と T0 golden
replay に使う record/replay 基盤。詳細: {% task-ref id="task-20260705-recorded-fake-fixtures" /%}
{% /milestone %}

{% milestone id="m12" %}
**規範のドキュメント統合** — AGENTS.md へ LLM 向け規範 4 行、enforcement note へ test-pin 節 (§8 tap routing /
§9 relay severance / §10 docker fidelity / §11 wire fixtures / §12 metadata priority)、testing note へ
Tier 体系を反映。実装が存在しない規範を先に書かないため最後に実施。詳細:
{% task-ref id="task-20260705-docs-llm-constraints" /%}
{% /milestone %}

## Targets

- 新設 package: `src/client/runtime/runtimetest/`, `src/client/driver/drivertest/`,
  `src/platform/sandbox/devcontainer/fakedocker/`
- seam 変更: `src/client/runtime/tap_manager.go` (EventSink), `src/client/runtime/terminal_relay.go`
  (channel 容量注入)
- fake 拡張: `src/platform/agent/fakecodex/presets.go`, `src/client/runtime/subsystem/stream/fake/appserver.go`
- wire: `src/server/web/wire_fixtures_test.go` (新規), `src/client/web/src/wire/testdata/` (新規),
  `src/client/web/src/wire/codec.test.ts` / `fixtures.ts` (置換)
- enforcement: `src/gorules/` (ruleguard 追加), `scripts/` (sibling check), `.github/workflows/`
  (nightly), `scripts/coverage-floors.txt`
- 再利用する既存パターン: stream backend の `RuntimeHook` seam + `recordingRuntime`、termvt の real-pty
  contract 流儀、`e2etest.PrepareCodexHome` の isolated-HOME bootstrap、codex-schema-check の
  regen+diff 方式、`mux_e2e_test.go` の real-binary spawn

## Verification

- 各 task の DoD は task doc に記載 (テスト green + `make lint` + 該当 note 更新)
- 受け入れ条件の正本は spec の `acceptance[]` (AC-001〜AC-006)
- 横断確認: `cd src && go test ./...`、`make test-race`、`make lint`、`scripts/check-coverage.sh`、
  `python3 <docs_cli> lint`
- T3 系 (m5 / m7 の e2e 面 / m8 backstop / m11) は `make test-e2e` + 各 `REACTOR_E2E_*` env での
  local 実行、および m9 の nightly workflow で検証


{% transition from="draft" to="active" date="2026-07-05" %}
All implementation tasks have landed and are being closed.
{% /transition %}


{% transition from="active" to="done" date="2026-07-05" %}
All 12 milestones are implemented and verification gates are present.
{% /transition %}
