# windows-shell: headless Host 経由 deep link に前面化権限がない

- status: open
- created: 2026-07-25
- area: clients/windows-shell (AgentGrid.Shell.Host / WorkspaceLauncher)
- origin: debug-run-workspace-not-activated の open question (`~/.dev-skills/debug/workspace-not-activated/`)

## 症状 (潜在)

workspace の前面化は「クリックを受けてフォアグラウンド権限を持つプロセスが
`AllowSetForegroundWindow(workspace_pid)` を委譲する」ことで成立している
(修正コミット bda14349)。この前提は WinUI Shell のチップ/ボタン経路では成り立つが、
**headless Host (`Program.cs`) 経由の deep link (トースト起動など) では、委譲元の
プロセス自身がフォアグラウンド権限を持たない可能性がある** — その場合
`AllowSetForegroundWindow` は無効で、workspace はタスクバー点滅止まりになる。

## 未確定事項 (要実測)

- トースト活性化 (AppNotification) の起動 grace が Host プロセスに
  フォアグラウンド権限として渡るかどうか (Windows の仕様依存)
- 渡らない場合の代替: トーストのハンドラを WinUI Shell 側 (常駐・NOACTIVATE でも
  クリック時は権限を得る) に寄せる / COM activation の権限伝搬を使う

## 検証案

実機で: 別アプリにフォーカスがある状態でトースト経由 deep link →
GetForegroundWindow が workspace になるかの単発観測から着手する。
