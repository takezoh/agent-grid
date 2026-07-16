---
id: adr-20260714-workspace-session-switch-lifecycle
kind: adr
title: Workspace session switch is a guarded lifecycle transaction
status: accepted
created: '2026-07-14'
decision_makers:
- unknown
tags: []
owners: []
relations:
- {type: partOf, target: change-20260714-workspace-session-switch}
source_paths: []
summary: active session selection と単一 Workspace state の終了を dirty-aware transaction
  として協調する。
updated: '2026-07-14'
---

## Context

{% context %}
activeSessionID は accepted adr-20260705-view-update-sessions-only により browser-local daemon store が所有する一方、Workspace state は別 store が保持する。session ID だけを先に変更すると新 session と旧 Workspace が混在する。web-ui-refresh FR-031 と UAC-016 は Terminal/Workspace mode visibility switch を pure visibility とし、terminal scrollback/subscriptions と Workspace open file/tabs/dirty buffer の保持を要求する。FR-032 は tree を tab ではなく persistent right-side Files panel とする。したがって mode visibility switch と active-session context switch を別の lifecycle event として定義する必要がある。
{% /context %}

## Decision

{% decision %}
useDaemonStore.selectSession を全 selection request の単一 policy とし、active-session selection を old Workspace session の明示的終了境界とすることにする。これは FR-031 の「explicit close だけが終了する」を mode visibility の文脈に限定する部分的 refinement であり、mode visibility switch 自体は state を完全に保持する。Workspace lifecycle の同期的な prepare 結果が clean なら old Workspace reset（activity history を除外）後に active を commitし、dirty なら active を変えず pending にする。visible な Workspace は mode を維持し、persistent Files panel を新 root 直下一覧・未展開に、editor を open target/file/diff のない empty state にする。focus は規定しない。pending 中の別 valid target は置換、同一 target は no-op とする。Confirm は target 再検証後に discard-before-commit、Cancel は pending のみを消去する。old active 消失時の dirty content は accepted adr-20260714-editor-root-disappearance-degrades-save の read-only + clipboard recovery に委ねる。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
active selection の決定権と transaction 順序が一本化され、新 session と旧 Workspace の同居を状態遷移で禁止できる。
{% /consequence %}

{% consequence kind="negative" %}
daemon store と Workspace store の公開 action 間に同期契約が増え、pending/disappearance の transition tests が必要になる。
{% /consequence %}

{% consequence kind="neutral" %}
per-session state map は導入しない。activity history と mode round-trip は既存 semantics を維持する。target disappearance は visible alert へ回復し、内部の不可能な transition は fail fast とする。
{% /consequence %}

## Alternatives

- App coordinator hook は React 外 caller が bypass できるため却下。
- 両 state を単一 store へ統合する案は今回必要な範囲を越えるため却下。
- per-session state map はユーザーが選ばなかった案であり scope 外。

## Confirmation

Store tests で clean、dirty Cancel/Confirm、pending replacement、両側 session disappearance、activity preservation を検証し、selection caller の検索で policy bypass が 0 件であることを確認する。


{% transition from="proposed" to="accepted" date="2026-07-14" %}
Workspace のセッション切替ライフサイクル方針を実装判断として採用したため
{% /transition %}
