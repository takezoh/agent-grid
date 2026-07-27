# Plan — Windows Native Container Sandbox

- **作成日**: 2026-07-25
- **ブランチ**: `main`
- **ステータス**: draft（技術検証前）
- **影響範囲**: `services/host/`、`services/platform/sandbox/`、`services/platform/framelaunch/`、`services/cmd/bridge/`、Windows 向け配布・検証基盤
- **関連**: [`multi-host-gateway.md`](./multi-host-gateway.md)

## 0. 決定の前提

本計画では、次を変更不能な前提とする。

1. **Server のホスト OS と sandbox/container OS は必ず同じ**。
2. **bridge は異なる OS 間を接続しない**。
3. Windows Server/host 上では Windows bridge と Windows driver/frame だけを扱う。
4. Linux Server/host 上では Linux bridge と Linux driver/frame だけを扱う。
5. sandbox は VM よりも、作成・起動・破棄が高速な軽量コンテナを優先する。
6. project workspace はホスト側を正本とし、sandbox へ bind mount する。
7. 求める隔離は、現在の Linux devcontainer と同程度の Docker-level isolation である。
8. container は原則 project 単位とし、その内部で複数の driver/frame を spawn する。

したがって、WSL2 を Windows host の Linux sandbox として利用する構成、Windows Sandbox、通常の Hyper-V VM、異種 OS bridge は本計画の対象外とする。

## 1. 目的とアウトカム

### 1.1 目的

Windows host 上でも、現在の Linux devcontainer と同じ sandbox lifecycle を成立させる。

```text
Windows host
  └─ Windows OCI container
       ├─ C:\AgentGrid\bridge.exe
       ├─ driver
       ├─ frame 1..N
       └─ C:\workspace  ← host project directory の bind mount
```

### 1.2 成功アウトカム

1. Windows host 上で project container を短時間に create/start/remove できる。
2. host project directory を container の `C:\workspace` に read-write bind mount できる。
3. container 内で Windows bridge を起動し、同一 Windows OS の driver/frame を spawn できる。
4. container を破棄しても workspace の変更は host に残り、それ以外の環境変更は破棄される。
5. project A と project B の process、filesystem、registry、network、runtime state が分離される。
6. bridge、driver、frame lifecycle が既存 sandbox contract に統合され、Windows 固有分岐が上位 domain に漏れない。
7. bare-host fallback と container execution を明示的に区別できる。

## 2. 採用候補

### 2.1 第一候補: Windows Container + process isolation

Windows の標準 sandbox backend は、**process-isolated Windows Container** とする。

これは host と Windows kernel を共有しつつ、container ごとに次を分離する。

- process namespace / silo
- container filesystem layer
- registry view
- network namespace
- resource controls
- container-local process tree
- scratch layer

要件との適合:

| 要件 | 評価 |
|---|---|
| host と container が同じ OS | 適合 |
| 軽量な create/start/remove | Windows の選択肢では最適 |
| Docker-level isolation | 適合 |
| bind mount | 対応 |
| OCI/containerd 統合 | 対応 |
| bridge を container 内に配置 | 対応 |
| project 単位 container | 対応 |

### 2.2 Hyper-V isolation の位置づけ

Hyper-V isolation は標準経路にしない。

理由:

- container ごとに utility VM と独立 kernel を持つ。
- process isolation より起動時間、memory、storage cost が増える。
- container density が下がる。
- 本計画の「同一 OS・軽量・Docker-level isolation」という主目的より強い隔離である。

ただし、process isolation を security boundary として扱えない workload や、host/image build compatibility を満たせない場合の**明示的な opt-in fallback**として残す余地はある。無言の自動 fallback は性能特性を変えるため行わない。

## 3. bind mount モデル

Windows Container は host directory の bind mount を利用できる。

想定構成:

```text
Host
├─ C:\Projects\ProjectA
└─ %LOCALAPPDATA%\AgentGrid\run\<project-id>

Container
├─ C:\workspace       ← ProjectA, read-write
├─ C:\AgentGrid\run  ← project runtime, read-write
└─ C:\AgentGrid\bin  ← image 内、bridge/driver
```

Docker 相当の例:

```powershell
docker run --isolation=process `
  --mount type=bind,source=C:\Projects\ProjectA,target=C:\workspace `
  --mount type=bind,source=C:\Users\me\AppData\Local\AgentGrid\run\abc,target=C:\AgentGrid\run `
  agent-grid/windows-core:<host-build>
```

### 3.1 不変条件

1. workspace mount は project root に限定する。
2. `C:\`、`C:\Users`、ユーザープロファイル全体を mount しない。
3. bridge/driver binary は workspace の外側に置く。
4. bind mount 先に image 内ファイルを置かない。mount により隠れるためである。
5. read-only で足りる config や policy は read-only mount とする。
6. project root は mount 前に canonicalize し、junction/reparse point による containment escape を検査する。
7. ACL 不整合は権限緩和で回避せず、明示的な container identity と access policy で解決する。

### 3.2 永続性

```text
bind-mounted workspace
  → host に永続化

