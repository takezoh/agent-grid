---
id: issue-20260811-fleet-identity-harness-followups
kind: issue
title: Bind the workspace session-ref mirror to a canonical fixture and make the real-stack
  terminal spec a standing gate
status: open
created: '2026-08-11'
issue_type: plan
tags:
- workspace
- test-harness
- e2e
owners: []
relations:
- {type: originatedFrom, target: change-20260811-fleet-session-id-ssot}
subject_paths:
- clients/workspace/src/shared/session-ref.ts
- clients/ui/e2e/real-stack/terminal-input.spec.ts
- test-harness/profiles.json
summary: 'Three harness follow-ups carved out of change-20260811-fleet-session-id-ssot:
  constrain the second fleet-key encoder with a canonical fixture, add the real-stack
  terminal spec to the nightly profile, and strengthen its discriminator.'
---

## 症状 / 機会

change-20260811-fleet-session-id-ssot から切り出した harness 3 件。
第 2 の fleet-key encoder を canonical fixture で拘束し、real-stack の terminal 入力 spec を常設 gate にし、
その判別子を強める。

## 証拠

- **第 2 encoder**: `clients/workspace/src/shared/session-ref.ts` が `clients/ui/src/fleet/session-protocol.ts`
  とは独立に fleet key の encode を実装している。現在は両者の一致を拘束する fixture が無い。
  change-20260811 は encoding 自体を変えないため drift は活性化していないが、拘束は無いままである。
- **real-stack spec の常設先**: `clients/ui/e2e/real-stack/terminal-input.spec.ts` は本 change で
  GREEN 化されるが、常設先は未定。T3 の唯一の常設先は `test-harness/profiles.json` の nightly profile
  (pr profile に real-stack は無い)。当該 file は `test-harness/protected.json` の tamper 保護対象
  (required_markers pre-push / nightly) なので、追加は trusted harness gate を通る変更になる。
- **判別子**: 現行 spec は入力後の出力を substring で待つ。`echo:` で始まる形へ強めると、
  偶然の一致で緑になる余地が減る (debug run で一度 false green を出した経緯がある)。

## 着手に必要な文脈

- 発見元: change-20260811-fleet-session-id-ssot (3 件とも non_goals に切断点付きで記録済み)。
- 既知の制約: `AGENTS.md` の E2E 3 種の区別 (Go gateway scenario / Go opt-in fidelity / Web Playwright smoke /
  Web UI/UX real stack)、`docs/note/note-20260624-agent-testing.md` の T0-T3 tier。
- real-stack harness は 1 worker 直列で約 16s (debug run 実測)。常設先の判断はこの実行時間を前提にする。
