---
id: note-20260726-user-windows-wsl-service
kind: note
title: Run the WSL host as a Windows Service
status: published
created: '2026-07-26'
tags:
- user
- windows
- wsl
- service
owners: []
relations:
- {type: references, target: design-client}
- {type: references, target: adr-20260725-wsl-supervisor-registration-bound-json-config}
source_paths:
- deploy/windows/register-wsl-host-service.ps1
- src/cmd/agent-grid-wsl-supervisor/
summary: Build, register, update, inspect, and remove the Agent Grid WSL host Windows
  Service.
updated: '2026-07-26'
---

## Summary

`AgentGridWSLHost` は、登録時に固定した Windows ユーザーと WSL
distribution の組み合わせで Linux `server` を常駐させる Windows Service
です。サービス登録には、管理者 PowerShell と
[`deploy/windows/register-wsl-host-service.ps1`](../../deploy/windows/register-wsl-host-service.ps1)
を使います。

サービスは LocalSystem ではなく、対象distributionを所有するWindowsユーザーで
実行します。スクリプトはその資格情報を安全な `PSCredential` として受け取り、
パスワードを引数や設定JSONへ保存しません。

## 前提

- Windows 10/11またはWindows ServerでWSLが利用可能である
- 対象WindowsユーザーにWSL distributionが登録されている
- 対象ユーザーへ「サービスとしてログオン」権限が付与されている
- Linux側の `server` とdata directoryが準備済みである
- 管理者として起動したWindows PowerShell 5.1以降を使う

distribution名は対象ユーザーで `wsl.exe --list --quiet` を実行して確認します。
サービス登録時には誤ったユーザーの既定distributionを採用しないよう、
distribution名を明示します。

## ビルドと配置

リポジトリのWSL/LinuxシェルでWindowsバイナリをクロスビルドできます。

```sh
make build-wsl-supervisor-windows
```

生成された `dist/windows/agent-grid-wsl-supervisor.exe` を
`C:\Program Files\AgentGrid\agent-grid-wsl-supervisor.exe` などの
Windowsローカルかつ一般ユーザーが書き換えられない場所へコピーします。
SCMの実行ファイルに `\\wsl.localhost\...` のようなUNCパスは使わないでください。

## 初回登録

管理者PowerShellでリポジトリのWindowsパスへ移動し、資格情報を取得して登録します。

```powershell
$credential = Get-Credential 'CONTOSO\agentgrid'
.\deploy\windows\register-wsl-host-service.ps1 `
  -Action Install `
  -SupervisorExe 'C:\Program Files\AgentGrid\agent-grid-wsl-supervisor.exe' `
  -Credential $credential `
  -Distribution 'Ubuntu-24.04' `
  -LinuxUser 'agentgrid' `
  -ServerExecutable '/home/agentgrid/.local/bin/server' `
  -DataDir '/home/agentgrid/.local/share/agent-grid' `
  -Addr '127.0.0.1:8443'
```

既定ではTLSを有効にし、登録後すぐにサービスを開始します。開始を保留する場合は
`-NoStart`、TLSを無効にする検証環境では `-TLSMode insecure` を指定します。
`-HealthURL` と `-LogPath` は通常省略し、登録スナップショットの既定値を使います。

登録処理は次を行います。

1. `%ProgramData%\AgentGrid\wsl-host-supervisor.json` を検証・原子的に作成する
2. 設定とログのACLを登録ユーザー向けに設定する
3. `AgentGridWSLHost` を指定ユーザー、Automatic startupでSCMへ登録する
4. `-NoStart` がなければサービスを開始する

## 更新

バイナリ、distribution、Linuxユーザー、server path、data directory、health設定の
いずれかを変更する場合は、初回登録と同じ必須引数を指定して更新します。

```powershell
$credential = Get-Credential 'CONTOSO\agentgrid'
.\deploy\windows\register-wsl-host-service.ps1 `
  -Action Update `
  -SupervisorExe 'C:\Program Files\AgentGrid\agent-grid-wsl-supervisor.exe' `
  -Credential $credential `
  -Distribution 'Ubuntu-24.04' `
  -LinuxUser 'agentgrid' `
  -ServerExecutable '/home/agentgrid/.local/bin/server' `
  -DataDir '/home/agentgrid/.local/share/agent-grid'
```

更新は既存サービスを停止してからスナップショットを置換し、SCM登録を再作成します。
途中で失敗した場合は表示されたエラーを解消し、同じ `-Action Update` を再実行します。

## 状態とログ

```powershell
Get-Service AgentGridWSLHost
sc.exe query AgentGridWSLHost
Get-Content "$env:ProgramData\AgentGrid\logs\wsl-host.log" -Tail 50
```

通常のライフサイクル記録は
`%ProgramData%\AgentGrid\logs\wsl-host.log` にJSON Linesで保存されます。
サービスプロセス自体を開始できない場合は、SystemイベントログのService Control
Managerイベントと `sc.exe query AgentGridWSLHost` を確認してください。

## 削除

```powershell
.\deploy\windows\register-wsl-host-service.ps1 -Action Uninstall
```

削除はサービスを停止してSCM登録を消します。再登録や事後調査を可能にするため、
`%ProgramData%\AgentGrid` の設定とログは削除しません。不要になったデータは、
サービス削除後に管理者が内容を確認してから手動で削除してください。

## 安全上の制約

- サービスのログオンユーザーは、対象WSL distributionの所有者と一致させる
- 複数distributionを停止する `wsl.exe --shutdown` は使用しない
- 強制復旧は登録時に固定したdistributionだけを対象にする
- 設定JSONやログへパスワード、token、環境変数値を記録しない
