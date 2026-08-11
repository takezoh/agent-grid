# design plan — Fleet session id ownership & Workspace IPC response envelope

> change package: `docs/changes/change-20260811-fleet-session-id-ssot/`
> granularity profile: `boundary-complete`
> base: draft-2 (critique winner)。draft-1 からの graft 10 件と critique issue 16 件を統合済み。

<!-- anchor: goal -->
## Goal

2 つの id 空間 — SPA の compound fleet key `JSON.stringify([serverId, sessionId])` と gateway の bare upstream id —
の変換について、**境界ごとに公開 API が話す id 空間を 1 つだけに固定する**ことで、交差点に散った 8 種類の
失敗方針を 3 人の所有者へ収束させる。あわせて、Electron IPC が HTTP レスポンスを再物質化する境界に
「null body status は非 null body を持てない」という不変条件の所有者を置き、成功した操作が network error として
誤報される欠陥を閉じる。

**母数の定義 (3 つの数を混ぜない)**

| 数 | 意味 | 出所 |
|---|---|---|
| 19 | 上流 horizontal_scan が列挙した call site 総数 | debug run `investigation-2.json` horizontal_scan |
| 15 | そのうち helper 名 (`sessionKey` / `parseSessionKey`) を key にして hit した交差点 | 同上 |
| 17 | invariant を key にした再走査で 2 件 (A-13 / A-14) を加えた**交差点の総数** | 本 design §additional-crossings |

19 − 17 = 2 は「helper を呼ぶが id 空間をまたがない call site」(表示・ログ用途) であり交差点ではない。
以後「全交差点を閉じた」は **17** を母数とする。所有者集合との一致は
`contract-identity-ownership-gate` の allowlist で機械的に検算できる。

本 change の成功は「3 本の RED が GREEN になること」ではなく、**新しい交差点が無音では作れなくなること**で測る。
症状駆動の修正が 2 世代 (9ef53ddc / 525b3091) 生き延びた原因は検出器の不在であり、検出器を残さない修正は
3 度目を構造的に招く。

<!-- anchor: scope -->
## Scope

**in scope**

- `clients/ui/src/socket/connection.ts` + `clients/ui/src/socket/retry.ts` — WS wire 境界の所有権確立
  (outbound frame 組み立て権威の移管、inbound 再修飾の完全化、retry 経路の直列化 bypass 閉塞)
- `clients/ui/src/components/TerminalPane.tsx` (+ `DriverShortcutBar.tsx` / `TerminalMobileOverlay.tsx`) — data plane を intent API へ移行
- `clients/ui/src/fleet/session-protocol.ts` からの identity 所有者モジュール切り出しと `SessionRoute` resolver 新設
- `clients/ui/src/wire/server.ts` — wire mirror 型と store row 型の分離
- `clients/ui/src/App.tsx` — fleet key を鋳造する唯一の producer の明示化
- `clients/ui/src/api/{sessions,transcripts,workspace,capsules}.ts` — 4 実装の resolver 集約と失敗方針の単一化
- `clients/ui/src/store/{daemon,statusSignals,workspaceActivity}.ts` — alias 総当たりの撤去と exact equality 化
- `clients/workspace/src/shared/http-envelope.ts` — IPC response envelope 型と builder の新設 (不変条件の所有者)
- `clients/workspace/src/main/electron-entry.ts` / `clients/workspace/src/preload/index.ts` /
  `clients/workspace/src/preload/electron-preload.cjs` / `clients/ui/src/auth.ts` / `clients/ui/src/fleet/provider.ts` —
  envelope 3 宣言 + 実行時 bridge の順序付き移行
- 交差点の追加を機械的に検出する gate (fitness test + 型 + API 形状)

**out of scope (change.md non_goals + 本 design で切り離した項目)**

- 症状「Workspace Electron でターミナルが表示されない」の修正 (未帰属。Electron 実機 1 回の観測待ち)
- gateway が解決不能な session id を無音で捨てる挙動の変更
  (`services/server/api/gateway_lifecycle_inbound.go:124-140`。別 issue で追跡。
  本 design では「なぜ無音だと 3 度目が起きるか」の根拠としてのみ参照)
- wire protocol (frame 形状・kind) の変更
- fleet 構成 / `servers.json` 契約の変更
- `connection.ts:327` / `:343` の `publishSessions !== false` 分岐 (production 未到達) の**削除**
  — ただし当該分岐は §decision-1 の帰結として単一再修飾経路へ合流させる (削除ではない)
- `store/frameMessaging.ts` が production で populate されない件の是正
- branded type (`FleetSessionKey` / `UpstreamSessionId`) の全面導入 — 別 change へ明示的に繰り延べ (§decision-4)
- **`clients/workspace/src/shared/session-ref.ts` の fleet key encoder 二重実装の fixture 拘束**
  — 本 design で切り離した (§decision-7)。follow-up issue で追跡する
- `e2e/real-stack/terminal-input.spec.ts` の CI 常設 (§decision-8)

<!-- anchor: approach -->
## Approach

境界を 3 つに引き直し、それぞれに **1 つの id 空間**と **1 人の所有者**を割り当てる。

| 境界 | 所有者 | 公開 API が話す id 空間 | 内側に閉じる id 空間 |
|------|--------|--------------------------|------------------------|
| WS wire | `Connection` (`socket/connection.ts`) | fleet key のみ | upstream bare id + wire frame の組み立て |
| store / HTTP routing | `SessionRoute` resolver + `FleetSessionRow` 型 | fleet key + 構造化 route | 文字列からの再導出 (廃止) |
| Electron IPC transport | `HttpResponseEnvelope` (`clients/workspace/src/shared/`) | `body: string \| null` | `Response` の再物質化差異 |

手段 (型・API 形状・test) はこの割り当ての **帰結としてのみ**導入する。特に:

- wire id が Connection の外へ出ないことを保証するために frame 組み立てと直列化を Connection private に閉じ、
  data plane は intent API を呼ぶ。これは「変換を 1 つ足す」ではなく **frame 組み立て権威の移管**である
  (RCA Why 3 が指摘した「変換責務は Connection、組み立て権威は TerminalPane」というねじれの解消)。
  この移管は `socket/retry.ts` の内部 bypass も同時に閉じなければ day-1 で偽になる (§decision-4)。
- store 側の権威 (`upstreamSessionId`) を「optional な field」から「required な別型のフィールド」へ移すことで、
  未設定という状態を構成不能にする。decision point 1 の「未設定時方針」は不要になる。
- envelope は `{status, headers, body}` の `body: ""` が「本文なし」を意味する状態を廃し、
  `design-host.md` の IPC Type Design Invariants (「別意味を持つ zero value は別の型へ」) を適用する。

## Design dimensions

`data_model` / `control_flow` / `state_lifecycle` / `concurrency` / `failure_recovery` /
`integration_contract` / `migration_compatibility` / `user_observability` / `performance_budget`

`concurrency` を宣言する根拠は `contract-store-identity-equality` の risk tag と 2 つの反例:
同一 upstream id を持つ別 server session (`["srv-a","s1"]` と `["srv-b","s1"]`) の同時存在で status signal の
consume/seed map が混ざらないこと (adversarial 2)、再接続 hello の replay 抑止が alias 撤去後も保たれること
(adversarial 3)。加えて `contract-http-route-resolution` の `input-fleet-session-row` は
`mutable_between_acquire_and_use: true` (解決と要求発行の間に session が消える race) を持ち、
resolver が存在を保証しないことを adversarial (3) で明示している。

## Functional requirements

- **FR-A1** SPA が送出する全 WS frame の `sessionId` は upstream (bare) id である。 <!-- anchor: fr-a1 -->
- **FR-A2** `Connection` の公開 API は fleet key のみを受け取り、`ClientFrame` の組み立てと直列化は
  `Connection` の内部に閉じる。 <!-- anchor: fr-a2 -->
- **FR-A3** data plane (`{k:"i"}` / `{k:"r"}`) の id 解決失敗は例外を送出せず、frame を破棄し、
  reason 別に分類された観測可能な記録を残す (内部契約違反はユーザーに到達可能、正常遷移由来は
  診断可能な counter)。 <!-- anchor: fr-a3 -->
- **FR-A4** gateway から到着する全ての session id 保持要素 (output frame[3] / `tt` / `et` /
  `activity_session_id` / activity event の `session_id` / hello・view の sessions) は
  store へ渡る前に **単一の再修飾経路**を通って fleet key へ再修飾される。 <!-- anchor: fr-a4 -->
- **FR-A5** store / HTTP 境界の id 解決は単一の resolver が行い、api モジュールは独自の再導出を持たない。 <!-- anchor: fr-a5 -->
- **FR-A6** 解決不能な session id に対する結果は 1 種類の typed failure であり、HTTP 要求は発行されず、
  その失敗は network error へ写像されない。 <!-- anchor: fr-a6 -->
- **FR-A7** store 行は `(serverId, upstreamSessionId)` を required で保持し、session 同一性の判定は
  fleet key の厳密一致で行う (alias 総当たりを持たない)。 <!-- anchor: fr-a7 -->
- **FR-A8** これまで無音で捨てられていた data plane frame (`{k:"r"}` と制御シーケンスを含む `{k:"i"}`) が
  初めて gateway へ到達することを挙動活性化として扱い、活性化が既存セッションの表示を壊さないことを
  観測で確認する。 <!-- anchor: fr-a8 -->
- **FR-B1** IPC response envelope は「本文なし」を `body: null` で表し、main は null body status の
  ときだけ null を、それ以外では string を出す。 <!-- anchor: fr-b1 -->
- **FR-B2** `FleetProvider.request` は browser host と Workspace host で、同一の HTTP 交換に対し
  観察上等価な `Response` を返す。null body を消費する全 consumer が null を安全に扱う。 <!-- anchor: fr-b2 -->
- **FR-B3** envelope 型の変更は consumer-first の順序で適用され、途中状態でも renderer が壊れない。 <!-- anchor: fr-b3 -->
- **FR-C1** identity 変換または wire frame 直列化を新たに行うモジュールが所有者集合の外に増えたとき、
  機械的に失敗する gate が存在し、その gate 自身が発火することが示されている。 <!-- anchor: fr-c1 -->

## Non-functional requirements

- **NFR-PERF-001** inbound の hot path (FR-β07 が kHz を想定する output frame) の 1 frame あたり処理は、
  現行 (`sessionKey()` 1 回) を上回る割り当てを増やさない。再修飾はセッション単位で memo 化する。 <!-- anchor: nfr-perf-001 -->
- **NFR-PERF-002** 打鍵経路 (`{k:"i"}`) の id 解決は、compound key でない値に対して JSON parse を行わない
  (先頭文字による早期棄却)。 <!-- anchor: nfr-perf-002 -->
- **NFR-DEP-001** 新規 runtime 依存を追加せず、lint は Biome のままとする。 <!-- anchor: nfr-dep-001 -->
- **NFR-TEST-001** 追加する fake は real より寛容にしない。fake が real と乖離した場合は fake を直す
  (AGENTS.md / `adr-20260705-test-tier-taxonomy` の external-dependency triple)。 <!-- anchor: nfr-test-001 -->

## Accepted ADR context

`docs/adr/` を走査し、id 所有権・client host parity・transport・typing policy に関わるものを抽出した。

**適用する (applicable)**

- `adr-20260727-server-qualified-identity-partial-failure` — 「Project/session identity is `(serverId, localId)` end-to-end」。
  本 change はこれを **refine** する (どの境界でどちらの成分が可視かを固定する) のであって覆さない。
- `adr-20260727-application-scoped-fleet-session-authority` — 「`serverId` is an internal routing identity and never
  creates ... an implicit default server」「Views own only `activeSessionRef`」。FR-A1 の直接根拠。
- `adr-20260803-resilient-fleet-health-single-authority` — `clients/ui/src/fleet/session-protocol.ts` が
  versioned credential-free wire schema と decoder を所有し、再実装は 1 つの canonical fixture suite で拘束される。
  identity 所有者モジュールの置き場所の根拠 (§decision-1)、および §decision-7 で切り離した
  `clients/workspace/src/shared/session-ref.ts` の扱いの根拠。
- `adr-20260727-host-neutral-fleet-provider` — 「Only transport differs」。FR-B2 の根拠。
- `adr-20260727-fleet-provider-boundary` — Electron main が認証付き fan-out を所有し、preload は
  credential-free な typed projection を露出する。IPC envelope 型がこの境界に属することを固定する。
- `adr-20260727-web-host-transport-continuity` — web host は `serverId` prefix を strip して gateway へ proxy する。
- `adr-20260624-0021-frontend-wire-types-hand-written` — TS wire 型は手書きで Go 生成 fixture の round-trip で拘束される。
  → `SessionInfo` に client 専用の required field を足せない (§decision-1 を決定づける)。
- `adr-20260705-view-update-sessions-only` — active session 選択は per-client local state で wire contract の外。
- `adr-20260624-0022-subscribe-retry-in-socket-layer` / `adr-20260624-0030-terminal-keyed-remount` —
  subscribe frame の発行は socket 層が所有し、component は store key を渡すだけ。
  → FR-A2 は新方針ではなく既存 ADR の未完了部分の完了である。
- `adr-20260624-0025-transcript-rest-backfill-then-ws-tail` — 304 support を定めた ADR。FR-B1 の必然性の根拠。
- `adr-20260727-exact-match-client-server-protocol` — 互換 fallback・dual decoder・degradation を禁止。
  → §decision-2 で「bare id を素通しする互換分岐」の温存を禁じる方向に働く。
  (**訂正**: draft-2 は §decision-2 の根拠文で `adr-20260727-exact-match-client-server-platform` と綴っていた。
  `docs/adr/` に `-platform` は存在せず、正は `-protocol`。内容 — compat fallback / dual decoder / degradation の
  禁止 — は `-protocol` の記述と整合するため、id の 1 語訂正のみ。)
- `adr-20260624-0020-biome-as-frontend-lint` — Biome が唯一の linter。本文 `:61-62` は
  「If a rule we need is missing, falling back to eslint later is a package.json swap, not a code change」と
  **反転コストの低さ**を明示している。→ §decision-4 の lint 却下根拠は「supersede 必須」ではない。
- `adr-20260705-test-tier-taxonomy` — T0-T3 と external-dependency triple。

**参照したが適用しない**

- `adr-20260724-hosted-mode-preload-contextbridge`, `adr-20260724-workspace-host-electron`,
  `adr-20260624-0019-react-ts-vite-zustand-stack`, `adr-20260727-independent-web-host-module`,
  `adr-20260724-protocol-message-schema-sot-rest-binding`, `adr-20260714-workspace-session-switch-lifecycle`
- branded/nominal typing に関する ADR は repo に存在しない (`grep -ril "branded type|nominal typ" docs/adr/` → 0 件)。
  branded types の導入は未立法の決定であり、採るなら新規 ADR が要る (§decision-4)。
- `adr-20260624-0008-...` / `adr-20260624-0023-...` は superseded。引用しない。

統治 design: `docs/design/design-clients-ui.md` (`RESP-002` wire codec 所有、`INV-006` daemon store が
session 選択状態の唯一の所有者)、`docs/design/design-workspace-desktop.md` (`INV-001` workspace は wire decoding を
再実装しない)、`docs/design/design-host.md` (`#legacy-source-component-20260624-client-ipc` →
`### IPC Type Design Invariants`: 「A zero value with distinct semantics belongs in a separate type」)。

## Components

