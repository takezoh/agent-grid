# Plan — Windows Service による WSL Host Server 監視

- **作成日**: 2026-07-25
- **ステータス**: draft
- **対象**: Windows 上で既存 Linux host server を WSL 内に foreground 起動し、Windows Service が直接監視・再起動・停止を管理する構成
- **非対象**: WSL 内 systemd への host server 登録、Windows native host 実装、Windows Container 実装、異なる OS 間を bridge で接続する構成

## 1. 結論

Windows Service を指定ユーザーアカウントで起動し、その子プロセスとして `wsl.exe` を実行して Linux host server を foreground で保持する構成は成立する。

```text
Windows Service Control Manager
  └─ agent-grid-wsl-supervisor.exe
       └─ wsl.exe -d <distro> -u <linux-user> --exec <host-server>
            └─ Linux host server
                 └─ Linux devcontainer
                      └─ Linux bridge / driver / frame
```

この構成では OS 境界を bridge が跨がない。

- Windows Service は WSL instance と Linux host server の lifecycle だけを監督する。
- host server、devcontainer、bridge、driver、frame はすべて Linux 上で動く。
- Linux host server は systemd service にせず foreground process として起動する。
- Windows Service が唯一の supervisor となる。

systemd を追加する必要はない。Windows と Linux の二重 supervisor にすると、終了原因、再起動所有権、ログ、停止順序が複雑になる。

## 2. 前提

1. Windows Service は LocalSystem ではなく、対象 WSL distribution を所有する Windows ユーザーアカウントで実行する。
2. WSL distribution と Linux user を起動引数で明示する。
3. Linux host server は foreground で動き、正常終了または異常終了時に `wsl.exe` も終了できる形にする。
4. Windows supervisor は host server のドメイン状態を持たない。
5. WSL 内の session、frame、driver、sandbox、credential policy は既存 Linux host server が所有する。

## 3. Windows Service の位置づけ

ここで使うのは Windows の per-user service template ではなく、通常の Windows Service を特定のユーザーアカウントで実行する方式である。

```text
SCM registered service
  Log On As: <WSL owner Windows account>
  Startup: Automatic
  Session: 0
```

サービス自体は machine-wide に登録されるが、process は指定ユーザーの security context で起動される。

### 採用しない構成

```text
LocalSystem
  └─ wsl.exe -d UserOwnedDistribution
```

WSL distribution は Windows ユーザーに紐づくため、LocalSystem から通常ユーザー所有の distribution を利用する構成は避ける。

### 専用ユーザーについて

専用 Windows ユーザーと専用 WSL distribution は必須ではない。既存ユーザー所有の distribution を、そのユーザーで起動する PoC から始められる。

ただし、次を分離したい場合は専用アカウントを検討する。

- パスワードや資格情報の運用
- WSL distribution の所有権
- filesystem access
- Agent Grid 専用設定
- 日常利用の WSL environment との干渉

## 4. 起動モデル

Windows supervisor は shell を挟まず `wsl.exe` を直接起動する。

```text
wsl.exe
  --distribution AgentGrid
  --user agentgrid
  --exec /home/agentgrid/.local/bin/server
```

Go の概念例:

```go
cmd := exec.Command(
    `C:\Windows\System32\wsl.exe`,
    "--distribution", "AgentGrid",
    "--user", "agentgrid",
    "--exec",
    "/home/agentgrid/.local/bin/server",
)
```

環境変数や working directory の準備が必要な場合は、WSL 内に小さな launcher を置く。

```sh
#!/bin/sh
set -eu

export AG_DATA_DIR="$HOME/.local/share/agent-grid"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/bin:/bin"

cd "$HOME/agent-grid"
exec "$HOME/.local/bin/server"
```

最後を `exec` にし、余計な shell process を監督階層に残さない。

## 5. Supervisor の責務

### 担当する

- `wsl.exe` の起動
- stdout / stderr の回収
- process exit の監視
- readiness / health check
- unexpected exit 時の再起動
- restart backoff
- Windows Event Log または専用 log file への記録
- Service Control Manager の stop request 対応
- graceful shutdown timeout
- 最終手段として対象 distribution の terminate

### 担当しない

- session 管理
- frame 管理
- driver registry
- sandbox lifecycle
- devcontainer lifecycle
- bridge protocol
- credential broker policy
- Linux process 単位の監督

Windows supervisor から見る Linux host 全体は一つの supervised unit である。

## 6. Readiness

`wsl.exe` が生きているだけでは host が利用可能とは判断しない。

```text
Starting
  → wsl.exe process created
  → Linux host process started
  → host listen socket ready
  → health endpoint success
  → Windows Service state = Ready
```

health check は Windows 側から host server の local endpoint に対して行う。

Supervisor の内部状態例:

```text
Stopped
Starting
Ready
Stopping
Backoff
Failed
```

## 7. 停止モデル

Windows Service stop 時は、直ちに `wsl.exe` を kill しない。

推奨順序:

