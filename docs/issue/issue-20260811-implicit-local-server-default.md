---
id: issue-20260811-implicit-local-server-default
kind: issue
title: Decide whether the implicit local-server default in App.tsx violates the ARCHITECTURE
  ban
status: open
created: '2026-08-11'
issue_type: plan
tags:
- fleet
- ui
- architecture
owners: []
relations:
- {type: originatedFrom, target: change-20260811-fleet-session-id-ssot}
subject_paths:
- clients/ui/src/App.tsx
- clients/ui/src/fleet/provider.ts
summary: design decision-6 measured that the local sentinel is transport-binding only
  and never enters the session id space, but whether the fallback itself is allowed
  under the implicit-default-server ban in ARCHITECTURE.md is still undecided.
---

## 症状 / 機会

`App.tsx` の serverId 解決が最終段で `?? "local"` に落ちる。
fleet 構成に `local` という server が無くてもこの既定値が生まれるため、ARCHITECTURE.md が禁じる
implicit default server に当たるかどうかを判断したい。

## 証拠

- `clients/ui/src/App.tsx:80-85` — `parseSessionKey(activeSessionID)?.serverId ?? activeSessionServerID`、
  無ければ `fleet.initialServerId()`、それも無ければ `"local"`。
- design decision-6 (change-20260811-fleet-session-id-ssot) が静的 call graph で実測した到達性:
  1. HTTP routing の serverId は `parseSessionKey` か `fleet.initialServerId()` から得るが、後者
     (`clients/ui/src/fleet/provider.ts:237-242` の `explicitServerId`) は `hostedModeInfo` か `?server=` を返し、
     無ければ `null` を返す (`"local"` は返らず `api/sessions.ts:472` が "No server selected" を throw する)。
  2. store 行の serverId は `App.tsx:134-141` が `FleetSnapshot` の `ref.serverId` から作る。
  3. `"local"` の唯一の consumer は `webProvider.activeConnection` (`provider.ts:441-452`) の
     `serverId === "local"` 明示分岐で、same-origin の `/api/ws-ticket` と `/ws` を選ぶ。
  4. その endpoint はどの shipping host にも存在しない (web-host は `/` と `/host/...` のみ、
     `services/server` に SPA 配信 handler なし、vite dev に `/api` proxy なし)。
- したがって `"local"` は transport binding 専用の sentinel であり、session id 空間には入らない。
- 限界: runtime 観測ではなく静的 call graph の実測。
- falsifier: 同一 origin で SPA と gateway を配信する host 構成が追加されると、`"local"` 接続と
  bare-id 素通し経路が同時に活性化する。

## 着手に必要な文脈

- 発見元: change-20260811-fleet-session-id-ssot (到達性は design decision-6 で実測済み。
  ARCHITECTURE.md への抵触判断そのものは non_goals として切り出した)。
- 判断に要るもの: `ARCHITECTURE.md` の implicit default server 禁止の文言と、上の falsifier が
  将来構成で成立しうるかどうか。
- 抵触すると判断する場合、`"local"` sentinel を消すか、transport binding 専用であることを型で表現するかの
  選択が生じる (どちらを採るかは着手セッションの仕事)。
