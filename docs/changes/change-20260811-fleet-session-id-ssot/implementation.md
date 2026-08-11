---
change: change-20260811-fleet-session-id-ssot
role: implementation
contracts:
- contract-wire-id-egress
- contract-dataplane-unresolved-drop
- contract-wire-id-ingress
- contract-egress-activation-compatibility
- contract-session-identity-types
- contract-http-route-resolution
- contract-store-identity-equality
- contract-ipc-null-body-envelope
- contract-host-transport-parity
- contract-identity-ownership-gate
contract_projections:
- id: contract-wire-id-egress
  verifications:
  - connection-fleet-input-outbound
  - terminal-pane-intent-api
  - retry-path-uses-private-serialization
  - raw-frame-escape-hatch-closed
  discretion:
  - discretion-upstream-parse-fast-reject
- id: contract-dataplane-unresolved-drop
  verifications:
  - dataplane-no-throw-on-serverid-mismatch
  - dataplane-drop-counter-by-reason
  - dataplane-decode-failed-notifies-once
  discretion:
  - discretion-drop-record-dedup-structure
- id: contract-wire-id-ingress
  verifications:
  - inbound-requalification-all-kinds
  - activity-session-id-requalified
  - requalification-memoized-per-session
  discretion:
  - discretion-requalification-memo
- id: contract-egress-activation-compatibility
  verifications:
  - resize-frame-reaches-wire
  - control-sequence-passthrough-verbatim
  discretion: []
- id: contract-session-identity-types
  verifications:
  - wire-fixture-roundtrip-unchanged
  - store-row-requires-authority
  - identity-types-pure
  discretion: []
- id: contract-http-route-resolution
  verifications:
  - resolver-precedence-row-over-decode
  - unresolved-issues-no-request
  - pinned-handle-not-routed
  - four-api-modules-single-policy
  discretion: []
- id: contract-store-identity-equality
  verifications:
  - strict-equality-after-ingress
  - alias-removal-order-constraint
  - reconnect-bell-suppression-preserved
  discretion: []
- id: contract-ipc-null-body-envelope
  verifications:
  - envelope-null-body-status-mapping
  - envelope-platform-oracle-guard
  - empty-200-body-not-null
  discretion:
  - discretion-envelope-builder-shape
- id: contract-host-transport-parity
  verifications:
  - provider-null-body-status
  - preload-bridge-passes-null-body-verbatim
  - repository-preparations-null-body-keeps-tracked
  - provider-status-parity-and-fidelity
  discretion: []
- id: contract-identity-ownership-gate
  verifications:
  - identity-owner-allowlist
  - serialize-client-frame-sole-importer
  - gate-fails-on-synthesized-violation
  - compile-time-boundary-closed
  discretion:
  - discretion-gate-scan-implementation
adrs:
- adr-fleet-session-identity-boundary-ownership
- adr-ipc-response-envelope-null-body-ownership
- adr-defer-branded-session-id-types
decision_dispositions:
- decision_input_ref: decision-input-a-ssot-consolidation
  disposition: adopted。SSOT を「変換関数の置き場所」ではなく「境界ごとに公開 API が話す id 空間を 1 つに固定すること」として実装する。局所修正
    (option-1) は却下済み。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-wire-id-egress
  - contract-wire-id-ingress
  - contract-http-route-resolution
  - contract-session-identity-types
- decision_input_ref: decision-input-a-branded-types
  disposition: rejected (本 change では採らない)。API 形状 (send private + intent API) と型分割で
    wire 境界と store 境界が閉じ、branded types の残余保護範囲は所有者を迂回して socket を直接触るコードに縮む。その経路は API
    形状で既に塞がれている。ADR に再訪条件 (迂回する新交差点が 1 つでも現れたら別 change として起票。その際 eslint への切替は package.json
    swap で可能) を記録する。
  adr_refs:
  - adr-defer-branded-session-id-types
  contract_refs:
  - contract-identity-ownership-gate
- decision_input_ref: decision-input-b-envelope-owner
  disposition: adopted。不変条件の所有者は clients/workspace/src/shared/http-envelope.ts に置く
    envelope 型と builder。renderer 側の分岐は恒久の二重化として併置する (build がコピー同梱で対が崩れうるため)。
  adr_refs:
  - adr-ipc-response-envelope-null-body-ownership
  contract_refs:
  - contract-ipc-null-body-envelope
  - contract-host-transport-parity
- decision_input_ref: decision-input-dp1-upstream-required
  disposition: rejected (どちらの候補も採らない)。SessionInfo は Go 生成 fixture の decode 先なので client
    専用 field を required にできない。型を分割し FleetSessionRow 側で required にすることで「未設定時方針」という問い自体を消す。あわせて
    SessionInfo から serverId? / serverLabel? / upstreamSessionId? を削除し、2 つの型が同時に権威を名乗る状態を残さない。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-session-identity-types
