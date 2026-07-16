---
change: change-20260715-terminal-scrollback-reflow
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260715-terminal-scrollback-reflow
kind: plan
title: Terminal scrollback reflow contract and structure
status: done
created: '2026-07-15'
goal: Preserve semantic terminal history and attach it at the current viewer geometry
  without stale-width hard breaks or seed/live races.
scope_in:
- Existing x/vt fork semantic history and opaque ANSI reattach snapshot
- termvt AttachAtGeometry and SubscribeCurrent contracts
- Geometry-bearing subscribe protocol, gateway, runtime, and browser desired controller
- Cross-layer ANSI fidelity and migration gates
scope_out:
- Persistent terminal history across daemon restart
- Multi-viewer arbitration policy changes
- Raw PTY replay and xterm private buffer APIs
milestones:
- id: m1
  title: Release the VT semantic-history contract
  status: done
- id: m2
  title: Introduce termvt attach and observation seams
  status: done
- id: m3
  title: Migrate protocol runtime and gateway consumers
  status: done
- id: m4
  title: Make browser desired attach geometry-aware
  status: done
- id: m5
  title: Close fidelity gates and delete legacy serialization
  status: done
contracts:
- x/vt fork is the only owner of hard/soft provenance and xterm-compatible ANSI snapshot
  rendering.
- AttachAtGeometry is a last-writer-wins session size write linearized by the termvt
  actor mailbox.
- SubscribeCurrent observes without changing global geometry.
- Geometry-bearing subscribe is required before seed publication; outbound output
  remains ANSI bytes.
- The old newline serializer is deleted after all consumers and gates migrate; it
  is never a fallback.
adrs:
- adr-20260715-terminal-semantic-history-reattach
- adr-20260715-geometry-bearing-terminal-attach
tags:
- terminal
- scrollback
- reflow
owners: []
relations:
- {type: implements, target: spec-20260715-terminal-scrollback-reflow}
- {type: hasPart, target: adr-20260715-terminal-semantic-history-reattach}
- {type: hasPart, target: adr-20260715-geometry-bearing-terminal-attach}
source_paths:
- src/platform/termvt/
- src/client/runtime/
- src/server/web/
- src/client/web/
- src/go.mod
methodology: sdd
summary: Plan the VT history model, reattach snapshot contract, ordered geometry application,
  consumer migration, and regression verification.
updated: '2026-07-15'
---

## Goal

terminal semantics の owner、geometry attach の linearization point、browser desired subscription の wire ownershipを一意にし、狭い幅で蓄積された履歴を広い device で正しく reflow する。旧 physical-row newline seed は移行完了時に削除する。

## Approach

変更は外側から新たな parser を足すのではなく、既存 `github.com/takezoh/x/vt` fork の capability を強化して release pin を更新する。fork は mutation-closed provenance と opaque ANSI snapshot を提供し、termvt actor は fallible PTY prepare と infallible VT commit を直列化する。runtime/protocol は viewer attach と internal observation を分離し、browser controller は初回 fit、reconnect、session switch の geometry-bearing subscribe を唯一の wire writer として送る。

## Implementation Sequence

### m1: Release the VT semantic-history contract

fork 側で hard/soft boundary、head-truncated、exact-column pending wrap、cell/style/cursor の保存則を定義する。write/overwrite、erase、insert/delete char/line、LF/index、region/full scroll、resize、cap eviction、primary/alternate transitionを全 row mutation の閉じた分類にする。target geometry から opaque xterm-compatible ANSI snapshot を生成する API、deterministic fake fixture、invariant-naming contract、FakeVsReal を同じ release に含める。

成果物は fork release/tag、API contract、fixture corpus であり、agent-grid の `src/go.mod` replace pin をその release に更新する。新規 dependency は追加しない。

### m2: Introduce termvt attach and observation seams

`Emulator` seam を opaque snapshot capability に更新し、`Session.Subscribe()` を `AttachAtGeometry` と `SubscribeCurrent` に分ける。actor command は geometry validation、PTY SetSize prepare、VT resize/snapshot/seed/subscriber commit を実装する。seed publish 後の live event は同じ mailbox orderで一度だけ続く。

PTY prepare failure は old size/VT/subscriber 不変、commit後の impossible VT/snapshot/enqueue failure は session unusable + subscriber sever とする。`pty_tap` は no-size-change API へ移す。

### m3: Migrate protocol runtime and gateway consumers

subscribe inbound request に cols/rows を必須追加し、runtime `PtyBackend` から `AttachAtGeometry` へ到達させる。gateway/protocol fixture は missing、0、negative/non-integer、maxDim超過を購読前に拒否する。outbound terminal output frame は ANSI bytes を維持する。

### m4: Make browser desired attach geometry-aware

