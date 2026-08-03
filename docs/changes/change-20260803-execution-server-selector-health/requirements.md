---
change: change-20260803-execution-server-selector-health
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Content

- R1: ユーザーが `Change` を押して接続中の別サーバーをクリックしたとき、選択中の Execution Server がそのサーバーへ変わること。
- R2: 接続中のサーバー名は通常の前景色で表示されること。
- R3: `unavailable` または `incompatible_protocol` のサーバー名は muted 色で表示され、button が disabled であること。
- R4: 現在選択中のサーバーについても同じ health 色規則を適用すること。
- R5: 接続理由などの内部詳細を palette に露出しないこと。
- R6: lifecycle hello 後は 5 秒の freshness window より短い間隔で application heartbeat が届き、セッション更新がない正常な接続を維持すること。
- R7: heartbeat は hello 前の接続確立や fleet membership の置換には使用しないこと。
- R8: session config の初回取得に失敗した後、enabled server が `unavailable` / `health_unavailable` から `connected` へ復旧したとき再取得すること。
- R9: connected revision の反復や復旧の連続通知で、同時に複数の session config 取得を開始しないこと。

Counterexample: 一覧が開くだけ、またはクリック handler が呼ばれても選択表示と store が変わらない実装は R1 を満たさない。

Counterexample: WebSocket が開いたままでも view-update が 5 秒以上ないだけで `health_unavailable` になる実装は R6 を満たさない。