container filesystem / registry / installed tools / temp state
  → container 破棄時に破棄
```

このモデルにより、「コードと成果物は host に残し、実行環境は使い捨てる」という既存 devcontainer の価値を維持する。

## 4. container の粒度

### 4.1 標準: project 単位

```text
Project container
├─ bridge
├─ driver
├─ frame A
├─ frame B
└─ frame C
```

Windows Container は Linux Container より image と起動 cost が大きいため、frame ごとの container 作成を標準にしない。

project container を起動した後は、frame spawn を container 内の通常の Windows process spawn として扱う。これにより frame 起動を高速化する。

### 4.2 frame 単位 container

frame 間にも強い隔離が必要な場合だけ opt-in とする。

代償:

- frame ごとの container create/start
- workspace mount と network setup の反復
- bridge bootstrap の反復
- memory、scratch layer、image metadata の増加

MVP には含めない。

## 5. container runtime

### 5.1 初期 backend: Docker API

既存 devcontainer 実装との接続を優先し、最初の技術検証は Docker API/CLI 上で行う。

必要操作:

- Windows image pull/build
- process isolation を指定した container create
- bind mount
- start/stop/remove
- bridge process の起動
- interactive frame exec
- exit code と stdio の取得
- orphan cleanup

### 5.2 長期 backend: containerd + runhcs

Docker Desktop への恒久依存を避ける場合は、Windows の標準 OCI runtime 経路へ移行する。

```text
Agent Grid
  └─ containerd API
       └─ runhcs
            └─ Host Compute Service
                 └─ Windows process-isolated container
```

Linux と Windows を次の対称構造にできる。

```text
ContainerProvider
  └─ containerd
       ├─ runc   — Linux
       └─ runhcs — Windows
```

HCS/hcsshim を Agent Grid が直接操作する実装は、image、snapshot、network、stdio、cleanup の責務が増えるため初期採用しない。

## 6. Windows image 戦略

### 6.1 Server Core を先に検証する

最初は互換性を優先し、Server Core を使う。

想定用途:

- PowerShell
- Git for Windows
- Node.js
- Codex/Claude/Gemini の Windows CLI
- build tools
- bridge.exe

### 6.2 Nano Server は最適化フェーズ

Nano Server は軽量だが Win32/API surface が狭く、agent CLI や toolchain が動作しない可能性がある。

次が成立した後に検証する。

1. Server Core 上で bridge/driver/frame lifecycle が完成している。
2. 各 agent CLI の runtime dependency が把握できている。
3. Nano Server 用 image に必要な component を再現可能に追加できる。

### 6.3 image を host build に対応づける

Windows process isolation は host と container image の build compatibility が Linux より厳しい。

Agent Grid は host build を検出し、対応 image を選ぶ。

```yaml
windows_images:
  - host_build: "26100"
    image: agent-grid/windows-core:26100
    isolation: process
  - host_build: "22631"
    image: agent-grid/windows-core:22631
    isolation: process
