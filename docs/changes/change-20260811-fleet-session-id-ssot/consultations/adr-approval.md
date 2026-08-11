---
change: change-20260811-fleet-session-id-ssot
role: consultation
---

<!-- lifecycle is owned by change.md -->

# ADR approval consultation

`consultation-fleet-session-id-ssot-1` — design consultation の typed record。

## Decision

2026-08-11、ユーザーは proposed だった 3 件の ADR すべてを accepted とした。判定は
`consultation.schema.json` の `adr_decisions[]` として型付けされ、いずれも
`kind: accept` / `source: user_consultation` / `confidence: confirmed`。free_text も reject も無く、
reconcile は `rework_required: false` で完了した。

- `adr-fleet-session-identity-boundary-ownership`
- `adr-ipc-response-envelope-null-body-ownership`
- `adr-defer-branded-session-id-types`

3 件とも `design-plan/spine.yaml` の `adrs[].status` が `proposed` → `accepted` へ遷移済み。

## 先行する選択 (debug handoff)

この change は debug run `debug-run-workspace-electron-terminal-dead` の handoff を起点とする。
fix size が「大」と機械判定されたため fixer は起動されず、根本原因ごとに選択肢が提示された。
ユーザーは 2026-08-11 に次を選んだ:

- **根本原因 A (fleet session id 変換の所有者不在)** — 局所修正 (`Connection.send()` に変換を 1 つ足す)
  ではなく **SSOT 統合**。理由は同一機構への 2 回目の修正であり、局所修正の継続が 3 回目を構造的に招くこと。
- **根本原因 B (Electron IPC response envelope の不変条件欠落)** — **推奨案**。不変条件の所有者を
  main 側 IPC envelope 型 (`body: string | null`) に置き、renderer 側の分岐は所有者ではなく二重化として扱う。

## follow-up の起票

同じ相談で、design が scope 外へ切り出した 3 件の issue 起票が承認された。

- `issue-20260811-gateway-silent-session-drop`
- `issue-20260811-implicit-local-server-default`
- `issue-20260811-fleet-identity-harness-followups`

## Typed record

判定の原本は同じ directory の [`consultation-fleet-session-id-ssot-1.json`](consultation-fleet-session-id-ssot-1.json)
(`consultation.schema.json` 適合、schema 検証済み)。
