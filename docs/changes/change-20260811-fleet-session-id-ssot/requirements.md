---
change: change-20260811-fleet-session-id-ssot
role: requirements
functional_requirements:
- id: FR-A1
  statement: SPA が送出する全 WS frame の sessionId は upstream (bare) id である。
  priority: must
- id: FR-A2
  statement: Connection の公開 API は fleet key のみを受け取り、ClientFrame の組み立てと直列化は Connection
    の内部に閉じる。
  priority: must
- id: FR-A3
  statement: data plane の id 解決失敗は例外を送出せず frame を破棄し、reason 別に分類された観測可能な記録を残す (内部契約違反はユーザーに到達可能、正常遷移由来は診断可能な
    counter)。
  priority: must
- id: FR-A4
  statement: gateway から到着する全ての session id 保持要素 (output frame[3] / tt / et / activity_session_id
    / activity event の session_id / hello・view の sessions) は store へ渡る前に単一の再修飾経路を通って
    fleet key へ再修飾される。
  priority: must
- id: FR-A5
  statement: store と HTTP 境界の id 解決は単一 resolver が行い、api モジュールは独自の再導出を持たない。
  priority: must
- id: FR-A6
  statement: 解決不能な session id に対する結果は 1 種類の typed failure であり、HTTP 要求は発行されず、その失敗は
    network error へ写像されない。
  priority: must
- id: FR-A7
  statement: store 行は serverId と upstreamSessionId を required で保持し、session 同一性の判定は
    fleet key の厳密一致で行う。
  priority: must
- id: FR-A8
  statement: これまで無音で捨てられていた data plane frame ({k:"r"} と制御シーケンスを含む {k:"i"}) が初めて gateway
    へ到達することを挙動活性化として扱い、活性化が既存セッションの表示を壊さないことを観測で確認する。
  priority: must
- id: FR-B1
  statement: IPC response envelope は本文なしを body null で表し、main は null body status のときだけ
    null を出す。
  priority: must
- id: FR-B2
  statement: FleetProvider.request は browser host と Workspace host で同一の HTTP 交換に対し観察上等価な
    Response を返し、body null を消費する全 consumer が null を安全に扱う。
  priority: must
- id: FR-B3
  statement: envelope 型の変更は consumer-first の順序で適用され、途中状態でも renderer が壊れない。
  priority: must
- id: FR-C1
  statement: identity 変換または wire frame 直列化を新たに行うモジュールが所有者集合の外に増えたとき機械的に失敗する gate が存在し、その
    gate 自身が発火することが示されている。
  priority: must
- id: NFR-PERF-001
  statement: inbound hot path (FR-β07 が kHz を想定する output frame) の 1 frame あたり割り当てを現行から増やさない。再修飾は
    session 単位で memo 化する。
  priority: should
- id: NFR-PERF-002
  statement: 打鍵経路の id 解決は compound key でない値に対して JSON parse を行わない。
  priority: should
- id: NFR-DEP-001
  statement: 新規 runtime 依存を追加せず、lint は Biome のままとする。
  priority: must
- id: NFR-TEST-001
  statement: 追加する fake は real より寛容にしない。fake が real と乖離した場合は fake を直す。
  priority: must
---

<!-- lifecycle is owned by change.md -->

# Requirements

## Content

SPA (`clients/ui`) は fleet セッションを compound key `JSON.stringify([serverId, sessionId])` で扱い、
gateway は bare な upstream id を期待する。両 id 空間が素の `string` であるため、変換の所有者が
境界ごとに定まらず、**17 の交差点**にローカル慣習が積み上がり 8 種類の失敗方針へ分岐していた。
あわせて Electron IPC の response envelope が「本文なし」を表現できず、成功した操作が
network error として誤報されていた。

母数の定義 (3 つの数を混ぜない):

| 数 | 意味 |
|---|---|
| 19 | 上流 horizontal_scan の call site 総数 |
| 15 | helper 名 (`sessionKey` / `parseSessionKey`) を key にして hit した交差点 |
| 17 | invariant を key にした再走査で 2 件 (A-13 / A-14) を加えた**交差点の総数** |

19 − 17 = 2 は helper を呼ぶが id 空間をまたがない call site。以後「全交差点を閉じた」は 17 を母数とする。

### Functional requirements (EARS)