- decision_input_ref: decision-input-dp2-failure-policy
  disposition: adopted (typed rejection へ収束)。失敗の名前は unresolved session。根拠は P5 の非 fleet
    経路がどの shipping host でも成功しないという観測 (web-host に /api ルートなし、server に SPA 配信なし、vite
    dev に /api proxy なし)。resolver は throw せず判別 union を返す (doFetch の network error
    写像に吸われないため)。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-http-route-resolution
- decision_input_ref: decision-input-dp3-dataplane-failure
  disposition: adopted。throw しない。frame を破棄し reason 別に記録する。decode-failed (内部契約違反) は
    notification + counter、server-mismatch (server 切替の正常遷移窓) は counter のみ。制御プレーンの
    throw 契約は維持する。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-dataplane-unresolved-drop
- decision_input_ref: decision-input-dp4-detector
  disposition: adopted (組合せで確定)。API 形状と型分割を主検出器、fitness test を補助とし、検出器自身が合成違反で落ちる
    negative test を必須成果物にする。lint rule 案は却下 — 根拠は ADR-0020 の supersede コストではなく (1)
    NFR-DEP-001 (typescript-eslint 一式で 4-5 個の依存追加と make lint 実行時間) と (2) type-aware
    rule が捕らえる交差点が API 形状 + 型分割の捕らえる集合の部分集合であること。branded types は繰り延べ。
  adr_refs:
  - adr-defer-branded-session-id-types
  contract_refs:
  - contract-identity-ownership-gate
  - contract-wire-id-egress
  - contract-session-identity-types
- decision_input_ref: decision-input-dp5-envelope-order
  disposition: adopted (consumer-first)。renderer/preload の型を広げる → main が null を出す。Electron
    が clients/ui の dist をコピー同梱するため producer 先行だと対の崩れた組合せが壊れうる。実行時 bridge (electron-preload.cjs)
    は型検査対象外なので runtime test で担保する。
  adr_refs:
  - adr-ipc-response-envelope-null-body-ownership
  contract_refs:
  - contract-host-transport-parity
- decision_input_ref: decision-input-rca-wire-owner
  disposition: adopted (強化)。Connection を所有者とするだけでなく、send を private 化し直列化を private
    sendFrame へ戻して frame 組み立て権威も移す。retry.ts の bypass もこの unit で閉じる。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-wire-id-egress
  - contract-wire-id-ingress
- decision_input_ref: decision-input-rca-store-owner
  disposition: adopted (担体を変更)。field (SessionInfo.upstreamSessionId) は権威の担体であって所有者ではない。所有者は
    identity モジュール (resolver + FleetSessionRow 型) とする。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-session-identity-types
  - contract-http-route-resolution
- decision_input_ref: decision-input-red-green
  disposition: adopted。3 本すべてを GREEN 化対象とする。connection.fleet-input.test.ts は 3 つの
    witness を保存したまま intent API 形へ書き換える。provider.null-body-status.test.ts は fake が
    real より寛容 (204 に body "") なため所有者移動を判別できない — fake を real 準拠へ修正し型注釈を string | null
    へ広げ、workspace 側に real builder の test と preload bridge の runtime test を追加して初めて要件を満たすとみなす。
  adr_refs:
  - adr-ipc-response-envelope-null-body-ownership
  contract_refs:
  - contract-host-transport-parity
  - contract-ipc-null-body-envelope
  - contract-wire-id-egress
- decision_input_ref: decision-input-critique-dataplane-unpinned
  disposition: adopted。serverId 不一致の {k:"i"} が throw せず drop と counter になり notification
    を出さないことを固定する test を本 change の必須成果物とする。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-dataplane-unresolved-drop
- decision_input_ref: decision-input-critique-invariant-rescan
  disposition: adopted (実施済み)。helper を呼ばない交差点を 2 つ確定 (connection.ts:356-363 の activity_session_id、workspaceActivity.ts:491-492
    の event.session_id)。helper 名を key にした hit 15 に 2 を加えて交差点は 17。上流 call site 総数 19
    とは別の母数であり、差分 2 件は id 空間をまたがない call site。両者を FR-A4 の対象に含める。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-wire-id-ingress
  - contract-store-identity-equality
