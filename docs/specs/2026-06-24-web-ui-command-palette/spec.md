---
id: spec-20260624-2026-06-24-web-ui-command-palette
kind: spec
title: Spec — Web UI Command Palette
status: draft
created: '2026-06-24'
updated: '2026-07-04'
tags:
- spec
- legacy-import
owners: []
relations:
- {type: referencedBy, target: adr-20260624-0036-palette-2phase-store-architecture}
- {type: referencedBy, target: adr-20260624-0037-palette-hotkey-capture-phase}
- {type: referencedBy, target: adr-20260624-0038-palette-fuzzy-pure-function}
- {type: referencedBy, target: adr-20260624-0039-palette-focus-trap-minimal}
- {type: referencedBy, target: adr-20260624-0040-palette-ime-suppression-in-store}
- {type: referencedBy, target: adr-20260624-0041-palette-session-config-rest-extension}
- {type: referencedBy, target: adr-20260624-0042-palette-new-session-payload-wire-mirror}
- {type: referencedBy, target: adr-20260624-0043-palette-createsessionform-replacement}
- {type: referencedBy, target: adr-20260624-0044-palette-no-per-session-occupant}
- {type: referencedBy, target: adr-20260624-0045-palette-push-route-sendcommand}
- {type: referencedBy, target: adr-20260624-0046-palette-push-active-mismatch-409}
- {type: referencedBy, target: adr-20260624-0047-palette-disabledreason-single-source}
- {type: implementedBy, target: plan-20260624-2026-06-24-web-ui-command-palette}
- {type: referencedBy, target: plan-20260624-2026-06-24-web-ui-command-palette}
- {type: references, target: adr-20260624-0036-palette-2phase-store-architecture}
- {type: references, target: adr-20260624-0037-palette-hotkey-capture-phase}
- {type: references, target: adr-20260624-0038-palette-fuzzy-pure-function}
- {type: references, target: adr-20260624-0039-palette-focus-trap-minimal}
- {type: references, target: adr-20260624-0040-palette-ime-suppression-in-store}
- {type: references, target: adr-20260624-0041-palette-session-config-rest-extension}
- {type: references, target: adr-20260624-0042-palette-new-session-payload-wire-mirror}
- {type: references, target: adr-20260624-0043-palette-createsessionform-replacement}
- {type: references, target: adr-20260624-0044-palette-no-per-session-occupant}
- {type: references, target: adr-20260624-0045-palette-push-route-sendcommand}
- {type: references, target: adr-20260624-0046-palette-push-active-mismatch-409}
- {type: references, target: adr-20260624-0047-palette-disabledreason-single-source}
- {type: references, target: plan-20260624-2026-06-24-web-ui-command-palette}
source_paths:
- src/client/web/
- src/server/web/mux.go
- src/client/web/src/components/palette/
- src/client/web/src/store/
- src/client/web/src/lib/tools.ts
functional_requirements: []
non_functional_requirements: []
acceptance: []
---

<!-- migrated_from: docs/specs/2026-06-24-web-ui-command-palette/spec.md -->

# Spec — Web UI Command Palette

- **作成日**: 2026-06-24
- **ブランチ**: `main`
- **plan**: [plan.md](../../specs/2026-06-24-web-ui-command-palette/plan.md)
- **ADRs**: [0036](../../adr/adr-20260624-0036-palette-2phase-store-architecture.md), [0037](../../adr/adr-20260624-0037-palette-hotkey-capture-phase.md), [0038](../../adr/adr-20260624-0038-palette-fuzzy-pure-function.md), [0039](../../adr/adr-20260624-0039-palette-focus-trap-minimal.md), [0040](../../adr/adr-20260624-0040-palette-ime-suppression-in-store.md), [0041](../../adr/adr-20260624-0041-palette-session-config-rest-extension.md), [0042](../../adr/adr-20260624-0042-palette-new-session-payload-wire-mirror.md), [0043](../../adr/adr-20260624-0043-palette-createsessionform-replacement.md), [0044](../../adr/adr-20260624-0044-palette-no-per-session-occupant.md), [0045](../../adr/adr-20260624-0045-palette-push-route-sendcommand.md), [0046](../../adr/adr-20260624-0046-palette-push-active-mismatch-409.md), [0047](../../adr/adr-20260624-0047-palette-disabledreason-single-source.md)