| id | 型 | 要件 |
|---|---|---|
| **FR-A1** | ubiquitous | SPA が送出する全 WS frame の `sessionId` は upstream (bare) id である。 |
| **FR-A2** | ubiquitous | `Connection` の公開 API は fleet key のみを受け取り、`ClientFrame` の組み立てと直列化は `Connection` の内部に閉じる。 |
| **FR-A3** | event_driven | data plane (`{k:"i"}` / `{k:"r"}`) の id 解決に失敗したとき、システムは例外を送出せず frame を破棄し、reason 別に分類された観測可能な記録を残す。内部契約違反 (`decode-failed`) はユーザーに到達可能とし、正常遷移由来 (`server-mismatch`) は診断可能な counter に留める。 |
| **FR-A4** | ubiquitous | gateway から到着する全ての session id 保持要素 (output frame[3] / `tt` / `et` / `activity_session_id` / activity event の `session_id` / hello・view の `sessions`) は、store へ渡る前に**単一の再修飾経路**を通って fleet key へ再修飾される。 |
| **FR-A5** | ubiquitous | store / HTTP 境界の id 解決は単一の resolver が行い、api モジュールは独自の再導出を持たない。 |
| **FR-A6** | event_driven | 解決不能な session id が api layer に渡ったとき、システムは HTTP 要求を発行せず 1 種類の typed failure を返す。その失敗は network error へ写像されない。 |
| **FR-A7** | state_driven | store 行が存在する間、その行は `(serverId, upstreamSessionId)` を required で保持し、session 同一性の判定は fleet key の厳密一致で行う (alias 総当たりを持たない)。 |
| **FR-A8** | event_driven | これまで無音で捨てられていた data plane frame (`{k:"r"}` と制御シーケンスを含む `{k:"i"}`) が初めて gateway へ到達したとき、その挙動活性化が既存セッションの表示を壊さないことが観測で確認できる。 |
| **FR-B1** | ubiquitous | IPC response envelope は「本文なし」を `body: null` で表し、main は null body status のときだけ null を、それ以外では string を出す。 |
| **FR-B2** | ubiquitous | `FleetProvider.request` は browser host と Workspace host で、同一の HTTP 交換に対し観察上等価な `Response` を返す。`body: null` を消費する全 consumer が null を安全に扱う。 |
| **FR-B3** | state_driven | envelope 型の移行中 (renderer が新しく main が古い状態) でも renderer は例外にならない。 |
| **FR-C1** | ubiquitous | identity 変換または wire frame 直列化を新たに行うモジュールが所有者集合の外に増えたとき機械的に失敗する gate が存在し、その gate 自身が合成違反で発火することが示されている。 |

**invariant requirement**: FR-A1 / FR-A2 / FR-A4 / FR-A5 / FR-A7 / FR-B1 / FR-B2 / FR-C1 は
ubiquitous / state_driven であり、入口 (最初の打鍵、最初の inbound frame、最初の routed request) の
観察を含めて常時成立する。event 発火時だけの assertion で満たしたとみなしてはならない。

### Non-functional requirements

- **NFR-PERF-001** inbound hot path (FR-β07 が kHz を想定する output frame) の 1 frame あたり割り当てを
  現行 (`sessionKey()` 1 回) から増やさない。再修飾は session 単位で memo 化する。
- **NFR-PERF-002** 打鍵経路の id 解決は、compound key でない値に対して JSON parse を行わない。
- **NFR-DEP-001** 新規 runtime 依存を追加せず、lint は Biome のままとする。
- **NFR-TEST-001** 追加する fake は real より寛容にしない。fake が real と乖離した場合は fake を直す。

### 挙動変更 (UX delta)

1. **P5 の撤去** — これまで 404 になっていた transcript / workspace 要求が、要求を出す前に typed failure で
   返る。ユーザーから見える差は「同じ失敗が速くなる」だけで、成功していた操作が失敗に転じることはない。
   この断定の前提 (`App.tsx:80-85` の `?? "local"` が id 空間に入らないこと) は design の decision-6 に
   静的 call graph の実測として記録されている。falsifier: 同一 origin で SPA と gateway を配信する host 構成の追加。
2. **data plane の drop 記録** — server 切替の窓で起きる `server-mismatch` は **通知を出さない** (counter のみ)。
   通知を出すのは内部契約違反 (`decode-failed`) だけ。良性の遷移を error として誤報しない。
3. **挙動活性化 (FR-A8)** — fix 後に初めて `{k:"r"}` が実寸 (debug run 実測 cols 107 / rows 32) で PTY へ届き、
   制御シーケンス (`[3~` = Delete) が初めて PTY へ届く。これは退行ではなく修正の効果であり、
   両者を判別する反証点を acceptance に持つ。