<!-- anchor: component-session-identity -->
### component-session-identity — fleet identity の所有者モジュール

**責務**: 2 つの id 空間の定義、encode/decode、`SessionRoute` への解決、解決失敗の typed outcome、
store 行型 `FleetSessionRow` の定義。**`clients/ui` で fleet key を構文的に扱ってよい唯一の場所**。

- kind: planned (`clients/ui/src/fleet/session-identity.ts` を `session-protocol.ts` から切り出す)
- paths: `clients/ui/src/fleet/session-identity.ts` (new), `clients/ui/src/fleet/session-protocol.ts` (carve-out 元)
- integration points: `socket/connection.ts`, `api/*.ts`, `store/*.ts`, `App.tsx`
- test seams: `clients/ui/src/fleet/session-identity.test.ts` (T0 pure), 既存 `session-protocol.test.ts`
- rationale: `adr-20260803` は `session-protocol.ts` を fleet wire schema の owner と名指すが、当該 file は
  既に 432 行で health schema と decoder を持つ。identity 変換を同居させると 500 行規約に触れ、責務も 2 つになる。
  同一 directory 内の carve-out なら ADR の所有 (fleet module) を維持したまま責務を分けられる。
  **同居 vs 分離は §decision-1 で「分離」に確定した** (open ではない)。
- 所有者集合の範囲の明示 — この「唯一の場所」は `clients/ui/src/**` に限る。
  `clients/workspace/src/shared/session-ref.ts` の第 2 実装は本 change の scope 外 (§decision-7) であり、
  この限界は gate の doc comment に明記する (AC-09)。

**内部の decision rule**

`resolveSessionRoute(input)` の入力は 2 形。

(a) 行を手にしている呼び出し側は `FleetSessionRow` を渡す → `{serverId, upstreamSessionId}` を**そのまま**返す
(evidence mode: total。文字列を parse しない。これが「権威があるのに使わない」6 箇所の再導出を消す本体)。

(b) id 文字列しか持たない呼び出し側は string を渡す → 先頭文字が `[` のときだけ decode し
`{serverId, upstreamSessionId}` を返す。decode 不能なら `{ok:false, reason:"unresolved", id}` を返す
(evidence mode: fallible)。

`unresolved` は 1 種類しかない。「bare id を素通しする」も「invalid fleet key として throw する」も持たない
— 前者は P5 到達不能 (§decision-2)、後者は失敗の名前が原因ではなく形式を指しており呼び出し側が分岐を再発明する。
戻り型は **判別 union であり throw ではない** (§decision-2 で確定)。

<!-- anchor: component-wire-connection -->
### component-wire-connection — WS wire 境界の所有者

**責務**: outbound frame の**組み立てと直列化**と id 変換、inbound frame の id 再修飾、data plane の失敗契約。
公開 API は fleet key だけを話す。

- kind: existing
- paths: `clients/ui/src/socket/connection.ts`, `clients/ui/src/socket/retry.ts`,
  `clients/ui/src/socket/terminalSubscription.ts`, `clients/ui/src/wire/client.ts`
- integration points: `components/TerminalPane.tsx`, `store/daemon.ts`, `store/transcripts.ts`,
  `store/workspaceActivity.ts`, `store/notifications.ts`, gateway `/ws`
- test seams: `connection.fleet-input.test.ts` (既存 RED), `connection.test.ts`,
  `connection.recovery-regression.test.ts`, `retry.test.ts`, `e2e/real-stack/terminal-input.spec.ts` (既存 RED)
- rationale: 既に 5 交差のうち 4 つ (`upstreamSessionID` による s/u/ld outbound と `:305-309` の inbound rewrite) を
  所有しており、統合方向はここしかない。TerminalPane 側へ置くと per-site 変換が 1 つ増えるだけで、
  bare id 前提の既存 TerminalPane test 群 (`newline-modifier` `:174` / `pc-baseline` `:208`) も総書き換えになる。
- **実在する bypass の明示 (draft-1 graft / critique blocker)**: `retry.ts:1` が `serializeClientFrame` を import し、
  `retry.ts:31` で `{k:"s",...}` を Connection の外で組み立て、`connection.ts:192` の
  `send: (s) => this.ws?.send(s)` を通して直接 socket へ流す。この経路が残る限り
  「Connection の外で frame を組み立てられない」は day-1 で偽である。§decision-4 で閉じる。

<!-- anchor: component-terminal-dataplane -->
### component-terminal-dataplane — 打鍵・リサイズの intent 発行者

**責務**: ユーザー入力の意図を Connection の intent API へ渡すだけ。wire frame を組み立てない。

- kind: existing
- paths: `clients/ui/src/components/TerminalPane.tsx`, `clients/ui/src/components/DriverShortcutBar.tsx`,
  `clients/ui/src/components/TerminalMobileOverlay.tsx`
- integration points: xterm `onData` / `attachCustomKeyEventHandler` / `onResize` の同期 emitter, `Connection`
- test seams: `TerminalPane.test.tsx`, `TerminalPane.newline-modifier.test.tsx`, `TerminalPane.pc-baseline.test.tsx`
- rationale: 現状 4 箇所 (`:172` `:182` `:193` `:276`) が `sessionRef.current` を verbatim で載せている。
  この 4 箇所は「変換を忘れた」のではなく「変換すべきだと知る手段がなかった」。intent API 化で知る必要がなくなる。

<!-- anchor: component-session-routing-http -->
### component-session-routing-http — HTTP 経路の route 解決利用者

**責務**: すべての **routing 値としての** session id を resolver 経由で得る。独自の id 再導出を持たない。

- kind: existing
- paths: `clients/ui/src/api/sessions.ts`, `clients/ui/src/api/transcripts.ts`,
  `clients/ui/src/api/workspace.ts`, `clients/ui/src/api/capsules.ts`
- integration points: `fleet/provider.ts` の `request`, gateway REST, `hooks/useTerminateSession.ts`, transcript polling
- test seams: `api/sessions.test.ts`, `api/transcripts.test.ts`, `api/capsules.test.ts`,
  `api/workspace.test.ts` (new)
- **責務の境界 (draft-1 graft)**: `api/workspace.ts:141-146` の `pinnedQuery` が
  `params.set("handle_session", pinned.sessionId)` に載せる値は `WorkspacePinnedHandle` 由来で
  **server が返した bare id であり routing 値ではない**。resolver を通してはならない
  (通すと gateway 側の workspace handle pin が解決できず save/diff が壊れる)。
  URL path の session 成分だけが resolver の対象である。
- rationale: 現状 4 実装が 3 方針 (P4 / P5 / P6) に分岐し、うち P5 の非 fleet 経路は
  どの shipping host でも 404 になる。互換性のために温存されている分岐は、実際には互換性を保存していない。

<!-- anchor: component-session-store -->
### component-session-store — fleet 行の保持者

**責務**: `FleetSessionRow` を fleet key で索引し、同一性判定を厳密一致で行う。

- kind: existing
- paths: `clients/ui/src/store/daemon.ts`, `clients/ui/src/store/statusSignals.ts`,
  `clients/ui/src/store/workspaceActivity.ts`, `clients/ui/src/wire/server.ts`
- integration points: `App.tsx`, `Connection`, `components/SessionList.tsx`, workspace drawer
- test seams: `store/daemon.test.ts`, `store/statusSignals.test.ts`, `store/__tests__/workspaceActivity.test.ts`,
  `wire/codec.test.ts` (Go 生成 fixture round-trip)
- rationale: `statusSignals.signalIdentityKey` (`:8-17`) と `connection.qualifyHelloSessions` (`:405-415`) は
  同じ解決式の逐語重複で、525b3091 が別モジュールへ独立に再実装したもの。
  `workspaceActivity.sessionIdAliases` (`:10-21`) は解決を諦めて全 alias を総当たりする方針で、
  「inbound の再修飾が不完全」という上流の欠陥を下流で吸収している。FR-A4 が満たされれば不要になる。

<!-- anchor: component-fleet-membership-producer -->
### component-fleet-membership-producer — fleet key を鋳造する唯一の場所

**責務**: `FleetSnapshot` から `FleetSessionRow` を作る唯一の mapping。fleet key と権威を同時に確定する。

- kind: existing
- paths: `clients/ui/src/App.tsx`
- integration points: `fleet/provider.ts` の `subscribe`, `store/daemon.ts` の `applyViewUpdate`,
  `Connection` の生成 (`:86-94`)
- rationale: `:134-141` が唯一の producer で、既に `id: sessionKey(ref)` と `upstreamSessionId: ref.sessionId` を
  両方書いている。権威は既に存在し populate 済みで、欠けているのは「他が使わない」ことの強制だけ。
- `App.tsx:80-85` の `?? "local"` については §decision-6 で実測と disposition を記録する。

<!-- anchor: component-ipc-response-envelope -->
### component-ipc-response-envelope — null-body-status 不変条件の所有者

**責務**: HTTP レスポンスを IPC 越しに運ぶ envelope の型と builder。「本文なし」を `null` で表す。

- kind: planned (`clients/workspace/src/shared/http-envelope.ts` を新設し、`electron-entry.ts` から使う)
- paths: `clients/workspace/src/shared/http-envelope.ts` (new), `clients/workspace/src/main/electron-entry.ts`
- integration points: `ipcMain.handle("agent-grid:session-fleet-request")` (`:429-460`), `fleetFetch`, preload の型宣言
- test seams: `clients/workspace/tests/http-envelope.test.ts` (new, T1/T2)。
  既存 `tests/control-envelope.test.ts` が同型の先例
- rationale: `electron-entry.ts:454-458` は無条件に `body: await response.text()` を作る。
  ipcMain handler は import 時に登録されるため直接 test できない。envelope 生成を pure function として
  `src/shared/` に切り出すことが、testability を設計制約として扱う repo 規約 (AGENTS.md) の要請。
  `src/shared/` は `control-envelope.ts` が既に居る確立済みの置き場所であり、preload (`src/preload/`) から
  import できる唯一の共有位置でもある (`src/main/` に置くと main → preload の依存方向が逆転する)。

**内部の decision rule**

`toResponseEnvelope(response)` は status が null-body status 集合に属するとき `body: null` を、
それ以外は `body: await response.text()` を返す。集合は我々の決定ではなく platform の決定なので、
**定数を持ちつつ platform を oracle として test で検証する**: 集合の各要素 s について
`new Response(null, {status: s})` が成功し `new Response("", {status: s})` が throw すること、
かつ集合外の代表 status (200/201/400/500) では逆であることを assert する。
platform が集合を変えたら test が落ちて定数の更新を強制する。これにより main 側と renderer 側は
定数を共有せずに同じ不変条件を見ていることが機械的に担保される (cross-package の型共有は不可能)。
先例として既存 RED `provider.null-body-status.test.ts:65-69` が 204 について同じ oracle を既に実測している。

<!-- anchor: component-fleet-provider-renderer -->
### component-fleet-provider-renderer — host transport parity の所有者

**責務**: `FleetProvider.request` が host によらず観察上等価な `Response` を返すこと。
envelope の `body: null` を消費する **全** consumer が null を安全に扱うこと。

- kind: existing
- paths: `clients/ui/src/fleet/provider.ts`, `clients/ui/src/auth.ts` (renderer 側 envelope 型宣言),
  `clients/workspace/src/preload/index.ts` (preload 側 pure builder の型宣言),
  `clients/workspace/src/preload/electron-preload.cjs` (実行時 bridge)
- integration points: `api/*.ts` の全 routed request, Electron preload IPC, web host `/host/servers/{id}/api/...`
- test seams: `clients/ui/src/fleet/provider.null-body-status.test.ts` (既存 RED, node environment),
  `clients/ui/src/fleet/provider.test.ts`, `clients/ui/src/fleet/provider.status-parity.test.ts` (new),
  `clients/workspace/tests/electron-preload-bridge.test.ts` (new)
- **envelope の第 2 consumer (critique)**: `provider.ts:644` の `new Response(result.body, {status})` だけでなく、
  `provider.ts:536-538` の repository-preparations tracker が `JSON.parse(result.body)` で同じ body を消費する。
  status guard は `result.status < 200 || result.status >= 300` なので **204 は guard を通過する**。
  `JSON.parse(null)` は throw せず `null` を返すため、`body: null` 導入後は catch に入らず
  `tracked.preparation` が無音で null に潰れる。**失敗が無音の成功へ退化する**経路であり、
  contract の変更 surface に含める。
- **型で語れない実行時 seam (critique)**: 型 import が届くのは pure builder 側の
  `clients/workspace/src/preload/index.ts:64-68` だけで、実行時 bridge は `electron-entry.ts:56` が読み込む
  `src/preload/electron-preload.cjs:72-73` の untyped passthrough である
  (`clients/workspace/tsconfig.json` は `allowJs` / `checkJs` を持たず `.cjs` は型検査対象外)。
  現状 `request: (serverId, pathname, init) => ipcRenderer.invoke(...)` は envelope を無加工で返すが、
  それを型で保証する手段が無い。runtime test へ移す (`preload-bridge-passes-null-body-verbatim`)。

<!-- anchor: component-identity-enforcement -->
### component-identity-enforcement — 交差点追加の検出器

**責務**: identity 変換または wire frame 直列化の所有者集合が広がったときに機械的に失敗する gate を持ち、
その gate 自身が合成違反で発火することを示す。

- kind: planned
- paths: `clients/ui/src/__meta__/session-identity-ownership.test.ts` (new),
  `docs/note/note-20260624-technical-code-enforcement.md` (catalogue 追記)
- integration points: `npm run test:unit` / `npm run test:web`, `tsc --noEmit`
- test seams: 自身が seam。先例は `src/__meta__/no-japanese.test.ts` (per-file allowlist + 締め方針),
  `src/__tests__/no-color-literals.test.ts`
- 走査対象と allowlist:
  | 走査する symbol | production importer の allowlist |
  |---|---|
  | `sessionKey` / `parseSessionKey` | `src/fleet/session-identity.ts`, `src/socket/connection.ts` |
  | `serializeClientFrame` (`src/wire/codec.ts` から) | `src/socket/connection.ts` |
  | `selectCodec` / `generatedCodecStub` (`src/wire/adapter.ts` から) | `src/wire/codec.ts` |
  走査範囲は `src/**/*.ts(x)` から `*.test.ts(x)` / `__meta__` / `__tests__` を除いた production source。
- rationale: §decision-4 参照。fitness test は補助であり単独を検出器と呼ばない。

## Implementation contracts

<!-- anchor: contract-wire-id-egress -->
### contract-wire-id-egress (integration_contract / owner: component-wire-connection)

**subject**: SPA が送出する全 WS frame の sessionId は upstream (bare) id であり、
`ClientFrame` の組み立てと直列化は `Connection` の内部に閉じる。

**reason**: 5 交差のうち 4 つを既に Connection が所有しているのに、5 つ目 (data plane) だけが
組み立て権威を component 側に残していたため無音で漏れた。変換を足すだけでは同型の交差点が同じ形で作れる。

**requirements**: FR-A1, FR-A2, NFR-PERF-002
**decision evidence mode**: fallible (route が解決できないことがある)

**decision rules** (id / evidence mode / input / epistemic state / observable)