`TerminalSubscriptionController` が fitted geometry を desired state に保持する。initial fit 前は wire subscribe を保留し、open/reconnect/session switch は最新 geometry を必須送信する。post-subscribe resize は既存 `Resize` path を使う。ownership/connection generation が失効した結果と seed は採用せず current desired を reconcile する。React は引き続き acquire/release と fit notificationだけを担当する。

### m5: Close fidelity gates and delete legacy serialization

共通 fixture を T0 fork/fake、T2 real x/vt+PTY、T1 gateway/browser の各 gate へ投影する。全 consumer 移行と matrix 通過後に `SerializeScrollback`、二段 scrollback/visible-grid seed、旧 adapter を削除する。旧 spec FR-004/FR-006とADR 0066の置換をrelationとmigration checkで閉じる。

## Task-grade Units

| Unit | Depends on | Objective / output | Boundary | Acceptance |
|---|---|---|---|---|
| U1 Fork semantic history | — | x/vt fork releaseにmutation-closed provenance、opaque ANSI snapshot、fake/contract/FakeVsRealを追加 | agent-grid parser/rendererは作らない | mutation全分類、matrix fixture、release tag |
| U2 Pin and narrow Emulator seam | U1 | `src/go.mod` pinと`src/platform/termvt/session_deps.go`をopaque snapshot APIへ更新 | actor/protocolはまだ変えない | compile-time capabilityとfake一致 |
| U3 Linearizable termvt attach | U2 | `AttachAtGeometry`、`SubscribeCurrent`、prepare/commit、fail-fastをactorへ追加 | browser wireは触らない | failure postconditionとseed/live ordering tests |
| U4 Migrate runtime consumers | U3 | `pty_tap`→SubscribeCurrent、PtyBackend→AttachAtGeometry | WebSocket controllerは触らない | tap no-resize、viewer size-write tests |
| U5 Geometry-bearing protocol | U3 | daemon/browser subscribe schema、gateway validation、wire fixturesを更新 | outbound ANSI shapeは変えない | invalid input cannot subscribe |
| U6 Desired controller geometry | U5 | initial/reconnect/switch geometry readinessとgeneration discardをcontrollerへ実装 | second wire writerを作らない | initial fit hold、latest geometry retry tests |
| U7 Cross-layer fidelity and deletion | U4, U6 | gateway scenario、Playwright matrix、legacy serializer削除 | arbitration/persistenceは変えない |全gate green、legacy symbol 0件 |

## Targets and Seams

| Target | Files / external deliverable | Seam and responsibility |
|---|---|---|
| VT semantic owner | existing `github.com/takezoh/x/vt` fork release; `src/go.mod` | provenance mutation closure、opaque ANSI snapshot、fork fake/real contract |
| termvt dependency boundary | `src/platform/termvt/session_deps.go` | opaque bytesとtyped failureのみ。semantic rowsを公開しない |
| termvt actor | `src/platform/termvt/session.go`, `session_actor.go`, focused tests | AttachAtGeometry/SubscribeCurrent、prepare/commit、last-writer-wins ordering、fail-fast |
| internal tap | `src/client/runtime/pty_tap.go`, tests | SubscribeCurrent; global geometryを変更しない |
| viewer relay | `src/client/runtime/pty_backend.go`, tests | geometry-bearing requestをAttachAtGeometryへ渡す |
| protocol/gateway | protocol message definitions、`src/server/web/gateway.go`, `wire*_test.go`, gateway scenarios | cols/rows parse/bounds、購読前拒否、outbound ANSI維持 |
| desired controller | `src/client/web/src/socket/terminalSubscription.ts`, `gatewaySocket.ts`, tests | fitted geometry state、唯一wire writer、generation adoption |
| fit owner | `src/client/web/src/components/TerminalPane.tsx`, tests | rAF-coalesced fitをcontrollerへ通知。wire orderingを所有しない |
| browser fidelity | `src/client/web/e2e/` and support fake backend | public xterm bufferでrows/isWrapped/cursor/re-resizeを観測 |

## ANSI Fidelity Contract Matrix

各 fixture は fork input transcript/mutations、attach geometry、expected ANSIまたはsemantic digest、xterm observable rows/isWrapped/cursor/style、もう一度 resize した結果を持つ。FakeVsReal は同じ fixture と geometry を使う。

