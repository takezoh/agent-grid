---
change: change-20260716-codex-runtime-restart-continuity
role: implementation
---

# Implementation design

## Goal

Shutdown を reducer-owned transaction にし、acknowledged all-session Save barrier と deadline-aware cleanup の各 result event を経て response と terminal action を直列化する。並行して、subsystem process の終了意図を race-safe に分類し、既存 Codex cold-start recovery へ保存済み identity を渡す。

## Contracts

### Runtime lifecycle and two-phase shutdown

`state.State` に process-local `LifecyclePhase` (`Running`, `Quiescing`) と active shutdown transaction metadata を置く。これは reducer だけが書き込み、`SessionSnapshot`、sessions store、published/Web read model から除外する。`New`、`Bootstrap`、`LoadSnapshot` は disk の内容にかかわらず必ず `Running` から開始する。

1. `EventShutdown` を Running で受理すると `Quiescing` へ遷移し、transaction ID と waiter を登録して `EffCommitShutdownSessions` だけを発行する。
2. interpreter は pre-teardown session 集合を既存 `Persist.Save` へ一回渡す。success は全 session file の upsert がそれぞれ temp+atomic rename まで完了したことを意味し、`EvShutdownSaveBarrierSucceeded{TransactionID}` を loop へ戻す。途中 error は `EvShutdownSaveBarrierFailed{TransactionID, Error}` を返す。通常の `EffPersistSnapshot` fire-and-log 契約を shutdown に流用しない。
3. barrier success event は `EffReleaseFrameSandboxes{Cause: RuntimeShutdown, Deadline}` だけを発行する。Runtime が signal timeout または IPC default timeout を absolute transaction deadline に正規化し、最初に受理された request の deadline を transaction の正本とする。duplicate waiter は deadline を延長しない。
4. interpreter は subsystem Stop と sandbox cleanup を deadline context 配下の worker として実行し、全 worker 完了または deadline のどちらか一方を `EvShutdownCleanupFinished{TransactionID, Outcome}` として exactly once loop へ返す。`Stop(ctx, cause)` は context を尊重し、非協調 worker も集約 goroutine から切り離すことで interpreter 自体は deadline 後に戻る。遅延 worker 完了は telemetry 以外の event、response、ack、terminal を生成しない。
5. matching cleanup result event だけが joined caller への typed response/ack と `EffTerminateRuntime` を発行する。全 attempt 完了は `committed`、deadline winner は `deadline_exceeded` とし、後者でも保存済み identity safety を巻き戻さず Runtime-owned terminal action へ進む。
6. barrier failure event は cleanup/成功 response/terminal effect を発行せず `Running` へ rollback する。session-per-file upsert-only store では、失敗前 prefix は新しい valid version、suffix は各 session の last-successful valid version となる。lifecycle teardown による delete は行わない。IPC waiter は retry 可能な typed error、signal waiter は `commit_failed` と mixed last-successful-per-session store の限定保証を受ける。
7. duplicate request は同じ transaction に waiter として join する。Save、cleanup coordinator、cleanup result、terminal action は once。Quiescing 中の別 mutation command は join ではなく unavailable。

`Running → Quiescing → Running` は Save barrier failure rollback のみ、`Running → Quiescing → Terminated` は barrier 成功後の cleanup result（completed または deadline_exceeded）受理時だけ許可する。`Terminated` は runtime terminal channel の状態であり durable state には保存しない。

`RequestShutdown(timeout) ShutdownResult` は `committed`、`commit_failed`、`deadline_exceeded` を返す。signal handler は `committed` なら正常完了し、`commit_failed` または `deadline_exceeded` なら outcome を log して fallback `cancel()` を明示的に実行する。ただし cleanup deadline 時は Runtime も同じ absolute deadline で worker 集約を打ち切って terminal action を発行するため、fallback cancel だけに終端性を依存しない。IPC failure は error response と Running rollback、IPC cleanup deadline は degraded error response の後に terminal となる。

この barrier は graph-wide point-in-time atomicity を提供しない。generation/manifest を追加すれば可能だが wire-format migration と reader protocol が必要なので採用しない。

### Central event admission matrix