| id | rule | evidence | epistemic | observable |
|---|---|---|---|---|
| `rule-egress-passthrough-unbound` | `cfg.serverId` が未設定なら値を変換せずそのまま載せる | total | determinate | `observable-egress-frame-session-id` |
| `rule-egress-fleet-key-match` | fleet key で `serverId === cfg.serverId` なら `upstreamSessionId` を載せる | total | determinate | `observable-egress-frame-session-id` |
| `rule-egress-bare-id` | `[A-Za-z0-9_-]+` の bare id はそのまま載せる (冪等) | total | determinate | `observable-egress-frame-session-id` |
| `rule-egress-server-mismatch` | fleet key で `serverId !== cfg.serverId` なら送出しない | fallible | unknown (`server-mismatch`) | `observable-dataplane-drop-counter` |
| `rule-egress-decode-failed` | fleet key でも bare id でもない値は送出しない | fallible | unknown (`decode-failed`) | `observable-dataplane-drop-notification` |

**normal case**: `cfg.serverId = "srv-a"` の Connection に対し `sendInput('["srv-a","s1"]', "ls\r")` を呼ぶと、
socket には `{"k":"i","d":"ls\r","sessionId":"s1"}` が出る。`sendResize` も同様に `"s1"` を載せる。

**adversarial case**
1. bare id `"s1"` を渡しても `"s1"` のまま出る (冪等・後方互換。session id は `services/server/api` の
   `[A-Za-z0-9_-]` 制限により JSON 配列文字列になり得ないので偶発一致しない)。
2. `cfg.serverId = "srv-a"` の Connection に `'["srv-b","s9"]'` を渡すと **frame は出ず、例外も出ない**。
3. 呼び出し側が `conn.send({k:"i",...})` を書こうとすると compile error になる (`send` は private)。
4. **`subscribeWithRetry` 経路も Connection の private 直列化を通る。** `retry.ts` が
   `serializeClientFrame` を import すると `contract-identity-ownership-gate` の
   `serialize-client-frame-sole-importer` が落ちる。

**risk tags**: boundary, malformed, unsupported_environment, repeated_usage
**expected observable**: `observable-egress-frame-session-id` — socket に渡る serialized frame の
`sessionId` field (scope: local)。必要 input: `input-connection-server-binding`。

**operational inputs**
- `input-connection-server-binding` (kind: authority, data_scope: 不要) — owner: component-wire-connection,
  producer: component-fleet-membership-producer (`App.tsx:80-94` の `serverId` memo + Connection 生成),
  source: `ConnectionConfig.serverId`, acquired_at: Connection 構築時, required_until: `close()`,
  preservation: capability_bound, mutable_between_acquire_and_use: false
  (serverId が変わると `useMemo` が新しい Connection を作る), stability_basis: `App.tsx:86-94` の
  `useMemo([fleet, serverId, token])`, invalidation_behavior: 旧 Connection は `close()` され pending は drain される,
  unavailable_outcome: `cfg.serverId` 未設定時は変換を行わず値をそのまま通す (hosted 単一サーバの後方互換)。

**semantic profiles**
- outcome_partition — outcomes: determinate / unknown。
  coverage basis: `cfg.serverId` 未設定の guard を先に置いたうえで、残る入力は
  「2 要素 JSON 配列として parse できる」か否かで二分され、前者はさらに `serverId` の一致で二分、
  後者は `[A-Za-z0-9_-]+` に適合するか否かで二分される。exclusivity basis: gateway の session id 文字集合が
  `[A-Za-z0-9_-]` に制限されているため、有効な bare id は JSON 配列リテラルになり得ず 4 分岐は排他。
  precedence: 未設定 guard > fleet key 判定 > bare id 判定。
  default policy: **fail-closed** — 解決できない id は wire に載せない。

**witness**
- normal (risk: boundary) — FR-A1/FR-A2。precondition: `cfg.serverId="srv-a"`。stimulus: `sendInput('["srv-a","s1"]',"ls\r")`。
  期待観察: `observable-egress-frame-session-id === "s1"`。禁止結果: `'["srv-a","s1"]'` が wire に出る。
  verification: `connection-fleet-input-outbound`。
- adversarial (risk: repeated_usage / boundary) — FR-A2。precondition: `retry.ts` が直列化を持たない。
  stimulus: `retry.ts` に `serializeClientFrame` import を 1 行戻す。期待観察: gate が落ちる。
  禁止結果: `npm run test:unit` が緑のまま。verification: `retry-path-uses-private-serialization` +
  `serialize-client-frame-sole-importer`。

**implementation discretion**: `discretion-upstream-parse-fast-reject` (早期棄却の書き方)。

<!-- anchor: contract-dataplane-unresolved-drop -->
### contract-dataplane-unresolved-drop (failure_recovery / owner: component-wire-connection)

**subject**: data plane の id 解決失敗は例外にせず frame を破棄し、reason 別に分類された観測可能な記録を残す。
**内部契約違反 (`decode-failed`) だけがユーザーに到達し、正常遷移由来 (`server-mismatch`) は counter にのみ残る。**

**reason**: `send()` は xterm の同期 emitter 内から呼ばれるため throw が xterm 内部へ伝播する。
一方で無音 drop は gateway 側の欠陥 (症状を出さないので 2 世代検出されなかった) と同型であり、
クライアント側で再現してはならない。

ただし draft-2 の既定 surface (`useNotificationsStore` の `level:"error"` を reason 問わず 1 件) は
**正常動作を失敗として誤報する**。発火条件 `cfg.serverId !== key.serverId` は、`App.tsx:80-94` の `useMemo` が
serverId 変更で Connection を再生成する **server 切替の窓**で成立し得る良性の遷移である。
記録の必要性 (無音にしない) と通知の妥当性を分離する。

**requirements**: FR-A3
**decision evidence mode**: fallible

**記録要件 (分割後)**
- (a1) **内部契約違反はユーザーに到達可能**: `decode-failed` (fleet key でも bare id でもない値が
  data plane へ来た) は `useNotificationsStore.add({level:"error"})` を出す。
- (a2) **正常遷移由来の drop は診断可能**: `server-mismatch` は通知を出さず、
  Connection が公開する reason 別 drop counter にのみ記録する。
- (b) 打鍵ごとに繰り返さない: 通知は `(sessionId, reason)` ごとに 1 Connection lifecycle 1 件。
  counter は毎回加算する (件数そのものが診断値)。
- (c) test から決定論的に観測できる。

**normal case**: 解決できる key ではいかなる記録も出ない (通常打鍵で notification も counter も増えない)。

**adversarial case**
1. `cfg.serverId="srv-a"` の Connection に `'["srv-b","s9"]'` で `{k:"i"}` を 50 回連続で送っても、
   (a) 例外は 1 度も送出されず、(b) socket へ frame は 1 件も出ず、
   (c) `server-mismatch` counter は 50 に達し、(d) **notification は 0 件**。
2. 同じ Connection に `"not a session id!"` (bare id 文字集合外) を渡すと `decode-failed` counter が増え、
   notification が **1 件だけ**増える。同じ (session, reason) を 50 回繰り返しても notification は 1 件のまま。
3. `{k:"r"}` (resize) の drop は **reason を key に畳む** (frame kind では分けない)。
   同じ session の同じ reason なら追加の notification を出さない。
4. Connection を `close()` して新しい lifecycle を始めると、再び 1 件出せる (counter は 0 から)。

**risk tags**: repeated_usage, boundary, recovery, unknown
**expected observable**
- `observable-dataplane-drop-counter` — Connection が公開する reason 別 drop counter (scope: local)。
  必要 input: `input-connection-server-binding`。
- `observable-dataplane-drop-notification` — `useNotificationsStore` の entry (scope: global)。
  必要 input: `input-connection-server-binding`。

**operational inputs**: なし。
**input_closure_reason**: 判定に必要な情報 (frame の key、`cfg.serverId`) はすべて
`contract-wire-id-egress` の `input-connection-server-binding` と呼び出し引数で閉じており、
記録の dedup 状態は Connection instance 内の private state。外部の権威も versioned state も要らない。

**semantic profiles**
- outcome_partition — outcomes: determinate (送出) / unknown (破棄+記録)。
  unknown は `server-mismatch` と `decode-failed` の 2 failure に細分される。
  coverage / exclusivity basis は `contract-wire-id-egress` の partition と同一の 4 分岐に依拠する。
  precedence: fleet key 判定 → serverId 一致判定 → bare id 判定。
  default policy: fail-closed (送出しない)。解決できない id を wire に載せることは gateway の無音 drop を誘発する。

**typed failure semantics**
| id | 分類 | 記録 | Requirement degradation |
|---|---|---|---|
| `failure-egress-server-mismatch` | 正常遷移由来 (外部要因) | counter のみ | なし (frame は本来到達すべきでない) |
| `failure-egress-decode-failed` | 内部契約違反 | counter + notification 1 件 | なし (FR-A1 は違反入力に対して送らないことで満たされる) |

**witness**
- normal (risk: repeated_usage) — FR-A3。precondition: `cfg.serverId="srv-a"`。stimulus: 解決できる key で 50 回打鍵。
  期待観察: counter 0 / notification 0。禁止結果: 打鍵中に toast が出る。verification: `dataplane-drop-counter-by-reason`。
- adversarial (risk: unknown / recovery) — FR-A3。precondition: 同上。stimulus: `'["srv-b","s9"]'` で 50 回。
  期待観察: 例外 0 / frame 0 / `server-mismatch` counter 50 / notification 0。
  禁止結果: notification が 1 件でも増える (= server 切替の窓で誤報が出る)。
  verification: `dataplane-no-throw-on-serverid-mismatch`, `dataplane-drop-counter-by-reason`。
- adversarial (risk: boundary) — FR-A3。stimulus: 文字集合外の値で 50 回。
  期待観察: `decode-failed` counter 50 / notification 1。禁止結果: 無音 (counter も notification も 0)。
  verification: `dataplane-decode-failed-notifies-once`。

**implementation discretion**: `discretion-drop-record-dedup-structure` (dedup 状態の保持構造)。

**counter の公開形は決定済み**: 専用 diagnostics store を新設せず、Connection の read-only getter として公開する。
新しい共有 store はどの requirement にも trace しないため (simplicity)、Connection lifecycle に束ねる方が
dedup の寿命 (adversarial 4) と同じ担体に載る。getter の内部データ構造は discretion。

<!-- anchor: contract-wire-id-ingress -->
### contract-wire-id-ingress (integration_contract / owner: component-wire-connection)

**subject**: gateway から到着する全ての session id 保持要素が、store へ渡る前に**単一の再修飾経路**を通って
fleet key へ再修飾される。

**reason**: 現状 output frame (`:305-309`) と `tt`/`et` (`fleetSessionID`) だけが再修飾され、
`activity_session_id` (`:360`) と activity event の `session_id` は素通しする。
その非対称性が下流の alias 総当たり (P7) を生んでいる。

**requirements**: FR-A4, NFR-PERF-001
**decision evidence mode**: total (`cfg.serverId` があれば常に修飾でき、無ければ常に素通し)

**normal case**: `cfg.serverId="srv-a"` で output `[t,"o",b64,"s1"]` が届くと `frame[3]` は `'["srv-a","s1"]'` に、
`tt`/`et` の `sessionId` も同様に、`activity_session_id: "s1"` は `'["srv-a","s1"]'` になって store へ渡る。
hello / view の `sessions[]` も同じ経路で `FleetSessionRow` として渡る。

**adversarial case**
1. `activity_events[]` が 100 件あり各 event の `session_id` が bare のとき、store へ渡る全 event が
   fleet key を持つ (1 件も素通ししない)。
2. `cfg.serverId` が未設定の Connection では全要素が bare のまま渡る (現行動作の保存)。
3. 同一 session の output frame が 10,000 件連続で届いても、再修飾の割り当ては session 数に比例し
   frame 数に比例しない (memo 化)。
4. `publishSessions !== false` 分岐 (`:326` / `:342` の `tagSessions`) も同じ再修飾経路へ合流する。
   分岐は削除しないが、`initialServerHint` による独立 tagging は残さない
   (残すと再修飾経路が 2 本になり FR-A4 の「単一経路」が偽になる)。
5. `cfg.serverId` が fleet snapshot の member でない sentinel (`"local"`) の場合、再修飾された id は
   どの store 行とも一致しない。この状態が production で到達不能であることは §decision-6 で実測し、
   反証条件を名指している。本 contract は `"local"` に対する特別扱いを**持たない**
   (magic string の分岐を socket 層に作らない)。

**risk tags**: partial_data, scale, stale, boundary
**expected observable**: `observable-ingress-store-identity` — store (`daemon` / `transcripts` / `workspaceActivity`)
に入った行と event の session 識別子 (scope: global)。必要 input: `input-connection-server-binding`。

**operational inputs**
- `input-connection-server-binding` を `contract-wire-id-egress` と共有する。
  **この共有こそが「wire 境界の所有者が 1 人である」ことの実体**であり、egress と ingress が別の権威を
  参照し始めた時点で境界が割れる。

**semantic profiles**
- scope_consistency — 入力は per-frame の stream / partial (view-update は sessions を省く activity-only 形がある)
  なのに、観測は store という global observable。部分更新が global の id 空間を割らないことを示す証拠として
  adversarial (1) と (4) を要求する。
- cost_convergence — FR-β07 が kHz の output を想定しており、frame 数に比例する割り当てを増やさない。
  収束基準: 10,000 frame の再修飾で `sessionKey` 呼び出し回数が session 数に比例すること (adversarial 3)。

**witness**
- normal (risk: boundary) — FR-A4。stimulus: `activity_session_id: "s1"` を含む view-update。
  期待観察: store が `'["srv-a","s1"]'` を受ける。禁止結果: bare のまま渡る。verification: `activity-session-id-requalified`。
- adversarial (risk: scale) — NFR-PERF-001。stimulus: 同一 session の output 10,000 件。
  期待観察: 再修飾コストが session 数に比例。禁止結果: frame 数に比例。verification: `requalification-memoized-per-session`。

**implementation discretion**: `discretion-requalification-memo` (memo 構造)。

<!-- anchor: contract-egress-activation-compatibility -->
### contract-egress-activation-compatibility (migration_compatibility / owner: component-wire-connection)

**subject**: これまで gateway に無音 drop されていた data plane frame が初めて到達することを
**挙動活性化**として固定し、退行と区別できるようにする。

**reason**: 本 change は「壊れていたものを直す」だけでなく、**これまで無音で捨てられていた frame が
初めて gateway へ到達する**ことを意味する。draft-2 はこれを Assumptions の 2 行に留めており、
実寸 resize 到達と Delete シーケンス到達を「退行」と誤認するか「修正の効果」と誤認するかを判別する
反証点が無かった。debug run が fix 前実測値を保存しているため即座に witness 化できる。

**requirements**: FR-A8
**decision evidence mode**: total

**normal case**: fix 後、SPA が送っていた `{k:"r", cols:107, rows:32}` (debug run の実測値) が初めて PTY に届き、
セッションの PTY サイズが SPA の実寸へ追随する。

**adversarial case**
1. debug run の実測フレームに含まれていた制御シーケンス `{k:"i","d":"\u001b[3~"}` (Delete) が
   **verbatim で** wire に載る (intent API 化で escape / 再 encode が入らない)。
2. fix 前に作られ 80x24 想定で動いている既存セッションが、再接続時の resize で scrollback を壊さない。
3. 同じ活性化が browser host でも起きる (この欠陥は Electron 固有ではない)。

**risk tags**: boundary, stale, unsupported_environment, repeated_usage
**expected observable**: `observable-activation-resize-frame` — wire に出る `{k:"r"}` の `cols`/`rows` が
SPA の実測値と一致し、`sessionId` が bare であること (scope: local)。
`observable-activation-control-sequence` — wire に出る `{k:"i"}` の `d` が入力バイト列と一致すること (scope: local)。
必要 input: `input-connection-server-binding`。

