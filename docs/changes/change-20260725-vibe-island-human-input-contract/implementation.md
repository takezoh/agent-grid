---
change: change-20260725-vibe-island-human-input-contract
role: implementation
contracts:
- contract-human-input-roundtrip
- contract-island-attention-observability
- contract-pending-card-lifecycle
contract_projections:
- id: contract-human-input-roundtrip
  decision_rules: [decision-first-writer-wins, decision-ttl-default, decision-teardown-cancel]
  observable_effects: [observable-resolution-broadcast, observable-agent-single-reply, observable-conflict-normalized]
  operational_inputs: [input-pending-registry, input-client-decision]
  semantic_profiles: []
  failures: [failure-frame-teardown, failure-invalid-response]
  verifications: [verify-gateway-scenario, verify-state-reducer]
  witnesses: [witness-approve-roundtrip-normal, witness-other-client-normal, witness-expiry-normal, witness-race-adversarial]
- id: contract-island-attention-observability
  decision_rules: [decision-autoexpand-on-arrival, decision-toast-gate, decision-quiet-collapse]
  observable_effects: [observable-card-and-badge, observable-toast-once, observable-silent-idle, observable-focus-untouched]
  operational_inputs: [input-supervision-snapshot, input-attention-state]
  semantic_profiles: []
  failures: [failure-ws-disconnect, failure-com-activation]
  verifications: [verify-island-core-t0, verify-toast-router-t0, verify-uia-e2e]
  witnesses: [witness-arrival-normal, witness-unwatched-toast-normal, witness-quiet-normal, witness-frame-flood-adversarial]
- id: contract-pending-card-lifecycle
  decision_rules: [decision-optimistic-remove, decision-submit-outcome, decision-focus-return]
  observable_effects: [observable-card-removed, observable-card-restored, observable-notice-shown, observable-focus-returned]
  operational_inputs: [input-submit-result, input-preengage-foreground]
  semantic_profiles: []
  failures: [failure-submit-network, failure-engage-target-destroyed]
  verifications: [verify-lifecycle-core-t0, verify-engage-focus-t0]
  witnesses: [witness-submit-normal, witness-offline-rollback-normal, witness-race-loss-normal, witness-lifecycle-adversarial]
adrs:
- adr-20260725-vibe-island-asis-contract-pin
- adr-20260725-vibe-island-verification-tiers
decision_dispositions: []
milestones:
- id: chunk-contract-verification
reference_algorithms: []
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Content

正本は [design-plan.json](design-plan.json)。本 change は **production code を変更しない** as-is 契約 pin であり、実装の実体は「witness を既存テスト資産へ帰着させ、欠落 assert のみテストを追加する」ことに閉じる。seam 追加が必要になった時点で本 change を中断し再設計する ([adr-20260725-vibe-island-asis-contract-pin](../../adr/adr-20260725-vibe-island-asis-contract-pin.md))。

## Approach

3 つの implementation contract で decision closure を確定する:

1. **contract-human-input-roundtrip** (integration_contract, owner: `src/server/api` + `src/host/state` + `src/host/runtime/subsystem/stream`) — Codex の保留 JSON-RPC ↔ authoritative registry ↔ クライアント応答の単一解決往復。first-writer-wins / TTL 既定決定 / teardown connection-lost の 3 決定はいずれも単一 writer ループ上の全域決定 (evidence_mode: total)。
2. **contract-island-attention-observability** (user_observability, owner: `AgentGrid.Shell.Core` 純粋層 + `Platform/Toast`) — 自動展開・トースト 1 回・要素非存在・フォーカス非奪取の観察契約。
3. **contract-pending-card-lifecycle** (state_lifecycle, owner: 同上 + `Platform/Engage`) — 楽観除去 → {維持 / 復元 / Handled elsewhere} の全域分割と、EnterEngage で捕捉した HWND への liveness 検証付きフォーカス返却 (operational input continuity の典型例)。

## 外部依存と注入点 (seams)

| 外部依存 | 注入点 (既存 seam) | 検証 |
|---|---|---|
| codex app-server (JSON-RPC) | gateway scenario tests の fake agent / stream helpers 単体 | verify-gateway-scenario |
| gateway WS フレーム (ar/ax/qr/qx/h/v) | `WsFrameParser` 純関数 + fixture | verify-island-core-t0 |
| gateway REST (respond 2xx/409/error) | `ApprovalSubmissionService` への fake gateway 注入 | verify-lifecycle-core-t0 |
| Windows トースト (AppNotificationManager) | `SupervisionToastRouter` への `IToastNotifier` fake 注入 + `ToastXml` 純関数 | verify-toast-router-t0 |
| OS フォーカス / HWND | `EngageFocusService` の liveness 分岐 (Platform.Tests) | verify-engage-focus-t0 |
| 実 UI ツリー | FlaUI/UIA3 AutomationId 契約 (run-dev ハーネス) | verify-uia-e2e |

## Milestone: chunk-contract-verification

依存なしの単一 chunk。3 unit (詳細・acceptance は design-plan.json の `chunks[].units`):

1. **UAC-テスト対応表と verification member の整備** — [verification.md](verification.md) に 14 UAC × Tier × テスト参照の対応表を materialize (テストコードは触らない)。
2. **島側契約テストの整合確認と補強** — contract-island-attention-observability / contract-pending-card-lifecycle の witness を Core.Tests / Platform.Tests へ帰着、欠落 assert を追加。
3. **gateway 往復契約テストの確認と補強** — contract-human-input-roundtrip の witness (単一解決 / TTL / teardown / 再接続非捏造) を Go テストへ帰着、欠落 assert (特に hello 非 replay) を補強。
