---
change: change-20260726-unified-driver-session-status
role: ux
phase: explore
status: draft
goal: OSC に依存しない全 Driver 共通状態と、三クライアントで統一された状態表示の観察契約を確定する。
target_users:
- 複数の agent Driver を切り替えて使う利用者
- Windows Shell・Workspace・Web UI でセッションを監視する利用者
- 状態表示を支援技術で確認する利用者
technology_candidates:
- id: candidate-shared-status-spec
  candidate: 検証可能な共有 status/icon specification
  source: planner_proposal
  proposed_for:
  - 三クライアントの表示整合
  required_capabilities:
  - 状態、形状、animation、アクセシブル名称の対応を表現できる
  ux_constraints_relevant_to_evaluation:
  - Driver 名で表示を分岐しない
  - 同じ状態は同じ意味で観察できる
  design_questions:
  - native UI と Web の生成・検証境界をどこに置くか
  disqualifiers:
  - client ごとに意味や名称が独立して drift する
- id: candidate-codex-message-inventory
  candidate: Codex app-server protocol message inventory
  source: planner_proposal
  proposed_for:
  - 既知 message の網羅分類と未知 message の互換性処理
  required_capabilities:
  - notification、request、response、lifecycle、elicitation、tool proposal を分類できる
  ux_constraints_relevant_to_evaluation:
  - 未知 message を idle として黙って受理しない
  design_questions:
  - protocol 追加をどの gate で検知するか
  disqualifiers:
  - default 分岐で未知 message を捨てる
design_handoffs:
- id: handoff-shared-status-spec
  required_outcome: 三クライアントが同じ状態意味、アイコン、animation、アクセシブル名称を表示できる検証境界を定める。
  design_obligations:
  - native stack の違いを越えて共有できる status/icon specification を評価する。
  verification_obligations:
  - 三クライアントで同じ status matrix とアクセシブル名称を照合する。
  provenance:
    source: explicit_user_requirement
    confidence: confirmed
    evidence_refs:
    - user:unified-icons
    rationale: 表示統一は確認済みだが実装方式は design が決める。
- id: handoff-message-inventory
  required_outcome: notification、server request、response、lifecycle、elicitation、tool proposal を分類し、protocol 追加時の未分類を検出する。
  design_obligations:
  - 既知 message inventory と未知 message の compatibility boundary を定める。
  verification_obligations:
  - fake、FakeVsReal、invariant-naming contract test で分類網羅性を検証する。
  provenance:
    source: explicit_user_requirement
    confidence: confirmed
    evidence_refs:
    - user:all-codex-app-server-events
    rationale: 全 app-server event の正確な処理を実現する設計入力。
---

# Goal

OSC 0/2 title と OSC notification による agent 状態推定を廃止し、全 Driver の観測を共通の利用者向け意味へ正規化する。Windows Shell・Workspace・Web UI は同じ状態に同じアイコンとアクセシブル名称を表示する。進行、要応答、失敗は Codex TUI の spinner、bell、`!` に着想を得る。

確認済みの制約は、OSC 状態推定の削除、Codex app-server の notification・server request・response・lifecycle・elicitation・tool proposal の分類、三クライアントの表示統一、全 Driver への共通定義適用である。端末描画用 VT 処理と Shell の OSC 133 prompt semantics の一律削除、agent 固有 raw event の client 公開、新しい通知 channel や設定は対象外とする。

<!-- requirements-decisions:start -->
## Decision State
Selected alternative: ALT-COMPOSITE
Decision DP-CONCURRENT-ATTENTION: unresolved
Recommendation DP-CONCURRENT-ATTENTION: OPT-ATTENTION-COMPOSITE
Decision DP-FAILURE-BOUNDARY: unresolved
Recommendation DP-FAILURE-BOUNDARY: OPT-FAIL-SESSION
Decision DP-SHELL-OBSERVATION: unresolved
Recommendation DP-SHELL-OBSERVATION: OPT-SHELL-REDUCED
Decision DP-STATUS-GRANULARITY: OPT-STATUS-FACETS
Decision DP-UNKNOWN-DISPLAY: unresolved
Recommendation DP-UNKNOWN-DISPLAY: OPT-UNKNOWN-ADJUNCT
<!-- requirements-decisions:end -->

