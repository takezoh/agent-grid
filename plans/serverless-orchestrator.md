# orchestrator サーバーレス化 — 二軸抽象（実行基盤 × Control Plane）設計

## Context

現状の `orchestrator`（`src/cmd/orchestrator/`, `src/orchestrator/`）は常駐 daemon。中核は純粋関数 `Reduce(State, Event, Config, now) → (State, []Effect)`（`reduce.go:14`）で、imperative shell（`scheduler.go`）が単一 `select` ループ（`scheduler.go:172-192`）で ticker・fsnotify・worker-exit channel・codex activity・retry timer を多重化し、不変 State と live handle（`s.workers`/`s.timers`）を保持する。外部の真実源は Linear のみ（tracker は read-only polling、`platform/tracker/`）。実行は codex を stdio JSON-RPC で in-process 駆動（`agent/runner.go`、devcontainer or direct）。

**目的**: 以下の二軸を独立に差し替え可能な抽象として再設計し、各実装を提供する。

- **軸1 — 実行基盤（data plane）**: codex 実行先を `Local`（現状ローカル/Docker）と `Cloud`（Docker を積んだクラウド compute = Cloud Run Job / Lambda Container 等）で差し替え可能に。
- **軸2 — Control Plane**: 判断ループを `Daemon`（poll 型・常駐）と `Serverless`（Linear webhook 駆動・stateless）の2実装で提供し、共通抽象の背後に置く。

二軸は直交（daemon+local=現状, daemon+cloud, serverless+cloud=主目標, serverless+local=ニッチ）。

---

## 設計原則との整合（ARCHITECTURE.md — これが本設計の骨格）

本設計は ARCHITECTURE.md の core principles を**そのまま全実装に適用**する。新規性は「原則の実現機構」を daemon/serverless・local/cloud で差し替える点のみ。

1. **Testability is primary（純粋 decision logic）**: `Reduce` と `reduce_*.go` 群は**無変更**。全ての新規 I/O（StateStore/RunStore/Executor/cloud client/webhook）は injectable interface 化し fake で置換。live 環境なしで decision を入出力検証できる状態を保つ。
2. **Single-writer event loop**: state 変更は常に1箇所が所有。
   - Daemon: 現 `scheduler.Run` の単一 goroutine（原則の canonical instance、維持）。
   - **Serverless: `StateStore` の fenced lease が single-writer の分散的実現**。同時に1つの `Handle` しか State を変更できないよう直列化する。daemon=goroutine／serverless=lease、**同一不変条件・異なる機構**であり原則の一般化（違反ではない）。
3. **Decisions separated from I/O（FC/IS）**: shell の effect 解釈部（`step`/`exec`）を再利用可能に factor するのみ。`Reduce` は `[]Effect` を返し、shell が I/O 実行→結果を次 event として核へ戻す現構造を保つ。**Executor（local/cloud）は state を一切変えず run-lifecycle event を loop に emit する I/O source**（原則「長寿命 I/O source は event を emit するのみ」に直結）。Local=in-process channel、Cloud=RunStore が durable な event channel。
4. **No fabricated fallbacks**: 真実を捏造しない。
   - **Linear webhook は wake/refresh トリガに留め、issue 真実は必ず tracker reconcile（既存 `EffRevalidate`/`revalidateIssue`）から取得**。webhook payload の state を権威として信じない。
   - 設定 reload 失敗時は last-known-good を保持しつつ dispatch を *gate*（現挙動踏襲）。
5. **orchestrator per-layer realization の保全**:
   - **single-authority**（`ErrDuplicateDispatch`, SPEC §7.4）を claim-before-spawn + lease で維持。
   - **agent-agnostic dispatch**: `AttemptSpec`/`RunEvent` は codex/claude-app-server で一様（`codexclient` がその一様プロトコル）。executor は agent 種別を知らない。
   - **reconcile = truth reconciliation** を維持。
6. **no-mutex 不変 State**（`forbidigo` enforced）: `Reduce`/`State` に mutex を入れない。lease は shell 側（StateStore）の機構で、核に侵入しない。
7. **層境界（depguard）**: `platform/*` は client/orchestrator 型を import しない。**汎用 persistence/launch primitive のみ platform**、orchestrator ドメイン型（`State` codec・`RunRecord`・Executor 合成）は `orchestrator/`。wire/persistence は stdlib のみ。

---

## アーキテクチャ全体像

