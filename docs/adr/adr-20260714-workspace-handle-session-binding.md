---
id: adr-20260714-workspace-handle-session-binding
kind: adr
title: Workspace handles are validated against URL session and current root
status: accepted
created: '2026-07-14'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: plan-20260714-workspace-session-switch}
source_paths: []
summary: 既存 WorkspaceRootHandle tuple を server 境界で current session 解決値へ照合する。
updated: '2026-07-14'
---

## Context

{% context %}
accepted adr-20260714-wsviewer-workspace-root-handle は session_id、frame_generation、resolved_root_path を drawer lifetime に pin する。しかし現 client pin/query は session_id を落とし、server は generation が一致すれば client supplied root を filesystem trust root に採用できる。これでは別 session/root の混用を拒否できない。
{% /context %}

## Decision

{% decision %}
既存 tuple を保持して全 workspace request に送信し、server は URL session を resolveWorkspaceSession で現在解決した後、handle session、resolved root、generation を GuardWorkspacePath、filesystem、git より前に一つの validator で照合することにする。session/root mismatch は 400 invalid_handle、generation drift は既存 409 handle_stale とする。一致時のみ server-resolved current root を trust root に使う。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
client supplied root は検証前に filesystem authority にならず、別 session/root 混用を typed contract test で拒否できる。accepted drawer-lifetime stale degradation も維持する。
{% /consequence %}

{% consequence kind="negative" %}
wire query/type と全 workspace endpoint の共通 validator、client error partition の更新が必要になる。
{% /consequence %}

{% consequence kind="neutral" %}
opaque token、署名鍵、server registry は導入しない。invalid_handle は外部入力として typed 4xx に回復し、generation drift は既存 handle_stale UI degradation へ流す。
{% /consequence %}

## Alternatives

- generation だけを検証する現状は root/session binding を保証しないため却下。
- signed token と server registry は既存 tuple で要件を満たせるのに lifecycle state を増やすため却下。
- mismatch を全て handle_stale に統合する案は contract violation と通常 drift の観測を失うため却下。

## Confirmation

Go handler contract tests で cross-session/cross-root は invalid_handle、generation drift は handle_stale、valid tuple は success となり、mismatch 時の filesystem/git access が 0 件であることを確認する。


{% transition from="proposed" to="accepted" date="2026-07-14" %}
workspace handle を session/root/generation に束縛する方針を採用したため
{% /transition %}