- decision_input_ref: decision-input-critique-e2e-discriminator
  disposition: not_applicable (本 change の contract には含めない)。real-stack spec の判別子を echo
    で始まる文字列へ強める改善は spec の GREEN 化とは別の test 改善であり、decision-input-realstack-ci-placement
    と同じ follow-up issue へ束ねる。
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-critique-hasdom-scope
  disposition: not_applicable (harness 側の hardening として記録)。test-setup.ts の hasDOM
    を module scope const から各 hook 内評価へ変える改善は本 change の contract に含めない。
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-harness-happy-dom
  disposition: adopted (限定形)。本 change では file 単位の node environment 隔離を継続し、規約化と happy-dom
    の real 準拠化は harness 側の別案件として残す。envelope の platform oracle guard がこの gap の局所的な代替になる。
  adr_refs:
  - adr-ipc-response-envelope-null-body-ownership
  contract_refs:
  - contract-ipc-null-body-envelope
- decision_input_ref: decision-input-realstack-ci-placement
  disposition: not_applicable (scope 外として具体形付きで閉じる)。本 change は GREEN 化までを担う。常設するなら
    test-harness/profiles.json の nightly profile (T3 の唯一の常設先。pr profile に real-stack
    は無い) へ command を追加することを意味し、当該 file は test-harness/protected.json の tamper 保護対象
    (required_markers pre-push / nightly) なので trusted harness gate を通る変更になる。この具体形を
    follow-up issue に記録する。
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-nongoal-gateway-silent-drop
  disposition: not_applicable (scope 外を維持)。ただし data plane の観測可能性契約 (contract-dataplane-unresolved-drop)
    の根拠として参照する。起票先を change の closure notes に明記する。
  adr_refs: []
  contract_refs:
  - contract-dataplane-unresolved-drop
- decision_input_ref: decision-input-nongoal-symptom1
  disposition: not_applicable (scope 外を維持)。critique が加えた候補 (根本原因 B の二次症状) は本 change
    適用後に自動判別されるため、bug-report の closure を Electron 実機再観測まで保留する条件だけを残す。
  adr_refs: []
  contract_refs: []
- decision_input_ref: decision-input-adr-serverid-internal
  disposition: adopted (制約として適用)。serverId 成分が upstream wire に出ないことが FR-A1 の根拠であり、views
    が ref を持つという ADR の記述が構造化 route を運ぶ設計を支持する。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-wire-id-egress
  - contract-http-route-resolution
- decision_input_ref: decision-input-adr-wire-fixture-roundtrip
  disposition: adopted (制約として適用)。SessionInfo に client 専用の required field を足せないことが
    decision-1 を決定づけた。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-session-identity-types
- decision_input_ref: decision-input-adr-exact-match-protocol
  disposition: adopted (制約として適用)。bare id 素通しを互換 path として温存する選択肢を排除する。参照 id は adr-20260727-exact-match-client-server-protocol
    (draft-2 が decision-2 の根拠文で -platform と綴っていた誤りを訂正)。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-http-route-resolution
- decision_input_ref: decision-input-adr-session-protocol-owner
  disposition: adopted (制約として適用)。identity 所有者は fleet module 内に置く。同居ではなく同一 directory
    への carve-out (fleet/session-identity.ts) に確定した (session-protocol.ts が既に 432 行で責務が
    2 つになるため)。clients/workspace/src/shared/session-ref.ts の第 2 encoder を canonical
    fixture で拘束する件は本 change から切り離し follow-up issue とする (change.md の scope 宣言に無く、FR-C1
    から導かれず、本 change が encoding を変えないため drift を活性化しない)。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-session-identity-types
- decision_input_ref: decision-input-adr-biome-sole-linter
  disposition: adopted (制約として適用)。ESLint / typescript-eslint を追加せず lint rule 案を選択肢から除外する。ただし却下根拠は
    ADR-0020 の supersede コストではない — ADR 本文 :61-62 は「falling back to eslint later is
    a package.json swap, not a code change」と反転コストの低さを明示している。根拠は NFR-DEP-001 と、type-aware
    rule の捕捉集合が API 形状 + 型分割の部分集合であること。
  adr_refs:
  - adr-defer-branded-session-id-types
  contract_refs:
  - contract-identity-ownership-gate
- decision_input_ref: decision-input-policy-ipc-zero-value
  disposition: adopted (制約として適用)。body 空文字が本文なしを意味する状態を廃し、別表現 (null) を与える。根本原因 B の修正は新方針ではなく既存
    design 規律の未適用の是正である。
  adr_refs:
  - adr-ipc-response-envelope-null-body-ownership
  contract_refs:
  - contract-ipc-null-body-envelope
