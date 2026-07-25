# windows-shell: App.UnhandledException が全例外を無音で握り潰す

- status: open
- created: 2026-07-25
- area: clients/windows-shell/AgentGrid.Shell.WinUI/App.xaml.cs
- origin: debug-run-shell-zero-sessions / debug-run-workspace-not-launched の両方で診断遅延の共犯

## 問題

`App.UnhandledException` ハンドラは `e.Handled = true` + `Debug.WriteLine` のみ:

- Release インストール版では **例外が発生した痕跡が一切残らない**
- 直近の 2 バグ (起動シーケンス中断 / workspace spawn 不達) の両方で、
  本来即座に気づけたはずの例外がここで消え、症状が「何も起きない」に化けた
- `StartFleetAsync` の catch-all も同様に `Debug.WriteLine` のみ

## 提案

最低限 (小):

- 既存の structured log SoT (`%LOCALAPPDATA%\agent-grid\logs\`) に
  `winui-unhandled-*.txt` として例外 + スタックを追記する
  (`winui-startup-error.txt` と同じ運用で e2e fixture からも読める)

検討 (中):

- 未処理例外の発生を tray tooltip / NoticeText に degrade 表示する
- `Handled = true` の一律継続が妥当かの見直し (状態破損後の続行リスク)

## 受け入れ基準

サブスクライバ例外や async void 内の例外を意図的に発生させる smoke で、
ログファイルに例外型 + スタックが記録されることを T3 で確認できること。
