---
id: adr-20260714-workspace-switch-confirmation-pattern
kind: adr
title: Dirty session switching reuses the App overlay confirmation pattern
status: accepted
created: '2026-07-14'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: plan-20260714-workspace-session-switch}
source_paths: []
summary: Workspace 非表示時も dirty session switch を確認できるよう既存 App overlay と ConfirmDialog
  を再利用する。
updated: '2026-07-14'
---

## Context

{% context %}
上流 ux-20260713-agent-workspace-viewer は既存 SessionDrawer の off-canvas accessibility guard と selection/cancel の分離を modeled_on とする。dirty Workspace は Terminal mode で非表示でも存在するため、WorkspaceDrawer 内の dialog では selection event を観測できない場合がある。
{% /context %}

## Decision

{% decision %}
既存 App overlay placement と ConfirmDialog の modal/focus restoration pattern を採用し、session-switch 専用 pending state/action を接続することにする。dialog は pending target を表示し、Cancel と Confirm を明示的に分離する。WorkspaceDrawer の close-warning state と汎用 reason hierarchy へ統合しない。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
Workspace 非表示時も確認が表示され、既存 keyboard、scrim、focus restoration の accessibility pattern と component tests を再利用できる。
{% /consequence %}

{% consequence kind="negative" %}
Workspace close warning と session switch warning は別 state/action のまま残り、表面的な dialog wiring が二系統になる。
{% /consequence %}

{% consequence kind="neutral" %}
GitHub/Slack/Linear の activity presentation や rejected な VS Code 常設 explorer/vim mutation pattern は本判断に関係しないため採用しない。新 UI library は追加しない。
{% /consequence %}

## Alternatives

- WorkspaceDrawer 内に置く案は Terminal mode で非表示になるため却下。
- close warning と switch warning の共通 reason 型は semantics の異なる操作を結合し、同一狭 interface の利用が二つ以上立証できないため却下。
- 専用 dialog component は既存 ConfirmDialog と accessibility responsibility が重複するため却下。

## Confirmation

App component test と Playwright smoke で、Workspace 非表示時の dialog visibility、target label、Cancel/Confirm、keyboard focus restoration を公開 DOM/aria から確認する。


{% transition from="proposed" to="accepted" date="2026-07-14" %}
dirty 切替を App レベル確認で扱う方針を採用したため
{% /transition %}