4. **Workspace の 204 / 304** — 停止・既読マーク・ログのポーリングが成功し、成功した操作が
   network error として報告されなくなる。

### Acceptance criteria

| id | criteria | requirement |
|---|---|---|
| **AC-01** | fleet-qualified なセッションで SPA の terminal に打鍵すると、gateway に upstream (bare) id を持つ `{k:"i"}` が届き、PTY が入力を受け取ってエコーが画面に現れる。real-stack e2e (T3) で end-to-end に観測できる。 | FR-A1, FR-A2 |
| **AC-02** | production source (`clients/ui/tsconfig.json` の include 範囲) において、`ClientFrame` の組み立てと `serializeClientFrame` の呼び出しが `clients/ui/src/socket/connection.ts` の外に存在しない。`retry.ts` に import を 1 行戻すと unit suite が落ちる。 | FR-A2, FR-C1 |
| **AC-03** | `cfg.serverId` と一致しない serverId を持つ key で `{k:"i"}` を 50 回送っても例外は 1 度も出ず、frame は 1 件も出ず、`server-mismatch` counter が 50 になり notification は **0 件**。decode 不能な値では `decode-failed` counter が増え notification は 1 件だけ増える。 | FR-A3 |
| **AC-04** | gateway から届く `activity_session_id` と activity event の `session_id` が fleet key に再修飾されて store に渡り、alias 総当たりなしで activity drawer の scope 照合が成立する。再接続 hello の bell 抑止も保たれる。 | FR-A4, FR-A7 |
| **AC-05** | 解決不能な session id を api layer に渡しても HTTP 要求は 1 件も発行されず、呼び出し側は 1 種類の typed failure を受け取る。その失敗は `Error("network")` として観察されない。4 モジュールで挙動が同一であり、`handle_session` query param は resolver を通らない。 | FR-A5, FR-A6 |
| **AC-06** | production source において権威 (`serverId` / `upstreamSessionId`) を持たない store 行はコンパイルできず、`SessionInfo` から client 専用 field が消えており、Go 生成 fixture の wire round-trip は引き続き緑である。 | FR-A7 |
| **AC-07** | Workspace で停止・既読マーク・ログのポーリングが成功し、成功した操作が network error として報告されない。204 と 304 が例外にならず status を保って renderer に届き、envelope の `body: null` が preload を coercion なしで素通しし、repository-preparations tracker が null で潰れない。 | FR-B1, FR-B2 |
| **AC-08** | renderer と main の envelope の対が崩れた中間状態 (renderer が新しく main が古い) でも renderer は例外にならない。判定媒体は `provider.null-body-status.test.ts` の legacy-empty-body ケース。 | FR-B3 |
| **AC-09** | 所有者集合の外から identity 変換 helper または `serializeClientFrame` を import すると unit suite が落ち、合成した違反 source に対して gate が実際に失敗することが negative test で示され、gate が検出できない範囲 (暗黙依存 / test tree / `clients/workspace`) が test 自身に明記されている。 | FR-C1 |
| **AC-10** | 打鍵経路で compound key でない値に対する JSON parse が起きず、output frame の再修飾割り当てが frame 数に比例しない。 | NFR-PERF-001, NFR-PERF-002 |
| **AC-11** | 新規 runtime 依存が増えておらず、lint は Biome のままで、fake が real より寛容だと status parity / fidelity test が落ちる。 | NFR-DEP-001, NFR-TEST-001 |
| **AC-12** | fix 後に `{k:"r"}` が SPA の実寸 (cols 107 / rows 32) で wire に出て gateway に到達し、制御シーケンス (`[3~`) が verbatim で載る。既存セッションの scrollback が壊れない。 | FR-A8 |

### Non-goals (この change で満たさない要件)

- 症状「Workspace Electron でターミナルが表示されない」の修正 (未帰属。Electron 実機観測待ち)
- gateway が解決不能な session id を無音で捨てる挙動の変更
- wire protocol (frame 形状・kind) の変更
- fleet 構成 / `servers.json` 契約の変更
- branded type (`FleetSessionKey` / `UpstreamSessionId`) の全面導入
- `clients/workspace/src/shared/session-ref.ts` の第 2 encoder 実装の fixture 拘束 (follow-up issue)
- real-stack e2e の CI 常設 (follow-up issue)