**operational inputs**: なし。
**input_closure_reason**: 期待観察は fix 前後の同一シナリオ比較で成立し、外部の情報・権限・versioned state を
追加で必要としない。debug run が fix 前の実測値 (cols 107 / rows 32、`\u001b[3~`) を既に保存している。

**semantic profiles**
- contract_evolution —
  変更 surface: これまで到達しなかった `{k:"r"}` と制御シーケンス入りの `{k:"i"}` が到達し始める。
  typed consumer: gateway の `tryResize` / `sess.WriteRaw` と、その先の PTY。
  互換性: wire 形状は変えない (non_goal)。変わるのは到達性のみ。
  移行 / rollout: 1 change 内で `{k:"i"}` と `{k:"r"}` を同時に通す (段階化しない — 段階化は 2 度の release を要し、
  中間状態で「入力は届くがサイズは古い」という新しい観察を作るため)。
  rollback: intent API を経由しない旧経路へ戻すことはしない。活性化が問題を起こした場合は
  gateway 側の resize 受理を別 change で扱う (本 change の scope 外)。
  baseline verification: debug run が保存した fix 前実測値 (frame が gateway に到達せず PTY が変わらない)。
  target verification: `resize-frame-reaches-wire` / `control-sequence-passthrough-verbatim` (T1) と
  real-stack e2e (T3、verification member に記載)。

**witness**
- normal (risk: boundary) — FR-A8。precondition: xterm が cols 107 / rows 32 に fit 済み。
  stimulus: `onResize` 発火。期待観察: `{k:"r", cols:107, rows:32, sessionId:"s1"}` が wire に出る。
  禁止結果: frame が出ない、または `sessionId` が compound。verification: `resize-frame-reaches-wire`。
- adversarial (risk: stale) — FR-A8。stimulus: Delete キー (`\u001b[3~`)。
  期待観察: `d` が `"\u001b[3~"` と byte 単位で一致。禁止結果: escape 済み文字列や欠落。
  verification: `control-sequence-passthrough-verbatim`。

<!-- anchor: contract-session-identity-types -->
### contract-session-identity-types (data_model / owner: component-session-identity)

**subject**: wire mirror 型 (`SessionInfo`) と store 行型 (`FleetSessionRow`) を分離し、
後者は `(id: fleet key, serverId, upstreamSessionId)` を required で持つ。

**reason**: 1 つの型が gateway payload と SPA 行という 2 つの権威を名乗っていることが
`upstreamSessionId?` の optional 性の正体であり、「required 化するか未設定時方針を決めるか」という
二択そのものを生んでいた。

**requirements**: FR-A7, FR-A5
**decision evidence mode**: total

**normal case**: `App.tsx` の fleet snapshot mapping が `FleetSessionRow[]` を作り、store がそれを保持する。
`SessionInfo` を読むだけの consumer (SessionList, palette, tools など) は無変更で通る (subtype 関係)。

**adversarial case**
1. Go 生成 fixture `wire/testdata/hello.json` (bare `"id":"s1"`、`upstreamSessionId` なし) が引き続き
   `parseServerFrame` で decode でき、`codec.test.ts:6` の round-trip が通る。
2. `upstreamSessionId` を書かずに store 行を作ろうとすると compile error になる。
3. `serverId` を書かずに fleet key だけを持つ行も作れない。
4. `SessionInfo` から `serverId?` / `serverLabel?` / `upstreamSessionId?` が**消えている**
   (2 つの型が同時に権威を名乗る中間状態を残さない)。

**risk tags**: boundary, malformed, unsupported_environment
**expected observable**: `observable-store-row-authority` — `tsc --noEmit` の結果と、store に入った行が必ず
権威を持つこと (scope: global)。必要 input: なし (静的宣言)。

**operational inputs**: なし。
**input_closure_reason**: 型分離は静的な宣言であり、実行時に外部の情報・権限・versioned state を必要としない。
権威の供給 (`ref.serverId` / `ref.sessionId`) は `FleetSnapshot` に同梱されており、その continuity は
`contract-http-route-resolution` の `input-fleet-session-row` が扱う。

**semantic profiles**: なし (fallible でも multi-source でもなく、global/partial の組合せも持たない)。

**witness**
- normal (risk: boundary) — FR-A7。stimulus: Go 生成 fixture の decode。期待観察: round-trip 一致。
  禁止結果: fixture が decode できない。verification: `wire-fixture-roundtrip-unchanged`。
- adversarial (risk: malformed) — FR-A7。stimulus: `upstreamSessionId` を欠いた行リテラル。
  期待観察: `tsc --noEmit` が落ちる。禁止結果: 通る。verification: `store-row-requires-authority`。

<!-- anchor: contract-http-route-resolution -->
### contract-http-route-resolution (integration_contract / owner: component-session-routing-http)

**subject**: **URL path の session 成分**は単一 resolver で `{serverId, upstreamSessionId}` を得る。
解決不能なら 1 種類の typed failure を返し、要求は発行せず、その失敗は network error へ写像されない。

**reason**: 4 実装が 3 方針に分岐し、うち 2 つ (P5/P6) は同じ入力に真逆の判断をしている。
P5 の非 fleet 経路はどの shipping host でも成功しない。

**requirements**: FR-A5, FR-A6
**decision evidence mode**: multi_source (行の権威と id 文字列の 2 系統から解決しうる)

**normal case**: 行を持つ呼び出し側は `resolveSessionRoute(row)` で文字列を parse せずに route を得る。
id しか持たない呼び出し側は `resolveSessionRoute('["srv-a","s1"]')` で同じ route を得る。
両者は同じ `serverId` と同じ `upstreamSessionId` を返す。

**adversarial case**
1. 行の `upstreamSessionId` と id 文字列の decode 結果が食い違う行を渡したとき、**行の権威が勝つ**。
   この優先順位が固定されていないと 525b3091 が作った「権威があるのに文字列から推測する」パターンが再発する。
2. bare id `"s1"` (fleet key でない) を渡すと `unresolved` が返り、**HTTP 要求は 1 件も発行されない**
   (現行 P5 は `/api/sessions/s1/transcript` を発行して 404 を受ける)。
   さらにその失敗は `Error("network")` として観察されない (下記 contract_evolution 参照)。
3. 削除済み session の行を保持したまま resolve すると route は返るが、要求は gateway 側で 404 になる
   — resolver は存在を保証しない。
4. **`pinnedQuery(pinned)` が `handle_session` を fleet key へ変換しない。** 変換すると gateway 側の
   workspace handle pin が解決できず save/diff が壊れる。`handle_session` は routing 値ではない。
5. `App.tsx` が合成する `"local"` は transport binding 専用の sentinel であり、resolver の入力にも
   store 行の `serverId` にもならない (§decision-6 の実測)。仮に `'["local","s1"]'` を resolver へ渡した場合は
   行が無いので decode 由来の determinate になり、要求は `/host/servers/local/...` へ出て host 側で解決に失敗する
   — **新たな成功経路は生まれない**。

**risk tags**: conflicting_evidence, stale, boundary, unknown, concurrency
**expected observable**: `observable-http-issued-request` — 発行された HTTP 要求の URL と server、および
呼び出し側が受け取る失敗の型 (scope: local)。必要 input: `input-fleet-session-row`。

**operational inputs**
- `input-fleet-session-row` (kind: versioned_state, data_scope: indexed) — owner: component-session-routing-http,
  producer: component-fleet-membership-producer (`App.tsx:134-141`),
  source: `FleetProvider.subscribe` が配る `FleetSnapshot` を store へ写した行,
  acquired_at: snapshot 受領時, required_until: HTTP 要求の発行時,
  preservation: snapshot (revision 単位の immutable full snapshot),
  mutable_between_acquire_and_use: **true** (session は解決と要求の間に消え得る — concurrency 面の反例),
  stability_basis: `FleetSnapshot` は monotonically revisioned な immutable full snapshot であり、
  行の `(serverId, upstreamSessionId)` は行の生存中は不変,
  invalidation_behavior: 新しい revision で行が消えたら store から消え、以後 `resolveSessionRoute(id)` は
  文字列 decode にフォールバックする (decode できれば route は返る),
  unavailable_outcome: 行も decode も不能なら `unresolved` を返し、要求を出さない。

**semantic profiles**
- outcome_partition — outcomes: determinate (行由来) / determinate (decode 由来) / unresolved / conflicting。
  coverage basis: 入力は「行」か「文字列」の 2 形しかなく、行は常に権威を持つ (型で required)、
  文字列は decode できるかできないかで二分される。exclusivity basis: 行由来と decode 由来が同時に
  成立する場合 (conflicting) は precedence で一意化する。
  precedence: **行の権威 > id 文字列の decode > unresolved**。
  default policy: unresolved は要求を発行しない (fail-closed)。
- contract_evolution —
  変更 surface: `api/transcripts.ts:26-40` (bare id 素通し) と `api/workspace.ts:191-195` (同) の撤去、
  および `api/capsules.ts:104` の throw を共通 typed failure へ置換。
  typed consumer: transcript polling (`hooks` 側の 304/204 分岐)、workspace viewer、capsule 表示。
  互換性: 旧挙動 (404 を受ける) と新挙動 (要求を出さず typed failure) の両方に test を持つ。
  移行 / rollout: 1 change 内の unit `http-route-resolver-adoption` で一括。
  rollback: resolver 呼び出しを旧分岐へ戻す (module 単位で可逆)。
  baseline verification: 既存 `api/transcripts.test.ts` / `api/capsules.test.ts` の 404 系。
  target verification: `unresolved-issues-no-request` / `four-api-modules-single-policy`。

**typed failure semantics**
| id | 分類 | 呼び出し側から見える形 | 発行される HTTP 要求 |
|---|---|---|---|
| `failure-route-unresolved` | 内部契約違反 (解決できない id が api layer に渡った) | 1 種類の typed failure (判別 union の `ok:false`) | 0 件 |

**resolver の失敗は `Error("network")` に化けてはならない (draft-1 graft)**:
`api/sessions.ts:289-309` の `doFetch` は AbortError 以外の throw をすべて `makeNetworkError(e)` へ写像する。
resolver が **throw** する設計を採ると、根本原因 B と同型の誤診断 (成功でない失敗が network error として表示される) が
A 側に新設される。したがって resolver は throw せず判別 union を返し、api モジュールは
**要求発行前** (`doFetch` の try に入る前) に `ok:false` を分岐する。
実測で裏づく: `routedSession` (`api/sessions.ts:469-474`) は `routedRequest` の先頭で同期に評価され、
`doFetch` の catch は `fetchImpl(...)` の周りにしか掛かっていない。

**witness**
- normal (risk: conflicting_evidence) — FR-A5。stimulus: 行の `upstreamSessionId` と id 文字列の decode が食い違う行。
  期待観察: 行の値が返る。禁止結果: decode 結果が返る。verification: `resolver-precedence-row-over-decode`。
- adversarial (risk: unknown) — FR-A6。stimulus: bare id `"s1"` を transcript API へ渡す。
  期待観察: fetch が 0 回呼ばれ、typed failure が返る。
  禁止結果: 要求が発行される、または `Error("network")` が観察される。verification: `unresolved-issues-no-request`。
- adversarial (risk: boundary) — FR-A5。stimulus: `pinnedQuery(pinned)`。
  期待観察: `handle_session` が `pinned.sessionId` のまま。禁止結果: fleet key へ変換される。
  verification: `pinned-handle-not-routed`。

<!-- anchor: contract-store-identity-equality -->
### contract-store-identity-equality (state_lifecycle / owner: component-session-store)

**subject**: session 同一性の判定は fleet key の厳密一致で行い、alias 総当たりを持たない。

**reason**: `workspaceActivity.sessionIdAliases` / `sessionIdsMatch` と `statusSignals.signalIdentityKey` は、
inbound 再修飾が不完全であることを下流で吸収する仕組み。上流 (FR-A4) が閉じれば不要になり、
残せば「どちらの id でも一致する」という曖昧さが次の交差点を再び隠す。

**requirements**: FR-A7, FR-A4
**decision evidence mode**: total (再修飾後は id 空間が 1 つしかない)

**normal case**: activity drawer の scope 照合、status signal の consume/seed map、
`daemon.applyGatewaySessionStatus` の突合が、いずれも fleet key の `===` で成立する。

**adversarial case**
1. `contract-wire-id-ingress` を適用せずに alias 総当たりだけを消すと、activity drawer の scope 照合が
   無音で壊れる (bare な `activity_session_id` が fleet key と一致しない)。
   **この順序制約を破る実装を test が検出する。**
2. 同じ upstream id を持つ別サーバの session (`["srv-a","s1"]` と `["srv-b","s1"]`) が同時に存在するとき、
   status signal の consume map が混ざらない (concurrency 面の反例)。
3. 再接続 hello の replay 抑止 (525b3091 の目的) が、alias 撤去後も同じ振る舞いを保つ (concurrency 面の反例)。

**risk tags**: stale, partial_data, boundary, concurrency
**expected observable**: `observable-store-equality-outcome` — activity drawer に表示される行、
status bell の再生有無 (scope: global)。必要 input: なし。

**operational inputs**: なし。
**input_closure_reason**: 判定は store 内の 2 つの id の比較のみで閉じる。権威の供給は
`contract-session-identity-types` と `contract-wire-id-ingress` が保証する前提条件であり、
本 contract 自身は追加の外部情報を必要としない。

**semantic profiles**
- contract_evolution —
  変更 surface: `store/workspaceActivity.ts:10-21,166,172,478,491-492` と `store/statusSignals.ts:8-17`。
  typed consumer: activity drawer と notification (bell)。
  互換性: 旧挙動 (alias 一致) と新挙動 (厳密一致) の両方に test を持つ。
  移行 / rollout: unit `store-row-authority-and-equality` は `connection-ingress-requalification` に
  `depends_on` する (順序制約の機械表現)。
  rollback: alias helper の復元 (module 単位で可逆)。
  baseline / target verification: `alias-removal-order-constraint` と `reconnect-bell-suppression-preserved`。

**`signalIdentityKey` の確定形**: `signalIdentityKey(row, epoch)` は `` `${row.id}\u0000${epoch}` `` **だけ**を返す。
`parseSessionKey` / `sessionKey` / `upstreamSessionId` / `serverId` を参照しない
(参照が残る限り再導出が残り、`contract-identity-ownership-gate` の allowlist にも違反する)。

**witness**
- normal (risk: stale) — FR-A7。stimulus: fleet key で索引された行の突合。期待観察: `===` で成立。
  禁止結果: alias 総当たりが必要。verification: `strict-equality-after-ingress`。
- adversarial (risk: boundary) — FR-A4。precondition: ingress 再修飾が未適用。stimulus: alias helper を削除。
  期待観察: activity drawer の scope 照合 test が落ちる。禁止結果: 緑のまま。
  verification: `alias-removal-order-constraint`。
- adversarial (risk: concurrency) — FR-A7。stimulus: 再接続 hello の replay。
  期待観察: bell が再生されない。禁止結果: 再生される。verification: `reconnect-bell-suppression-preserved`。

<!-- anchor: contract-ipc-null-body-envelope -->
### contract-ipc-null-body-envelope (integration_contract / owner: component-ipc-response-envelope)