## Goal

agent-reactor の Web UI に、2 フェーズ操作 (ファジー検索でツール選択 → パラメータ順次入力) を持つコマンドパレットを Cmd/Ctrl+K + 常設ボタンで起動できるモーダルとして実装する。standard スコープ (new-session / stop-session) と push スコープ (push_commands 配列) をセグメント切替で同居させ、push 送信経路 (POST /api/sessions/{id}/push) を本 spec で新設し、CreateSessionForm はパレット new-session に一本化して撤去する。push 可否判定は既存の daemon-global ActiveSessionID + ActiveOccupant を流用し、SessionInfo proto / wire への per-session occupant 追加は行わない。ADR-0030 (TerminalPane subscribe 唯一所有) / ADR-0033 (displayLabel 空文字=空) / WS は I/O 専用の既存規約を破らないことを設計制約とする。

## Scope

### In Scope

- src/client/web 配下のパレット UI コンポーネント (CommandPalette / 検索 input / listbox / paramSelect / scope segment / ヘッダーバー / 戻る・閉じる button)
- Cmd/Ctrl+K の document capture-phase ハンドラ + 常設『Command (⌘K / Ctrl+K)』ボタン (Header 既存 New Session ボタンの再配線含む)
- xterm blur 戦略: パレット open 時にアクティブ TerminalPane.blur() を呼び、close 時に直前 focus へ復帰 (subscribe/unsubscribe は発行しない)
- ファジー検索純関数 (items × query → ranked + match ranges) を web/src/lib/fuzzy.ts に新設、sahilm/fuzzy 相当の挙動
- phaseToolSelect / phaseParamSelect 2 フェーズ state machine (Zustand store: store/palette.ts) — DOM 操作は持たず純粋 state のみ
- ToolRegistry (lib/tools.ts): 宣言的 ToolDef 配列で standard 2 (new-session / stop-session) + push 動的展開を定義、disabledReason(daemonSnapshot) と submit(ctx) を ToolDef に集約
- new-session の条件付きトグル: projectIsGit gating で worktree (Tab)、projectIsSandboxed gating で host (Shift+Tab)
- 送信前の対象再検証 (active session 失効 → toast + close)、push 失効再検証は ToolDef.disabledReason の 1 関数を ScopeSegment / submit の双方で共有
- HTTP POST /api/sessions/{id}/push の Go 側 route 新設 (src/server/web/mux.go) と既存 SendCommand(ctx, proto.CmdEvent{Event: state.EventPushDriver, ...}) パターンによる daemon RPC 発行
- GET /api/session-config の apiSessionConfig 拡張: push_commands []string を追加 + projects を {path, isGit, isSandboxed} 配列に拡張し、projectIsGit / projectIsSandboxed gating の判定源を server から配信
- CreateSessionForm.tsx / CreateSessionForm.test.tsx の撤去と App.tsx の差し替え、New Session ボタンはパレット new-session 起動に再配線
- NotificationToast 連携 (成功/失敗/失効/認証失敗 toast)、focus 復帰、IME composition 抑止、focus trap、role=dialog/listbox/option aria
- Vitest テスト (CommandPalette / fuzzy / palette store / 起動キー / xterm focus 中の hotkey 発火 / push 失効再検証 / scope disabledReason / IME 抑止 / hotkey 連打冪等性 / Tab vs focus 巡回ルール) と Go テスト (POST /api/sessions/{id}/push の正常 + 404 + 409 + 401 / GET /api/session-config 拡張 fields)

### Out of Scope

