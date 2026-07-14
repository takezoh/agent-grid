---
id: adr-20260714-workspace-request-epoch
kind: adr
title: Workspace request commits require session and epoch identity
status: accepted
created: '2026-07-14'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: plan-20260714-workspace-session-switch}
source_paths: []
summary: 旧 session の非同期応答は store-owned monotonic epoch と session identity で commit
  を拒否する。
updated: '2026-07-14'
---

## Context

{% context %}
WorkspaceDrawer と WorkspaceTree は root handle、file、diff、tree、reconnect mtime を非同期取得する。component cleanup だけでは abort 不能 Promise、store action、error/finally commit を覆えず、session 切替後に旧応答が新 state を上書きできる。
{% /context %}

## Decision

{% decision %}
Workspace lifecycle owner が monotonic epoch を持ち、各 request は開始時の sessionId と epoch を捕捉することにする。success、error、finally を含む全 state commit は current identity と一致するときだけ許可する。effect cleanup、AbortController、keyed remount は資源節約の補助であり正本にしない。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
UI local state と Zustand state の双方で、旧 request の resolve/reject を一つの機械的契約により拒否できる。
{% /consequence %}

{% consequence kind="negative" %}
全 commit point に guard が必要であり、loading/error の漏れを delayed-Promise tests で水平検査する必要がある。
{% /consequence %}

{% consequence kind="neutral" %}
identity mismatch は例外ではなく no-op と意味論を再定義する。既存 fetch seam を再利用し、新規 dependency は追加しない。
{% /consequence %}

## Alternatives

- keyed remount と effect cleanup だけでは store commit と abort 不能 Promise を覆わないため却下。
- AbortController のみでは cancellation を守らない Promise を覆わないため却下。
- epoch のみで sessionId を比較しない案は診断性が弱いため却下。

## Confirmation

Controllable Promise tests で、旧 resolve と reject の双方が file/diff/tree/root/loading/error/conflict state を変更しないことを確認する。


{% transition from="proposed" to="accepted" date="2026-07-14" %}
非同期応答を sessionId と epoch で隔離する方針を採用したため
{% /transition %}
