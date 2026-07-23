# contracts/

スキーマでは表現できない**振る舞い契約**の正本。全クライアントと記録済みシナリオ (シミュレータ) がここを共通仕様として参照する。native-clients plan Phase 0-1 の成果物。

予定ファイル:

```text
session-state-machine.md    # セッション状態遷移
approval-contract.md        # 承認: 単回裁定・期限・二重応答の解決
question-contract.md        # 質問: 構造化/自由回答・期限
reconnect-contract.md       # 再接続と replay (既存 ADR-0025/0011/0022 の統合)
command-acknowledgement.md  # コマンド受理と冪等性
notification-policy.md      # 何をいつ通知するか
handoff-contract.md         # クライアント間ハンドオフと deep link 解決
compatibility-policy.md     # バージョン交渉と互換性 (shell⇔daemon の skew 含む)
```

規約: 契約の変更はスキーマ (`protocol/`) と記録済みシナリオの更新を伴うこと。クライアント実装の都合で契約を曲げない — 契約が先、実装が後。
