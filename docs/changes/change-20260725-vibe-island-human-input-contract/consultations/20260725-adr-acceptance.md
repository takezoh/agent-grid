---
change: change-20260725-vibe-island-human-input-contract
role: consultation
---

<!-- lifecycle is owned by change.md -->

# Consultation 2026-07-25: design ADR 採否

design-plan.json (design-r1) の proposed ADR 2 件の採否をユーザーへ提示し、回答を得た。回答原文を監査用に記録する。

## 回答

### adr-20260725-vibe-island-asis-contract-pin → accepted

- 提示: 検証層のみで as-is 契約を pin する (production code 変更なし)。却下済み代替案 2 件を併記。
- 回答原文: 「accept (Recommended)」(1 回目の提示で即答)

### adr-20260725-vibe-island-verification-tiers → accepted

- 提示: witness を既存 T0/T1/T3 資産へ帰着させ、UAC-011 (IME 実フォーカス返却)・UAC-031 (実機トースト操作) は自動化せず手動確認と明記する。
- 経緯: 1 回目の提示で「詳しく説明して」、2 回目の説明にも「意味が分からない。背景・課題と何を意思決定すべきなのかを説明しろ」との差し戻しを受けた。3 回目に「背景 (調査→要件化→契約 pin の流れ) / 課題 (14 シナリオはロジック・実画面・OS 実挙動の 3 性質が混在し単一のテスト手段でカバー不能) / 意思決定 (新テスト基盤へ投資するか、既存資産への割り当て + 2 項目の手動確認明記で済ませるか)」の構成で説明し直した。
- 回答原文: 「accept」

## 適用

- design-plan.json: 両 ADR の `status: proposed → accepted`、`status_provenance` (user_consultation / confirmed / 本記録) を付与。revision を design-r1 → design-r2 へ前進 (base_revision: design-r1)。open_questions から採否待ち項目を除去。
- ADR 文書 (docs/adr/): 両文書の status を accepted へ同期。
- reject が無いため adr_impacts (retain / rework_required) は不要。

## 教訓 (次回の consultation 提示への反映)

ADR 採否の提示は、決定内容の要約だけでなく「背景 → 課題 (なぜこの判断が必要になったか) → 意思決定の問い → 提案とトレードオフ」の順で、リポジトリ固有の用語 (Tier / witness / UAC 等) を前提にせず説明すること。