- decision_input_ref: decision-input-policy-external-triple
  disposition: adopted (制約として適用)。fake (bridge) / fidelity backstop (fake が real より寛容でない)
    / 不変条件を名指す contract test の 3 点を envelope seam に置く。実行時 bridge の runtime test も同
    seam に加える。
  adr_refs:
  - adr-ipc-response-envelope-null-body-ownership
  contract_refs:
  - contract-host-transport-parity
  - contract-ipc-null-body-envelope
- decision_input_ref: decision-input-policy-workspace-no-reimpl
  disposition: adopted (制約として適用)。envelope 型を workspace 側に置くことは wire decoding の再実装に当たらない
    (status から body の有無を決めるのは HTTP semantics の転写であり、session presentation でも auth resolution
    でもない)。既存の sessionKey 再実装 (shared/session-ref.ts) は本 change の scope 外として follow-up
    issue へ送る。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-ipc-null-body-envelope
- decision_input_ref: decision-input-policy-file-length
  disposition: adopted (制約として適用)。session-protocol.ts が既に 432 行であることが identity 所有者を同一
    directory の別 file へ carve-out する根拠のひとつ。分離を確定した。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-session-identity-types
- decision_input_ref: decision-input-implicit-local-server-fallback
  disposition: not_applicable (scope 外。到達性を実測して decision-2 の前提に接地させた)。App.tsx:80-85
    の `return selected ?? "local"` は transport binding 専用の sentinel であり id 空間に入らない。
    実測 (静的 call graph) — (1) HTTP routing の serverId は parseSessionKey(id)?.serverId
    か fleet.initialServerId() から得るが、後者 (provider.ts:237-242 の explicitServerId) は
    hostedModeInfo か ?server= を返し、無ければ null を返す ("local" は返らず api/sessions.ts:472
    が "No server selected" を throw する)。 (2) store 行の serverId は App.tsx:134-141 が
    FleetSnapshot の ref.serverId から作る。 (3) "local" の唯一の consumer は fleet.activeConnection
    と Connection cfg で、 webProvider.activeConnection (provider.ts:441-452) は serverId
    === "local" の明示分岐で same-origin の /api/ws-ticket と /ws を選ぶ。(4) その endpoint はどの
    shipping host にも存在しない。 (5) "local" が生じるのは activeSessionID も explicitServerId も無いときだけで、その状態では
    data plane 送出も起きない。 したがって decision-2 の無退行主張は成立する。限界 — runtime 観測ではなく静的 call graph
    の実測。 falsifier — 同一 origin で SPA と gateway を配信する host 構成が追加されると "local" 接続と P5
    が 同時に活性化する (decision-2 と同一の falsifier)。ARCHITECTURE.md の implicit default server
    禁止への 抵触判断は follow-up issue で起票する。
  adr_refs:
  - adr-fleet-session-identity-boundary-ownership
  contract_refs:
  - contract-http-route-resolution
  - contract-wire-id-ingress
milestones:
- id: chunk-1-identity-authority
- id: chunk-2-wire-boundary
- id: chunk-3-store-http-boundary
- id: chunk-4-ipc-envelope
- id: chunk-5-enforcement
reference_algorithms: []
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Content

境界を 3 つに引き直し、それぞれに **1 つの id 空間**と **1 人の所有者**を割り当てる。
手段 (型・API 形状・test) はこの割り当ての帰結としてのみ導入する。

| 境界 | 所有者 | 公開 API が話す id 空間 | 内側に閉じる id 空間 |
|------|--------|--------------------------|------------------------|
| WS wire | `Connection` (`clients/ui/src/socket/connection.ts`) | fleet key のみ | upstream bare id + wire frame の組み立て・直列化 |
| store / HTTP routing | `SessionRoute` resolver + `FleetSessionRow` (`clients/ui/src/fleet/session-identity.ts`) | fleet key + 構造化 route | 文字列からの再導出 (廃止) |
| Electron IPC transport | `HttpResponseEnvelope` (`clients/workspace/src/shared/http-envelope.ts`) | `body: string \| null` | `Response` の再物質化差異 |

### 確定した設計判断 (ADR 候補)

- **adr-fleet-session-identity-boundary-ownership** — fleet session identity の境界所有権。
  型を分割し (`SessionInfo` は gateway payload のみ、`FleetSessionRow` が store 行)、
  wire 境界の所有者を `Connection` に置いて frame 組み立て権威まで移し、
  store / HTTP 境界を単一 resolver に収束させる。失敗は `unresolved session` の 1 種類とし、
  resolver は throw せず判別 union を返す。
- **adr-ipc-response-envelope-null-body-ownership** — IPC response envelope の null-body 不変条件の所有者を
  `clients/workspace/src/shared/http-envelope.ts` に置き、consumer-first の順序で移行する。
  renderer 側の defensive 分岐は恒久の二重化として残す。