**subject**: IPC response envelope の `body` は `string | null` で、null body status のときだけ null。

**reason**: `{status, headers, body: string}` が「本文なし」を表現できないため、main は 204/304 に対して `""` を作り、
renderer の `new Response("", {status:204})` が TypeError になる。`design-host.md` の IPC Type Design Invariants は
既に「別意味を持つ zero value は別の型へ」と述べており、本件はその未適用。

**requirements**: FR-B1
**decision evidence mode**: total (status から一意に決まる)

**normal case**: 200 の JSON 応答は `body: '{"ok":true}'` を持つ。
204 (`DELETE /api/sessions/{id}`、`POST .../messages/read`) は `body: null`。
304 (`GET /api/sessions/{id}/{kind}` の not-modified) は `body: null` かつ `headers.etag` を保持。

**adversarial case**
1. upstream が 204 に本文を付けて返してきても envelope は `body: null` を出す
   (status が権威で、本文の有無は権威でない)。
2. 200 で本文が空文字のときは `body: ""` であり `null` ではない (「空の本文」と「本文なし」を潰さない)。
3. null body status 集合の各要素で `new Response(null, {status})` が成功し `new Response("", {status})` が
   throw することを platform に問い合わせて確認する — platform が集合を変えたら test が落ちる。

**risk tags**: boundary, malformed, unsupported_environment
**expected observable**: `observable-envelope-body` — IPC を越えて renderer に届く envelope の `body` (scope: local)。
必要 input: `input-null-body-status-set`。

**operational inputs**
- `input-null-body-status-set` (kind: information, data_scope: full) — owner: component-ipc-response-envelope,
  producer: external (`WHATWG Fetch Standard` を実装する Node / Chromium の `Response`),
  source: 実行環境の `Response` constructor の受理/拒否挙動,
  acquired_at: test 実行時 (production では定数として持つ), required_until: envelope を組み立てるたび,
  preservation: immutable_value (集合は仕様で固定され、我々が決める値ではない),
  mutable_between_acquire_and_use: false,
  stability_basis: WHATWG Fetch の null body status の定義と、Node / Chromium がそれを強制する事実
  (既存 RED `provider.null-body-status.test.ts:65-69` が 204 について実測済み),
  invalidation_behavior: platform が集合を変えると oracle test が落ちて定数の更新を強制する,
  unavailable_outcome: oracle を評価できない環境 (happy-dom のような寛容な実装) では test を走らせない
  — file 単位で node environment を選ぶ。

**定数の外延の決め方 (open ではない)**: 定数の初期値は `{101, 204, 205, 304}`。
`103` を含めるか否かは我々の決定ではなく **oracle test の結果が決める** —
oracle は「集合の各要素で `new Response("", {status})` が throw し、集合外の代表 status では throw しない」を
assert するので、実行環境が 103 を null body status として扱うなら定数を足さない限り test が落ちる。
定数は oracle に従属する。

**semantic profiles**: なし (status → body 有無は total な写像で、outcome partition を要しない)。

**witness**
- normal (risk: boundary) — FR-B1。stimulus: 204 応答。期待観察: `body: null`。
  禁止結果: `body: ""`。verification: `envelope-null-body-status-mapping`。
- adversarial (risk: malformed) — FR-B1。stimulus: 200 + 空本文。期待観察: `body: ""`。
  禁止結果: `body: null`。verification: `empty-200-body-not-null`。
- adversarial (risk: unsupported_environment) — FR-B1。stimulus: platform が集合を変える。
  期待観察: oracle test が落ちる。禁止結果: 定数と platform が黙って乖離する。
  verification: `envelope-platform-oracle-guard`。

**implementation discretion**: `discretion-envelope-builder-shape` (型と builder を分けるか 1 関数にするか、
集合を `Set` で持つか配列定数で持つか)。

<!-- anchor: contract-host-transport-parity -->
### contract-host-transport-parity (migration_compatibility / owner: component-fleet-provider-renderer)

**subject**: `FleetProvider.request` は browser host と Workspace host で観察上等価な `Response` を返す。
`body: null` を消費する全 consumer が null を安全に扱う。型変更は consumer-first の順序で適用する。

**reason**: `adr-20260727-host-neutral-fleet-provider` は「Only transport differs」と定めるが、
現状 Workspace 経路だけが null body status で例外になる。これは transport の差ではなく挙動の fork。

**requirements**: FR-B2, FR-B3
**decision evidence mode**: total

**normal case**: 同じ status/headers/body の HTTP 交換に対し、両 provider が返す `Response` の
`status` / `ok` / `headers.get(k)` / `await text()` が一致する。
204 は `status=204, ok=true, text()=""`、304 は `status=304, ok=false, text()=""`。

**adversarial case**
1. status 行列 {200, 201, 204, 205, 304, 400, 404, 500} の全要素で両 provider が一致する。
2. renderer が新しく main が古い (envelope が `""` を返す) 組合せでも renderer は例外にならない
   — consumer-first 順序が守られていることの witness (AC-08)。
   この耐性は `provider.ts:644` 側の defensive 分岐が**恒久的に**存在することで担保される (§decision-5)。
3. fake bridge が real の envelope builder より寛容な値 (204 に `""`) を返した場合、fidelity test が落ちる
   (fake を real に合わせる。assertion を弱めない)。
4. `/api/repository-preparations/{id}` が 204 + `body: null` を返したとき、
   `provider.ts:536-538` は `JSON.parse` せず `tracked.preparation` を維持する。
   `JSON.parse(null)` は throw せず `null` を返すため、この分岐が無いと **失敗が無音の成功へ退化する**。
5. preload の実行時 bridge (`electron-preload.cjs:72-73`) が `body: null` を `?? ""` などで coerce せず
   verbatim で renderer へ渡す。

**risk tags**: unsupported_environment, boundary, stale, conflicting_evidence
**expected observable**: `observable-provider-response` — 両 provider が返す `Response` の観察可能属性の組
(scope: local)。`observable-preparation-tracker` — `tracked.preparation` の値 (scope: local)。
必要 input: `input-main-envelope-behavior`。

**operational inputs**
- `input-main-envelope-behavior` (kind: versioned_state, data_scope: partial) — owner: component-fleet-provider-renderer,
  producer: component-ipc-response-envelope, source: preload IPC 越しに届く envelope,
  acquired_at: 各 routed request の応答時, required_until: `new Response(...)` の構築時,
  preservation: version_bound (Electron app の build 単位で main と renderer dist が対になる),
  mutable_between_acquire_and_use: false (1 応答内では不変),
  stability_basis: `clients/workspace/package.json` の build が `clients/ui` の dist をコピーして同梱するため、
  出荷単位では main と renderer が対になる,
  invalidation_behavior: **dev 実行や部分再ビルドで対が崩れうる** — だから consumer-first 順序を要求し、
  renderer 側の分岐を恒久的に残す,
  unavailable_outcome: `fleet.request` が未提供の bridge では従来どおり
  `Error("Workspace routed request is unavailable")`。

**semantic profiles**
- contract_evolution —
  変更 surface: envelope 型の 3 宣言 (`electron-entry.ts:454-458` の推論戻り型 / `preload/index.ts:64-68` /
  `clients/ui/src/auth.ts:80-84`) + 実行時 bridge `electron-preload.cjs:72-73` +
  `provider.ts:644` の再物質化 + **`provider.ts:536-538` の repository-preparations 直接 body 消費**。
  typed consumer: `api/sessions.ts` (delete / markRead)、`api/transcripts.ts` (304/204 分岐)、
  **repository-preparations tracker (`provider.ts` 内部)**。
  互換性: 「renderer が `string | null` を受け入れてから main が null を出す」順序で保つ。
  移行 / rollout: unit `ipc-envelope-producer-null-body` が `ipc-envelope-consumer-widening` に `depends_on` する。
  rollback: main 側の 1 行 (`null` を出さない) で戻せる。
  baseline verification: 既存 `provider.test.ts` の 200 系。
  target verification: `provider-null-body-status` / `provider-status-parity-and-fidelity` /
  `preload-bridge-passes-null-body-verbatim` / `repository-preparations-null-body-keeps-tracked`。

**witness**
- normal (risk: boundary) — FR-B2。stimulus: 204 の DELETE。期待観察: `status=204` の `Response`。
  禁止結果: TypeError → `Error("network")`。verification: `provider-null-body-status`。
- adversarial (risk: stale) — FR-B3。precondition: main が旧実装 (`body: ""`)。stimulus: 204 応答。
  期待観察: renderer が例外を出さない。禁止結果: 例外。verification: `provider-null-body-status`
  (同 file 内の legacy-empty-body ケース)。
- adversarial (risk: conflicting_evidence) — FR-B2。stimulus: `/api/repository-preparations/{id}` が 204 + null。
  期待観察: `tracked.preparation` が不変。禁止結果: null に潰れる。
  verification: `repository-preparations-null-body-keeps-tracked`。
- adversarial (risk: unsupported_environment) — FR-B2。stimulus: `.cjs` bridge を経由した 204。
  期待観察: `body` が null のまま。禁止結果: `""` へ coerce される。
  verification: `preload-bridge-passes-null-body-verbatim`。
- adversarial (risk: boundary) — NFR-TEST-001。stimulus: fake が 204 に `""` を返す。
  期待観察: fidelity test が落ちる。禁止結果: 緑のまま。verification: `provider-status-parity-and-fidelity`。

<!-- anchor: contract-identity-ownership-gate -->
### contract-identity-ownership-gate (integration_contract / owner: component-identity-enforcement)

**subject**: identity 変換または wire frame 直列化の所有者集合が広がったとき機械的に失敗する gate が存在し、
**その gate 自身が合成違反で発火することが示されている**。

**reason**: 症状駆動の修正が 2 世代生き延びた直接原因は「可視な症状だけが唯一の検出器だった」こと。
検出器を残さない修正は 3 度目を招く。さらに **落ちない検出器は偽の緑を作る**ので、
検出器が発火することそのものを成果物にする (draft-1 graft)。

**requirements**: FR-C1
**decision evidence mode**: total

**normal case**: 所有者集合の内側 (§component-identity-enforcement の allowlist 表) だけが
identity 変換と直列化を行っている限り gate は通る。

**adversarial case**
1. 新しい module が `sessionKey` / `parseSessionKey` を import すると fitness test が allowlist 逸脱として落ちる。
2. `Connection` の外で `ClientFrame` を組み立てて socket に渡そうとすると `tsc --noEmit` が落ちる。
3. store 行を権威なしで作ろうとすると `tsc --noEmit` が落ちる。
4. `socket/retry.ts` に `serializeClientFrame` の import を 1 行戻すと fitness test が落ちる
   (production importer の allowlist = {`src/socket/connection.ts`})。
5. **合成した違反 source** (allowlist 外 module からの `sessionKey` import、および `serializeClientFrame` import) に
   対して gate が実際に失敗することを negative test が示す。
6. **gate の到達範囲の witness (限界の明示)**: 下記 3 つの盲点を test 自身の doc comment に明記する。
   - helper を呼ばずに `session.id ===` で突合する暗黙依存 (今回発見した A-13/A-14 と同型) は捉えられない。
   - `clients/ui/tsconfig.json` の `exclude` (`src/__meta__` / `src/**/*.test.ts(x)` / `src/__tests__` /
     `src/store/__tests__` / `src/test-setup.ts`) により、**型分割・API 形状・branded types のいずれも
     test tree では検査されない**。AC-02 / AC-06 の判定媒体 `npm run typecheck` は production source のみを見る。
   - `clients/workspace/src/**` は走査対象外。第 2 encoder 実装 (§decision-7) はこの gate の外にある。

**risk tags**: unsupported_environment, repeated_usage, unknown
**expected observable**: `observable-gate-outcome` — `npm run test:unit` と `npm run typecheck` の合否
(scope: global)。必要 input: なし。

**operational inputs**: なし。
**input_closure_reason**: gate は source tree と型検査だけを入力とし、外部の情報・権限・versioned state を必要としない。

**semantic profiles**: なし。

**witness**
- normal (risk: repeated_usage) — FR-C1。stimulus: 所有者集合内の import だけ。期待観察: gate が通る。
  禁止結果: 偽陽性で落ちる。verification: `identity-owner-allowlist`。
- adversarial (risk: unknown) — FR-C1。stimulus: 合成違反 source を用意する。期待観察: gate が落ちる。
  禁止結果: 通る (= 偽の緑)。verification: `gate-fails-on-synthesized-violation`。
- adversarial (risk: boundary) — FR-A2/FR-C1。stimulus: `retry.ts` に `serializeClientFrame` import を戻す。
  期待観察: `serialize-client-frame-sole-importer` が落ちる。禁止結果: 緑のまま。
  verification: `serialize-client-frame-sole-importer`。

**implementation discretion**: `discretion-gate-scan-implementation` (allowlist 走査の実装)。
**allowlist の粒度は決定済み**: module (file) 単位。`no-japanese.test.ts` の per-file allowlist と
締め方針の書き方に倣う。symbol 単位への細分化は行わない (走査対象の symbol は上表で既に列挙済みであり、
file 単位で十分に判別できる)。

## Decision inputs (全件を disposition する)

| id | subject | source | authority | candidate | classification |
|----|---------|--------|-----------|-----------|----------------|
| `decision-input-a-ssot-consolidation` | 根本原因 A の修正規模 | user_requirement | constraint | SSOT 統合 (option-2) | design_choice |
| `decision-input-a-branded-types` | compile 時検出の全面導入 | user_requirement | candidate | branded type | design_choice |
| `decision-input-b-envelope-owner` | 根本原因 B の不変条件所有者 | user_requirement | constraint | main 側 IPC envelope 型 | design_choice |
| `decision-input-dp1-upstream-required` | `wire/server.ts:90` の required 化 | user_requirement | handoff | required 化 | design_choice |
| `decision-input-dp2-failure-policy` | P5 素通し / P6 拒否の収束先 | user_requirement | handoff | どちらかへ寄せる | design_choice |
| `decision-input-dp3-dataplane-failure` | data plane 解決失敗の契約 | user_requirement | handoff | throw しない drop + 記録 | design_choice |
| `decision-input-dp4-detector` | 交差点追加の機械的検出手段 | user_requirement | handoff | branded / lint / contract test / fitness | design_choice |
| `decision-input-dp5-envelope-order` | envelope 3 レイヤの波及順序 | user_requirement | handoff | 順序を決める | design_choice |
| `decision-input-rca-wire-owner` | wire 境界の所有者 | user_requirement | handoff | `Connection` | design_choice |
| `decision-input-rca-store-owner` | store/HTTP 境界の所有者 | user_requirement | handoff | `SessionInfo.upstreamSessionId` | design_choice |
| `decision-input-red-green` | 既存 RED を GREEN にする対象 | user_requirement | constraint | 3 本 | design_choice |
| `decision-input-critique-dataplane-unpinned` | data plane throw 契約の検出器不足 | user_requirement | constraint | 不一致で throw しない test を必須化 | design_choice |
| `decision-input-critique-invariant-rescan` | invariant を key にした再走査 | user_requirement | constraint | 暗黙依存を hits へ繰り入れる | design_choice |
| `decision-input-critique-e2e-discriminator` | real-stack e2e の判別子強化 | user_requirement | candidate | `echo: hi` へ強める | implementation_detail |
| `decision-input-critique-hasdom-scope` | `test-setup.ts` の `hasDOM` | user_requirement | candidate | 各 hook 内で評価 | implementation_detail |
| `decision-input-harness-happy-dom` | happy-dom の寛容な `Response` | user_requirement | candidate | node 隔離の規約化 | design_choice |
| `decision-input-realstack-ci-placement` | real-stack spec の CI 常設先 | user_requirement + repository_policy (`test-harness/profiles.json`) | handoff | nightly / PR gate | implementation_detail |
| `decision-input-nongoal-gateway-silent-drop` | gateway 無音 drop | user_requirement | constraint | scope 外 | design_choice |
| `decision-input-nongoal-symptom1` | 症状 1 (ターミナル非表示) | user_requirement | constraint | scope 外 | design_choice |
| `decision-input-adr-serverid-internal` | serverId は internal routing identity | accepted_adr | constraint | wire へ出さない | design_choice |
| `decision-input-adr-wire-fixture-roundtrip` | 手書き wire 型の Go fixture 拘束 | accepted_adr | constraint | fixture を decode できねばならない | design_choice |
| `decision-input-adr-exact-match-protocol` | 互換 fallback の禁止 | accepted_adr | constraint | dual decoder を持たない | design_choice |
| `decision-input-adr-session-protocol-owner` | fleet wire schema の owner file | accepted_adr | constraint | fleet module が owner | design_choice |
| `decision-input-adr-biome-sole-linter` | lint 手段 | accepted_adr | constraint | Biome が唯一の linter | design_choice |
| `decision-input-policy-ipc-zero-value` | zero value と別型の doctrine | repository_policy | constraint | 別意味の zero value は別型へ | design_choice |
| `decision-input-policy-external-triple` | 外部依存 test の三点セット | repository_policy | constraint | fake + backstop + contract | design_choice |
| `decision-input-policy-workspace-no-reimpl` | workspace の再実装禁止 | repository_policy | constraint | INV-001 | design_choice |
| `decision-input-policy-file-length` | file/function 長の heuristic | repository_policy | constraint | 500 / 80 行 | implementation_detail |
| `decision-input-implicit-local-server-fallback` | `App.tsx:80-85` の `?? "local"` | repository_policy (`ARCHITECTURE.md`) | candidate | 捏造 server identity への抵触の疑い | design_choice |