```

互換 image がない場合は、明示的な unsupported error を返す。Hyper-V isolation への自動切替はしない。

## 7. bridge と frame spawn

### 7.1 同一 OS 制約

```text
Windows host → Windows bridge.exe → Windows driver/frame
Linux host   → Linux bridge      → Linux driver/frame
```

bridge protocol は OS-neutral な wire contract を持ってよいが、bridge deployment と execution target は host OS と一致しなければならない。

### 7.2 container 内配置

```text
C:\AgentGrid\bin\bridge.exe
C:\AgentGrid\bin\driver.exe
C:\AgentGrid\run\...
C:\workspace\...
```

bridge binary は image に含めるか、versioned read-only mount で注入する。workspace 内には置かない。

### 7.3 interactive process

Windows frame は interactive agent CLI を扱うため、runtime が次を満たす必要がある。

- stdin/stdout/stderr の双方向 stream
- ConPTY または container runtime が提供する terminal attachment
- terminal resize
- Ctrl+C / soft interrupt
- force termination
- exit code
- container/bridge disconnect 時の cleanup

Docker/containerd の Windows exec が Agent Grid の PTY contract を満たすかを、最初の spike で検証する。満たさない場合は bridge が container 内で ConPTY を所有し、host とは protocol stream だけを交換する。

## 8. 隔離とセキュリティの位置づけ

process-isolated Windows Container は、project 間の通常の環境汚染を防ぐ sandbox として採用する。

防ぐ対象:

- project 外 filesystem への通常アクセス
- registry state の混在
- process の残留
- dependency と environment の汚染
- network identity の混在
- container scratch state の永続化

保証しない対象:

- hostile kernel exploit
- Windows kernel 自体の脆弱性
- privileged container escape
- host に意図的に公開した bind mount の保護

したがって、workspace は agent から変更可能な明示的 trust boundary であり、container isolation は bind-mounted project を保護しない。

## 9. Windows 固有の主要課題

### P0

1. Windows 11/Server 上で process isolation が実際に利用可能な edition/build matrix。
2. host build と Server Core/Nano Server image compatibility。
3. project directory の read-write bind mount と ACL。
4. bridge を container 内で起動し、interactive frame を spawn できること。
5. terminal input/output/resize/interrupt の fidelity。
6. container destroy 時に全 child process と runtime state が確実に消えること。

### P1

1. NTFS file locking と agent/editor の競合。
2. junction/reparse point、UNC、long path、case-insensitive path。
3. Windows Defender による image extraction、container start、workspace I/O への影響。
4. Git、Node、PowerShell、各 agent CLI の Windows Container 対応。
5. network policy と localhost の意味。
6. hostexec、MCP、credential broker の Windows endpoint/stream transport。
7. container runtime の導入・起動・更新・障害診断。

### P2

1. Server Core から Nano Server への縮小。
2. warm image/cache による start latency 改善。
3. containerd/runhcs backend。
4. frame 単位 container の opt-in。
5. Hyper-V isolation の明示的 fallback profile。

## 10. 実装フェーズ

### Phase 0 — Compatibility spike

最小 image と手動コマンドで次を証明する。

1. Windows process-isolated container を create/start/remove。
2. host project directory を `C:\workspace` に bind mount。
3. bridge fixture を container 内で起動。
4. bridge から PowerShell または fake agent を interactive spawn。
5. input、output、resize、interrupt、exit code を往復。
6. workspace の変更が host に残る。
7. workspace 外の変更が container remove で消える。
8. 連続 create/start/remove の latency と failure rate を測定。

成果物:

- 対応 Windows edition/build matrix
- image tag matrix
- cold/warm start 計測
- PTY/stdio contract の可否
- bind mount/ACL の制約
- go/no-go 判定

### Phase 1 — WindowsContainerProvider

既存 sandbox interface に Windows provider を追加する。

```go
type NativeContainerProvider interface {
    HostOS() OS
    CreateProjectContainer(context.Context, ProjectSpec) (Container, error)
    StartBridge(context.Context, Container) (BridgeConnection, error)
    SpawnFrame(context.Context, Container, FrameSpec) (Frame, error)
    Destroy(context.Context, Container) error
}
```

実装:

```text
LinuxContainerProvider
WindowsContainerProvider
```

上位の session/frame domain は OS 分岐を持たない。

### Phase 2 — Production lifecycle

- crash recovery
- orphan container pruning
- image compatibility check
- structured diagnostics
- resource limits
- network policy
- ACL validation
- update/version negotiation
- Windows CI runner による integration test

### Phase 3 — Runtime independence

Docker backend が成立した後、必要性を評価して containerd/runhcs backend を追加する。

## 11. 検証指標

「インスタント」は曖昧なため、実測値で判断する。

最低限記録する指標:

- image が local にある状態での container create latency
- container start から bridge ready まで
- bridge ready から frame interactive ready まで
- container remove latency
- concurrent project container 数
- idle memory/container
- active frame memory/container
- workspace I/O throughput
- filesystem event latency
- 100 回連続 create/start/remove の成功率

初期目標値は spike の実測後に設定する。Linux devcontainer と同じ数値を先に要求せず、Windows の最小実用値と user experience から定める。

## 12. 非目的

本計画では次を行わない。

- Windows host から Linux container を bridge で操作する。
- Linux host から Windows container を操作する。
- WSL2 を Windows sandbox provider として扱う。
- Windows Sandbox を long-lived interactive frame runtime として採用する。
- AppContainer/Restricted Token だけで devcontainer 相当を再現する。
- Hyper-V VM を標準 sandbox とする。
- Unreal Engine や完全な Visual Studio IDE を Windows Container 内で動かせると先に仮定する。
- bind mount した workspace を container から保護する。

## 13. 決定

Windows host の devcontainer 相当 sandbox は、次を標準方針とする。

> **Windows process-isolated OCI container を project 単位で作成し、host project directory を bind mountし、同一 Windows OS の bridge/driver/frame を container 内で実行する。**

runtime は初期検証で Docker を使用し、長期的には必要に応じて containerd + runhcs に移行する。

Hyper-V isolation は標準経路にせず、より強い security boundary または image compatibility が必要な場合の明示的 opt-in とする。

この方針により、Linux と Windows の双方で次の対称性を維持する。

```text
Server OS = Container OS = Bridge OS = Driver/Frame OS

Linux:
  Linux host → Linux OCI container → Linux bridge → Linux driver/frame

Windows:
  Windows host → Windows OCI container → Windows bridge.exe → Windows driver/frame
```
