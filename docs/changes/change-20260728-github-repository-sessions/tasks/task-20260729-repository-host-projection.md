---
id: task-20260729-repository-host-projection
kind: task
title: Repository host and protocol projection
status: todo
created: '2026-07-29'
priority: normal
effort: large
files_touched:
- clients/web-host/internal/webhost/
- clients/web-host/internal/webhost/repository_lease*.go
- clients/web-host/internal/webhost/repository_lease*_test.go
- clients/workspace/src/main/fleet-session-runtime*.ts
- clients/workspace/src/main/settings-page*.ts
- clients/workspace/src/main/protocol-version*.ts
- clients/workspace/src/main/electron-entry.ts
- clients/workspace/src/main/repository-lease*.ts
- clients/workspace/src/preload/electron-preload.cjs
- clients/workspace/src/preload/index.ts
- clients/workspace/tests/fleet-session-runtime*.test.ts
- clients/workspace/tests/settings-page*.test.ts
- clients/workspace/tests/app-entry.test.ts
- clients/workspace/tests/repository-settings-ipc*.test.ts
- clients/workspace/tests/repository-lease*.test.ts
- clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/ProtocolVersion.cs
- clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/WsFrameParser.cs
- clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/ShellGatewayClient.cs
- clients/windows-shell/AgentGrid.Shell.Core.Tests/GatewayClient/WsFrameParserTests.cs
- clients/windows-shell/AgentGrid.Shell.Core.Tests/GatewayClient/SupervisionWsSessionTests.cs
- clients/windows-shell/AgentGrid.Shell.Core.Tests/Composition/ShellFleetTests.cs
pr: null
tags:
- github
- fleet
- electron
- windows
owners: []
relations:
- {type: partOf, target: change-20260728-github-repository-sessions}
- {type: dependsOn, target: task-20260729-workspace-github-account-catalog}
- {type: dependsOn, target: task-20260728-durable-repository-preparation}
- {type: dependsOn, target: task-20260728-secure-clone-authority}
source_paths:
- clients/web-host/internal/webhost/
- clients/workspace/src/
- clients/workspace/tests/
- clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/
pinned_context:
- docs/changes/change-20260728-github-repository-sessions/requirements.md
- docs/changes/change-20260728-github-repository-sessions/verification.md
- docs/adr/adr-20260728-client-host-github-device-flow.md
- clients/workspace/src/main/electron-entry.ts
- clients/workspace/src/preload/index.ts
done_criteria:
- Web/Workspace credential-free FleetRuntime parity
- Electron main/preload Settings action boundary
- Web/Workspace host-only credential lease mediation
- Windows exact-version parser/transport migration
max_diff_loc: 920
change: change-20260728-github-repository-sessions
summary: Connect credential-free GitHub and server preparation state through Web and
  Electron main/preload boundaries and migrate the Windows parser.
---

# Repository host and protocol projection

## 責務

GitHub account/catalogとserver-owned preparation/environment snapshotをcredential-free FleetRuntimeへ統合し、Web hostとElectron main/preloadから共有SPAへ同一projection/Settings actionを公開する。host authorityだけがcredentialをrepository-bound lease callへ渡し、変更されたpublic protocolをWindows parser/transportにも同時反映する。

## 詳細手順

1. Web/Workspace providerが同一fixtureからdeep-equalなsource/status/target/preparation projectionを生成する。
2. Electron mainでGitHub action IPCを登録し、preload bridgeはtyped credential-free action/statusだけをSettings rendererへ公開する。
3. Web/Workspace hostでserver auth mode、exact protocol、HTTPS/loopbackをtoken transfer前にpreflightし、合格時だけaccount authorityからcredentialを取り出してrepository-bound one-use lease APIを呼ぶ。
4. protocol tokenとrepository/preparation shapeをWindows parser/transportへ反映し、credential値をSPA/preload/frame/diagnosticへ含めない。
5. host parity、no-transfer sentinel、IPC boundary、preload surface、Windows parser/transport contract testsを同じpublic projection changeに含める。

## 前提

`task-20260729-workspace-github-account-catalog`、`task-20260728-secure-clone-authority`、`task-20260728-durable-repository-preparation`を消費する。`max_diff_loc: 920`は、credential authorityとserver lease APIをhost-only boundaryで接続し、Web/Electron/Windowsのshipped consumersとcross-platform全体gateを一つのexact-version projectionとして検証する校正済み上限である。

## スコープ外

React picker/DOM、OAuth state machine、server lease table/lifecycle、credentialをrendererへ渡すbridgeは扱わない。

## 受け入れ条件

- Web/Workspace host projectionが同一fixtureでdeep-equalになり、GitHub failureがProjects sourceを減らさない。
- Electron main/preload Settings actionがsuccess/denied/cancel/failureをcredential-freeで往復する。
- auth.mode=none、protocol mismatch、非HTTPS/非loopbackはWeb/Workspace双方でcredential read/transfer、lease request、preparation/filesystem mutation前に拒否され、sentinelはSPA/preload/URL/log/diagnosticに現れない。
- Windows parser/transportがrepository/preparation exact-version framesをround tripし、旧新mismatchをside effect前に拒否する。
- `cd clients/web-host && GOCACHE=/tmp/gocache-agent-grid go test ./...`、`cd clients/workspace && npm run test`、`dotnet test clients/windows-shell/AgentGrid.Shell.Core.Tests/AgentGrid.Shell.Core.Tests.csproj`が成功する。