## Decisions — 結論と、覆される条件

<!-- anchor: adr-fleet-session-identity-boundary-ownership -->
### decision-1 — `upstreamSessionId` の required 化と型の所在

**結論: required 化しない。型を分割する。identity 所有者は `fleet/session-identity.ts` へ carve-out する。**

`wire/server.ts` の `SessionInfo` は `clients/ui/src/wire/testdata/hello.json` (Go 生成) の decode 先であり、
fixture は `"id":"s1"` の bare id を持ち `upstreamSessionId` を持たない (`wire/codec.test.ts:6` の round-trip)。
client 専用 field を required にすると `adr-20260624-0021` の fixture 拘束と整合しない。

代わりに 2 つの型に分ける。`SessionInfo` は gateway payload のみ (bare id、client field なし)。
`FleetSessionRow` は store/UI 行で `id` (fleet key) / `serverId` / `upstreamSessionId` を required で持ち、
`fleet/session-identity.ts` に置く (`wire/` に置くと再び wire 型と混ざる)。
`FleetSessionRow` は `SessionInfo` の subtype なので `SessionInfo` を**読むだけ**の consumer は無変更で通る。
churn は producer 側と test fixture factory に限定される (実測: fixture factory 9 箇所、literal 構築 21 file)。
この churn は許容する — 「権威を持たない行」を型で禁じられることの対価として妥当であり、
代替 (optional のまま resolver の単一 default 方針) はその禁止を失う。

**`SessionInfo` からの field 削除**: `serverId?` / `serverLabel?` / `upstreamSessionId?` を **削除する**。
残すと 2 つの型が同時に権威を名乗る中間状態が残り、本 change が閉じようとしている曖昧さを型に残すことになる。
`publishSessions !== false` 分岐 (`connection.ts:326` / `:342` の `tagSessions`) は production 未到達だが
削除もしない (non_goal) — 代わりに `contract-wire-id-ingress` の**単一再修飾経路へ合流**させる。
これは削除ではなく統合であり、FR-A4 の「単一経路」要求と同じ帰結である。

**所有者モジュールの分離根拠**: `session-protocol.ts` は既に 432 行で health schema と decoder を持つ。
identity 変換を同居させると 500 行規約 (AGENTS.md) に触れ責務も 2 つになる。
同一 directory 内の carve-out なら `adr-20260803` の所有 (fleet module) を維持したまま責務を分けられる。

**覆される条件**: Go 側が `SessionInfo` に fleet 概念を持ち込む決定が入った場合 (そのときは型分割の前提が消える)。

### decision-2 — P5 (bare id 素通し) / P6 (bare id 拒否) の収束と resolver の戻り型

**結論: typed rejection へ収束する。失敗の名前は `unresolved session`。resolver は throw せず判別 union を返す。**

根拠は「どちらが望ましいか」ではなく **P5 が到達不能である**という観測:
- `clients/web-host/internal/webhost/host.go:40-56` は `/` (静的) と `/host/...` のみ。`staticHandler` は
  `http.FileServer(http.FS(assets))` のみで SPA fallback を持たないので `/api/sessions/{id}/transcript` は 404。
- `services/server` に SPA を配信する static handler は存在しない (`FileServer`/`embed.FS` の grep が 0 件)。
- `clients/ui/vite.config.ts` に `/api` proxy 設定も無い (dev server も同じ)。
- Electron 経路は `provider.request` (IPC) を通るので素通し分岐に入らない。

したがって P5 は互換性を保存しておらず、温存は `adr-20260727-exact-match-client-server-protocol` が禁じる
「成功しない互換 path の維持」に当たる。
(critique の pass1 はこの根拠を独立に検証し、攻撃不成立と判定した。)

失敗の名前を変える理由: 「invalid fleet session key」は原因ではなく形式を指しており、
呼び出し側が「では bare id のときはどうする」と分岐を再発明する余地を残す。
「unresolved session」は結果を指し、分岐先を持たない。

**戻り型を throw ではなく判別 union にする理由 (draft-1 graft)**: `api/sessions.ts:289-309` の `doFetch` は
AbortError 以外の throw をすべて `makeNetworkError(e)` へ写像する。resolver が throw する設計を採ると
**根本原因 B と同型の誤診断 (成功でない失敗が network error として表示される) が A 側に新設される**。
判別 union なら api モジュールは要求発行前に `ok:false` を分岐でき、`doFetch` の catch に入らない。
実測: `routedSession` は `routedRequest` の先頭で同期に評価され、`doFetch` の catch は `fetchImpl(...)` の
周りにしか掛かっていない。したがって「resolver の失敗が `doFetch` の catch を通らない位置で起きる」ことは
実装で保証でき、`unresolved-issues-no-request` がそれを test で固定する。

**`api/sessions.ts:471` の `fleet.initialServerId()` fallback (方針 P4) の扱い**: resolver には取り込まない。
これは「id から server を解く」問題ではなく「行が無いときの既定 server」という別の問い (fleet 構成の問題) であり、
resolver に混ぜると解決順序が 4 段になって precedence の検算ができなくなる。
現行どおり `api/sessions.ts` の factory に残し、resolver は `(row | id) → route | unresolved` に閉じる。

**挙動変化 (明示)**: これまで 404 になっていた transcript/workspace 要求が、要求を出す前に typed failure で
返るようになる。ユーザーから見える差は「同じ失敗が速くなる」だけで、成功していた操作が失敗に転じることはない
(この断定の前提は §decision-6 の到達性実測に載っている)。

**覆される条件**: 同一 origin で SPA を配信する host 構成を将来サポートする決定が入った場合。
**この falsifier は §decision-6 の `"local"` 到達性の falsifier と同一**である (同じ host 構成が両方を活性化する)。

### decision-3 — data plane の解決失敗契約と記録 surface

**結論: throw しない。frame を組み立てず破棄し、reason で分けて記録する。
制御プレーン (`s`/`u`/`ld`) は従来どおり throw。**

`send()` は `term.onData` / `attachCustomKeyEventHandler` / `term.onResize` から xterm のイベント emitter 内で
同期に呼ばれる (`TerminalPane.tsx:169-194`)。ここで throw すると xterm 内部へ例外が伝播する。
制御プレーンは async 経路 (`subscribeOnce` / `unsubscribeOnce` / `publishDesired`) なので
`upstreamSessionID` の throw 契約 (`connection.ts:245`) をそのまま維持できる。

観測可能性が必須である理由: 無音の drop は本 change が閉じようとしている欠陥そのもの
(gateway `gateway_lifecycle_inbound.go:133` の `slog.Warn` だけの drop が、2 世代の症状集合からこの交差点を隠した)。
クライアント側で同じ形の無音 drop を作れば、次の欠陥も同じ理由で見えなくなる。

**記録 surface は reason で分ける (critique の誤報指摘への応答)**。draft-2 の既定
(reason を問わず `useNotificationsStore` の `level:"error"` 1 件) は、発火条件
`cfg.serverId !== key.serverId` が **server 切替の正常遷移中に成立し得る良性の窓**であることを見落としていた
(`App.tsx:80-94` の `useMemo` が serverId 変更で Connection を再生成する窓)。

- `decode-failed` (fleet key でも bare id でもない値 = **内部契約違反**) → notification (`level:"error"`) + counter
- `server-mismatch` (`cfg.serverId !== key.serverId` = **正常遷移由来**) → counter のみ、通知なし

counter は Connection の read-only getter として公開する (専用 diagnostics store は新設しない — どの requirement にも
trace しないため)。dedup は notification 側にのみ適用し `(sessionId, reason)` ごとに 1 Connection lifecycle 1 件。

**critique finding-dataplane-throw-contract-unpinned への応答**: 既存 RED 3 本は
`cfg.serverId === key.serverId` のケースしか通していないため、実装者が `send()` 内で `upstreamSessionID` を
そのまま再利用しても 3 本すべて GREEN になる。serverId 不一致の `{k:"i"}` が throw せず drop + counter になることを
固定する test を **本 change の必須成果物**とする。

**覆される条件**: server 切替の窓で drop が起きないことが実装上保証された場合
(そのときは `server-mismatch` を内部契約違反へ昇格できる)。

<!-- anchor: adr-defer-branded-session-id-types -->
### decision-4 — 交差点追加の機械的検出、および egress 所有者境界の閉じ方

**結論: (i) API 形状 + (ii) 型分割 を主検出器とし、(iii) fitness test を補助に置く。
branded types は別 change へ明示的に繰り延べ、再訪条件を ADR に記録する。
あわせて `socket/retry.ts` の直列化 bypass を閉じ、AC-02 を実測可能な形へ書き換える。**

#### (a) egress 所有者境界の閉じ方 (critique blocker)

draft-2 の AC-02「Connection の外で wire frame を組み立てるコードは typecheck が通らない」は day-1 で **偽**だった。
`retry.ts:1` が `serializeClientFrame` を import し、`:31` で `{k:"s",...}` を組み立て、
`connection.ts:192` の `send: (s) => this.ws?.send(s)` を通して socket へ直接流していたためである。

閉じ方:
1. `RetryDeps.send` を `(frame: string) => void` から `(frame: ClientFrame) => void` へ変える。
2. `retry.ts` の `serializeClientFrame` import と `:31` の直列化を削除する (frame オブジェクトを渡すだけにする)。
3. `connection.ts:192` の `send: (s) => this.ws?.send(s)` を `send: (f) => this.sendFrame(f)` へ置き換え、
   直列化を `connection.ts:130` (private `sendFrame`) の 1 箇所へ戻す。
4. 公開 `send(frame: ClientFrame)` を private 化し、data plane は intent API
   (`sendInput(fleetKey, data)` / `sendResize(fleetKey, cols, rows)`) を通す。

**intent API の粒度は 2 メソッドに確定する**。`sendSessionFrame(key, frame)` の 1 メソッド案は却下 —
呼び出し側が `frame` を組み立てることになり、移管したはずの組み立て権威が component 側に戻る。

**AC-02 の書き換え**: 「Connection の外で wire frame を組み立てるコードは typecheck が通らない」から
**「`ClientFrame` の組み立てと `serializeClientFrame` の呼び出しが production source では
`clients/ui/src/socket/connection.ts` の外に存在しない」**へ。
実測 (`grep -rn serializeClientFrame clients/ui/src`): production importer は `connection.ts` (2 行) と
`retry.ts` (2 行) の 2 file のみ。`wire/adapter.ts` は codec を構造型で受けるので import しない
(`selectCodec` / `generatedCodecStub` の唯一の呼び出し元は `wire/codec.ts` 自身)。
`wire/codec.test.ts` / `wire/lifecycleWire.test.ts` は test tree。
したがって修正後の allowlist は {`src/socket/connection.ts`} で閉じる。

**既存 RED の書き換え**: `src/socket/connection.fleet-input.test.ts:90,102,112` は `conn.send({k:"i"|"r",...})` を
直接叩いている。`send` private 化に合わせて `conn.sendInput(FLEET_KEY, "ls\r")` /
`conn.sendResize(FLEET_KEY, 100, 30)` / bare id 素通しの 3 件へ書き換える。
**3 つの witness (fleet key→bare id 変換 / resize / bare id 素通し) を保存したまま intent API 形へ書き換えて緑にする**
— assertion を弱めない。

**graft[0] の open choice をここで閉じる**: 変換を `serializeClientFrame` の入口 (`wire/codec.ts`) に置く案は
**却下**する。retry.ts の bypass も同時に覆えるという利点はあるが、codec に fleet 概念を持ち込むと
`adr-20260803-resilient-fleet-health-single-authority` の owner 割当 (identity は fleet module) と
`adr-20260624-0021-frontend-wire-types-hand-written` の wire 型中立性 (wire 型は Go payload の鏡であり
client 概念を持たない) の双方を壊す。変換は intent API (Connection) 側に置き、
retry の bypass は上記 1-3 の書き換えで閉じる。

#### (b) 検出器の選択

選択肢の到達範囲を対にして比較する (draft-1 graft)。

| 手段 | 捕らえる交差点 | 捕らえない交差点 | この repo での可否 |
|---|---|---|---|
| tsc の型分割 (`FleetSessionRow` required) | 権威を持たない store 行の構築 | 型を名乗らず `string` で入る値、手書き JSON 操作、**test tree 全体** (tsconfig exclude) | 可 (`strict` 済み) |
| tsc の API 形状 (`send` private / intent API) | Connection 外での frame 組み立て | Connection 内部の誤変換、**test tree 全体** | 可 |
| branded / nominal 型 | 型を名乗る全 id 交差点 | 外部から `string` で入る値、**test tree 全体** | 可だが **未立法** (branded/nominal の ADR は 0 件) |
| Biome `nursery.noRestrictedImports` + `overrides` | 所有者外からの import | import しない手書き実装。**module specifier 照合が相対 path の深さで揺れる** | 可 (1.9.4 schema で確認) |
| T2 source scan fitness test | source pattern に一致する import | pattern を外れる書き方、helper を呼ばない暗黙依存 (A-13/A-14 型) | 可。repo の支配的 pattern |
| T3 real-stack e2e | outcome が観察可能な交差点 | 観察に出ない交差点 | 可。ただし常設先が未決 (§decision-8) |