```
   ┌──────────── 軸2: Driver（state 寿命 & event 源だけが差分）────────────┐
   │  DaemonDriver = 単一 goroutine loop   │  ServerlessHandler = stateless  │
   │  (ticker/fsnotify/run-event chan)     │  HTTP, lease で single-writer    │
   └───────────────┬───────────────────────┴────────────┬───────────────────┘
                   │       両者とも Event を作り          │
                   ▼       shell.Step(ctx, Event) を叩く  ▼
   ┌──────────────── 再利用可能 imperative shell（FC/IS の shell 部）─────────┐
   │  WithLease( load State → Reduce(無変更) → exec Effects → save State )    │
   │  effect 実行は Executor / RunStore / Tracker 経由（全て DI seam）         │
   └──────┬────────────────────────┬───────────────────────┬─────────────────┘
          ▼                         ▼                       ▼
     StateStore(KV+lease)     Executor(軸1)             Tracker(Linear, 真実源)
      InMem/File/Cloud      Local / Cloud                   │
                                  │ state 変更せず event emit │
                                  ▼ (Local:chan / Cloud:RunStore=durable event channel)
```

設計の要: **Control Plane の差は「State の寿命」と「Event の源」のみ**。両 Driver は同一の shell・`Reduce`・`Executor`・`StateStore` を共有する。

---

## 軸1: Executor 抽象（実行基盤）

**継ぎ目**: 現 `SpawnFunc`（`scheduler/dispatch.go:28`）/ `Runner.spawn`（`agent/agent.go:57`）。`LaunchPlan`（`agentlaunch/types.go:11`）+ rendered prompt + Thread/TurnOptions（`runner.go:414,430`）が「何を実行するか」の完全記述でシリアライズ可能。

orchestrator 層（agent-agnostic）に Executor を定義（`src/orchestrator/exec/` or `agent` 拡張）:
```go
type AttemptSpec struct {                 // agent 非依存（codexclient が一様プロトコル）
    Issue   tracker.Issue
    Attempt int
    Launch  agentlaunch.LaunchPlan
    Prompt  string
    Thread  codexclient.ThreadOptions
    Turn    codexclient.TurnOptions
}
type Executor interface {
    Start(ctx, AttemptSpec) (RunHandle, error) // run lifecycle は RunStore に書く（真実源）
}
type RunHandle interface {
    Kill(reason string) error
    Events() <-chan RunEvent                   // daemon の低遅延最適化（任意・正しさに不要）
}
```
**Executor は state を変えず run-lifecycle event を emit する I/O source**（原則2/3）。

- **LocalExecutor**: 現 `runner.go` の in-process turn loop を内包。codex stdio から RunEvent を流し、起動/heartbeat/終端を RunStore にも記録。`Kill`=context cancel。**実行モデル無変更**。
- **CloudExecutor**: `AttemptSpec` をシリアライズしクラウド job を起動。**コンテナ entrypoint は agent runner の one-shot 実行**（同一 codex イメージ; `cmd/` に one-shot mode を用意）で、1試行を自律実行し RunStore に heartbeat/終端を書く。orchestrator は stdio を保持しない。`Kill`=`RunRecord.KillRequested` 書込（コンテナが poll し self-terminate）＋クラウド cancel API。
- **クラウド起動 primitive は `agentlaunch` の兄弟 backend として platform に**（agent-agnostic launch は platform の責務）。orchestrator は `buildDispatcher`（`cmd/orchestrator/dispatcher_build.go`）で local/cloud を選択。クラウド SDK client は interface 化し fake でテスト。

> 設計判断: Cloud 実行は stdio 対話を捨て「コンテナ内で1試行完走 + durable 報告」へ転換し、runner を*再配置可能な実行単位*にする。これは原則「長寿命 I/O source は event を emit するのみ」の自然な拡張。

---

## 軸2: Control Plane 抽象（再利用 shell + 2 Driver）

### 再利用可能 shell（FC/IS の shell 部を factor）
現 `scheduler.step`/`exec`（`scheduler.go:226`, `effects_exec.go`）の effect 解釈を、Driver から呼べる単位へ抽出する。`s.workers`/`s.timers` 依存を Executor/RunStore/state 由来スケジューリングへ置換。**新概念ではなく現 shell の再利用化**。`Reduce`/`reduce_*.go`/`State` は無変更。
```go
// WithLease 内で: load State → Reduce(folding ev & 派生 event) → exec Effects → save State
Step(ctx, ev Event) error
```