1. host server の shutdown endpoint または control channel へ停止要求を送る。
2. host server が新規 frame 受付を停止する。
3. 実行中 frame と container を既存 teardown contract で終了する。
4. host server が終了する。
5. `wsl.exe` の終了を待つ。
6. timeout 時に supervisor が子 process を terminate する。
7. それでも残る場合だけ `wsl.exe --terminate <distribution>` を使う。

`wsl.exe --shutdown` は全 distribution を停止するため使用しない。

## 8. 再起動ポリシー

異常終了時は backoff 付きで再起動する。

例:

```text
1st failure: 1 sec
2nd failure: 2 sec
3rd failure: 5 sec
4th failure: 15 sec
subsequent: 60 sec
```

次の permanent failure と crash を区別する。

- WSL executable not found
- distribution not found
- Linux executable not found
- permission denied
- invalid configuration
- health check timeout
- host server crash
- WSL subsystem unavailable

configuration error を無限再起動しない。一定回数の連続失敗後は `Failed` として停止し、明確な原因を記録する。

## 9. ログ

Linux host server の stdout / stderr は Windows supervisor が受け取る。

候補:

- Windows Event Log
- `%LOCALAPPDATA%\AgentGrid\logs\wsl-host.log`
- size-based rotation

最低限記録する項目:

- service start / stop
- WSL command line の非秘密部分
- distribution / Linux user
- child PID
- readiness 結果
- exit code
- restart count
- graceful shutdown timeout
- force terminate 実行

secret や environment variable の値は記録しない。

## 10. セキュリティと資格情報

Windows Service は対象 WSL distribution を所有するユーザーで実行する。

注意点:

- service account password の変更・期限切れ
- `Log on as a service` 権限
- Windows user profile の読み込み
- WSL distribution registration
- Windows Credential Manager との将来連携
- service executable と config の ACL

現在の Linux host credential model は WSL 内に残す。Windows Service が credential broker policy や agent credential を保持しない。

## 11. Sleep / Hibernate / Windows Update

長時間 daemon として次を故障前提で扱う。

- sleep / hibernate 復帰
- WSL subsystem restart
- network interface 再構成
- localhost forwarding の再確立
- Windows Update 後
- WSL update 後

復帰時は process alive だけでなく health check を再実行する。health failure が継続する場合は、graceful restart を試み、その後に対象 distribution terminate を行う。

## 12. PoC

まず production supervisor を実装せず、最小 Windows Service で成立性を検証する。

### PoC 起動対象

```text
wsl.exe -d AgentGrid -u agentgrid --exec /bin/sh -c 'exec /path/to/server'
```

または lifecycle だけを確認する場合:

```text
wsl.exe -d AgentGrid -u agentgrid --exec /bin/sleep infinity
```

### 検証項目

1. service account から `wsl.exe -l -q` で distribution が見える。
2. ユーザー未ログイン状態で Windows 起動後に自動起動する。
3. host server が foreground で維持される。
4. host server 終了時に `wsl.exe` が終了する。
5. abnormal exit を Windows Service が検出できる。
6. backoff 付き再起動が働く。
7. Service stop が graceful shutdown になる。
8. timeout 時に対象 distribution だけを terminate できる。
9. sleep / hibernate 復帰後に health が戻る。
10. Windows client から local endpoint に接続できる。
11. Windows Update / WSL update 後に再起動できる。

## 13. 実装案

```text
src/cmd/agent-grid-wsl-supervisor/
  main_windows.go

src/platform/wslhost/
  supervisor_windows.go
  process_windows.go
  health_windows.go
  service_windows.go
  config.go
```

設定例:

```toml
[host]
distribution = "AgentGrid"
linux_user = "agentgrid"
server_path = "/home/agentgrid/.local/bin/server"
health_url = "http://127.0.0.1:7780/healthz"
startup_timeout = "60s"
shutdown_timeout = "30s"

[restart]
max_fast_failures = 5
max_backoff = "60s"
```

Windows Service wrapper は Go の `golang.org/x/sys/windows/svc` 系 API を利用できる。

## 14. 非目的

- WSL 内 systemd unit の作成
- systemd と Windows SCM の二重監督
- Linux host server の Windows native port
- Windows Container sandbox
- Windows process-isolated frame
- Windows と Linux の bridge 接続
- WSL distribution の一般-purpose daemon manager 化

## 15. 判断

採用候補とする。

理由:

- 現在の Linux host server をほぼそのまま利用できる。
- Linux devcontainer、PTY、Unix socket、SCM_RIGHTS を維持できる。
- systemd を追加せず監督関係を単純化できる。
- Windows Service が host 全体の起動・停止・再起動を一元管理できる。
- bridge は同一 Linux OS 内だけを接続するという不変条件を維持する。

ただし、本採用前に Windows Service の指定ユーザー context から WSL distribution を安定して起動できることを PoC で確認する。これはアーキテクチャ上の否定材料ではなく、Windows / WSL integration の受け入れ条件である。
