---
change: change-20260716-codex-runtime-restart-continuity
role: requirements
---

# Requirements

## Goal and observable meaning of restart continuity

Runtime server の意図的な再起動後、Web client が同じ Codex session を同じ SessionID/FrameID で再発見でき、persisted ThreadID または RolloutPath による observer attach が Ready になった時点で status が Stopped ではないことを必須とする。

「Claude と同様の Cold/Warm start」は次の三軸に分ける。本変更の必須範囲は (1) identity/status continuity と、その根拠となる conversation locator continuity である。(2) は未解決の compatibility decision、(3) は non-goal とする。

1. **Identity/status continuity**: SessionID、FrameID、ThreadID/RolloutPath と Ready 後の nonStopped status。
2. **Container adoption continuity**: signal restart で既存 container を preserve/adopt するか、graceful cleanup 後に再作成するか。
3. **PTY/turn continuity**: live PTY や実行中 turn を exactly-once で引き継ぐか。

## Functional requirements (EARS)

- **FR-001 (ubiquitous, must)**: システムは、shutdown transaction 開始前に明示的な user stop が commit されていない session を、daemon lifecycle teardown だけを理由に durable store から削除または Stopped へ変更してはならない。
- **FR-002 (event-driven, must)**: shutdown request を受理したとき、システムは Running から Quiescing へ一度だけ遷移し、pre-teardown session 集合の全 file upsert が個別の atomic rename まで完了した all-session Save barrier success を result event として確認しなければならない。
- **FR-003 (state-driven, must)**: Quiescing の間、システムは shutdown transaction event と read-only/connection bookkeeping event 以外の durable session mutation を state と effect の両方で neutralize しなければならない。
- **FR-004 (event-driven, must)**: all-session Save barrier が成功したとき、システムは transaction deadline を持つ `ReleaseFrameSandboxes(RuntimeShutdown)` を開始し、cleanup が完了または deadline に達した result event を受けてから shutdown caller への typed outcome と Runtime-owned terminal action を一度だけ発行し、coordinator cancel の有無に依存せず event loop を有限時間で終了しなければならない。
- **FR-005 (unwanted, must)**: もし all-session Save barrier が一部または全部失敗した場合、システムは通常 cleanup と成功応答を実行せず、IPC caller へ error を返して retry 可能な Running へ rollback し、signal caller へ `commit_failed` と mixed last-successful-per-session store の限定保証を返さなければならない。
- **FR-006 (event-driven, must)**: shutdown request が重複したとき、システムは既存 shutdown transaction に join させ、all-session Save、cleanup result、response、terminal transition を重複実行してはならない。
- **FR-007 (event-driven, must)**: Quiescing 中に外部 mutation request を受けたとき、システムは unavailable error を返し、内部 late mutation event を受けたときは reason-labelled debug counter/log を残して state/effect を変更してはならない。
- **FR-008 (event-driven, must)**: subsystem が `RuntimeShutdown` または `LastFrameRelease` StopCause で process Wait より先に停止意図を確定したとき、システムはその停止を `SubsystemFailed` として通知してはならない。
- **FR-009 (unwanted, must)**: もし Running 中に stream app-server の process Wait が expected StopCause より先に確定した場合、システムは still-bound frame を failure/Stopped として公開し、session identity を durable store から削除してはならない。
- **FR-010 (event-driven, must)**: runtime が再起動したとき、システムは sessions store、ThreadID/RolloutPath、LoadSnapshot/RecreateAll/PrepareLaunch と observer subscription/Ready の既存契約を用い、同じ SessionID、FrameID、conversation locator を Ready 後に nonStopped として Web REST/WS projection へ提示しなければならない。
- **FR-011 (event-driven, must)**: user が Running 中に session/frame stop を明示し、その mutation が shutdown transaction より先に commit されたとき、システムは従来の eviction と persistence delete を実行し、その session を次回 boot で復元してはならない。

## Non-functional requirements

