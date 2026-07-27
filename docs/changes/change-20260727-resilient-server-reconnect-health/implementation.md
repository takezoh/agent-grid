---
change: change-20260727-resilient-server-reconnect-health
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Connection lifecycle

`Connection.reconnectAttempt` はdaemonのhelloでのみリセットする。WebSocket openはgateway到達だけを意味するため、daemon復旧として扱わない。自動再試行の上限到達後は`closed`へ遷移し、`retry()`がカウンタとuser-close状態を初期化して同じ接続手順を再開する。

## Presentation ownership

`StatusBanner`を削除し、`StatusBar`をセッション未選択時も常設する。closed時はRetry actionを表示する。現在接続が正常でfleetに障害があれば、enabledサーバーを分母に集約表示する。

## Fleet health

Workspace mainの`FleetHealthMonitor`が`GET /api/sessions`を認証付きでprobeする。これはgatewayとdaemonの両方が利用可能な場合だけ成功する既存のread-only経路である。同時renderer要求は1 promiseへ統合し、結果を5秒キャッシュする。Electron IPC/preloadはruntime snapshotだけを公開し、DesktopConfigのload/save型へ混ぜない。

設定ページは同じsnapshotの詳細をサーバーカードへ表示する。disabledはprobeせずDisabledとする。
