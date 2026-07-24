---
id: note-20260715-test-harness-north-star-hardening-roadmap
kind: note
title: Test harness north-star hardening roadmap
status: draft
created: '2026-07-15'
tags:
- testing
- harness
- roadmap
owners: []
relations: []
source_paths:
- test-harness/
- src/internal/harnesspolicy/
- .github/workflows/
- scripts/
- clients/ui/
summary: 2026-07-15再監査で判明したfalse green、profile drift、Web silent failureをM1-M7で解消する実装roadmap
updated: '2026-07-24'
---

## Summary

2026-07-15 の再入監査では、T0〜T3、race/fuzz/coverage/mutation、skip inventory、anti-tampering まで既存資産が揃う一方、次の false green を観測した。

- dependency admission が marker 存在だけを検査し、fake でない test symbol や T3 でない production symbol を triple として受理する。
- nightly が Grok binary を必須化する一方、実行 package matrix に Grok を含めない。
- local verification profile と CI が別々に command を所有し、AC-008 の command-level artifact を生成しない。
- Web unit test が CodeMirror plugin crash と React act warning を stderr に出しても green になる。

本 roadmap は既存の模範象限（stream routing / fakecodex / fakedocker）の契約共有・negative fixture・static pin を複製し、全経路を fail-closed にする。

## North-star

外部依存は名前ではなく executable な fake / named contract / real fidelity の結線で admission される。local と CI は同じ verification profile を実行し、command ごとの結果・時間・skip 理由を共通 artifact に残す。T3 package matrix、unexpected test console、flaky retry、coverage/mutation baseline、docs/version pin の drift は static policy が検知する。

## Milestones

| M | 状態 | 対策 | Exit criteria | 規模 |
|---|---|---|---|---|
| M1 | done | skip inventory / static pin の現行 RED を復旧 | `go test ./...` と skip checker が green。追加 skip は理由・owner・期限・evidence を持つ | 中 |
| M2 | done | T3 suite registry を SSOT 化 | Makefile / nightly / reporter が同じ package matrix を使用し Grok を含む。欠落の negative test が red→green | 中 |
| M3 | done | dependency admission v2 | AST で test/fake/build-tag/env/shared-suite を検証し、未登録 external callsite と偽 triple を拒否 | 大 |
| M4 | done | verification profile の local/CI 共通化 | profile が全 PR gate を列挙し、CI は profile contract を再利用。command-level artifact を生成 | 大 |
| M5 | done | Web silent failure / flaky 防御 | unexpected console error/warn、plugin crash、retry 依存を fail。意図的 console は test 単位 allowlist | 中 |
| M6 | done | docs milestone / Codex pin drift 解消 | plan milestone status が実態と一致し、schema pin と runtime fidelity pin が別々に静的検証される | 中 |
| M7 | done | Web coverage / mutation 検出力 | package group 別 coverage floor と critical-path mutation baseline が PR gate になる | 大 |

## Re-entry

先頭の未完了 milestone から再開し、各 exit criteria をコマンド出力で照合する。観測と状態が食い違う場合は状態を進めず drift を先に修復する。

## Web coverage provider selection

M7 では Vitest 2.1.8 と同一 version の `@vitest/coverage-v8` を採用する。

- `@vitest/coverage-v8`: Vitest native、V8 の実行 coverage をそのまま使い、既存 test command と設定を共有できる。追加依存と変換誤差が最小。
- `@vitest/coverage-istanbul`: source instrumentation により詳細な source map 制御ができる一方、この project の SWC/Vite pipeline に instrumentation 層と実行時間を追加する。
- `c8`: 小さく成熟しているが、Vitest native threshold/report lifecycle を再実装する glue script が必要になる。

既存 API への適合、保守面、CI artifact の標準化を優先し V8 provider を選ぶ。mutation は既存の deterministic manifest runner を拡張する。StrykerJS は operator 探索には強いが、大きな dependency graph と非決定的な全探索を PR gate に持ち込むため baseline 更新用の候補に留め、PR は identity-pinned must-kill mutant を使う。

## Verification evidence

- 2026-07-15: `go test ./...` は skip inventory と gateway static pin で RED。
- 2026-07-15: `scripts/check-harness-dependencies.sh` は偽 triple を含む 11 rows を GREEN として受理。
- 2026-07-15: `npm run test:web` は green だが CodeMirror plugin crash と React act warning を出力。
- 2026-07-15: M1〜M4 の registry/checker/profile を実装し、policy unit と共通 runner を green 化。
- 2026-07-15: Vitest 1,764 tests と Playwright 5 tests が unexpected console/pageerror gate、retry 0 で green。
- 2026-07-15: schema bundle は Codex 0.133.0 と byte-semantic 一致し、0.128.0 とは不一致。schema 0.133.0 / fidelity 0.142.5 を別 role として SSOT 化。
- 2026-07-15: Web coverage は lines/statements 92.21%、functions 89.75%、branches 87.78%。全体および protocol/state/UI/core group floor を通過。
- 2026-07-15: deterministic mutation は 5/5 must-kill、score 1.0。Web codec と workspace conflict resolution の二つの critical path を含む。
- 2026-07-15: `go test ./...`、Go/Web lint、race、build-all、dependency/skip/e2e-vet、`npm run test:web` は green。Go coverage 全体は対象外の未コミット `platform/mcpoverlay` 変更が 88.9%（floor 90%）のため保留。
- 2026-07-15: security follow-up で Vitest / coverage-v8 4.1.10、happy-dom 20.10.6、peer contract に必要な Vite 7.3.6 / react-swc plugin 4.3.1 へ更新。`npm audit` は 5 vulnerabilities から 0 になった。
- 2026-07-15: Vitest 4 の AST-based V8 coverage では計測分母が変更され、同一 1,764 tests で lines 89.56%、statements 86.79%、functions 90.01%、branches 76.96% となる。Vitest 2 の lines/statements 92.21%、functions 89.75%、branches 87.78% と直接比較せず、新モデル実測値から1〜2ポイント下に全体・四群 floor を再基準化した。