- detach / create-project / shutdown / 監視系ツールの Web 露出 (送信経路未整備)
- activeSessionID の client 単独管理化の正式リファクタ (store 改修は別 spec、本 spec は new-session 成功時の setActive を従来挙動のまま流用)
- fuse.js 等のファジー検索ライブラリ追加 (依存ゼロ純関数で実装)
- WS 経由の mutating フレーム種別追加 (push は HTTP に寄せる)
- Go proto SessionInfo / TS wire SessionInfo への per-session occupant フィールド追加 (本 spec では行わない)
- WS view-update フレームへの session-scoped occupant / push 関連フィールドの新設
- '(no label)' 等の displayLabel 規約二重化 (ADR-0033 維持)
- push_commands の個数上限ガード / ページング (現実的な数を想定、観測のみ)

## Requirements (EARS)

> EARS = Easy Approach to Requirements Syntax。`ubiquitous` (常に成立) / `event_driven` (X したとき) / `state_driven` (X の間) / `unwanted` (X したら) / `optional` / `complex` の型を持つ。

- **FR-001** *(event_driven)* — ユーザーが Cmd+K (macOS) または Ctrl+K (other) を押した時、システムは document の capture phase でキーを横取りし、フォーカスが xterm にある場合でもコマンドパレットを開かなければならない。
  - *Rationale*: xterm.js は textarea で keydown を消費するため bubble phase では奪われる。capture phase で先取りすることで ADR-0030 (TerminalPane subscribe 唯一所有) を破らずに hotkey を成立させる。
- **FR-002** *(ubiquitous)* — システムは Header に常設の『Command (⌘K / Ctrl+K)』ボタンを表示し、クリックでコマンドパレットを開かなければならない。
  - *Rationale*: Firefox の Ctrl+K (検索バー) など preventDefault 不能環境の保険として常設ボタンを露出する。
- **FR-003** *(event_driven)* — コマンドパレットが開いた時、システムは検索 input にフォーカスを移し focus trap を有効化し、アクティブ TerminalPane を blur しなければならない。
  - *Rationale*: 起動後に xterm が keydown を奪い続けると palette 操作が成立しない。focus trap で dialog 内に閉じ込める。
- **FR-004** *(event_driven)* — コマンドパレットが開いた時、アクティブセッションが存在し daemon-global ActiveOccupant が 'frame' である場合、システムは push スコープを既定選択しなければならない。それ以外の場合、システムは standard スコープを既定選択しなければならない。
  - *Rationale*: push 可否は per-session occupant ではなく既存の daemon-global ActiveOccupant を使う (per-session occupant モデルが daemon に存在しないため)。
- **FR-005** *(unwanted)* — もしアクティブセッションが存在しないなら、システムは push スコープセグメントを disabled にし、サブテキスト『アクティブセッションなし』を表示しなければならない。
  - *Rationale*: 選択不可な scope の理由をユーザに明示する。
- **FR-006** *(unwanted)* — もしアクティブセッションが存在し ActiveOccupant が 'frame' でないなら、システムは push スコープセグメントを disabled にし、サブテキスト『push 対象 driver なし』を表示しなければならない。
  - *Rationale*: 選択不可な scope の理由をユーザに明示する。
- **FR-007** *(state_driven)* — phaseToolSelect である間、システムは role=listbox / option の構造で候補を表示し、aria-activedescendant で現在選択を伝えなければならない。
  - *Rationale*: スクリーンリーダ / キーボード操作の標準準拠。
- **FR-008** *(event_driven)* — 検索 input への入力が発生した時、システムは候補を fuzzy filter しランクトップを選択状態にし、マッチ位置を <mark> でハイライト描画しなければならない。
  - *Rationale*: sahilm/fuzzy 相当の体験。ハイライト range の消費は ToolSelectPhase に限定 (ParamSelectPhase は使わない)。
- **FR-009** *(event_driven)* — ↑/↓ または Ctrl+P/Ctrl+N が押された時、システムは候補カーソルを移動しなければならない。Enter が押された時、システムは選択ツールを確定し phaseParamSelect に遷移しなければならない。
  - *Rationale*: command palette の業界標準キーバインド。