- **adr-defer-branded-session-id-types** — branded types を本 change では採らず、
  API 形状 + 型分割を主検出器、fitness test を補助とする。再訪条件を記録する。

### Implementation contracts (10)

| contract | dimension | owner |
|---|---|---|
| `contract-wire-id-egress` | integration_contract | component-wire-connection |
| `contract-dataplane-unresolved-drop` | failure_recovery | component-wire-connection |
| `contract-wire-id-ingress` | integration_contract | component-wire-connection |
| `contract-egress-activation-compatibility` | migration_compatibility | component-wire-connection |
| `contract-session-identity-types` | data_model | component-session-identity |
| `contract-http-route-resolution` | integration_contract | component-session-routing-http |
| `contract-store-identity-equality` | state_lifecycle | component-session-store |
| `contract-ipc-null-body-envelope` | integration_contract | component-ipc-response-envelope |
| `contract-host-transport-parity` | migration_compatibility | component-fleet-provider-renderer |
| `contract-identity-ownership-gate` | integration_contract | component-identity-enforcement |

### 実装順 (chunk / unit と依存)

```
chunk-1  identity-owner-module
           |
           +-- chunk-2  connection-ingress-requalification
           |               |
           +-- chunk-2  connection-egress-intent-api
           |               |
           |               +-- chunk-2  terminal-dataplane-intent-migration
           |
           +-- chunk-3  http-route-resolver-adoption
           |
           +-- chunk-3  store-row-authority-and-equality   (depends also on ingress-requalification)

chunk-4  ipc-envelope-consumer-widening
           |
           +-- chunk-4  ipc-envelope-producer-null-body

chunk-5  identity-ownership-gate
           (depends on egress-intent-api + store-row-authority + http-route-resolver)
```

chunk-4 は chunk-1..3 と独立 (根本原因 B は A と別の真バグ)。並行して進めてよい。

#### chunk-1 — identity authority

**identity-owner-module** (`clients/ui/src/fleet/session-identity.ts` を `session-protocol.ts` から carve-out)

- `SessionRef` / `sessionKey` / `parseSessionKey` を新 module へ移し、`session-protocol.ts` からは参照のみにする。
  分離の根拠: `session-protocol.ts` は既に 432 行で health schema と decoder を持つ (責務 2 つ + 500 行規約)。
  同一 directory 内の carve-out なので `adr-20260803` の所有 (fleet module) は維持される。
- `FleetSessionRow` を同 module に定義し `id` (fleet key) / `serverId` / `upstreamSessionId` を **required** にする。
- `resolveSessionRoute(row | id)` を追加し、**判別 union** で返す (throw しない)。
  優先順位は **行の権威 > id 文字列の decode > unresolved** に固定する。
- **throw にしない理由**: `api/sessions.ts:289-309` の `doFetch` は AbortError 以外の throw を
  `makeNetworkError(e)` へ写像する。resolver が throw すると根本原因 B と同型の誤診断が A 側に新設される。
- 挙動は変えない。既存 test は全て緑のまま。

#### chunk-2 — wire boundary

**connection-ingress-requalification** (`socket/connection.ts`)

- `handleMessage` の全 case を単一の再修飾経路へ通す: `:305-309` output / `fleetSessionID` 経由の `tt`,`et` /
  **`:356-363` の `activity_session_id` (新規)** / `:405-415` `qualifyHelloSessions` / `:326`,`:342` `tagSessions`。
- `publishSessions !== false` 分岐は **削除しない**が、`initialServerHint` による独立 tagging を残さず
  同じ再修飾経路へ合流させる (経路が 2 本あると FR-A4 の「単一経路」が偽になる)。
- 再修飾は session 単位で memo 化する (NFR-PERF-001)。`cfg.serverId` 未設定時の素通しは保つ。
- `cfg.serverId` が fleet member でない場合の magic string 分岐 (`"local"` の特別扱い) は**作らない**。

**connection-egress-intent-api** (`socket/connection.ts`, `socket/retry.ts`, `wire/client.ts`)

これが critique の blocker を閉じる unit。

1. intent API を 2 メソッドで追加: `sendInput(fleetKey, data)` / `sendResize(fleetKey, cols, rows)`。
   `sendSessionFrame(key, frame)` の 1 メソッド案は採らない (呼び出し側が frame を組み立てることになり
   移管したはずの組み立て権威が component に戻る)。
2. `send(frame: ClientFrame)` を **private 化**し、直列化を `connection.ts:130` の private `sendFrame` 1 箇所へ戻す。
3. `RetryDeps.send` を `(frame: string) => void` から `(frame: ClientFrame) => void` へ変え、
   `retry.ts:1` の `serializeClientFrame` import と `:31` の直列化を **削除**する。
