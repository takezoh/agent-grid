---
change: change-20260727-resilient-server-reconnect-health
role: requirements
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Functional requirements

- FR-001 (state-driven): 接続が自動再試行上限へ達した場合、UIは接続をclosedとして表示し、利用者がRetryを実行できなければならない。
- FR-002 (event-driven): daemon-disconnectedを受信した後、daemon由来のhelloを受信したときに限りdaemon disconnected状態を解除しなければならない。
- FR-003 (ubiquitous): 接続状態の簡易表示はステータスバーだけが所有し、ヘッダー下へ重複表示してはならない。
- FR-004 (state-driven): enabledサーバーの一部が接続不能なら、ステータスバーは接続不能数とenabled総数を表示しなければならない。
- FR-005 (event-driven): 設定のサーバー一覧を表示したとき、各サーバーのConnected、Unavailable、Disabledおよび取得可能な失敗理由を表示しなければならない。
- FR-006 (ubiquitous): disabledサーバーはfleet集計の分母に含めず、runtime healthは設定ファイルへ保存してはならない。

## Non-functional requirements

- NFR-001: サーバー観測はrenderごとに実行せず、Workspace mainで同時要求を統合し5秒以上キャッシュする。
- NFR-002: 各health probeは3秒以内に終了する。
- NFR-003: browser単体モードはElectron bridgeなしで従来どおり単一接続として動作する。