- **FR-010** *(event_driven)* — 選択されたツールが ParamDef を持たない場合、Enter による確定で即座に submit を実行しなければならない。
  - *Rationale*: paramless push コマンドは確定即送信。
- **FR-011** *(state_driven)* — phaseParamSelect である間、システムは選択ツールの必須パラメータを縦並びで全表示し、入力済みフィールドを画面に残さなければならない。
  - *Rationale*: 後戻りせずに入力状況を一覧確認できる UX。
- **FR-012** *(event_driven)* — ParamDef.options が non-null の時、システムはそのフィールドを listbox 表示にし、options が null の時は text input 表示にしなければならない。自由入力フィールドでは候補 0 件でも Enter を有効にしなければならない。
  - *Rationale*: 選択式と自由入力を ParamDef で宣言的に切り替える。
- **FR-013** *(optional)* — 選択された project の isGit フラグが true である場合、システムは command フィールド上で Tab が押された時に worktree トグルを切り替えなければならない。
  - *Rationale*: git リポジトリでのみ worktree 概念が成立。
- **FR-014** *(optional)* — 選択された project の isSandboxed フラグが true である場合、システムは command フィールド上で Shift+Tab が押された時に host トグルを切り替え、ON 時の送信ペイロードに sandbox: 'host' を含めなければならない。
  - *Rationale*: sandbox 対象 project でのみ host 切替 UI が出る。送信形式は既存 POST /api/sessions の wire (sandbox?: 'host') に合わせる。
- **FR-015** *(unwanted)* — もし project の isGit が false なら、システムは worktree トグルを UI に出さず送信ペイロードに含めてはならない。もし isSandboxed が false なら、システムは host トグルを UI に出さず sandbox フィールドを送信ペイロードに含めてはならない。
  - *Rationale*: 条件付きトグルの正規化。
- **FR-016** *(event_driven)* — command フィールド以外にフォーカスがある時、システムは Tab / Shift+Tab を focus trap の標準巡回として動作させなければならない。command フィールド上に限り、システムは Tab を worktree トグル / Shift+Tab を host トグルとして処理しなければならない。
  - *Rationale*: FR-013/014 と focus trap (NFR の a11y) の衝突を回避する明示ルール。
- **FR-017** *(event_driven)* — phaseParamSelect で Esc が押された時、システムは phaseToolSelect に戻らなければならない。phaseToolSelect で Esc が押された時、システムはパレットを閉じ起動前 focus (opener) に focus を戻さなければならない。
  - *Rationale*: 段階的後退と完全離脱を分ける。
- **FR-018** *(ubiquitous)* — システムはヘッダーの戻る (←) ボタンを Esc 相当 (phase 後退 or close) として、閉じる (×) ボタンとオーバーレイ外側クリックを常に close として動作させなければならない。
  - *Rationale*: マウス/タッチ操作でも同等の離脱経路を提供。
- **FR-019** *(state_driven)* — IME composition 中 (compositionstart〜compositionend) である間、システムはパレットの Enter / ↑↓ / Ctrl+P/N キーハンドラを抑止しなければならない。
  - *Rationale*: 日本語入力中に変換確定 Enter で誤送信を防ぐ。store.composing の単一フラグで集約。
- **FR-020** *(state_driven)* — submitting である間、システムは listbox と確定ボタンを disabled にしインライン進行表示を出さなければならない。
  - *Rationale*: 二重送信防止と UX フィードバック。
- **FR-021** *(event_driven)* — new-session の submit が実行された時、システムは POST /api/sessions { project, command, worktree?, sandbox? } を発行しなければならない。成功時、システムは activeSessionID を新セッションに設定し成功 toast を表示し対象 terminal に focus を戻しパレットを閉じなければならない。
  - *Rationale*: 従来の CreateSessionForm 経路の置換。setActive は既存 store の従来挙動を流用 (本 spec で client 単独管理の正式化はしない)。
