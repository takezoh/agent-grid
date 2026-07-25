---
change: change-20260725-vibe-island-human-input-contract
role: consultation
---

<!-- lifecycle is owned by change.md -->

# Consultation 2026-07-25: 未解決 decision point の相談

2026-07-25、requirements-plan.json (specify-r1) の未解決 decision point 3 件をユーザーに提示し、回答を得た。提示は各 DP の宣言済み option + 推奨 (recommendation) の形式で行った。回答の原文 (選択ラベル) を監査用にそのまま記録する。

## 回答

### DP-PENDING-REPLAY → OPT-REPLAY-ON-CONNECT (answered)

- 質問: 島の接続/再接続時に、その時点で未解決の承認/質問をスナップショット配信して表示しますか?
- 回答原文: 「接続時に replay する (Recommended)」
- 適用: `answer_status: answered`, `selected_option_ref: OPT-REPLAY-ON-CONNECT`。normative product decision として projection に追加。実装は slice-gap-closure (handoff-pending-replay が design/verification 義務を保持)。

### DP-CLAUDE-FIRST-CLASS → 未解決のまま維持 (design 検証を先行)

- 質問: Claude セッションのパーミッション要求も島のカードとして表示・応答できるよう一級化しますか?
- 回答原文: 「design 検証を先行 (Recommended)」
- 適用: 宣言済み option (OPT-CLAUDE-HOOK-APPROVAL / OPT-CLAUDE-STATUS-ONLY) のいずれの選択でもないため、近似 mapping せず `answer_status: unanswered` を維持する。ユーザーの方向性は「hook 仲介の実現可否を design stage で検証し、可否判明後に採否を再相談する (方針としては一級化志向)」。handoff-claude-approval-source の design 義務 (技術検証) が先行する。

### DP-QUESTION-OPTIONS → OPT-OPTION-BUTTONS (answered)

- 質問: 選択肢付き質問 (AskUserQuestion 型) をワイヤと島 UI に載せ、ボタン 1 タップで回答できるようにしますか?
- 回答原文: 「選択肢ボタンを導入 (Recommended)」
- 適用: `answer_status: answered`, `selected_option_ref: OPT-OPTION-BUTTONS`。normative product decision として projection に追加。実装は slice-gap-closure (handoff-question-options が後方互換ワイヤ拡張の design 義務を保持)。

## 補足

- 本相談は /requirements の reconcile phase (typed consultation event) を経由せず、ユーザー指示によりセッション内で直接実施した。plan への適用は本記録を provenance evidence として行う。
- 回答済み 2 件は本 change (as-is 契約 pin) の scope を変えない。回答は slice-gap-closure の要求として確定し、flow/scenario の具体化は後続 change の requirements (specify) で行う。UAC-043 の「非捏造」不変条件は replay 導入後も維持される (replay は未解決 pending のみを表示する)。
