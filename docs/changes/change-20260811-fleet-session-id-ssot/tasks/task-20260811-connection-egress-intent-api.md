---
id: task-20260811-connection-egress-intent-api
kind: task
title: connection-egress-intent-api
status: done
created: '2026-08-11'
priority: normal
effort: medium
files_touched:
- clients/ui/src/socket/connection.ts
- clients/ui/src/socket/retry.ts
- clients/ui/src/socket/retry.test.ts
- clients/ui/src/socket/connection.fleet-input.test.ts
- clients/ui/src/wire/client.ts
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260811-fleet-session-id-ssot}
- {type: dependsOn, target: task-20260811-identity-owner-module}
source_paths: []
change: change-20260811-fleet-session-id-ssot
summary: wire 境界の所有者を API 形状で確定し、Connection の外に frame 組み立て権威が残る経路 (retry.ts の bypass
  を含む) をすべて閉じる。
max_diff_loc: 300
pinned_context:
- docs/changes/change-20260811-fleet-session-id-ssot/tasks/task-20260811-connection-egress-intent-api.md
- /workspace/agent-grid/docs/changes/change-20260811-fleet-session-id-ssot/design-plan
updated: '2026-08-11'
---

## Responsibility

wire 境界の所有者を API 形状で確定し、Connection の外に frame 組み立て権威が残る経路 (retry.ts の bypass を含む) をすべて閉じる。

## Execution contract

- Output: Connection の public API 変更、retry.ts の直列化削除、T1 test。
- Tool guidance: 既存の upstreamSessionID / subscribeOnce / unsubscribeOnce / publishDesired の throw 契約を維持したまま data plane だけ別契約にする。connection.ts:192 の `send: (s) => this.ws?.send(s)` を `send: (f) => this.sendFrame(f)` へ置き換える。grep -rn serializeClientFrame clients/ui/src で production hit が socket/connection.ts だけになることを確認する。
- Boundaries: TerminalPane の呼び出し側の移行は次の unit。store / api は触らない。counter の公開先を Connection の read-only getter 以外へ広げない。

## Acceptance

- data plane の intent API (sendInput(fleetKey, data) / sendResize(fleetKey, cols, rows)) が追加され、fleet key を受け取り upstream id を wire に載せる
- send(frame) が private になり Connection の外から呼べなくなる。直列化は connection.ts の private sendFrame 1 箇所に戻る
- RetryDeps.send が (frame ClientFrame) => void になり、retry.ts は serializeClientFrame を import せず直列化も持たない
- serverId 不一致の入力で例外が出ず frame も出ず、server-mismatch counter だけが増え notification は 0 件であることが test で固定される
- decode 不能な入力で decode-failed counter が増え notification が (sessionId, reason) ごとに 1 lifecycle 1 件だけ出ることが test で固定される
- bare id を渡したときの素通しが保たれる
- 既存 RED connection.fleet-input.test.ts の 3 つの witness (fleet key→bare id 変換 / resize / bare id 素通し) を保存したまま intent API 形へ書き換えて緑になる
- 制御シーケンス ("\u001b[3~" など) が verbatim で wire に載る


{% transition from="todo" to="in_progress" date="2026-08-11" %}
delivery/fleet-session-id-ssot
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-11" %}
Implemented on delivery/fleet-session-id-ssot; unit suite 1918/1918 green, real-stack e2e 3/3 green.
{% /transition %}
