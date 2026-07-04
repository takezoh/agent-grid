---
id: plan-20260624-2026-06-24-web-ui-command-palette
kind: plan
title: Plan — Web UI Command Palette
status: draft
created: '2026-06-24'
updated: '2026-07-04'
tags:
- plan
- legacy-import
owners: []
relations: []
source_paths: []
goal: Plan — Web UI Command Palette
scope_in: []
scope_out: []
milestones: []
contracts: []
---

<!-- migrated_from: docs/specs/2026-06-24-web-ui-command-palette/plan.md -->

# Plan — Web UI Command Palette

- **spec**: [spec.md](../../specs/2026-06-24-web-ui-command-palette/spec.md)
- **ADRs**: [0036](../../adr/adr-20260624-0036-palette-2phase-store-architecture.md), [0037](../../adr/adr-20260624-0037-palette-hotkey-capture-phase.md), [0038](../../adr/adr-20260624-0038-palette-fuzzy-pure-function.md), [0039](../../adr/adr-20260624-0039-palette-focus-trap-minimal.md), [0040](../../adr/adr-20260624-0040-palette-ime-suppression-in-store.md), [0041](../../adr/adr-20260624-0041-palette-session-config-rest-extension.md), [0042](../../adr/adr-20260624-0042-palette-new-session-payload-wire-mirror.md), [0043](../../adr/adr-20260624-0043-palette-createsessionform-replacement.md), [0044](../../adr/adr-20260624-0044-palette-no-per-session-occupant.md), [0045](../../adr/adr-20260624-0045-palette-push-route-sendcommand.md), [0046](../../adr/adr-20260624-0046-palette-push-active-mismatch-409.md), [0047](../../adr/adr-20260624-0047-palette-disabledreason-single-source.md)

## Goal

agent-reactor の Web UI に、2 フェーズ操作 (ファジー検索でツール選択 → パラメータ順次入力) を持つコマンドパレットを Cmd/Ctrl+K + 常設ボタンで起動できるモーダルとして実装する。standard スコープ (new-session / stop-session) と push スコープ (push_commands 配列) をセグメント切替で同居させ、push 送信経路 (POST /api/sessions/{id}/push) を本 spec で新設し、CreateSessionForm はパレット new-session に一本化して撤去する。push 可否判定は既存の daemon-global ActiveSessionID + ActiveOccupant を流用し、SessionInfo proto / wire への per-session occupant 追加は行わない。ADR-0030 (TerminalPane subscribe 唯一所有) / ADR-0033 (displayLabel 空文字=空) / WS は I/O 専用の既存規約を破らないことを設計制約とする。

## Components

