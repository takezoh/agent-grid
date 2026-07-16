---
id: task-20260706-frame-messaging-web-surface
kind: task
title: MESSAGES web surface for inbox and replies
status: done
created: '2026-07-06'
priority: normal
effort: medium
files_touched: []
pr: null
tags:
- mcp
- frame-messaging
- phase-1
- web
owners: []
relations:
- {type: dependsOn, target: task-20260706-frame-messaging-store-broker}
- {type: partOf, target: change-20260706-frame-messaging}
source_paths:
- src/server/web/
- src/client/web/
- src/client/state/
summary: View summary と messages API を追加し、Web UI の MESSAGES tab で inbox/reply status
  を閲覧できるようにする
updated: '2026-07-06'
change: change-20260706-frame-messaging
---

# MESSAGES web surface for inbox and replies

## 責務

Web UI に Phase 1 の閲覧 surface を追加する。`TERMINAL` と既存 `LogTabs` は維持し、`MESSAGES` tab/panel で inbox/reply status を確認できるようにする。Phase 1 では Web からの送信 UI は作らない。

## 詳細手順

1. `state/view.View` または対応する view payload に `FrameMessagingSummary` を追加する。
2. `server/web` に `GET /api/sessions/{sessionId}/messages` を追加し、message/reply 一覧と本文を返す。raw transcript は返さない。
3. Web client の tab model を、synthetic `TERMINAL`、driver `LogTabs`、stateful `MESSAGES` surface が共存できる形に拡張する。
4. `MESSAGES` panel で source/target frame、topic、body preview、reply status、finalAnswer preview、unread count を表示する。
5. read API による既読更新を接続する。Web 既読と agent 既読は Phase 1 では frame 単位で扱う。
6. 既存 session hydrate、terminal rendering、transcript/events tab の regression を防ぐ。

## 前提

- broker/store が message/reply summary と read API を提供している。
- UI から message を作成する要件は Phase 1 にはない。

## スコープ外

- Web 送信 form
- approval UI
- audit viewer
- delivery status の詳細 UI

## 受け入れ条件

- active session に message/reply がある場合、`MESSAGES` surface が表示される。
- `TERMINAL` tab は常時利用できる。
- 既存 `TRANSCRIPT` / `EVENTS` などの `LogTabs` は従来どおり表示される。
- `MESSAGES` は log file tailing ではなく daemon snapshot / event update から描画される。
- messages API response は raw transcript を含まない。


{% transition from="todo" to="in_progress" date="2026-07-06" %}
Started MESSAGES surface verification and integration.
{% /transition %}


{% transition from="in_progress" to="done" date="2026-07-06" %}
Verified MESSAGES web surface, messages API, and existing tab regression coverage.
{% /transition %}