主検出器を API 形状と型分割に置く理由は、これらが**境界の帰結として自動的に得られる**から:
`send` が private になれば Connection の外で wire frame を組み立てるコードは compile error になる。
`FleetSessionRow` が権威を required で持てば「権威を持たない行」を作れない。
branded types が追加で守る範囲は「所有者を迂回して socket を直接触るコード」に縮み、その経路は API 形状で既に塞がれている。

**lint rule 案の却下根拠 (critique の非対称比較指摘への応答)**。draft-2 は「ESLint 導入は ADR-0020 の supersede を要する」と
書いたが、`docs/adr/adr-20260624-0020-biome-as-frontend-lint.md:61-62` は
「If a rule we need is missing, falling back to eslint later is a package.json swap, not a code change」と
**反転コストの低さを明示している**。したがって却下根拠は supersede コストではなく次の 2 つ:
1. **NFR-DEP-001** — typescript-eslint 一式は 4-5 個の npm 依存追加と `make lint` の実行時間増を伴う。
   本 change は新規 runtime/開発依存を追加しない方針を持つ。
2. **部分集合性** — type-aware lint rule が捕らえる交差点は、API 形状 (send private) + 型分割
   (`FleetSessionRow` required) が捕らえる集合の**部分集合**である (上表の 1-3 行目)。
   追加の依存を払って得られる残余が無い。
Biome 側の `noRestrictedImports` を採らない理由は上表の脆さ (相対 path の深さで module specifier 照合が揺れる) で、
同じ判定を安定に書ける fitness test を選ぶ。

**検出器自身が発火することの検証 (draft-1 graft)**: 合成した違反 source
(allowlist 外 module からの `sessionKey` import、および `serializeClientFrame` import) に対して
gate が実際に失敗する negative test を成果物に含める。**落ちない検出器は偽の緑を作る**。

**検出器の到達範囲の開示 (critique)**: `clients/ui/tsconfig.json` の `exclude` は
`src/__meta__` / `src/**/*.test.ts` / `src/**/*.test.tsx` / `src/__tests__` / `src/store/__tests__` /
`src/test-setup.ts` を含む。したがって AC-02 / AC-06 の判定媒体 `npm run typecheck` (`tsc --noEmit`) は
**test tree を見ない**。型分割・API 形状・branded types のいずれも同じ限界を持つ (branded types 固有ではない)。
AC-02 / AC-06 の criteria に「production source (tsconfig の include 範囲) において」を明記し、
同じ限界を fitness test の doc comment にも書く。

**再訪条件 (ADR に記録する)**: 本 change 適用後に、所有者を迂回する新しい identity 交差点が 1 つでも
production に現れた場合、branded types を別 change として起票する。
その際 eslint への切替は ADR-0020 の consequence どおり **package.json swap で可能**であり、
ADR の supersede を要しない (再訪時のコスト見積もりを誤らないためにここに記録する)。

<!-- anchor: adr-ipc-response-envelope-null-body-ownership -->
### decision-5 — envelope 3 宣言 + 実行時 bridge の波及順序と互換

**結論: consumer-first。(1) renderer (`provider.ts` + `auth.ts` の型宣言) が `string | null` を受け入れる →
(2) preload の型宣言を広げる → (3) main が null を出す。renderer 側の defensive 分岐は恒久の二重化として残す。**

envelope 型は 3 箇所に独立宣言されており (main `electron-entry.ts:454-458` の推論戻り型、
preload `preload/index.ts:64-68`、renderer `clients/ui/src/auth.ts:80-84`)、**型そのものに owner がいない**。
これは根本原因 B と同型の欠陥が 1 段上に存在するということ。

型の owner を `clients/workspace/src/shared/http-envelope.ts` に置く。
**訂正 (critique)**: 「preload はそこから import する」が届くのは pure builder 側の
`clients/workspace/src/preload/index.ts:64-68` **だけ**である。実行時 bridge は `electron-entry.ts:56` が
読み込む `src/preload/electron-preload.cjs:72-73` の untyped passthrough で、
`clients/workspace/tsconfig.json` は `allowJs` / `checkJs` を持たないため `.cjs` は型検査対象外。
型で担保できないこの部分は **runtime test** へ移す (`preload-bridge-passes-null-body-verbatim`):
`electron` を stub して `.cjs` を require し、`sessionFleet.request` が `ipcRenderer.invoke` の戻り envelope を
`body: null` のまま (`?? ""` などの coercion なしで) 返すことを assert する。
先例は `clients/workspace/tests/app-entry.test.ts` (エントリ file を require する既存 test)。

renderer は `clients/workspace` から import できない (`rootDir: src` / `include: ["src/**/*"]` かつ別 package) ので
構造的宣言のまま残し、**platform oracle による両側独立検証**で一致を担保する。

順序を consumer-first にする理由: Electron は `clients/ui` の dist を **コピーして**同梱する
(`clients/workspace/package.json` の build)。producer を先に変えると、古い renderer dist と新しい main が
組み合わさった瞬間に `new Response(null, ...)` を想定しない経路が壊れうる。逆順なら
「null を受け入れられるが null が来ない」という無害な中間状態にしかならない。

**defensive 分岐を残すことの確定 (critique)**: draft-2 は「二重化として残すか消すか」を open にしたまま
AC-08 (renderer が新しく main が古い組合せでも例外にならない) をその上に載せていた。
分岐を消す選択を取ると AC-08 は原理的に達成不能になるため、**残す**に確定する。
恒久である根拠は `input-main-envelope-behavior` の `invalidation_behavior` に既に書かれている:
build がコピー同梱であるため dev 実行や部分再ビルドで main/renderer の対が崩れうる。
したがってこの分岐は一時的な移行措置ではなく恒久の二重化であり、その理由を code comment に書く。
ユーザー確定方針 (「renderer 側 defensive 分岐は所有者ではなく二重化として扱う」) とも整合する。

**envelope の第 2 consumer (critique)**: 変更 surface は `provider.ts:644` の再物質化だけでなく
`provider.ts:536-538` の repository-preparations 直接 body 消費を含む。status guard が 2xx を通すので
204 が到達し、`JSON.parse(null)` は throw せず `null` を返すため、現状のままだと catch に入らず
`tracked.preparation` が無音で null に潰れる。`result.body` が null なら `JSON.parse` せず
`tracked.preparation` を維持する分岐を必須要件とする。

**既存 RED の限界 (F10)**: `provider.null-body-status.test.ts` の fake bridge は 204 に対して `body: ""` を返し、
bridge 型注釈も `body: string`。つまりこの RED は renderer 側の defensive 分岐だけで GREEN になり、
「owner は main 側 envelope」という確定方針を判別できない。
本 change では (a) fake を real に合わせて null body status で `body: null` を返すよう修正し型注釈を
`string | null` へ広げ、(b) workspace 側に real の builder を叩く T1 test を置き、
(c) 「fake が real より寛容にならない」ことを名指す contract test を置く
— AGENTS.md の external-dependency triple をこの seam に適用する。

**覆される条件**: Electron が `clients/ui` の dist をコピー同梱しなくなった場合
(そのとき main/renderer の対は build 単位で保証され、defensive 分岐は削除候補になる)。

### decision-6 — `App.tsx` の `?? "local"` (到達性の実測と disposition)

**結論: scope 外を維持する。ただし「到達し得ない」を assumption ではなく実測に接地させ、
反証条件と follow-up issue を明記する。**

`ARCHITECTURE.md` は「Host-kind-dependent cardinality, implicit default servers, and browser-origin server identity are
architectural violations」と述べており、`App.tsx:80-85` の `return selected ?? "local"` は config に存在しない
server identity を合成しているように読める。§decision-2 は「成功していた操作が失敗に転じることはない」と断定するので、
この経路の到達性は決定の前提になっている。

**実測 (静的 call graph。fact)**

1. `"local"` は **routing 値にならない**。HTTP 経路の serverId は `parseSessionKey(sessionId)?.serverId`
   (`api/sessions.ts:470`, `api/workspace.ts:189`) か `fleet.initialServerId()` から得る。
   `initialServerId` = `explicitServerId()` (`provider.ts:237-242`) は
   `window.hostedModeInfo?.serverId ?? window.agentGridWorkspace?.hostedModeInfo?.serverId` か
   URL の `?server=` を返し、無ければ **`null`** を返す。`"local"` は返らない。
   `api/sessions.ts:472` はそこで `throw new Error("No server selected")` する。
2. `"local"` は **store 行の `serverId` にもならない**。行は `App.tsx:134-141` の唯一の producer が
   `FleetSnapshot` の `ref.serverId` から作る。
3. `"local"` の唯一の consumer は **transport binding** — `fleet.activeConnection(serverId, token)` と
   `new Connection({serverId})`。`webProvider.activeConnection` (`provider.ts:441-452`) には
   `if (serverId === "local")` の**明示的な分岐**があり、same-origin の `/api/ws-ticket` と `/ws` を選ぶ。
   つまり `"local"` は偶発的な捏造ではなく「同一 origin の非修飾接続」を表す **sentinel** である。
4. その sentinel が選ぶ endpoint は、どの shipping host にも存在しない (§decision-2 の 3 観測と同じ根拠:
   web-host は `/` + `/host/...` のみ、`services/server` は SPA を配信せず、vite dev に `/api` proxy が無い)。
   Workspace では `workspaceProvider.activeConnection` が `WorkspaceSocket("local", openChannel)` を作るが、
   main は `servers.json` に無い server を解決できない。
5. `"local"` が生じるのは `activeSessionID` が無く `explicitServerId()` が null のときに限られる。
   その状態では選択中の session が無いので data plane の送出も起きない。

**推論 (inference)**: したがって `"local"` を持つ Connection は socket を確立できず、
FR-A4 の ingress 再修飾がこの値で store へ到達することはない。§decision-2 の無退行主張は成立する。

**限界 (honest disclosure)**: これは repository の静的 call graph に対する実測であって runtime 観測ではない。
本 change の `mutation_scope` では production code / test を変更できないため runtime 確認は行っていない。

**反証条件 (falsifier)**: 同一 origin で SPA と gateway を配信する host 構成が追加された場合、
`activeConnection("local")` が実際に接続でき、同時に §decision-2 の P5 も到達可能になる。
**この 2 つの決定は同一の falsifier を共有する** — その host 構成が入ったら decision-2 と decision-6 を同時に再訪する。

**follow-up**: `?? "local"` が `ARCHITECTURE.md` の implicit default server 禁止に抵触するかの判断は
本 change の scope 外。`dev-docs` で issue を起票する (gateway 無音 drop の起票と同じ扱い)。

**pass2 の patch hint からの逸脱を明記する**: pass2 は
「`serverId="local"` が `servers.json` に無い状態で resolve すると `unresolved` になり HTTP 要求は 1 件も発行されない」を
adversarial case として追加せよと指示したが、これは resolver の実際の意味論と一致しない。
`resolveSessionRoute('["local","s1"]')` は行が無くても文字列 decode で **determinate** を返す (precedence: 行 > decode)。
実測 (上記 1-2) が示すとおり `"local"` はそもそも id 空間に入らないため、この反例は構成できない。
代わりに `contract-http-route-resolution` の adversarial (5) として**真である**記述
(sentinel は resolver の入力にも行の serverId にもならない / 仮に渡しても新たな成功経路は生まれない) を置いた。

### decision-7 — `clients/workspace/src/shared/session-ref.ts` の第 2 encoder 実装

**結論: 本 change から切り離す。component / contract / unit / AC を 1 組で除去し、follow-up issue を起票する。**

`clients/workspace/src/shared/session-ref.ts` は `sessionKey` を逐語で再実装しており、
`adr-20260803-resilient-fleet-health-single-authority` は再実装を禁じてはいないが
「1 つの canonical fixture suite で拘束される」ことを条件にしている。その fixture は存在しない。
draft-2 はこれを contract 1 件 / unit 1 件 / AC-10 として取り込んでいた。

切り離す根拠:

1. **change scope 宣言との不一致**。`change.md` の `scope` に当該 file は列挙されていない。
   draft-2 自身も「scope-adjacent」と開示していた。開示と同時に成果物へ組み込むと、
   「切り離す場合の切断点」が acceptance に混ざる。
2. **要件 trace が無い**。FR-C1 は `clients/ui` の identity 変換の**所有者集合**を対象にしている。
   2 実装間の encoder drift はこの要件からは導かれず、取り込むには別要件 (FR-C2「同一 encoding の 2 実装は
   canonical fixture で拘束される」) の新設が必要になる。新要件は change.md の outcomes にも無い。
3. **本 change が drift を活性化しない**。本 change は `sessionKey` を `session-protocol.ts` から
   `session-identity.ts` へ **移動**するだけで encoding を変えない。第 2 実装が新たに壊れるリスクは生じない。
4. **simplicity (削除を先に検討する)**。取り込むと、package-local な変更集合に唯一の cross-package 結合が入る
   (`discretion-key-fixture-location` は `boundary_reach: cross_component` / `context_scope: repository` /
   `coupling: shared_across_units`)。この 1 件のために fixture 配置の決定 (`clients/config/fixtures/` か
   各 package の testdata か) を本 change に持ち込む必要が生じる。

**切り離しに伴う明示的な限界の開示 (silent omission にしない)**: `component-session-identity` が名乗る
「fleet key を構文的に扱ってよい唯一の場所」は **`clients/ui/src/**` に限る**。
`contract-identity-ownership-gate` の走査範囲も同じであり、`clients/workspace/src/**` は gate の外にある。
この限界は gate の doc comment に書く (AC-09 の「検出できない範囲」の一項目)。

**follow-up issue に載せる内容 (切断点の保存)**: component `component-workspace-key-mirror`
(`clients/workspace/src/shared/session-ref.ts` + `src/main/window-registry.ts`) /
contract `contract-workspace-key-mirror-conformance` / unit `workspace-key-mirror-fixture` /
AC「2 つの encoder が同一 fixture で一致し、片方の変更がもう片方の test を落とす」/
新要件 FR-C2。fixture の置き場所は `clients/config/fixtures/` に先例がある
(`servers-valid-bearer.json` などが両言語から読まれる)。

**覆される条件**: 本 change 内で fleet key の encoding を変える必要が生じた場合
(そのとき第 2 実装は即座に drift するので、fixture 拘束を同 change へ引き戻す)。

### decision-8 — real-stack e2e の CI 常設先

**結論: 本 change は `e2e/real-stack/terminal-input.spec.ts` の GREEN 化までを担い、CI 常設は行わない。
常設の具体形と保護 file を issue に記録する。**

接地 (draft-1 graft): `test-harness/profiles.json` の `pr` profile に real-stack は含まれておらず、
T3 の常設先は `nightly` profile の `scripts/run-nightly-e2e-report.sh` **のみ**である。
したがって「常設する」は `nightly` profile へ command を追加することを意味し、
`test-harness/profiles.json` 自身が `test-harness/protected.json` の tamper 保護対象
(`required_markers: ["pre-push", "nightly"]`) なので、trusted harness gate を通る変更になる。

これは harness の変更であり本 change の scope (change.md) に無い。open のまま残すのではなく、
**「常設するなら nightly profile への command 追加であり protected.json の保護対象 file を触る」**という
具体形を issue に記録して閉じる。

`decision-input-critique-e2e-discriminator` (判別子を `echo: hi` へ強める) も同じ issue に束ねる。
本 change の必須成果物ではない (spec を GREEN にすることが必須であり、判別子の強化は別 test の改善)。

## 追加で発見した交差点 (invariant-keyed 再走査の結果)