- **NFR-001 (reliability)**: deterministic IPC/signal restart scenario における Codex session identity loss と false-Stopped を 0 件とする。partial failure と強制 signal timeout は mixed last-successful-per-session store までの限定保証として別計測し、graph-wide point-in-time atomicity を主張しない。
- **NFR-002 (observability)**: shutdown transaction ID、phase、all-session barrier outcome、partial upsert count、terminal owner、cleanup degradation、StopCause、late-event class を構造化 log/counter に含める。expected stop は error/failure metric に算入しない。
- **NFR-003 (maintainability)**: LifecyclePhase、shutdown result、StopCause、event admission class は typed enum と exhaustive switch/table contract で検査し、exit code または context error による分散推論を禁止する。
- **NFR-004 (compatibility)**: sessions store wire format、ThreadID/RolloutPath、per-session app-server topology、既存 observer Ready contract、Claude/Gemini behavior を変更しない。
- **NFR-005 (performance)**: restart protocol は既存 persistence/cleanup 以外の network round-trip を追加せず、shutdown transaction ごとの all-session Save 呼出しは最大一回とする。

## Acceptance criteria

- **AC-001 — IPC success**: Given Running の Codex session が sessions store に locator と共に存在する, when IPC shutdown と再起動を行う, then 同じ SessionID/FrameID/locator が REST/WS に現れ observer Ready 後は nonStopped である。
- **AC-002 — signal success**: Given 同じ persisted session, when SIGINT/SIGTERM が graceful deadline 内で shutdown を完了して再起動する, then AC-001 と同じ identity/status continuity を満たし runtime loop は coordinator cancel だけに依存せず終了する。
- **AC-003 — persist failure**: Given multi-session Save seam が一部 upsert 後に失敗を返す, when IPC shutdown を要求する, then prefix/suffix の全 file は load 可能で delete/Stopped はなく、cleanup/ack/terminal は 0、error response 後 retry 可能な Running へ戻る。signal ingress は `commit_failed` を観測し coordinator が fallback cancel を明示的に選ぶ。
- **AC-004 — mutation freeze**: Given Quiescing transaction, when subsystem、frame、spawn、driver hook、job、tick、file/OSC/prompt または mutation RPC の各 representative event が到着する, then durable state と durable effects は変わらず、外部 request は unavailable、内部 event は分類付き診断になる。
- **AC-005 — termination race**: Given Stop intent、Wait completion、parent cancel、duplicate/different StopCause の全順序, when terminal classifier を競合実行する, then 最初の terminal observation だけが正本となり failure emission は高々一回である。
- **AC-006 — real restart/resume fidelity**: Given adapter A が生成した canonical locator を同じ store に保存する, when A を停止して adapter B で resume する, then fake と configured real Codex の双方が同じ canonical identity と observer Ready を返す。
- **AC-007 — explicit deletion**: Given Running 中に user stop が shutdown より先に commit された, when runtime を再起動する, then session は REST/WS projection に復元されない。
- **AC-008 — cleanup deadline**: Given subsystem Stop または sandbox cleanup が永久に block する, when transaction deadline に達する, then `RequestShutdown` は `deadline_exceeded` を返し、Runtime-owned terminal action により `Runtime.Done` は有限時間で閉じ、遅延 cleanup 完了は response/ack/terminal を再発行しない。

## Failure semantics

| Condition | Classification | Required behavior |
|---|---|---|
| Expected StopCause wins | Semantically not an error | reason-labelled lifecycle telemetry; no `SubsystemFailed` |
| Process Wait wins while Running | Recoverable external failure | still-bound frame becomes visibly failed/Stopped; durable identity retained |
| All-session Save partial failure | Recoverable shutdown failure | prefix is newly and individually committed, suffix retains each session's last-successful version; no cleanup/success ack/terminal; IPC rollback to Running; signal returns `commit_failed` |
| Cleanup transaction deadline | Degraded committed shutdown | stop/cleanup workers receive cancellation and may be abandoned for process exit; emit exactly one `deadline_exceeded` result, then terminate without waiting for late completion |
| Illegal lifecycle transition or unknown admission class | Internal contract violation | fail-fast in tests; production error telemetry; never silently admit mutation |
| Signal deadline force-cancel | Degraded external termination | coordinator explicitly chooses fallback cancel; recover only the mixed last-successful-per-session store and emit `deadline_exceeded` |

The existing store is upsert-only and session-per-file. The barrier guarantees that all requested upserts completed before teardown; it does not provide graph-wide point-in-time atomicity. A partial failure may leave a valid mixed store, but lifecycle teardown itself performs no delete.

## Open decision

Signal-time container preserve/adoption と現行 graceful destroy のどちらを compatibility policy とするかは、この session continuity 修正から分離する。どちらを選んでも FR-001〜FR-011 は成立しなければならない。
