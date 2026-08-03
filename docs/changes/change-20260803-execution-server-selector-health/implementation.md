---
change: change-20260803-execution-server-selector-health
role: implementation
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Content

- Selector 内で server state から health class と選択可否を一意に導出する。
- 選択可能な option の pointer click を user-reachable test で固定する。
- summary と options のサーバー名へ同じ health class を付与し、CSS variable で色を表現する。
- unavailable / incompatible option は native `disabled` を維持する。
- server lifecycle WS は hello 後に `{"k":"p"}` を 2 秒間隔で送る。
- Web Host と Workspace Host は hello 後の `k:p` を freshness evidence としてのみ扱う。
- App は server ごとの直前 health kind を保持し、unavailable 系から connected への edge で session config を再取得する。
- config 再取得は in-flight Promise を共有して coalesce する。