| Component | Responsibility | Depends on |
|-----------|----------------|------------|
| CommandPalette (src/client/web/src/components/CommandPalette.tsx) | role=dialog / aria-modal=true のオーバーレイ。mount で opener を記録 + アクティブ TerminalPane.blur()、unmount で opener.focus() の DOM 副作用を単一所有。store/palette の純粋 state を読み、phase に応じて ScopeSegment + ToolSelectPhase or ParamSelectPhase を描画。Esc / 外側クリック / 戻る / 閉じるは store actions に変換する薄い container。 | `store/palette`, `components/palette/ScopeSegment`, `components/palette/ToolSelectPhase`, `components/palette/ParamSelectPhase`, `hooks/useFocusTrap` |
| ToolSelectPhase (src/client/web/src/components/palette/ToolSelectPhase.tsx) | phaseToolSelect の listbox + 検索 input。lib/fuzzy.rank で候補と match ranges を取得、ranges を <mark> でハイライト描画。↑/↓/Ctrl+P/N/Enter/IME 抑止を store action に変換 (store.composing 参照)。 | `lib/fuzzy`, `store/palette`, `lib/tools` |
| ParamSelectPhase (src/client/web/src/components/palette/ParamSelectPhase.tsx) | phaseParamSelect の縦並び paramField 群。ParamDef.options 有無で listbox / text input を分岐。command フィールド上でのみ Tab=worktree / Shift+Tab=host トグル (project の isGit / isSandboxed が true のときのみ)、それ以外のフィールドは focus trap の標準巡回。最終フィールド Enter で store.submit() を呼ぶ。fuzzy ranges は使わない (描画は match 順 + index のみ)。 | `store/palette`, `lib/tools`, `lib/fuzzy` |
| ScopeSegment (src/client/web/src/components/palette/ScopeSegment.tsx) | standard / push の 2 値セグメント。各 scope について ToolRegistry.disabledReason(daemonSnapshot) を呼び、null でなければ disabled + サブテキスト表示。自身は dumb component。 | `store/palette`, `store/daemon`, `lib/tools` |
| palette store (src/client/web/src/store/palette.ts) | Zustand store: open/close, phase (toolSelect\|paramSelect), scope (standard\|push), selectedToolId, paramValues, paramCursor, query, composing, submitting, error, opener (HTMLElement\|null) の 1 箇所集約。DOM 操作は持たない (focus 復帰は CommandPalette / close action 経由の opener 参照のみ)。actions: open(opts)/close()/back()/setScope/setQuery/moveCursor/confirmTool/setParam/toggleWorktree/toggleHost/setComposing/submit(ctx)。submit は ToolDef.submit(ctx) に I/O を委譲する薄い wrapper。 | `lib/tools`, `store/daemon` |
| lib/tools (src/client/web/src/lib/tools.ts) | ToolDef 型定義と ToolRegistry。各 ToolDef は { id, label, scope, params: ParamDef[]\|null, disabledReason(daemonSnapshot): string\|null, submit(ctx, payload): Promise<void> }。ctx = { http: api/sessions, daemon: store/daemon snapshot, notify: notifications, store: palette actions }。standard 2 (new-session / stop-session) を静的登録、push は daemonSnapshot.pushCommands から動的展開。disabledReason に push 失効判定を集約 (ScopeSegment と submit pre-check が共有)。 | `api/sessions client`, `store/daemon (型のみ)` |
| lib/fuzzy (src/client/web/src/lib/fuzzy.ts) | 純関数 fuzzyRank<T>(items: T[], query: string, getText: (item: T) => string): Array<{item: T, score: number, ranges: [number, number][]}>。連続一致優先 + マッチ位置返却。依存ゼロ、~50 行。ハイライト range の消費は ToolSelectPhase に限定。 | — |
| hooks/useGlobalHotkey (src/client/web/src/hooks/useGlobalHotkey.ts) | document の capture phase で Cmd (mac) / Ctrl (other) + K を listen、store.open() を呼ぶ。App.tsx のトップで 1 回だけ mount される不変条件をコメント / README で明示。既に開いている場合は store の検索 input ref に focus を戻すだけ呼び phase / 入力を保持する (FR-029)。 | `store/palette` |
| hooks/useFocusTrap (src/client/web/src/hooks/useFocusTrap.ts) | ref 内の最初/最後の tabbable に Tab/Shift+Tab で循環させる極小フック (~30 行)。Esc handling と opener 復帰は store/palette.close() に委ね、本フックは循環のみ責務とする。store を import しない。 | — |
| api/sessions client (src/client/web/src/api/sessions.ts) | POST /api/sessions / DELETE /api/sessions/{id} / POST /api/sessions/{id}/push / GET /api/session-config の HTTP client 薄 wrap。Bearer 認証ヘッダは既存 auth.ts から取得。fetch 失敗 / 401 / 4xx / 5xx を typed Error として throw (status を含む)。 | `auth.ts` |
| App / Header 改修 (src/client/web/src/App.tsx) | 常設『Command (⌘K / Ctrl+K)』ボタンの追加、既存 New Session ボタンをパレット new-session 起動に再配線。useGlobalHotkey をトップで 1 回 mount。CreateSessionForm 参照を削除。 | `store/palette`, `hooks/useGlobalHotkey`, `CommandPalette` |
| push route (src/server/web/mux.go) | POST /api/sessions/{id}/push を新設。body {command:string} を読み、現在 web gateway が保持している daemon-global ActiveSessionID と path id を照合。不一致なら 409、未存在なら 404、認証拒否なら 401、一致なら SendCommand(ctx, proto.CmdEvent{Event: state.EventPushDriver, Payload: state.PushDriverParams{...}}) を発行 (handleCreateSession 同形)。slog で warn を残す。 | `daemon_client.go`, `src/client/state (EventPushDriver / PushDriverParams)` |
| session-config 拡張 (src/server/web/mux.go handleSessionConfig) | apiSessionConfig レスポンスに push_commands ([]string) と projects ([{path:string, isGit:bool, isSandboxed:bool}]) を追加。isGit は path 配下に .git が存在するかで判定、isSandboxed は config 由来の sandbox 対象 list と path のマッチで判定。 | `cfg.Session.PushCommands`, `platform 既存の git / sandbox 判定 helper` |
| CreateSessionForm 撤去 | src/client/web/src/components/CreateSessionForm.{tsx,test.tsx} 削除、App.tsx 内の使用箇所削除、App.test.tsx の関連 case 削除/書き換え。 | `CommandPalette`, `App / Header 改修` |

## Build Sequence (chunks 依存順)

依存方向: `f1-palette-shell → f2-createsessionform-removal → f3-push-scope-and-route`

### Chunk: `f1-palette-shell`

- **Depends on**: (なし、起点)

