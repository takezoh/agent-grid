# contracts/

スキーマでは表現できない**振る舞い契約**の正本。全クライアントと記録済みシナリオ (シミュレータ) がここを共通仕様として参照する。native-clients plan Phase 0-1 の成果物。

ファイル (Phase 0/1 で入ったもの + 予定):

```text
approval-contract.md        # 承認: 単回裁定・期限・二重応答の解決  [landed]
question-contract.md        # 質問: 自由回答・期限                    [landed]
reconnect-contract.md       # 再接続と pending 権威スナップショット   [landed]
compatibility-policy.md     # バージョン交渉 (bundled/remote 二軸)    [landed]
handoff-contract.md         # deep link 解決 (スケルトン)             [landed]
session-state-machine.md    # セッション状態遷移                      [planned]
command-acknowledgement.md  # コマンド受理と冪等性                    [planned]
notification-policy.md      # 何をいつ通知するか                      [planned]
```

規約: 契約の変更はスキーマ (`protocol/`) と記録済みシナリオの更新を伴うこと。クライアント実装の都合で契約を曲げない — 契約が先、実装が後。
