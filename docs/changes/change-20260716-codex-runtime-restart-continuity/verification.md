---
change: change-20260716-codex-runtime-restart-continuity
role: verification
---

# Verification

## Profiles

| Profile | Tier | Command | Pass criterion |
|---|---|---|---|
| lifecycle-pure | T0 pure | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/state` | Running/Quiescing transition、exhaustive admission matrix、projection exclusion、duplicate join、commit failure rollback が table tests で成立する。 |
| termination-pure | T0 pure | `cd src && GOCACHE=/tmp/gocache-agent-grid go test -race ./client/runtime/subsystem/stream` | Stop/Wait/parent-cancel/duplicate/different-cause の全順序で winner と failure emission が一意。 |
| runtime-wired | T1 wired | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime` | `runtimetest.Harness` が all-session Save success/failure-after-prefix、typed IPC/signal outcome、deadline-aware cleanup result、bounded Runtime.Done、A→B same-store recovery、explicit deletion を公開操作で証明する。 |
| gateway-user | T1 wired | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./server/web` | REST/WS から restart 前後の same SessionID/FrameID/locator と Ready 後 nonStopped を観測できる。 |
| subsystem-contract | T2 contract | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./client/runtime/subsystem/...` | `Contract_*` invariant tests が全 Subsystem/test double の StopCause と unexpected-exit semantics を固定する。 |
| full-go | T0-T2 gate | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...` | always-on suite が pass し、新しい Event/Stop signature の未分類 caller がない。 |
| static-and-race | T0-T2 gate | `GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint && make test-race` | lint、compile-time boundary、race detector が pass。 |
| restart-resume-fidelity | T3 fidelity | `GOCACHE=/tmp/gocache-agent-grid make test-e2e` | configured real Codex と fake の双方で adapter A locator 保存→A停止→same-store B resume→same canonical identity + observer Ready が成立し、unset binary は既存 policy どおり skip。 |

## Required test matrix

### T0 lifecycle admission

- 全 `state.Event` concrete type を Running/Quiescing の classification table に登録する。event.go に型追加して table 未追加なら test/compile gate が失敗する。
- Quiescing の internal mutation event は state deep-equal、durable effect 0、reason counter 1。
- external mutation RPC は `unavailable` response だけを返し、persist/broadcast/spawn/kill effect は 0。
- read-only/connection event は sessions/driver state と persistence に触れない。
- LifecyclePhase と transaction metadata が `snapshotSessions()` と published/Web projection に含まれず、boot は Running。
- Save barrier partial failure は Running rollback、cleanup/ack/terminal 0。成功は release を開始し、matching cleanup result 後だけ response/ack→terminal を各一回、duplicate は join。

### T0/T1 termination races

- `RuntimeShutdown → Wait`, `LastFrameRelease → Wait`, `Wait → each Stop`, duplicate same cause, duplicate different cause, parent cancel before/after each side。
- first observation/cause retention、failure emission at-most-once、still-bound frame snapshot、zero-bound last-frame case。
- expected stop は `SubsystemFailed` 0、unexpected Wait は bound frame ごとに 1。

### T1 restart and Web observation

- `runtimetest.Harness` で daemon A に persisted Codex frame を作り、shutdown success 後 daemon B を同じ store で bootstrap。
- gateway の REST response と WS view update の双方で same SessionID/FrameID を確認し、ThreadID/RolloutPath の canonical equality と observer Ready 後 `status != stopped` を確認。
- Save error-after-prefix、signal `commit_failed`/`deadline_exceeded` fallback、explicit stop-before-shutdown、late `EvFrameCommandExited(0/137)`、late `SubsystemFailed` を含める。
- 2件以上の session で途中失敗させ、iteration order に依存せず全 file が valid-loadable で、各 session が新 version または last-successful version のいずれか、意図しない delete/Stopped なし、cleanup/ack/terminal 0 を確認する。graph-wide point-in-time equality は assert しない。
- subsystem Stop または sandbox cleanup が永久 block する fake を deadline context 配下で実行し、`RequestShutdown=deadline_exceeded`、fallback cancel 後も `Runtime.Done` が bounded time で閉じること、遅延 cleanup 完了が二重 response/ack/terminal を起こさないことを確認する。
- gateway test は内部 state 直接挿入を outcome assertion に使わず、公開 create/list/WS/restart 操作で到達する。

### External dependency triple

1. **Fake**: connection/store-scoped app-server A/B を deterministic に起動し locator を保存・resume する。
2. **Contract**: `Contract_RestartResumePreservesCanonicalIdentityAndObserverReady` を fake adapter と protocol client の invariant とする。StopCause 自体は agent-grid internal contract なのでこの T3 invariant へ混在させない。
3. **Fidelity**: 同じ harness を configured real Codex adapter に適用し、A の canonical locator を A 停止後の B が同じ store から resume して canonical identity + observer Ready を返すことを比較する。

## Completion gate

- AC-001〜AC-008 が対応 profile で pass。
- NFR-001 reliability、NFR-002 observability、NFR-003 maintainability、NFR-004 compatibility、NFR-005 performance が各 profile の判定基準へ trace される。
- unexpected app-server failure の visibility を弱めず、expected shutdown failure metric が 0。
- sessions store fixture/wire format に差分なし。
- 新規 third-party dependency なし。
- proposed ADR は user consultation 前に accepted へ遷移しない。
- container preserve/destroy 未決でも identity/status continuity test が同じ contract で pass する。
