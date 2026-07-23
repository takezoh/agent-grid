# protocol/

クライアント⇔デーモン間ワイヤ契約の**スキーマ正本**。native-clients plan Phase 1 の成果物がここに入る。

予定ファイル:

```text
openapi.yaml              # REST 面
events.schema.json        # サーバー→クライアント イベント
commands.schema.json      # クライアント→サーバー コマンド
capabilities.schema.json  # capability 交渉
deep-links.schema.json    # agent-grid:// スキーム
notifications.schema.json # 通知ペイロード
```

規約:

- ここが唯一の正本。生成コード (C# / Swift / Kotlin / TS / Go) は各消費者の木に置き、手編集しない。
- 破壊的変更は `contracts/compatibility-policy.md` に従う。
- Go 側の生成物/手書きワイヤ型は stdlib のみ (AGENTS.md)。多言語生成の採用は ADR-0021 の supersede として ADR を起こすこと。

中身が入るまでの現行ワイヤ実体: `src/host/proto` (IPC) と `src/server/api/wire.go` (HTTP/WS)。