- **FR-022** *(event_driven)* — stop-session の submit が実行された時、システムは選択された対象セッションに対し DELETE /api/sessions/{id} を発行し、成功時に成功 toast を表示し対象が active なら active を解除しなければならない。
  - *Rationale*: stop-session の対象は ParamDef.options = store/daemon.sessions[] の listbox から選ぶ (getText は displayLabel)。
- **FR-023** *(event_driven)* — push の submit が実行された時、システムは送信直前に ToolDef.disabledReason(daemonSnapshot) を再評価し、null でない場合はエラー toast を出し HTTP を発行せずパレットを閉じ opener に focus を戻さなければならない。null の場合のみシステムは POST /api/sessions/{id}/push body {command} を発行しなければならない。
  - *Rationale*: push 失効再検証を 1 関数に集約 (ScopeSegment の disabled 判定と submit 直前の race 検出を共有)。close 時の focus 復帰経路を明示。
- **FR-024** *(unwanted)* — もし HTTP レスポンスが 4xx/5xx もしくは fetch が失敗したなら、システムは入力内容を保持したままパレットを開き続けエラー toast を表示しなければならない。ただし 401 の場合は『認証エラー (再ログインしてください)』toast を出してパレットを閉じなければならない。
  - *Rationale*: 再送可能性を保ちつつ、認証期限切れだけは再認証フローへ誘導する。
- **FR-025** *(event_driven)* — POST /api/sessions/{id}/push を受信した時、Go サーバは body {command:string} を読み、id が daemon-global ActiveSessionID と一致する場合のみ proto.CmdEvent{Event: state.EventPushDriver, Payload: state.PushDriverParams{SessionID: id, Command: command}} を SendCommand 経由で発行しなければならない。
  - *Rationale*: handleCreateSession と同形の SendCommand パターンを再利用し、daemon_client.go の既存経路を流用する。
- **FR-026** *(unwanted)* — もし POST /api/sessions/{id}/push の id が daemon-global ActiveSessionID と一致しないなら、Go サーバは 409 Conflict を返さなければならない。もし id に対応するセッションが存在しないなら、Go サーバは 404 Not Found を返さなければならない。もし認証ミドルウェアが拒否したなら、Go サーバは 401 Unauthorized を返さなければならない。
  - *Rationale*: race / 競合 / 認可失敗を HTTP ステータスで区別。daemon-global active との照合は同一 web gateway が SubscribeEvents から保持している ActiveSessionID と body の id を比較して判定する。
- **FR-027** *(ubiquitous)* — GET /api/session-config のレスポンスは projects (各要素 {path:string, isGit:bool, isSandboxed:bool}) と commands ([]string) と push_commands ([]string) を返さなければならない。
  - *Rationale*: projectIsGit / projectIsSandboxed gating の判定源と push スコープのツール列挙源を 1 つの REST エンドポイントに集約。WS フレームを増やさず ADR-0030 (WS は I/O 専用) を守る。
- **FR-028** *(ubiquitous)* — システムは standard パレットの初期ツール列挙から shutdown を除外しなければならない。
  - *Rationale*: 現状 Web 経路の送信対象に含めない判断。
- **FR-029** *(event_driven)* — コマンドパレットが既に開いている状態で Cmd/Ctrl+K が再度押された時、システムは現在の phase / paramValues / query を保持したまま検索 input に focus を戻すだけにし、状態をリセットしてはならない。
  - *Rationale*: 連打による入力消失を防ぐ冪等性の明示。

## Open Questions

> 設計判断ではなく plan-impl 段階で grep / 観察により確定する implementation-time の確認事項。

