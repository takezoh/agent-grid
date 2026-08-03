---
id: task-20260803-host-fleet-health
kind: task
title: Fleet health authority, delivery, StatusBar and Connections presentation
status: done
created: '2026-08-03'
priority: critical
effort: large
files_touched:
- clients/workspace/src/main/fleet-session-runtime.ts
- clients/workspace/src/main/server-health.ts
- clients/workspace/src/main/protocol-version.ts
- clients/workspace/src/main/electron-entry.ts
- clients/workspace/src/main/app-entry.ts
- clients/workspace/src/main/settings-page.ts
- clients/workspace/src/preload/**
- clients/workspace/tests/fleet-session-runtime.test.ts
- clients/workspace/tests/server-health.test.ts
- clients/workspace/tests/github-action-dispatch.test.ts
- clients/workspace/tests/fleet-health-wire.test.ts
- clients/workspace/tests/app-entry.test.ts
- clients/workspace/tests/settings-page.test.ts
- clients/workspace/tests/repository-lease.test.ts
- clients/workspace/tests/desktop-config.test.ts
- clients/web-host/internal/webhost/fleet.go
- clients/web-host/internal/webhost/host.go
- clients/web-host/internal/webhost/*fleet*test.go
- clients/web-host/internal/webhost/host_test.go
- clients/ui/src/fleet/session-protocol.ts
- clients/ui/src/fleet/session-protocol.test.ts
- clients/ui/src/fleet/provider.ts
- clients/ui/src/fleet/provider.test.ts
- clients/ui/src/fleet/fixtures/**
- clients/ui/src/fleet/presentation.ts
- clients/ui/src/fleet/presentation.test.ts
- clients/ui/src/fleet/connections-presentation.ts
- clients/ui/src/fleet/connections-presentation.test.ts
- clients/ui/src/auth.ts
- clients/ui/src/api/sessions.test.ts
- clients/ui/src/components/StatusBar.tsx
- clients/ui/src/components/StatusBar.test.tsx
- clients/ui/src/components/ConnectionsPanel.tsx
- clients/ui/src/components/ConnectionsPanel.test.tsx
- clients/ui/src/components/AppShell.tsx
- clients/ui/src/components/palette/TargetPicker.test.tsx
- clients/ui/src/components/palette/ParamSelectPhase.test.tsx
- clients/ui/src/App.tsx
- clients/ui/src/App.test.tsx
- clients/ui/src/css/view.css
pr: null
tags: []
owners: []
relations:
- {type: partOf, target: change-20260727-resilient-server-reconnect-health}
- {type: dependsOn, target: task-20260803-active-recovery}
source_paths: []
change: change-20260727-resilient-server-reconnect-health
summary: Implement fleet health authority, host-neutral delivery, singleton SPA state,
  and the complete StatusBar and Connections presentation in one worktree.
updated: '2026-08-03'
---

<!-- 本文セクション構成はproducer方法論 (skills/_parts/decompose/knowledge/core.md) が規定する。
     dev-docs skill 自体はセクション構成を強制しない。 -->

## 責務

WebとWorkspaceの既存application-scoped `FleetRuntime`をconfigured member healthの唯一のhost writerへ拡張し、そのauthoritative snapshotをversioned wire、host-neutral provider、application-level singleton storeへ届ける。authorityからdeliveryまでを同じworktreeで閉じ、persistent StatusBarの唯一のRetry owner、independent active/fleet presentation、Web/Workspace共通Connections diagnosisまで完成させる。

## 詳細手順

- enabled memberごとに最大1 workerを起動し、Disabledはworkerなしでimmutable configからprojectionへjoinする。capabilities、ticket、helloを各3秒で終端し、Connectedを5秒で失効させ、bounded refresh failureをHealth unavailable＋nested stale priorへ分類する。
- exact protocol mismatchをmember generation内terminalなIncompatible protocolにし、config/process generation変更までoperational workを停止する。raw evidenceはclosed `HealthReason`へ分類し、shutdown/replacementでfetch、timer、socket/channel、publicationをcancelまたはgeneration fenceする。
- `electron-entry.ts`の独立`FleetHealthMonitor` authorityを除去し、`session-protocol.ts`を`FleetHealthSnapshotV1` schema/decoder ownerにする。canonical fixtureをWorkspace IPC/preloadとWeb same-origin codecへ適用し、`auth.ts`のWindow bridge宣言、FleetProviderの全typed object-literal caller/test（TargetPicker、ParamSelectPhase、sessions、StatusBar、provider、App、Workspace preload/repository lease）を同じ破壊的変更単位で追従させる。provider delivery lossはlast host identity/revisionを保つ非authoritative overlayだけを生成する。
- App lifecycleにsubscription/storeを1つだけ置き、StatusBar-owned provider生成、`listConfigs` polling、5秒timerを除去する。active recoveryとenabled-fleet aggregateを独立にreduceし、dedicated polite status text、sibling native Retry、focused Retry消滅時だけのpersistent landmark focusを実装する。
- `connections-presentation.ts`をstable `serverId` row/label/reasonのpure ownerにし、Web panelとWorkspace settingsへ同じdiagnosisを届ける。browserはexact config pathのCopy＋restart instruction、Workspaceは既存edit controlsをhealth failure中も維持する。

## 前提

`task-20260803-active-recovery`がRetry API、active lifecycle state、hello-only recoveryを提供する。accepted ADR `adr-20260803-resilient-fleet-health-single-authority`、既存fleet/session authority、host-neutral provider、server-qualified partial-failure契約を維持する。

## スコープ外

active connection retry engine、gateway supervisor、health persistence、mixed-version fallback、config mutation endpoint、Header/Sidebar/toast/modal/Connectionsへの重複Retryは扱わない。

## 受け入れ条件

- deterministic host testsで、全configured serverがexclusive qualified observationを1件持ち、enabled worker最大1・Disabled worker 0、各phase 3秒、Connected 5秒、nested stale recovery、exact mismatch terminal、generation fence、shutdown後live handle 0が成立する。
- canonical fixture/codec/provider testsで、TypeScript・Workspace・Goが同じcredential-free health meaningへround-tripし、unknown/invalid/stale-as-currentをrejectし、delivery loss overlayがhost identity/revisionを進めず次のvalid snapshotで解除される。
- application/component testsとrepository inspectionで、SPA subscriptionがapplication lifecycleにつき1つ、legacy `FleetHealthMonitor` writerとStatusBar polling/timerが0、consumer/render増加によるprobe・worker・timer・socket増加が0になる。
- accessibility/presentation testsで、no-sessionでもStatusBarの同じnative Retryへpointer/keyboard/ATで到達でき、live textとbuttonがsiblings、focused Retry消滅時だけlandmarkへfocusし、active/fleet軸とenabled denominator/partial/total/recovery表示が独立する。
- UI/Workspace testsで、両host Connectionsが同じserver-qualified rows/reasonsを表示し、browser Copy configuration path＋restart instructionとWorkspace edit controlsがfailure中も操作でき、他surfaceにRetryまたはcredential/raw-error sentinelがない。`clients/workspace/tests/desktop-config.test.ts`がhealthを含むruntime projectionの後にsettingsをsave/reloadしても`servers.json`および再読込configにhealth fieldが存在しないことを判定する。対象unit tests、両TypeScript typecheck、Web host fleet-health testsがgreenになる。


{% transition from="todo" to="in_progress" date="2026-08-03" %}
plan-delivery implementation and owning gates accepted
{% /transition %}


{% transition from="in_progress" to="done" date="2026-08-03" %}
implementation integrated; affected mechanical and conformance gates verified
{% /transition %}