critique-2 の `finding-horizontal-scan-key-is-helper-name` が要求した「invariant を key にした再走査」を実施し、
helper を呼ばない交差点を 2 つ確定した。helper 名を key にした 15 hit には含まれていない。

- **A-13** `socket/connection.ts:356-363` — view-update の `activity_session_id` を **無変換で**
  `applyViewUpdate` へ渡す。gateway 由来なので bare、store 側は fleet key。
- **A-14** `store/workspaceActivity.ts:491-492` — activity event の `session_id` (bare) を
  store key と照合するために `sessionIdsMatch` の総当たりを 2 回行う。

この 2 つが、方針 P7 (`store/workspaceActivity.ts:10-21` の alias 総当たり) が存在する理由である。
P7 は「解決を諦めた」のではなく「上流 (inbound 再修飾) が不完全なことを下流で吸収していた」。
FR-A4 が満たされれば P7 は不要になり、`sessionIdAliases` / `sessionIdsMatch` は削除できる。
逆に、この 2 交差点を直さずに P7 だけを消すと activity drawer の scope 照合が無音で壊れる
— **順序制約であり、`contract-store-identity-equality` の前提条件**として固定する
(spine では unit `store-row-authority-and-equality` の `depends_on` が機械表現)。

交差点は 15 + 2 = **17**。これが「全交差点を閉じた」の母数である (§goal)。

## Assumptions

- production の `Connection` は `App.tsx:88-93` の 1 箇所でのみ生成され `publishSessions: false` 固定。
  したがって `connection.ts:327` / `:343` の `publishSessions !== false` 分岐は production では到達しない。
  本 change はこの分岐を削除しないが、単一再修飾経路へ合流させる (§decision-1)。この前提が誤りなら
  合流の順序が観察に影響する。
- `store/frameMessaging.ts` が production で populate されないのは上記の帰結。scope 外だが、
  型分割で顕在化するので実装者は「壊した」と誤認しないこと。
- session id は localStorage / sessionStorage に永続化されていないため、旧 id 形式の migration 対象データは無い。
- gateway の session id は `[A-Za-z0-9_-]` に制限されており JSON 配列文字列になり得ない。
  したがって bare id に対する decode の冪等性 (`parseSessionKey` が null を返す) は偶発一致しない。
  この前提は `contract-wire-id-egress` の outcome partition の exclusivity basis そのものである。
- `App.tsx:80-85` の `?? "local"` は transport binding 専用の sentinel であり id 空間に入らない
  (§decision-6 に実測・限界・falsifier を記録)。
- 挙動活性化 (fix 後に初めて `{k:"r"}` と制御シーケンスが PTY へ届く) は Assumption ではなく
  `contract-egress-activation-compatibility` として固定した (§FR-A8)。

## Constraints

- lint は Biome のまま。ESLint / typescript-eslint を追加しない (根拠は §decision-4 の 2 点。
  ADR-0020 の supersede コストではない)。
- 新規 runtime 依存を追加しない (AGENTS.md の library selection 手順を発動させない)。
- wire protocol (frame 形状・kind) と `servers.json` 契約は変更しない。
- `wire/server.ts` の `SessionInfo` は Go 生成 fixture (`wire/testdata/*.json`) を decode できる形を保つ
  (`adr-20260624-0021`)。
- `clients/workspace` は `clients/ui` の source を import できない (`rootDir: src` / package 分離)。
  型の共有は不可能なので、一致は platform oracle または runtime test で担保する。
- 互換 fallback / dual decoder / degradation を導入しない (`adr-20260727-exact-match-client-server-protocol`)。
- coverage 閾値: `src/{api,socket,wire}/**` は lines 86 / statements 81 / functions 80 / branches 70、
  `src/{store,hooks}/**` は 92 / 90 / 93 / 83 を下回らない (`vitest.config.ts`。
  当該 file は `test-harness/protected.json` の tamper 保護対象)。
- fake は real より寛容にしない。乖離時は fake を直す (AGENTS.md)。
- file 500 行 / function 80 行を責務分割の目安とする。

## Verification 表 (spine の `contracts[].verification[].id` と 1:1)

| verification id | tier | 対象 contract | 既存 RED / 新規 | 対応 AC |
|---|---|---|---|---|
| `connection-fleet-input-outbound` | T1 | contract-wire-id-egress | **既存 RED** (intent API 形へ書き換え) | AC-01, AC-02 |
| `terminal-pane-intent-api` | T1 | contract-wire-id-egress | 新規 (既存 test 群の追随を含む) | AC-02 |
| `raw-frame-escape-hatch-closed` | T2 | contract-wire-id-egress | 新規 (typecheck) | AC-02 |
| `retry-path-uses-private-serialization` | T1 | contract-wire-id-egress | 新規 | AC-02 |
| `dataplane-no-throw-on-serverid-mismatch` | T1 | contract-dataplane-unresolved-drop | 新規 | AC-03 |
| `dataplane-drop-counter-by-reason` | T1 | contract-dataplane-unresolved-drop | 新規 | AC-03 |
| `dataplane-decode-failed-notifies-once` | T1 | contract-dataplane-unresolved-drop | 新規 | AC-03 |
| `inbound-requalification-all-kinds` | T1 | contract-wire-id-ingress | 新規 | AC-04 |
| `activity-session-id-requalified` | T1 | contract-wire-id-ingress | 新規 | AC-04 |
| `requalification-memoized-per-session` | T1 | contract-wire-id-ingress | 新規 | AC-10 |
| `resize-frame-reaches-wire` | T1 | contract-egress-activation-compatibility | 新規 | AC-12 |
| `control-sequence-passthrough-verbatim` | T1 | contract-egress-activation-compatibility | 新規 | AC-12 |
| `wire-fixture-roundtrip-unchanged` | T1 | contract-session-identity-types | 既存 (回帰保護) | AC-06 |
| `store-row-requires-authority` | T2 | contract-session-identity-types | 新規 (typecheck) | AC-06 |
| `identity-types-pure` | T0 | contract-session-identity-types | 新規 | AC-06 |
| `resolver-precedence-row-over-decode` | T0 | contract-http-route-resolution | 新規 | AC-05 |
| `unresolved-issues-no-request` | T1 | contract-http-route-resolution | 新規 | AC-05 |
| `pinned-handle-not-routed` | T1 | contract-http-route-resolution | 新規 | AC-05 |
| `four-api-modules-single-policy` | T2 | contract-http-route-resolution | 新規 | AC-05 |
| `strict-equality-after-ingress` | T1 | contract-store-identity-equality | 新規 | AC-04 |
| `alias-removal-order-constraint` | T1 | contract-store-identity-equality | 新規 | AC-04 |
| `reconnect-bell-suppression-preserved` | T1 | contract-store-identity-equality | 既存 (回帰保護) | AC-04 |
| `envelope-null-body-status-mapping` | T1 | contract-ipc-null-body-envelope | 新規 | AC-07 |
| `envelope-platform-oracle-guard` | T2 | contract-ipc-null-body-envelope | 新規 | AC-07 |
| `empty-200-body-not-null` | T1 | contract-ipc-null-body-envelope | 新規 | AC-07 |
| `provider-null-body-status` | T1 | contract-host-transport-parity | **既存 RED** (fake を real 準拠へ + AC-08 ケース追加) | AC-07, AC-08 |
| `preload-bridge-passes-null-body-verbatim` | T1 | contract-host-transport-parity | 新規 | AC-07 |
| `repository-preparations-null-body-keeps-tracked` | T1 | contract-host-transport-parity | 新規 | AC-07 |
| `provider-status-parity-and-fidelity` | T2 | contract-host-transport-parity | 新規 | AC-11 |
| `identity-owner-allowlist` | T2 | contract-identity-ownership-gate | 新規 | AC-09 |
| `serialize-client-frame-sole-importer` | T2 | contract-identity-ownership-gate | 新規 | AC-02, AC-09 |
| `gate-fails-on-synthesized-violation` | T2 | contract-identity-ownership-gate | 新規 | AC-09 |
| `compile-time-boundary-closed` | T2 | contract-identity-ownership-gate | 新規 (typecheck) | AC-02, AC-06 |

**T3 (spine 外)**: `clients/ui/e2e/real-stack/terminal-input.spec.ts` — **既存 RED**。AC-01 と AC-12 の
end-to-end 観察点。GREEN 化は必須、CI 常設は §decision-8 により scope 外。

## Critique disposition

### 解消した issue (verdict Y / 13 件)

| issue | severity | 解消先 |
|---|---|---|
| `issue-d2-egress-authority-bypass` | blocker | §decision-4(a)。`RetryDeps.send` を `ClientFrame` 化、`retry.ts` の直列化削除、`connection.ts:192` を private `sendFrame` 経由へ、AC-02 を実測形へ書き換え、`serialize-client-frame-sole-importer` を追加、graft[0] の codec 入口案を却下として記録、既存 RED を intent API 形へ書き換え |
| `issue-d2-http-handle-session-collateral` | major | §component-session-routing-http の責務境界 + `contract-http-route-resolution` adversarial(4) + `pinned-handle-not-routed` |
| `issue-d2-ac08-depends-on-open-choice` | major | §decision-5。defensive 分岐を「残す」に確定。AC-08 の充足媒体を `provider-null-body-status` に固定 |
| `issue-d2-preload-runtime-binding` | major | §decision-5 の訂正 + `component-fleet-provider-renderer.paths` へ `electron-preload.cjs` 追加 + `preload-bridge-passes-null-body-verbatim` |
| `issue-d2-envelope-second-consumer` | major | `contract-host-transport-parity` の変更 surface / typed consumer 拡張 + adversarial(4) + `repository-preparations-null-body-keeps-tracked` |
| `issue-d2-adr0020-asymmetric-comparison` | major | §decision-4(b)。却下根拠を NFR-DEP-001 と部分集合性へ差し替え、「supersede 必須」を削除、再訪条件に package.json swap を明記 |
| `issue-d2-dataplane-error-surface-default` | major | §decision-3。記録要件を (a1)/(a2) に分割、`server-mismatch` は counter のみ、AC-03 を「notification 0」へ |
| `issue-d2-implicit-local-server` | major | §decision-6。静的 call graph の実測、fact / inference / 限界 / falsifier の分離、follow-up issue |
| `issue-d2-typecheck-excludes-tests` | minor | §decision-4(b) の到達範囲表と開示、AC-02 / AC-06 の criteria に「production source において」を明記、gate doc comment 要件 |
| `issue-d2-workspace-mirror-scope` | minor | §decision-7。切り離しを決定。切断点 1 組と follow-up issue、gate の範囲限界の開示 |
| `issue-d2-crossing-count-arithmetic` | minor | §goal の母数表 (19 / 15 / 17) と §additional-crossings の 15 + 2 = 17 |
| `issue-d2-exact-match-adr-id` | minor | §accepted-adr-context と §decision-2 で `-protocol` へ訂正 (訂正の事実も記録) |
| `issue-d2-concurrency-dimension-undeclared` | minor | §design-dimensions に `concurrency` を追加し、3 つの反例を名指し |

### 攻撃不成立 (verdict N / 3 件) — draft-2 の主張を維持する

| issue | 維持する主張 | 維持の根拠 |
|---|---|---|
| `issue-d2-red-fake-discriminability-claim` | 「既存 RED は renderer 側 defensive 分岐だけで GREEN になり owner=main を判別できない」 | critic 自身が repo 実体 (fake の 3 箇所とも `body: ""`、bridge 型注釈も `body: string`、`clients/workspace/tests` に session-fleet-request の test は 0 件) と一致すると検証。誇張ではない。fake 修正 + workspace 側 real builder test + fake-vs-real 忠実性 test を必須成果物に昇格させた判断を維持する (§decision-5) |
| `issue-d2-p5-unreachability-evidence` | 「P5 の非 fleet 経路はどの shipping host でも 404」 | critic が `webhost/host.go:184-192` の `staticHandler` に SPA fallback が無いこと、`services/server` 配下に `FileServer`/`embed.FS` が 0 件であること、`vite.config.ts` に `/api` proxy が無いことを独立に確認。挙動変更判断を「望ましさ」ではなく到達性の観測に接地した点は反証を通った (§decision-2) |
| `issue-d2-perf-frbeta07-grounding` | NFR-PERF-001 の FR-β07 引用 | critic が `connection.ts:303` の `// OutputFrame — direct callback per FR-β07 (kHz output, UI must not block)` を確認。定義文書の不在だけでは根拠を崩せない。`cost_convergence` profile を ingress contract に置いた判断も維持 |

### graft の取り込み先 (10 件)

| graft | 取り込み先 |
|---|---|
| [0] retry.ts bypass の明示 + open choice | §decision-4(a) (open choice は却下として閉じた) |
| [1] `handle_session` を routing 対象外として除外 | §component-session-routing-http + `contract-http-route-resolution` adversarial(4) |
| [2] `?? "local"` の decision input と effective 条件 | §decision-6 + `decision-input-implicit-local-server-fallback` |
| [3] 合成違反に対する negative test | `contract-identity-ownership-gate` adversarial(5) + `gate-fails-on-synthesized-violation` |
| [4] 検出器到達範囲表 + Biome の脆さ | §decision-4(b) の表 |
| [5] activation compatibility contract | `contract-egress-activation-compatibility` + FR-A8 + AC-12 |
| [6] resolver throw が `Error("network")` に化ける | §decision-2 (判別 union に確定) + `contract-http-route-resolution` |
| [7] server 切替の窓は正常であり toast は誤報 | §decision-3 の (a1)/(a2) 分割 |
| [8] `test-harness/profiles.json` / `protected.json` 接地 | §decision-8 |
| [9] verification 表形式 | §verification 表 |

## Closure notes

- 本 change の完了は「3 本の RED が GREEN」ではなく、
  (a) 交差点の追加が compile 時または gate で落ちること、(b) data plane の失敗が reason 別に観測可能であること、
  (c) envelope の不変条件が real 側 (builder + 実行時 bridge) で検証されていること、の 3 点で判定する。
- **起票する follow-up issue (3 件)**
  1. gateway が解決不能な session id を無音で捨てる件 (`gateway_lifecycle_inbound.go:124-140`)。
  2. `App.tsx:80-85` の `?? "local"` が `ARCHITECTURE.md` の implicit default server 禁止に抵触するかの判断 (§decision-6)。
  3. `clients/workspace/src/shared/session-ref.ts` の第 2 encoder 実装を canonical fixture で拘束する
     follow-up change (§decision-7 に切断点 1 組を保存)。あわせて real-stack e2e の CI 常設
     (nightly profile への command 追加 + protected.json 保護対象) と判別子強化 (§decision-8)。
- 症状 1 (ターミナル非表示) は未帰属のまま。A/B 修正後の Electron 実機再観測まで
  `docs/bugs/workspace-electron-null-body-status.md` および元 bug-report の closure を保留する。
  critique が加えた候補 (根本原因 B の二次症状: Workspace 経路の transcript/messages が常時例外) は
  本 change 適用後に自動的に判別される。
- `decision-input-critique-hasdom-scope` (`test-setup.ts` の `hasDOM` を module scope const から
  各 hook 内評価へ) は harness 側の hardening として記録するが、本 change の contract には含めない。
- `decision-input-harness-happy-dom` は本 change では file 単位の node environment 隔離を継続し、
  規約化と happy-dom の real 準拠化は harness 側の別案件として残す。
  envelope の platform oracle guard がこの gap の局所的な代替になる。
- governance gate は `hard`。承認 evidence の取得は host / conductor が行い、change へ記録する。
