# protocol/

クライアント⇔デーモン間ワイヤ契約の**スキーマ正本**。native-clients plan Phase 1 の成果物がここに入る。

ファイル:

```text
openapi.yaml              # REST binding 宣言 (OpenAPI 3.1)。SoT は *.schema.json、これは REST carrier の annex
events.schema.json        # サーバー→クライアント イベント
commands.schema.json      # クライアント→サーバー コマンド
capabilities.schema.json  # capability 交渉
transport-close.schema.json # 接続終了理由の閉じた語彙 + {"k":"c"} 離脱フレーム + upgrade 前 rejection body
deep-links.schema.json    # agent-grid:// スキーム
notifications.schema.json # 通知ペイロード
simulator/                # fixtures + recordings + sim server (ADR simulator)
```

規約:

- **メッセージ正本**は `*.schema.json` のみ。`openapi.yaml` は REST binding の annex（generator 入力ではない）。
- 生成コードの **models** は `clients/sdk/*/generated`（quicktype）。**transport** は各言語手書き。
- 破壊的変更は `contracts/compatibility-policy.md` に従う。
- Go 側の手書きワイヤ型は stdlib のみ (AGENTS.md / ADR superseding 0021 for cross-language only)。
- SDK 生成: `scripts/generate-sdks.sh` / `make generate-sdks`（pin: `clients/sdk/package-lock.json` + `.quicktype-version`）。
- 互換ゲート: `scripts/check-protocol-compatibility.sh` / `make protocol-compat`。

Go IPC/HTTP 実体: `services/host/proto` と `services/server/api/wire.go` (スキーマに対して round-trip 検証)。

`transport-close.schema.json` の語彙は閉じている。実装側の正本は `services/server/api/transport_close.go` で、
両者の一致と「語彙外の理由を送る close 経路が無いこと」は
`cd services && go test ./gorules/ -run TestTransportCloseReasonVocabularyIsRegisteredAndExhaustive` が守る
(adr-20260814-transport-close-reason-taxonomy)。
