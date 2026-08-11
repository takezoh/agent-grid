---
change: change-20260811-fleet-session-id-ssot
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

## Content

本 change の完了は「3 本の RED が GREEN」ではなく、次の 3 点で判定する。

1. 交差点の追加が compile 時または gate で落ちること。
2. data plane の失敗が reason 別に観測可能であること (無音でない、かつ良性遷移を誤報しない)。
3. envelope の不変条件が real 側 (builder + 実行時 bridge) で検証されていること。

### 成功条件と verification の対応 (T0-T2 / spine と 1:1)

| verification id | tier | 対象 contract | 既存 RED / 新規 | 対応 AC | 実行コマンド |
|---|---|---|---|---|---|
| `connection-fleet-input-outbound` | T1 | contract-wire-id-egress | **既存 RED** (intent API 形へ書き換え) | AC-01, AC-02 | `cd clients/ui && npx vitest run src/socket/connection.fleet-input.test.ts` |
| `terminal-pane-intent-api` | T1 | contract-wire-id-egress | 新規 (既存 test 群の追随を含む) | AC-02 | `cd clients/ui && npx vitest run src/components` |
| `retry-path-uses-private-serialization` | T1 | contract-wire-id-egress | 新規 | AC-02 | `cd clients/ui && npx vitest run src/socket/retry.test.ts` |
| `raw-frame-escape-hatch-closed` | T2 | contract-wire-id-egress | 新規 | AC-02 | `cd clients/ui && npm run typecheck` |
| `dataplane-no-throw-on-serverid-mismatch` | T1 | contract-dataplane-unresolved-drop | 新規 | AC-03 | `cd clients/ui && npx vitest run src/socket` |
| `dataplane-drop-counter-by-reason` | T1 | contract-dataplane-unresolved-drop | 新規 | AC-03 | `cd clients/ui && npx vitest run src/socket` |
| `dataplane-decode-failed-notifies-once` | T1 | contract-dataplane-unresolved-drop | 新規 | AC-03 | `cd clients/ui && npx vitest run src/socket` |
| `inbound-requalification-all-kinds` | T1 | contract-wire-id-ingress | 新規 | AC-04 | `cd clients/ui && npx vitest run src/socket` |
| `activity-session-id-requalified` | T1 | contract-wire-id-ingress | 新規 | AC-04 | `cd clients/ui && npx vitest run src/store/__tests__/workspaceActivity.test.ts` |
| `requalification-memoized-per-session` | T1 | contract-wire-id-ingress | 新規 | AC-10 | `cd clients/ui && npx vitest run src/socket` |
| `resize-frame-reaches-wire` | T1 | contract-egress-activation-compatibility | 新規 | AC-12 | `cd clients/ui && npx vitest run src/components` |
| `control-sequence-passthrough-verbatim` | T1 | contract-egress-activation-compatibility | 新規 | AC-12 | `cd clients/ui && npx vitest run src/socket` |
| `wire-fixture-roundtrip-unchanged` | T1 | contract-session-identity-types | 既存 (回帰保護) | AC-06 | `cd clients/ui && npx vitest run src/wire/codec.test.ts` |
| `store-row-requires-authority` | T2 | contract-session-identity-types | 新規 | AC-06 | `cd clients/ui && npm run typecheck` |
| `identity-types-pure` | T0 | contract-session-identity-types | 新規 | AC-06 | `cd clients/ui && npx vitest run src/fleet/session-identity.test.ts` |
| `resolver-precedence-row-over-decode` | T0 | contract-http-route-resolution | 新規 | AC-05 | `cd clients/ui && npx vitest run src/fleet/session-identity.test.ts` |
| `unresolved-issues-no-request` | T1 | contract-http-route-resolution | 新規 | AC-05 | `cd clients/ui && npx vitest run src/api` |
| `pinned-handle-not-routed` | T1 | contract-http-route-resolution | 新規 | AC-05 | `cd clients/ui && npx vitest run src/api/workspace.test.ts` |
| `four-api-modules-single-policy` | T2 | contract-http-route-resolution | 新規 | AC-05 | `cd clients/ui && npx vitest run src/api` |
| `strict-equality-after-ingress` | T1 | contract-store-identity-equality | 新規 | AC-04 | `cd clients/ui && npx vitest run src/store` |
| `alias-removal-order-constraint` | T1 | contract-store-identity-equality | 新規 | AC-04 | `cd clients/ui && npx vitest run src/store/__tests__/workspaceActivity.test.ts` |
| `reconnect-bell-suppression-preserved` | T1 | contract-store-identity-equality | 既存 (回帰保護) | AC-04 | `cd clients/ui && npx vitest run src/store/statusSignals.test.ts` |
| `envelope-null-body-status-mapping` | T1 | contract-ipc-null-body-envelope | 新規 | AC-07 | `cd clients/workspace && npx vitest run tests/http-envelope.test.ts` |
| `envelope-platform-oracle-guard` | T2 | contract-ipc-null-body-envelope | 新規 | AC-07 | `cd clients/workspace && npx vitest run tests/http-envelope.test.ts` |
| `empty-200-body-not-null` | T1 | contract-ipc-null-body-envelope | 新規 | AC-07 | `cd clients/workspace && npx vitest run tests/http-envelope.test.ts` |
| `provider-null-body-status` | T1 | contract-host-transport-parity | **既存 RED** (fake を real 準拠へ + AC-08 ケース追加) | AC-07, AC-08 | `cd clients/ui && npx vitest run src/fleet/provider.null-body-status.test.ts` |
| `preload-bridge-passes-null-body-verbatim` | T1 | contract-host-transport-parity | 新規 | AC-07 | `cd clients/workspace && npx vitest run tests/electron-preload-bridge.test.ts` |
| `repository-preparations-null-body-keeps-tracked` | T1 | contract-host-transport-parity | 新規 | AC-07 | `cd clients/ui && npx vitest run src/fleet/provider.test.ts` |
| `provider-status-parity-and-fidelity` | T2 | contract-host-transport-parity | 新規 | AC-11 | `cd clients/ui && npx vitest run src/fleet/provider.status-parity.test.ts` |
| `identity-owner-allowlist` | T2 | contract-identity-ownership-gate | 新規 | AC-09 | `cd clients/ui && npx vitest run src/__meta__/session-identity-ownership.test.ts` |
| `serialize-client-frame-sole-importer` | T2 | contract-identity-ownership-gate | 新規 | AC-02, AC-09 | `cd clients/ui && npx vitest run src/__meta__/session-identity-ownership.test.ts` |
| `gate-fails-on-synthesized-violation` | T2 | contract-identity-ownership-gate | 新規 | AC-09 | `cd clients/ui && npx vitest run src/__meta__/session-identity-ownership.test.ts` |
| `compile-time-boundary-closed` | T2 | contract-identity-ownership-gate | 新規 | AC-02, AC-06 | `cd clients/ui && npm run typecheck` |