4. `connection.ts:192` の `send: (s) => this.ws?.send(s)` を `send: (f) => this.sendFrame(f)` へ置き換える。
5. data plane の解決失敗は **throw しない**。frame を破棄し reason 別に記録する:
   - `decode-failed` (fleet key でも bare id でもない = 内部契約違反) → notification (`level:"error"`) + counter
   - `server-mismatch` (`cfg.serverId !== key.serverId` = server 切替の正常遷移窓) → **counter のみ、通知なし**
   counter は Connection の read-only getter として公開する (専用 diagnostics store は新設しない)。
   notification の dedup は `(sessionId, reason)` ごとに 1 Connection lifecycle 1 件。
   制御プレーン (`s`/`u`/`ld`) の `upstreamSessionID` throw 契約 (`connection.ts:245`) は維持する。
6. 既存 RED `connection.fleet-input.test.ts:90,102,112` は `conn.send({k:...})` を直接叩いているので、
   **3 つの witness (fleet key→bare id 変換 / resize / bare id 素通し) を保存したまま** intent API 形へ書き換える。
   assertion を弱めない。

確認: `grep -rn serializeClientFrame clients/ui/src --include='*.ts'` の production hit が
`socket/connection.ts` の 2 行だけになること (`wire/adapter.ts` は codec を構造型で受けるので対象外、
`wire/codec.test.ts` / `wire/lifecycleWire.test.ts` は test tree)。

**terminal-dataplane-intent-migration** (`components/TerminalPane.tsx` ほか)

- `:172` `:182` `:193` `:276` の 4 emission site を intent API 呼び出しへ置換する。frame を組み立てない。
- `DriverShortcutBar.tsx` / `TerminalMobileOverlay.tsx` の `sendInput` 経路も同じ intent API へ合流させる。
- fake conn を使う既存 test 群 (`newline-modifier` `:174` / `pc-baseline` `:208`) を intent API 形へ追随させる。
  bare id 前提の assertion を壊さない。
- xterm の初期化・fit・geometry publish のロジックには触らない。

#### chunk-3 — store / HTTP boundary

**http-route-resolver-adoption** (`api/{sessions,transcripts,workspace,capsules}.ts`)

- 4 モジュールの現行方針を列挙してから 1 方針へ寄せる:
  P4 = `sessions.ts:469-474` (row → initialServerId → throw "No server selected")、
  P5 = `transcripts.ts:26` / `workspace.ts:191` (bare id 素通し)、
  P6 = `capsules.ts:104` (bare id を throw で拒否)。
- **URL path の session 成分だけ**を resolver 経由にする。
  `api/workspace.ts:141-146` の `pinnedQuery` が載せる `handle_session` は
  `WorkspacePinnedHandle` 由来の server が返した bare id であり **routing 値ではない**。
  resolver を通すと gateway 側の workspace handle pin が解決できず save/diff が壊れる。**変換しない。**
- resolver の `ok:false` 分岐は **`doFetch` の try に入る前** (要求発行前) に行う。
  これにより失敗が `Error("network")` へ写像されない。
- `api/sessions.ts:471` の `fleet.initialServerId()` fallback は resolver に**取り込まない**。
  これは「id から server を解く」問題ではなく「行が無いときの既定 server」という別の問い (fleet 構成) であり、
  混ぜると解決順序が 4 段になって precedence の検算ができなくなる。
- `api/workspace.test.ts` を新規作成する (現在この module に test file が無い)。

**store-row-authority-and-equality** (`store/{daemon,statusSignals,workspaceActivity}.ts`, `App.tsx`, `wire/server.ts`)

- store が `FleetSessionRow` を保持する。権威なしの行を作ると typecheck が落ちる。
- `SessionInfo` から `serverId?` / `serverLabel?` / `upstreamSessionId?` を **削除**する
  (残すと 2 つの型が同時に権威を名乗る中間状態が型に残る)。
- `workspaceActivity` の `sessionIdAliases` / `sessionIdsMatch` (`:10-21`, `:166`, `:172`, `:478`, `:491-492`) を削除し、
  同一性判定を厳密一致にする。
- `statusSignals.signalIdentityKey(row, epoch)` は `` `${row.id}\u0000${epoch}` `` **だけ**を返す。
  `parseSessionKey` / `sessionKey` / `upstreamSessionId` / `serverId` を参照しない。
- **順序制約**: `connection-ingress-requalification` が先に入っていることを確認してから alias を消す。
  逆順だと activity drawer の scope 照合が無音で壊れる。この制約は unit の `depends_on` で機械表現し、
  `alias-removal-order-constraint` test で検出する。