### DaemonDriver（`src/orchestrator/.../daemon`）
現 `scheduler.go` loop を refactor。`InMemoryStore` で State 寿命=プロセス。ticker→`Step(EvTick)`、fsnotify→reload、LocalExecutor の `RunHandle.Events()`→`Step(EvRunStatus/EvCodexActivity)`、retry は state 由来（`DueAtMS<=now` を tick で導出）。**single-writer=単一 goroutine**（原則2 canonical）。挙動は現行同等。

### ServerlessHandler（`src/orchestrator/.../serverless`）
`http.Handler`。**各リクエストで durable StateStore+fenced lease の shell を構築し event を1つ fold して返す stateless 設計**（常駐 scheduler を持たない）。lease が single-writer を保証（原則2）。
- `POST /webhook/linear` — 署名検証（HMAC、新規 `platform/tracker/linear/webhook.go`）→ **issue 変化を refresh トリガとして** `Step`（reconcile/revalidate が tracker から真実取得; 原則4）。
- `POST /tick` — Cloud Scheduler 定期 ping → `Step(EvTick)`。**webhook 単独では retry（`DueAtMS`）と stall という時間駆動遷移を網羅できない**ため必須。
- `POST /run/callback`（or `/tick` 内で RunStore 走査）— クラウド job 完了 → `Step(EvRunStatus)`。
- read 系 observability（現 `httpserver/handler.go`）を併設。

---

## 共有 substrate（層配置を原則7 に整合）

### platform: 汎用 persistence primitive のみ（DI seam, agent-agnostic, stdlib）
`src/platform/kvstore/`（または `statestore/`）— **byte 指向**で orchestrator 型を知らない:
```go
type Store interface {
    Load(ctx, key string) (data []byte, ok bool, err error)
    Save(ctx, key string, data []byte) error
    List(ctx, prefix string) ([]string, error)
    WithLease(ctx, key string, fn func(context.Context) error) error // fenced
}
```
実装: `InMemoryStore`（daemon）、`LocalFileStore`（`~/.roost/orchestrator/`、temp+rename atomic、flock）。後続でクラウド KV（Firestore/DynamoDB）。**ライブラリ**: flock は `github.com/gofrs/flock`（vs 生 `syscall.Flock`／`flock(1)`）を PR で正当化。クラウド起動 client も platform（agent-agnostic launch backend）。

### orchestrator: ドメイン型 + codec + store 合成
- `statecodec.go`（`orchestrator/scheduler` or 新 pkg）: `State`↔bytes。`RetryEntry.Err error`→string、`metrics.Accumulator`→値、stdlib `encoding/json`。
- `RunRecord`（orchestrator ドメイン型）+ RunStore 利用を platform KV 上に合成:
```go
type RunRecord struct {
    IssueID, Identifier string
    Attempt             int
    Phase               string // running|succeeded|failed
    Err                 string
    ExecRef             string // local:pid / cloud:job id
    LastActivityAtMS    int64  // heartbeat（stall 検出）
    KillRequested       string
    StartedAtMS, UpdatedAtMS int64
}
```

**永続化必須**（Linear から復元不能）: `Claimed`・`RetryAttempts`（attempt/Kind/`DueAtMS`）・`Running` の存在と attempt/Session/StartedAt。**毎 tick 再導出**: candidate eligibility・`Running[].Issue`。`Usage`/`Runtime`/`CodexTotals` は観測用（永続化するが非クリティカル）。

---

## 設計に織り込む正しさの不変条件（stateless/detach が壊す箇所）

1. **claim を spawn より先に永続化（write-ahead）**: crash 時 orphan/stuck-claim 防止。claim record に issue ID・attempt・run ID・`claiming`＋timestamp。
2. **spawn は run ID で冪等**: claim 後 crash → 再 tick で二重起動しない。single-authority（原則5）の stateless 版実現。
3. **orphan-claim reaper**: spawn-grace 超過の `claiming` で run 不在→release / ExecRef で発見→adopt。現 `StartupCleanup` を拡張。
4. **fenced lease**（`WithLease`）= single-writer（原則2）。クラウドは fenced token 必須（失効後 zombie 書込防止）。per-issue atomic CAS を持つ backend なら defense-in-depth 併用。
5. **attempt 増分＋`DueAtMS` 更新は atomic & 冪等**（`(issueID, attempt)` キー）: partial write replay を no-op に。
6. **durable heartbeat**（`LastActivityAtMS`）: 無いと stateless 側で全 worker が `StallTimeoutMS` 後 false-positive kill。鮮度閾値は最悪 tick/poll 間隔超。