### T3 (spine 外)

`clients/ui/e2e/real-stack/terminal-input.spec.ts` — **既存 RED**。AC-01 と AC-12 の end-to-end 観察点。

```sh
cd clients/ui && npm run test:e2e:real
```

GREEN 化は必須。CI 常設は本 change の scope 外 (下記 follow-up)。

### 判別可能性 (discriminating) の要点

- **AC-03 の判別**: 既存 RED 3 本は `cfg.serverId === key.serverId` のケースしか通していないため、
  実装者が `send()` 内で `upstreamSessionID` をそのまま再利用しても 3 本すべて GREEN になる。
  `dataplane-no-throw-on-serverid-mismatch` は serverId **不一致**の `{k:"i"}` で
  「例外 0 / frame 0 / counter 増 / notification 0」を固定する。これが無いと誤実装が緑のまま通る。
- **AC-02 の判別**: `retry.ts` に `serializeClientFrame` の import を 1 行戻すと
  `serialize-client-frame-sole-importer` が落ちること (反証操作) を確認する。
- **AC-08 の判別**: `provider.null-body-status.test.ts` の fake bridge を「旧 main」形
  (`{status:204, body:""}`) にしたケースで renderer が例外を出さないことを assert する。
  fake を real 準拠 (`body: null`) にした主ケースと**両方**残す。片方だけでは
  consumer-first 順序の witness にならない。
- **AC-09 の判別**: gate 自身が合成違反で落ちる negative test を持つ。
  落ちない検出器は偽の緑を作るため、positive assertion だけでは要件を満たさない。