- fixture factory 9 箇所と session literal を作る test file 21 を grep で列挙して一括更新する。
- `publishSessions !== false` 分岐そのものの削除と `frameMessaging` の production 未到達問題は直さない。

#### chunk-4 — IPC envelope (consumer-first)

**ipc-envelope-consumer-widening** (先に入れる)

- `clients/ui/src/auth.ts:80-84` と `clients/workspace/src/preload/index.ts:64-68` の envelope 型宣言を
  `body: string | null` へ広げる。
- `provider.ts:644` の `new Response(result.body, {status, headers})` が null を受けられるようにし、
  旧 main の `{status:204, body:""}` も例外にしない **defensive 分岐を恒久的に残す**。
  残す理由 (`clients/workspace/package.json` の build が `clients/ui` の dist をコピー同梱するため
  dev 実行や部分再ビルドで main/renderer の対が崩れうる) を code comment に書く。
- **envelope の第 2 consumer**: `provider.ts:536-538` の repository-preparations tracker が
  `JSON.parse(result.body)` で同じ body を消費する。status guard (`< 200 || >= 300`) は 204 を通し、
  `JSON.parse(null)` は throw せず `null` を返すため、そのままだと catch に入らず
  `tracked.preparation` が無音で null に潰れる。`result.body` が null なら `JSON.parse` せず
  `tracked.preparation` を維持する分岐を入れる。
- **実行時 bridge**: 型 import が届くのは pure builder 側 (`preload/index.ts`) だけ。
  実行時は `electron-entry.ts:56` が読み込む `src/preload/electron-preload.cjs:72-73` の untyped passthrough
  (`clients/workspace/tsconfig.json` は `allowJs`/`checkJs` を持たない)。
  型で担保できないので runtime test (`tests/electron-preload-bridge.test.ts`) を置く。
  `electron` を stub して `.cjs` を require し、`sessionFleet.request` が envelope を
  `body: null` のまま (`?? ""` などの coercion なしで) 返すことを assert する。
  先例は `clients/workspace/tests/app-entry.test.ts`。
- **fake の real 準拠化**: 既存 RED `provider.null-body-status.test.ts` の fake bridge は 3 箇所とも
  `body: ""` を返し型注釈も `body: string`。この RED は renderer 側 defensive 分岐だけで GREEN になり
  「owner は main 側 envelope」を判別できない。fake を null body status で `body: null` を返すよう修正し、
  型注釈を `string | null` へ広げる。

**ipc-envelope-producer-null-body** (後に入れる)

- `clients/workspace/src/shared/http-envelope.ts` に型と `toResponseEnvelope(response)` を新設する。
  `src/shared/` は `control-envelope.ts` の先例があり、preload から import できる唯一の共有位置
  (`src/main/` に置くと main → preload の依存方向が逆転する)。
- `electron-entry.ts:454-458` の無条件 `body: await response.text()` を builder 呼び出しへ置換する。
  `ipcMain.handle` は import 時に登録され直接 test できないため、envelope 生成を pure function に切り出してから
  handler を書き換える。
- null body status 集合の初期値は `{101, 204, 205, 304}`。**定数は platform oracle に従属する** —
  集合の各要素で `new Response("", {status})` が throw し、集合外の代表 status (200/201/400/500) では
  throw しないことを assert する。platform が集合を変えたら test が落ちて定数の更新を強制する。
- `fleetFetch` の認証・redirect 拒否ロジックには触らない。webProvider 側は変更しない。

#### chunk-5 — enforcement

**identity-ownership-gate** (`clients/ui/src/__meta__/session-identity-ownership.test.ts`)

走査対象と production importer の allowlist:

| 走査する symbol | allowlist |
|---|---|
| `sessionKey` / `parseSessionKey` | `src/fleet/session-identity.ts`, `src/socket/connection.ts` |
| `serializeClientFrame` (`src/wire/codec.ts` から) | `src/socket/connection.ts` |
| `selectCodec` / `generatedCodecStub` (`src/wire/adapter.ts` から) | `src/wire/codec.ts` |

- 走査範囲は `src/**/*.ts(x)` から `*.test.ts(x)` / `__meta__` / `__tests__` を除いた production source。
  allowlist の粒度は module (file) 単位。`src/__meta__/no-japanese.test.ts` の per-file allowlist と
  締め方針の書き方に倣う。
- **negative test 必須**: 合成した違反 source (allowlist 外 module からの `sessionKey` import、
  および `serializeClientFrame` import) に対して gate が実際に失敗することを示す。落ちない検出器は偽の緑を作る。
