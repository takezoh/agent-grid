---
id: note-20260724-windows-shell-ui-e2e-authoring
kind: note
title: Windows Shell UI e2e (FlaUI/UIA) authoring rules for AI agents
status: published
created: '2026-07-24'
tags:
- testing
owners: []
relations: []
source_paths:
- clients/windows-shell/AgentGrid.Shell.WinUI.UiTests
- clients/windows-shell/AgentGrid.Shell.WinUI/Panel/PanelWindow.xaml
- clients/windows-shell/scripts/e2e.sh
summary: AI agents が Windows Shell (WinUI 3) の UI を変更・検証するときの UI e2e 実装規約。AutomationId
  契約、FlaUI/UIA テストの書き方、retry 規律、副作用規律、デバッグ手段 (Accessibility Insights) を定める。
updated: '2026-07-24'
---

## Summary

Windows Shell (WinUI 3) のネイティブ UI は **UI Automation (UIA)** を介して
Playwright 相当の e2e で検証する。ドライバは
[FlaUI](https://github.com/FlaUI/FlaUI) (UIA3)、スイートは
`clients/windows-shell/AgentGrid.Shell.WinUI.UiTests`(T3 opt-in、
`AG_E2E_WINUI_UI=1`)。ビルド済み self-contained exe をブラックボックス駆動し、
アプリ本体への ProjectReference は持たない。ハーネス全体像・コマンド・env は
`clients/windows-shell/docs/e2e.md` が正本 — 本 note は **UI を変更・テストする
agent が守る規約**だけを定める。

## Rules

### 1. AutomationId は UI 契約 — 必ず付与する

- `PanelWindow.xaml`(および今後の WinUI window)に**インタラクティブ要素や
  状態表示要素を追加したら、`AutomationProperties.AutomationId` を必ず付与**する。
  Web の `data-testid` に相当し、同時に Narrator / S3 アクセシビリティ
  (`clients/windows-shell/docs/s3-prototypes-checklist.md`)の契約でもある。
- id は要素の責務を表す PascalCase(`ConnectionText`, `EngageSendButton`)。
  **一度公開した id は rename しない**(テストと支援技術の両方が壊れる)。
  WinUI 3 では x:Name は AutomationId に自動反映されない前提で書く。
- テスト側のセレクタは AutomationId のみ。表示文字列・座標・ツリー順序で
  要素を特定しない(文字列は locale/文言変更で、座標は DPI で壊れる)。

### 2. テストの書き方(FlaUI/UIA)

- 置き場所: `AgentGrid.Shell.WinUI.UiTests/E2E/`。ガードは `[WinUiUiFact]`
  (`AG_E2E_WINUI_UI=1` かつ Windows でのみ実行、それ以外は skip)。
  常時実行の `dotnet test` を赤にしない — ガードなしの UI テストを書かない。
- アプリ起動は `PanelUiSession` fixture 経由(lazy 起動・stale プロセス kill・
  dispose で kill)。アプリは single-instance(AppInstance redirect)なので
  **fixture を迂回して自前で Process.Start しない**。
- **UIA ツリーは非同期に構築される**。要素取得・状態assert は必ず
  `Retry.WhileNull` / `Retry.WhileFalse` でポーリングする。起動直後の即時
  assert は flaky の典型源。Playwright の auto-wait は存在しない。
- 操作は UIA パターン(`Invoke()`, `AsTextBox().Text =`)を使う。
  マウス座標クリック・`SendKeys` 相当のグローバルキー送出は使わない。

### 3. 副作用規律

- ハーネスは live な `make run-dev` に attach されることがある。テストは
  **セッション状態を変異させない操作**に限定する(例: Engage 送信テストは
  「pending question が無ければ box がクリアされるだけ」という no-op 経路を使う)。
  approve/deny や `OpenSessionButton` の Invoke のように実セッションへ作用しうる
  操作を足す場合は、`--start-run-dev` の隔離 fixture 前提であることをテスト内で
  保証(または明示 skip)すること。

### 4. 失敗診断

- 要素未発見・タイムアウト時は `%LOCALAPPDATA%\agent-grid\logs\ui-e2e-*.png`
  にスクリーンショットが落ちる(`PanelUiSession.TryCaptureWindow`)。
- 起動自体の失敗は既存の structured SoT
  `%LOCALAPPDATA%\agent-grid\logs\winui-startup-error.txt` を読む
  (fixture がエラーメッセージへ自動添付する)。
- 実行中シェルの UIA ツリーを対話的に確認するには
  **Accessibility Insights for Windows**(または Windows SDK の `inspect.exe`)
  を使う — Playwright inspector 相当で、AutomationId と UIA パターンを
  ライブで確認できる。

### 5. 実行

```sh
make test-windows-shell-e2e     # fixture → xUnit → smoke → FlaUI UI stage
./clients/windows-shell/scripts/e2e.sh --skip-ui   # UI stage を除外
```

制約: UIA は対話デスクトップセッションが必要(WSL から `powershell.exe` 経由は
可。session-0 の CI runner では動かない)。UI stage は smoke stage のビルド成果物
(`AG_E2E_WINUI_EXE`)を前提にするため、`--skip-winui` は UI stage も含めて skip する。