- GET /api/session-config の projects 要素拡張 ({path, isGit, isSandboxed}) は claude-app-server の /api/session-config 利用箇所に影響しないか — 現時点で apiSessionConfig は web 専用エンドポイントだが、設定回りの helper 共有がある場合は確認が必要 (plan 実装着手前に grep で確認、影響あれば backward-compatible に保つ)
- isSandboxed の判定源 — config.Session の sandbox 対象 list と project path のマッチングを mux.go で行うか、platform 側に helper を切り出すか (plan-impl で実装位置を確定する)
- /api/session-config の再 fetch タイミング — palette open 毎に再 fetch するか、App 起動時 + 任意の reload トリガで足りるか。push_commands は config 編集時に変わるが、変更通知の仕組みは現状なし (plan-impl で観察ベースで決める)
- stop-session の対象 listbox の getText (displayLabel か title か id か) — ADR-0033 の displayLabel 純関数を再利用する想定だが、ParamDef.options の getText 指定方法は ToolRegistry 実装時に確定する
- command 大きさ上限 — push の command body に巨大文字列が来たとき mux.go で 4xx を返すかどうかの上限値 (plan-impl で既存 mux のリミットに準ずる)

## Resolved Issues (plan-how 統合役による収束)

否定役 (critic) と最適化役 (optimizer) が指摘した論点と、その解決内容。

- **Issue**: [blocker] SessionInfo.occupant per-session フィールドの意味論が daemon に存在しない (否定役)
  - **Resolution**: per-session occupant 追加を撤回 (ADR-20260624-palette-no-per-session-occupant)。push 可否判定は既存の daemon-global ActiveSessionID + ActiveOccupant を入力源とする (FR-004/005/006)。proto / wire の鏡像改修ゼロ。
- **Issue**: [blocker] push_commands list が web に配信されていない (否定役)
  - **Resolution**: GET /api/session-config の apiSessionConfig に push_commands を追加 (ADR-20260624-palette-session-config-rest-extension, FR-027)。WS フレームには載せず REST 拡張に閉じる。
- **Issue**: [blocker] push route の active 照合キーが未確定 (否定役)
  - **Resolution**: web gateway が SubscribeEvents から保持する daemon-global ActiveSessionID と path id を照合 (ADR-20260624-palette-push-active-mismatch-409, FR-025/026)。client 側は store/daemon snapshot で同じ照合を二段で行う (FR-023)。
- **Issue**: [major] viewUpdateFrame の activeSessionID ドロップ規約と session-scoped occupant の衝突 (否定役)
  - **Resolution**: per-session occupant を導入しないので衝突自体が消える (ADR-20260624-palette-no-per-session-occupant)。
- **Issue**: [major] paramless push 失効時の focus 復帰経路が未明示 (否定役)
  - **Resolution**: FR-023 で『エラー toast + close + opener に focus 復帰』を明示し、close 経路を CommandPalette の unmount → opener.focus() に統一 (ADR-20260624-palette-focus-trap-minimal)。
- **Issue**: [major] xterm focus 中の hotkey 発火 (capture phase 挙動) の担保 (否定役)
  - **Resolution**: in_scope テストに『xterm focus 中の hotkey 発火 (capture phase)』を Vitest 項目として追加 (ADR-20260624-palette-hotkey-capture-phase)。
- **Issue**: [major] SessionInfo Occupant 追加の consumer 影響と source データ欠如 (否定役)
  - **Resolution**: そもそも追加しない判断に変更 (ADR-20260624-palette-no-per-session-occupant)。
- **Issue**: [major] activeSessionID 設定の out-of-scope と new-session 成功時 setActive の矛盾 (否定役)
  - **Resolution**: new-session 成功時の setActive は既存 store の従来挙動を流用するに留め、activeSessionID の client 単独管理化の正式リファクタは out_of_scope のまま (FR-021 で明示)。
- **Issue**: [major] stop-session 対象選択の paramField 未定義 (否定役)
  - **Resolution**: FR-022 で『ParamDef.options = store/daemon.sessions[]、getText は displayLabel』として明示 (open_questions に getText の最終仕様確認 1 項目を残す)。