---

## 段階的実装計画（各 stage 独立 ship・test 可能、常に動く順序）

**S1. platform KV + StateStore 配線（daemon 背後・挙動不変）**: `platform/kvstore` + orchestrator `statecodec`。`InMemoryStore` を daemon に（load on boot / save on step）。`Reduce` 無変更。
**S2. retry を state 由来に（timer 撤廃）**: `reduceTick` に `DueAtMS<=now` 導出、`s.timers`/`armTimer`/`EvRetryDue` 削除。レイテンシは tick 間隔律速（conformance §8.4 更新）。
**S3. shell factor + RunStore + detach 観測**: `step`/`exec` を Driver から呼べる `Step` に。orchestrator RunStore（platform KV 上）追加。LocalExecutor が RunStore に書き、完了/stall を RunStore からも検出（`EvRunStatus`、`workerExit*` 再利用）。daemon は `Events()` で低遅延維持。
**S4. Executor 抽象 + LocalExecutor**: `orchestrator/exec` 導入。現 `runner.Spawn` を `LocalExecutor` で包み `Deps.Spawn`→`Executor` 置換（`SpawnFunc` は薄い adapter で段階移行）。agent-agnostic 維持。
**S5. DaemonDriver 再構成 + CLI**: 現 loop を Driver 化。`orchestrator serve`（daemon）/ `orchestrator tick`（単発）サブコマンド分割。
**S6. ServerlessHandler**: serverless Driver + `linear/webhook.go`（署名検証・parse は純粋関数）。`/webhook/linear`・`/tick`・`/run/callback` を durable StateStore+lease の shell に配線。`orchestrator webhook`（HTTP-only）追加。webhook=refresh トリガ、真実は reconcile（原則4）。
**S7. CloudExecutor + cloud launch backend**: platform にクラウド起動 backend（agentlaunch 兄弟）＋ client interface。`buildDispatcher` で選択。コンテナ entrypoint（runner one-shot + RunStore 報告 + KillRequested poll）。client は fake でテスト、実 backend は後続。

---

## テスト可能性（原則1・層ごとの seam）

- **Reduce**: 純粋・mock 不要。table test 維持＋拡張（retry due, run-status 遷移）。
- **shell（Step）**: fake `StateStore`＋fake `Executor`＋fake `Tracker`＋fake clock で `Step(ev)` 1回の state 遷移・save・Executor 呼出・**claim-before-spawn 順序**・lease 直列化を検証。
- **Executor**: `LocalExecutor` は fake codex（既存 `agent` 資産）、`CloudExecutor` は fake クラウド client/`httptest` で job 起動・RunStore 報告・Kill。
- **KV/RunStore**: round-trip・並行 writer 直列化・atomic write・golden file。
- **DaemonDriver**: fake clock で tick 前進、reconcile→dispatch→retry→完了が現行同等。
- **ServerlessHandler**: `httptest` で webhook（正/不正署名）・`/tick`・`/run/callback`→`Step` 呼出と永続化を検証。署名検証・parse は純粋 table test。
- **crash 耐性**: tick 中 kill→再 tick で double-dispatch せず orphan claim が reaper で解消。

---

## スコープ外 / 後続
実 Firestore/DynamoDB backend・実 Cloud Run/Lambda デプロイ & IaC（interface 実装は S7、実 backend は別タスク）／tracker の Linear 以外置換／`client/`（tmux は orchestrator 経路に不在）。

## 検証
- `cd src && go test ./orchestrator/... ./platform/kvstore/... ./cmd/orchestrator/...`
- `make build-orchestrator && make vet && make lint`（depguard で層境界、forbidigo で no-mutex を機械検証）
- 統合(local): `LocalExecutor`＋ローカル KV で `orchestrator tick` 連続起動が daemon 同等／`orchestrator webhook` に fake webhook→dispatch。
- 統合(cloud, S7後): fake クラウド client で job 起動→RunStore 完了→次 tick で reconcile。
- conformance: `docs/technical/orchestrator/symphony-conformance.md` §8.4/§16 posture 更新。