- **AC-12 の判別**: fix 前は frame が gateway へ到達せず PTY が変わらない (debug run の保存済み baseline)。
  fix 後に実寸 (cols 107 / rows 32) の `{k:"r"}` と制御シーケンス (`[3~`) が verbatim で載ることを固定し、
  挙動活性化を退行と誤認しない反証点にする。
- **fake 忠実性**: `provider-status-parity-and-fidelity` は fake が real の envelope builder より
  寛容な値 (204 に `""`) を返した場合に落ちる。fake が落ちたら **fake を直す** — assertion を弱めない。
- **platform oracle**: `envelope-platform-oracle-guard` は null body status 集合の各要素で
  `new Response("", {status})` が throw し、集合外の代表 status では throw しないことを assert する。
  happy-dom はこの不変条件を強制しないので、当該 file は `// @vitest-environment node` を使う。

### 実行 gate

```sh
# clients/ui
cd clients/ui && npm run test:unit
cd clients/ui && npm run typecheck
cd clients/ui && npm run lint

# clients/workspace
cd clients/workspace && npm test
cd clients/workspace && npm run typecheck

# 横断 (cross-cutting change のため)
cd clients/ui && npm run test:web
cd clients/ui && npm run test:e2e:real     # T3、AC-01 / AC-12
```

CI profile 上の位置づけ: `clients/ui` の unit / typecheck / lint と `clients/workspace` の test / typecheck は
`test-harness/profiles.json` の `pr` profile (`web-gate` / `workspace-test` / `workspace-typecheck`) に含まれる。
real-stack e2e は `pr` profile に**含まれておらず**、T3 の常設先は `nightly` profile の
`scripts/run-nightly-e2e-report.sh` のみである。

### coverage

`clients/ui/vitest.config.ts` の閾値を下回らないこと
(`src/{api,socket,wire}/**` = lines 86 / statements 81 / functions 80 / branches 70、
`src/{store,hooks}/**` = 92 / 90 / 93 / 83)。当該 file は `test-harness/protected.json` の tamper 保護対象。

### Closure に必要な evidence

| 成功条件 | evidence |
|---|---|
| 交差点の追加が落ちる | `identity-owner-allowlist` / `serialize-client-frame-sole-importer` / `gate-fails-on-synthesized-violation` / `compile-time-boundary-closed` が緑、かつ negative test が存在する |
| data plane の失敗が観測可能 | `dataplane-*` 3 件が緑 (notification 0 件の assertion を含む) |
| envelope 不変条件が real 側で検証済み | `envelope-*` 3 件 + `preload-bridge-passes-null-body-verbatim` + `provider-status-parity-and-fidelity` が緑 |
| 既存 RED 3 本 | `connection-fleet-input-outbound` / `provider-null-body-status` が緑、real-stack spec が緑 |
| 全交差点 (17) を閉じた | gate の allowlist と所有者集合の一致 |
| governance | gate は `hard`。承認 evidence は host / conductor が取得し change へ記録する |

### Follow-up issue (本 change の closure 条件ではない)

1. gateway が解決不能な session id を無音で捨てる件 (`services/server/api/gateway_lifecycle_inbound.go:124-140`)。
2. `App.tsx:80-85` の `?? "local"` が `ARCHITECTURE.md` の implicit default server 禁止に抵触するかの判断。
   design decision-6 に静的 call graph の実測と falsifier (同一 origin で SPA と gateway を配信する host 構成の追加) を記録済み。
3. `clients/workspace/src/shared/session-ref.ts` の第 2 encoder 実装を canonical fixture で拘束する follow-up change
   (切断点 1 組を design decision-7 に保存)。あわせて real-stack e2e の CI 常設
   (nightly profile への command 追加。`test-harness/profiles.json` は `test-harness/protected.json` の
   保護対象 `required_markers: ["pre-push","nightly"]` なので trusted harness gate を通る) と
   spec 判別子の強化 (`echo: hi`)。

症状 1 (Workspace Electron でターミナルが表示されない) は未帰属のまま。A/B 修正後の Electron 実機再観測まで
`docs/bugs/workspace-electron-null-body-status.md` および元 bug-report の closure を保留する。