| Case | Required input distinction | Attach observable | Re-resize observable |
|---|---|---|---|
| soft boundary | autowrap only | wider widthでlogical textが連結し、必要なcontinuationだけisWrapped | shrinkでsoft rowが再生成されexpandで再連結 |
| hard boundary | CR/LF/NEL | full-width rowでも次行と連結しない | width変更後もhard boundary維持 |
| exact-column pending wrap | glyphが最終列を埋め、次print前/後を区別 | pending stateが不要な空行やhard breakを作らない | 次printのsoft continuation位置が一致 |
| trailing erased cells | text後にEL/erase/overwrite | visual blanksとlogical continuationが混同されない | blanksを理由にwrap provenanceが変わらない |
| SGR spans | rowを跨ぐstyle start/reset | cell styleとreset leakageがexpectedに一致 | reflow後もgraphemeにstyleが追随 |
| wide + combining | CJK wide cell、combining mark、境界直前配置 | replacement glyphやsplit graphemeなし | cols変更ごとにcell widthとwrapが一致 |
| cursor + EL | snapshot endでCUP/ELが必要 | cursor row/colとtrailing eraseが一致 | reflow後cursor anchorがsemantic contentに追随 |
| cap head fragment | soft line prefixがevict済み | retained fragmentをcol 0から表示、prefix/hard break捏造なし | fragmentだけを再flow |
| combined stress | soft+hard+SGR+wide+EL+cursor+cap | rows/isWrapped/style/cursorの全digest一致 | shrink→expand後もdigest一致 |

## Failure and Observability Contract

| Phase | Failure | Classification | Observable result |
|---|---|---|---|
| request | missing/invalid geometry | external permanent for request; degrade | warn/drop or protocol response per existing posture; no actor mutation |
| prepare | PTY SetSize | external transient; retry | desired remains; old size/VT/subscribers unchanged |
| commit | VT invariant/snapshot | internal contract; fail fast | session unusable、all subscribers severed、structured error with session/geometry/stage |
| commit | seed queue impossible admission | internal contract; fail fast | no silent partial attach; session unusable if PTY already committed |
| adoption | disconnect/cancel generation mismatch | lifecycle race; retry | stale seed dropped; current desired reconciled with latest geometry |
| release | fork fake/real drift | dependency contract; fail fast gate | CI/release stops; no runtime legacy fallback |

## Verification

project AGENTS の Tier を優先し、Playwright は T1 とする。T3 external-binary fidelity はこの変更の必須 gate にしない。

| Tier | Command (verbatim) | 判定基準 | Milestone |
|---|---|---|---|
| T0 pure/fake/proto/controller | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./platform/termvt/... ./client/runtime/... ./server/web/...` | mutation invariants、prepare/commit postconditions、proto bounds、consumer mode、seed/live orderがgreen | m2, m3 |
| T0 web unit | `cd src/client/web && npm run test:unit` |初回fit保留、latest geometry、reconnect/switch、generation discard、single wire writerがgreen | m4 |
| T1 Go gateway scenario | `cd src && GOCACHE=/tmp/gocache-agent-grid go test ./...` | real gateway + fake agentでgeometry-bearing subscribeからANSI seedまでwired | m3, m5 |
| T1 browser xterm fidelity | `cd src/client/web && PLAYWRIGHT_BROWSERS_PATH=/tmp/ms-playwright npm run test:e2e` | matrixのrows/isWrapped/cursor/styleとshrink→expandが一致 | m5 |
| T2 real x/vt/PTY contract | `cd src && GOCACHE=/tmp/gocache-agent-grid go test -run 'Test(ReattachSnapshotContract|FakeVsRealReattachSnapshot|AttachAtGeometryContract)' ./platform/termvt/...` | hermetic real PTY + pinned forkがfakeと同一fixture/digest、invariant nameとfailure postcondition一致 | m1, m2, m5 |
| static/lint | `GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint` | dependency direction、API deletion、Go static gatesがgreen | m5 |
| web lint | `cd src/client/web && NPM_CONFIG_CACHE=/tmp/npm-cache-agent-grid npm run lint` | TypeScript protocol/controller contractがgreen | m4, m5 |

T0/T2の外部依存三点セットは (1) deterministic semantic fake、(2) `TerminalSemanticMutationClosure` / `AttachSeedPrecedesLiveWithoutGap` 等の invariant-naming contract、(3) `FakeVsRealReattachSnapshot` である。T1 gatewayは wiring、Playwrightは xterm public API fidelityを担当し、同じ責務を重複しない。

## Migration Completion

- fork release と `src/go.mod` pin が同じ semantic snapshot contract を指す。
- `pty_tap` は `SubscribeCurrent`、browser relay/PtyBackend は `AttachAtGeometry` に移行済み。
- subscribe inbound fixture は geometry required、outbound output fixture は ANSI bytes維持。
- `rg 'SerializeScrollback|uv\.Lines\(.*Render' src` が legacy seed pathについて0件。
- ANSI matrix、FakeVsReal、gateway、Playwright、lintがgreen。
- 新ADRの supersedes/reference/partOf relationがdocs lintで整合し、旧 spec FR-004/FR-006のmigration mapを本specから追跡できる。

## Open Questions

実装開始前に追加の product decision はない。fork release version と concrete protocol field names は実装単位で決められるが、このplanのownership/behaviorを変更してはならない。


{% transition from="draft" to="active" date="2026-07-15" %}
Implementation started by user request
{% /transition %}


{% transition from="active" to="done" date="2026-07-15" %}
All milestones implemented and verified against the published VT commit
{% /transition %}

````