`state.Reduce` の型 switch より前に pure `classifyEvent(LifecyclePhase, Event) Admission` を置き、new event type を追加したとき exhaustive classification が必要になる closed contract とする。Quiescing で `neutralize` された event は state も durable/non-diagnostic effect も生成しない。

| Event class | Running | Quiescing | Quiescing observation |
|---|---|---|---|
| Shutdown request/result (`EventShutdown`, Save barrier succeeded/failed, cleanup finished/deadline) | begin/advance transaction | duplicate joins; matching result advances; stale/late result ignored | transaction ID, phase, outcome |
| Connection bookkeeping (`EvConnOpened/Closed`, subscribe/unsubscribe) | allow | allow only in-memory bookkeeping/read response | no durable effect |
| Read-only commands (`list`, surface read, driver list, message read) | allow | allow if they cannot enqueue durable effects | current committed pre-teardown view |
| External mutation commands (`create/stop/fork/push/set-head`, surface send/key/write/resize, driver hooks) | allow | reject | `unavailable`, transaction ID |
| Internal session mutation (`EvSubsystem`, `EvFrameSpawned`, `EvSpawnFailed`, `EvFrameVanished`, `EvFrameCommandExited`, `EvJobResult`, `EvFileChanged`, `EvTick`, `EvFrameOsc`, `EvFramePrompt`) | normal reducer | neutralize | debug counter/log with concrete event class and reason `runtime_quiescing` |
| Unknown/new Event implementation | compile/test failure | compile/test failure | no permissive default |

Surface subscribe/unsubscribe は connection/read delivery bookkeeping として allow できるが、backend I/O を起動する effect がある場合は Quiescing では unavailable に分類する。分類は effect の偶然ではなく event contract で固定する。

### Subsystem termination linearization

`Subsystem.Stop(ctx, StopCause)` を必須 contract とし、cause は `RuntimeShutdown` と `LastFrameRelease` に限定する。stream backend は mutex/once（または等価な CAS state）で `terminalObservation` を一度だけ確定する。

| First terminal observation | Later observation | Canonical result |
|---|---|---|
| `Stop(RuntimeShutdown)` | Wait/cancel/duplicate Stop | expected `RuntimeShutdown`; no failure |
| `Stop(LastFrameRelease)` | Wait/different StopCause | expected `LastFrameRelease`; first cause retained |
| process Wait | Stop/parent cancel | unexpected exit; still-bound frames receive failure once |
| parent context cancel without recorded Stop | Wait/Stop | unexpected; cancellation alone does not invent an expected cause |

Stop は意図を確定してから process cancel を行う。Wait は process result を観測してから同じ linearization function を呼ぶ。最初の observation が SSOT で、duplicate/different cause は診断だけを残す。unexpected Wait は bound-frame snapshot を同じ critical section で確定し、failure emit は once とする。CLI implementation は process を所有しなくても typed cause を明示的に受ける。

### Recovery reuse

新しい warm-rebind mechanism は作らない。committed sessions store の `SessionID`、`FrameID`、opaque driver state (`ThreadID`/`RolloutPath`) を `LoadSnapshot → restoreSession → RecreateAll → Driver.PrepareLaunch(LaunchModeColdStart) → stream.BindFrame` に渡す。accepted observer subscription/Ready ADR の canonical identity validation と one-shot Ready を経て初めて Web status continuity を満たしたと判定する。

## Implementation sequence

### M1 — lifecycle core and all-session Save transaction

- `src/client/state/state.go`, `event.go`, `effect.go`, `reduce.go`, `reduce_lifecycle.go`: process-local lifecycle、shutdown result events/effects、central admission matrix を追加。
- `src/client/runtime/interpret.go`, `runtime.go`, `persist.go`: all-session Save result feedback、absolute transaction deadline、deadline-aware cleanup coordinator、joined waiters、Runtime-owned terminal channel、`RequestShutdown(timeout) ShutdownResult` を結線。
- `src/cmd/server/coordinator.go`: normal termination を runtime terminal result に収束し、signal deadline のみ cancel fallback とする。
- T0 matrix/projection tests と `runtimetest.Harness` の success/failure-after-prefix/duplicate/deadline tests を同じ milestone で追加する。multi-session partial failure は Save order に依存せず、全 file が valid-loadable、delete/Stopped なし、cleanup/ack/terminal 0 を検証する。永久 block fake では `deadline_exceeded`、bounded `Runtime.Done`、late completion の二重 terminal なしを検証する。