- **Issue**: [major] projectIsGit / projectIsSandboxed の判定源欠如 (否定役)
  - **Resolution**: GET /api/session-config の projects を {path, isGit, isSandboxed} 配列に拡張 (FR-027, ADR-20260624-palette-session-config-rest-extension)。
- **Issue**: [minor] hotkey 連打時の冪等性未定義 (否定役)
  - **Resolution**: FR-029 で『既に open なら phase/paramValues/query を保持し search input に focus 戻すのみ』を明示 (ADR-20260624-palette-hotkey-capture-phase)。
- **Issue**: [minor] Tab vs focus trap の衝突 (否定役)
  - **Resolution**: FR-016 で『command フィールド上に限り Tab=worktree / Shift+Tab=host、他フィールドは標準巡回』として明示。
- **Issue**: [minor] WS は I/O 専用と occupant 配信の整合 (否定役)
  - **Resolution**: per-session occupant 自体を WS にも REST にも載せない設計に変更 (ADR-20260624-palette-no-per-session-occupant)。push_commands と project metadata は REST に集約 (ADR-20260624-palette-session-config-rest-extension)。
- **Issue**: [minor] HTTP 401 (認証期限切れ) の取り扱い未定義 (否定役)
  - **Resolution**: FR-024 で『401 は認証エラー toast + close、他 4xx/5xx は入力保持で open 継続』を明示。
- **Issue**: [minor] 性能仮定 (候補数 5〜10) と push_commands 任意拡張の不整合 (否定役)
  - **Resolution**: scope.out_of_scope に push_commands 個数上限ガード/ページングを明示し本 spec の対象外、現実的な数を想定して観察に留める旨を明文化。
- **Issue**: [minor] daemon PushDriver 経路が SendCommand 統一と齟齬 (否定役)
  - **Resolution**: ADR-20260624-palette-push-route-sendcommand で SendCommand + proto.CmdEvent パターン採用を明示し、daemon_client.go に専用 wrapper を追加しない。
- **Issue**: [improvement] Optimizer: store/palette を純粋 state にし I/O は ToolDef.submit(ctx) に局所化
  - **Resolution**: 採用 (ADR-20260624-palette-2phase-store-architecture)。
- **Issue**: [improvement] Optimizer: focus trap は 30 行未満の自前フックに留める
  - **Resolution**: 採用 (ADR-20260624-palette-focus-trap-minimal)。
- **Issue**: [improvement] Optimizer: IME 抑止は store.composing 1 箇所集約
  - **Resolution**: 採用 (ADR-20260624-palette-ime-suppression-in-store)。
- **Issue**: [improvement] Optimizer: TerminalPane.blur() は CommandPalette の useEffect に閉じる
  - **Resolution**: 採用 (CommandPalette responsibility に明記)。store/palette は DOM 操作を持たない。
- **Issue**: [improvement] Optimizer: ScopeSegment は dumb component に降格し disabledReason は ToolDef に持たせる
  - **Resolution**: 採用 (ADR-20260624-palette-disabledreason-single-source)。
- **Issue**: [improvement] Optimizer: fuzzy ranges 消費を ToolSelectPhase に限定
  - **Resolution**: 採用 (ADR-20260624-palette-fuzzy-pure-function + ParamSelectPhase responsibility に明記)。
- **Issue**: [improvement] Optimizer: CreateSessionForm 撤去を F1/F2/F3 の 3 段 commit phase で実施
  - **Resolution**: 採用 (ADR-20260624-palette-createsessionform-replacement, chunks の f1/f2/f3 がこの順序)。
- **Issue**: [improvement] Optimizer: 観測性は NotificationToast + console.debug + slog の既存パターンに留め新規 metric は追加しない
  - **Resolution**: 採用 (本 plan は新規 metric を導入しない方針を scope.out_of_scope と plan の Verification 節で扱う)。
- **Issue**: [improvement] Optimizer: new-session payload は host?:bool ではなく sandbox?:'host' (既存 wire) に揃える
  - **Resolution**: 採用 (ADR-20260624-palette-new-session-payload-wire-mirror, FR-014/021)。