- **doc comment に限界を明記する** (AC-09):
  1. helper を呼ばずに `session.id ===` で突合する暗黙依存 (A-13/A-14 と同型) は捉えられない。
  2. `clients/ui/tsconfig.json` の `exclude` (`src/__meta__` / `src/**/*.test.ts(x)` / `src/__tests__` /
     `src/store/__tests__` / `src/test-setup.ts`) により、型分割・API 形状・branded types のいずれも
     **test tree では検査されない**。`npm run typecheck` は production source のみを見る。
  3. `clients/workspace/src/**` は走査対象外 (第 2 encoder 実装は follow-up issue)。
- `docs/note/note-20260624-technical-code-enforcement.md` に節を追加する。
- 新規依存を追加しない。ESLint / typescript-eslint も追加しない。

### 検出器の選択根拠 (lint rule を採らない理由)

主検出器を **API 形状 (send private + intent API)** と **型分割 (`FleetSessionRow` required)** に置く。
これらは境界の帰結として自動的に得られる。fitness test は補助であり、単独を検出器と呼ばない。

lint rule 案の却下根拠は **ADR-0020 の supersede コストではない** —
`docs/adr/adr-20260624-0020-biome-as-frontend-lint.md:61-62` は
「If a rule we need is missing, falling back to eslint later is a package.json swap, not a code change」と
反転コストの低さを明示している。実際の根拠は次の 2 つ:

1. **NFR-DEP-001** — typescript-eslint 一式は 4-5 個の npm 依存追加と `make lint` の実行時間増を伴う。
2. **部分集合性** — type-aware lint rule が捕らえる交差点は、API 形状 + 型分割が捕らえる集合の部分集合である。

Biome 側の `nursery.noRestrictedImports` を採らない理由は module specifier 照合が相対 path の深さで揺れる脆さ。
同じ判定を安定に書ける fitness test を選ぶ。

### 実装者へ委任する選択 (implementation discretion)

| id | 委任内容 | carrier unit |
|---|---|---|
| `discretion-upstream-parse-fast-reject` | compound key でない値への JSON parse 回避の書き方 (先頭文字判定 / try-catch / 正規表現) | connection-egress-intent-api |
| `discretion-drop-record-dedup-structure` | drop counter と notification dedup の保持構造、getter の戻り形 | connection-egress-intent-api |
| `discretion-requalification-memo` | 再修飾 memo の構造 (Map / 直近 1 件キャッシュ / 事前計算) | connection-ingress-requalification |
| `discretion-envelope-builder-shape` | 型と builder を分けるか 1 関数にするか、集合を Set か配列定数か | ipc-envelope-producer-null-body |
| `discretion-gate-scan-implementation` | allowlist 走査の実装と合成違反 source の作り方 | identity-ownership-gate |

いずれも private・単一 unit の files に閉じ、可逆で、保存 contract と機械検証を参照できる。
escalate 条件は spine.yaml の各 discretion に記載。

### Test seam の所在

| 外部依存 / 境界 | seam |
|---|---|
| WebSocket transport | `Connection` の `wsFactory` 注入 (既存)。intent API は fake conn で観測できる |
| HTTP fetch | `makeSessionsApi(fetchImpl)` / `makeWorkspaceApi(fetchFn)` / `makeFleetProvider(fetchFn)` の注入 (既存) |
| Electron IPC (main 側) | `toResponseEnvelope(response)` を pure function に切り出す (`ipcMain.handle` は import 時登録で直接 test できない) |
| Electron IPC (実行時 bridge) | `electron` を stub して `electron-preload.cjs` を require する (先例: `tests/app-entry.test.ts`) |
| Platform `Response` の null-body 規則 | node environment を file 単位で選ぶ (`// @vitest-environment node`)。happy-dom は寛容で判別できない |
| source tree | `src/__meta__/` の fitness test (既存 pattern) |

### 制約

- lint は Biome のまま。新規 runtime 依存を追加しない。
- wire protocol (frame 形状・kind) と `servers.json` 契約は変更しない。
- `wire/server.ts` の `SessionInfo` は Go 生成 fixture (`wire/testdata/*.json`) を decode できる形を保つ。
- `clients/workspace` は `clients/ui` の source を import できない (`rootDir: src` / package 分離)。
  一致は platform oracle または runtime test で担保する。
- 互換 fallback / dual decoder / degradation を導入しない。
- coverage 閾値: `src/{api,socket,wire}/**` は lines 86 / statements 81 / functions 80 / branches 70、
  `src/{store,hooks}/**` は 92 / 90 / 93 / 83 (`clients/ui/vitest.config.ts` は
  `test-harness/protected.json` の tamper 保護対象)。
- fake は real より寛容にしない。乖離時は fake を直す。
- file 500 行 / function 80 行を責務分割の目安とする。