### M2 — typed subsystem termination

- `src/client/runtime/subsystem/subsystem.go`: `StopCause` と required `Stop(ctx, cause)`。
- `src/client/runtime/subsystem/stream/backend.go`: terminal observation linearizer と reason telemetry。
- `src/client/runtime/subsystem/cli`, factory Reaper、interpreter、全 fake/test double を compile-time contract へ移行。
- race table と T2 invariant-named conformance を追加する。

### M3 — restart recovery and user-reachable scenario

- 既存 bootstrap/recovery path に lifecycle projection が混入しないことを固定し、same-store daemon A→B restart harness を追加。
- `src/server/web` Go gateway scenario で restart 前後の REST list と WS view update を観測し、same SessionID/FrameID/locator、Ready 後 nonStopped を assert。
- explicit user stop の negative scenario を追加。

### M4 — external dependency triple and promotion readiness

- deterministic fake app-server に store-preserving A→B restart/resume scenario を追加。
- invariant-named T2 contract で canonical locator/observer Ready を固定。
- opt-in T3 `FakeVsRealCodexRestartResumeContinuity` を同一 adapter harness で fake/real に適用。
- full verification 後、ユーザーが ADR と container policy を承認した場合だけ `design-client` promotion を行う。

## Targets and seams

| Boundary | Production target | Seam / pure core |
|---|---|---|
| Lifecycle/admission | `src/client/state/{state,event,effect,reduce,reduce_lifecycle}.go` | pure `classifyEvent` と reducer; no I/O |
| Persistence | `src/client/runtime/{interpret,persist}.go` | existing session-per-file upsert-only `Persist.Save/Load`; fake all-session success/failure-after-prefix and per-file atomic implementation |
| Runtime termination | `src/client/runtime/runtime.go` | Runtime-owned typed result/done channel and absolute shutdown deadline; caller cancel is fallback only |
| Signal ingress | `src/cmd/server/coordinator.go` | existing shutdown request function seam; fake terminal result/deadline |
| Subsystem process | `src/client/runtime/subsystem/subsystem.go`, `stream/backend.go`, `cli/backend.go` | pure/mutex terminal classifier; existing spawn `Wait` seam and deterministic fake process |
| Codex protocol/store | `src/client/runtime/subsystem/stream/{resume,subscription,fake}` | common restart/resume adapter used by fake and real T3 |
| Recovery | `src/client/runtime/{bootstrap,bootstrap_coldstart}.go`, Codex driver | existing Persist loader, driver state, `PrepareLaunch` seams |
| Web observation | `src/server/web/mux_scenario_test.go` | real gateway over `runtimetest.Harness`; deterministic fake backend, REST/WS public contract |

No new third-party dependency is required.

## Error semantics and observability

- Semantic redefinition: expected StopCause is not an error and must not increment failure metrics.
- Recoverable external failures: Save partial failure and unexpected Wait return/emit typed outcomes while preserving each session's loadable last-successful identity version. Cleanup deadline is a degraded committed shutdown: worker completion is abandoned, `deadline_exceeded` is emitted once, and terminal progress continues.
- Internal contract violations: unknown lifecycle transition, stale transaction accepted as current, or unclassified Event fail tests and emit production error telemetry.
- Structured fields: `shutdown_transaction`, `lifecycle_phase`, `save_barrier_outcome`, `sessions_attempted`, `sessions_upserted`, `shutdown_result`, `cleanup_degraded`, `terminal_owner`, `stop_cause`, `terminal_winner`, `event_class`, `late_event_reason`, `session_id`, `frame_id` where applicable.
- Metrics/counters: Save barrier success/partial failure/forced fallback, cleanup degradation, late event by class, expected stop by cause, unexpected process exit, duplicate stop cause mismatch. High-cardinality IDs remain logs, not metric labels.

## Open question

Signal-time containers の preserve/adopt と graceful destroy の選択は compatibility ADR に残す。この implementation sequence は `EffReleaseFrameSandboxes` の sandbox cleanup policy を独立注入点として扱い、どちらの決定でも session snapshot/terminal contract を変更しない。