# User Goals

- 任意の Driver とクライアントで、進行、要応答、停止、失敗、不明を同じ意味として判別できる。
- OSC の副作用や未分類 message によって状態が idle へ誤変換されない。
- human-input request、elicitation、tool proposal を visual と支援技術で発見し、keyboard と pointer の双方で応答地点へ到達できる。

# UX Alternatives

## ALT-EXCLUSIVE: 排他的な単一 status

一時点に一つの icon と状態名だけを表示する。既存 wire contract と連続しやすいが、running と要応答、既知状態と未知観測が重なった場合に優先順位を決める必要がある。この案は deferred とする。

## ALT-COMPOSITE: 基底 status と注意 indication

activity・lifecycle と attention を別軸として保持し、spinner、bell、`!`、または既知状態と未知 indication を組み合わせて伝える。ユーザー確認済みの primary alternative であり、wire contract と三クライアントの表示構造への影響を Specify と design で評価する。

# Decision Matrix

| 観点 | ALT-EXCLUSIVE | ALT-COMPOSITE |
|---|---|---|
| 既存単一 Status との連続性 | 高い | schema 変更の可能性がある |
| 同時 activity と要応答 | 優先順位で一方を表示 | 双方を観察可能 |
| 既知状態と未知観測 | 主 status の切替が必要 | 併記可能 |
| client 実装の単純さ | 比較的単純 | 複数 indication の整合が必要 |

# Recommended Direction

ユーザー確認済みの ALT-COMPOSITE と直交 facets を採用する。bell は notification signal、spinner と `!` は排他的な主状態として扱い、bell と `!` は併存可能とする。ただし、この観測契約は既存の DP-CONCURRENT-ATTENTION options では表現できず、`!` を failure とみなす DP-FAILURE-BOUNDARY の前提も否定するため、両 decision の options を改訂して再相談する。

# Unresolved Product Decisions

1. DP-CONCURRENT-ATTENTION の options を、spinner と `!` の排他性、bell と `!` の併存を表現できるよう改訂して再相談する。
2. DP-FAILURE-BOUNDARY の options を、`!` は未解決 human-input request、failure は別表示という境界を表現できるよう改訂して再相談する。
3. Shell を process lifecycle で写像するか、観測不能を unknown とするか、表現可能な subset の縮退 profile とするか。
4. 未知 Codex message の受信時に主 status を unknown へ切り替えるか、最後の既知状態と不明 indication を併記するか。

# Technology Candidates for Design Evaluation

- `candidate-shared-status-spec`: 状態、形状、animation、アクセシブル名称を三クライアントで照合できる共有 specification。client ごとに意味や名称が drift する方式は不適格。
- `candidate-codex-message-inventory`: notification、request、response、lifecycle、elicitation、tool proposal を分類し、未知 method を default で捨てない inventory。
- `handoff-shared-status-spec`: native stack の違いを越えて同じ意味を検証する境界を design で決める。
- `handoff-message-inventory`: protocol 追加時の未分類検出、fake、FakeVsReal、contract test の配置を design で決める。

# Validation Questions

- DP-CONCURRENT-ATTENTION の改訂 options で、spinner と `!` の排他性、および bell と `!` の併存を再確認する。
- DP-FAILURE-BOUNDARY の改訂 options で、`!` と failure の別表示を再確認する。
- Shell の非 OSC 観測はどの適合方式にしますか。
- 未知 Codex message は主 status を unknown にしますか、それとも最後の既知状態と不明 indication を併記しますか。

# Open Questions

primary alternative と直交 facets は確認済みである。DP-CONCURRENT-ATTENTION と DP-FAILURE-BOUNDARY は option 改訂と再相談が必要であり、DP-SHELL-OBSERVATION と DP-UNKNOWN-DISPLAY も未回答のため、具体的な Given/When/Then を approved Specify contract としてまだ固定しない。