- **Members**:
  - component:lib/fuzzy (src/client/web/src/lib/fuzzy.ts)
  - component:lib/tools (src/client/web/src/lib/tools.ts)
  - component:palette store (src/client/web/src/store/palette.ts)
  - component:hooks/useFocusTrap (src/client/web/src/hooks/useFocusTrap.ts)
  - component:hooks/useGlobalHotkey (src/client/web/src/hooks/useGlobalHotkey.ts)
  - component:api/sessions client (src/client/web/src/api/sessions.ts)
  - component:ScopeSegment (src/client/web/src/components/palette/ScopeSegment.tsx)
  - component:ToolSelectPhase (src/client/web/src/components/palette/ToolSelectPhase.tsx)
  - component:ParamSelectPhase (src/client/web/src/components/palette/ParamSelectPhase.tsx)
  - component:CommandPalette (src/client/web/src/components/CommandPalette.tsx)
  - component:App / Header 改修 (src/client/web/src/App.tsx)
  - req:FR-001
  - req:FR-002
  - req:FR-003
  - req:FR-007
  - req:FR-008
  - req:FR-009
  - req:FR-011
  - req:FR-012
  - req:FR-017
  - req:FR-018
  - req:FR-019
  - req:FR-020
  - req:FR-029
  - adr: [0036-palette-2phase-store-architecture](../../adr/adr-20260624-0036-palette-2phase-store-architecture.md)
  - adr: [0037-palette-hotkey-capture-phase](../../adr/adr-20260624-0037-palette-hotkey-capture-phase.md)
  - adr: [0038-palette-fuzzy-pure-function](../../adr/adr-20260624-0038-palette-fuzzy-pure-function.md)
  - adr: [0039-palette-focus-trap-minimal](../../adr/adr-20260624-0039-palette-focus-trap-minimal.md)
  - adr: [0040-palette-ime-suppression-in-store](../../adr/adr-20260624-0040-palette-ime-suppression-in-store.md)

### Chunk: `f2-createsessionform-removal`

- **Depends on**: `f1-palette-shell`

- **Members**:
  - component:CreateSessionForm 撤去
  - component:session-config 拡張 (src/server/web/mux.go handleSessionConfig)
  - req:FR-013
  - req:FR-014
  - req:FR-015
  - req:FR-016
  - req:FR-021
  - req:FR-022
  - req:FR-027
  - req:FR-028
  - adr: [0041-palette-session-config-rest-extension](../../adr/adr-20260624-0041-palette-session-config-rest-extension.md)
  - adr: [0042-palette-new-session-payload-wire-mirror](../../adr/adr-20260624-0042-palette-new-session-payload-wire-mirror.md)
  - adr: [0043-palette-createsessionform-replacement](../../adr/adr-20260624-0043-palette-createsessionform-replacement.md)

### Chunk: `f3-push-scope-and-route`

- **Depends on**: `f2-createsessionform-removal`

- **Members**:
  - component:push route (src/server/web/mux.go)
  - req:FR-004
  - req:FR-005
  - req:FR-006
  - req:FR-010
  - req:FR-023
  - req:FR-024
  - req:FR-025
  - req:FR-026
  - adr: [0044-palette-no-per-session-occupant](../../adr/adr-20260624-0044-palette-no-per-session-occupant.md)
  - adr: [0045-palette-push-route-sendcommand](../../adr/adr-20260624-0045-palette-push-route-sendcommand.md)
  - adr: [0046-palette-push-active-mismatch-409](../../adr/adr-20260624-0046-palette-push-active-mismatch-409.md)
  - adr: [0047-palette-disabledreason-single-source](../../adr/adr-20260624-0047-palette-disabledreason-single-source.md)

## Verification

各 chunk の完了は次の手段で検証する:

- **静的検証**: `cd src && go vet ./...` / `make lint` (golangci-lint depguard / funlen / staticcheck) / `cd src/client/web && pnpm biome check` / `pnpm tsc --noEmit`
- **テスト**: `cd src && go test ./server/web/... ./client/state/...` / `cd src/client/web && pnpm vitest run`
- **手動**: `make build && ./server` で実機起動し、Cmd/Ctrl+K でパレット起動 → new-session / stop-session / push 動作を確認する

## Open Questions (実装段階で確定する事項)

> いずれも plan-impl 段階で grep / 観察により確定する。設計判断は ADR で決着済み。

- GET /api/session-config の projects 要素拡張 ({path, isGit, isSandboxed}) は claude-app-server の /api/session-config 利用箇所に影響しないか — 現時点で apiSessionConfig は web 専用エンドポイントだが、設定回りの helper 共有がある場合は確認が必要 (plan 実装着手前に grep で確認、影響あれば backward-compatible に保つ)
- isSandboxed の判定源 — config.Session の sandbox 対象 list と project path のマッチングを mux.go で行うか、platform 側に helper を切り出すか (plan-impl で実装位置を確定する)
- /api/session-config の再 fetch タイミング — palette open 毎に再 fetch するか、App 起動時 + 任意の reload トリガで足りるか。push_commands は config 編集時に変わるが、変更通知の仕組みは現状なし (plan-impl で観察ベースで決める)
- stop-session の対象 listbox の getText (displayLabel か title か id か) — ADR-0033 の displayLabel 純関数を再利用する想定だが、ParamDef.options の getText 指定方法は ToolRegistry 実装時に確定する
- command 大きさ上限 — push の command body に巨大文字列が来たとき mux.go で 4xx を返すかどうかの上限値 (plan-impl で既存 mux のリミットに準ずる)
