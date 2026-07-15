# Repro: credproxy gcloud token file lost across daemon restart (2026-07-15)

## 症状
既存 devcontainer が動作中のまま agent-grid `server` (daemon) を再起動すると、
gcloudcli provider が host 側に書く `<runBase>/<projectHash>/gcloud-token`
(container 側 `/opt/agent-grid/run/gcloud-token`) が再生成されず、コンテナ内の
`gcloud` CLI 系コマンドが認証エラーで失敗し続ける。一方 metadata proxy
(`gcp-metadata.sock` 経由の `127.0.0.1:8181` bridge) は健全で、
`curl -H 'Metadata-Flavor: Google' .../token` は成功する。

## 環境 (必須)
- Repo: /home/dev/dev/agent-grid, branch main, HEAD b932902e
- Runtime: agent-reactor frame 内の devcontainer で本セッション自体が稼働
- Host daemon: systemd unit (`bash -lc` wrapper で login-shell env 継承、b932902e で追加)
- credproxy: github.com/takezoh/credproxy v0.0.0-20260527021126-c5e0af72e46e
  (ローカル fork /home/dev/dev/credproxy は replace 未配線 — HEAD ad8d1b25 だが
  providers/gcloudcli 配下は pin 後に変更なしを `git log c5e0af72e46e..HEAD --
  providers/gcloudcli/` で確認済み。読解の正当性に影響なし)
- sandbox mode: devcontainer, per-project isolation

## 再現手順 (必須)
1. GCP 認証が設定された project (`Proxy.GCP.Account`/`Active` 設定済み) で
   devcontainer セッションを起動し、`gcloud auth print-access-token` が
   コンテナ内で成功することを確認する。
2. host 側で `systemctl restart agent-grid` 等により daemon を再起動する
   (devcontainer 自体は停止させない — warm-start adoption の対象にする)。
3. 再起動完了後、コンテナ内で:
   - `ls /opt/agent-grid/run/gcloud-token` → `No such file or directory`
   - `curl -H 'Metadata-Flavor: Google' http://127.0.0.1:8181/computeMetadata/v1/instance/service-accounts/default/token` → token を含む JSON が返る (metadata bridge 健全)
   - `gcloud auth print-access-token` (または `gcloud auth application-default print-access-token`) → 失敗

## 発生頻度
不明 (今回 1 件観測)。daemon boot 時点の一過性条件 (gcloud subprocess の
一時的失敗、あるいは host env/umask 起因の書き込み権限問題) に依存すると
推測されるため、毎回ではない可能性が高い。git log 確認の結果、
`providers/gcloudcli/spec.go` / `metadata.go` にはこの「書き込み失敗の
不可視化・未検証」という機構そのものへの過去の修正コミットは存在せず、
この機構としては初回の顕在化 (recurrence.count = 1)。b932902e (systemd
env 継承修正) や e2ab83c0 (shared container proxy socket の mount path 一致
修正) は同じサブシステム領域だが、機構としては別 (前者は PATH 解決失敗、
後者は hash-key 不一致によるパス不一致であり、いずれも今回の「登録=成功と
みなす設計 + 書き込みエラー不可視化」とは異なる)。

## 期待 / 実際
期待: daemon 再起動後も、再起動を生き延びた devcontainer 内で `gcloud` CLI が
継続して動作する (token file が再生成される)。
実際: token file が生成されず、`gcloud` コマンドは認証エラー。metadata proxy
自体は健全。

## 特定した原因 (RCA 概要 — 詳細は investigation-2.json 参照; investigation.json の
一部記述は critique により訂正済み)

**訂正 (2nd-pass investigation):** 初回調査は「25 分周期の `refreshAllTokens`
まで再試行が一切発生しない」と記載したが、これは誤り。
`metadata.go:68-86` の token endpoint は curl/gcloud からのリクエスト毎に
`gcpPrintAccessToken` → `os.WriteFile(tokenFilePath, ...)` を実行しており、
これ自体が on-demand の再試行として機能する (`_ = os.WriteFile` で戻り値を
捨てているため成否は不可視だが、「試行が起きていない」わけではない)。

正しい機構: `ensureMetadataServer` (spec.go:180-219) は `tokenTargets` への
登録 (line 195, listener bind 成功時に無条件) と `writeTokenFile` の実行
(line 213-217, pre-populate) を分離しており、後者の失敗は `slog.Warn` の
みで握りつぶされる。登録後は `ContainerSpec` 呼び出しがガードで早期 return
する (line 181-185) ため、以後の materialize は (a) 25 分周期の
`refreshAllTokens`、または (b) metadata endpoint への実際のリクエスト
(per-request write) でのみ発生する。両経路とも書き込みエラーを完全に
不可視化する (spec.go 側は log のみ、metadata.go 側は log すら出さず
discard) ため、**「再試行が起きているかどうか」ではなく「再試行の結果が
成功したかどうかを外部から確認する手段が一切ない」ことが本質**。

bind-mount / inode 乖離説 (2nd-pass critique が提起) は静的証拠で否定できる:
(1) `RunBase/<hash>` を削除するコードパスはリポジトリ全体に存在せず
(`os.MkdirAll` は既存ディレクトリに対して no-op)、dataDir は既定で
`~/.agent-grid` (永続ディスク、tmpfs ではない) なので再起動やホスト再起動で
ディレクトリが消える経路がない。(2) 同じ bind-mount ディレクトリ配下に
`gcp-metadata.sock` も存在し、これはコンテナ作成時に一度だけ起動する
長命の `bridge sockbridge` プロセスが毎回 dial するソケットである。もし
mount が古い (削除済み) inode を指しているなら、今回の再起動で新規作成
された listener socket ファイルはコンテナから見えず、curl は失敗するはず
だが、バグ報告は curl 成功を記録している — つまり mount は現在の
ホストディレクトリを正しく反映しており、gcloud-token の不在は mount
経路の問題ではない。

構造的根本原因 (5 検査質問、トリガー非依存で成立): `container.Provider.ContainerSpec`
という単一契約に、(a) コンテナオーバーレイ合成のための冪等・ベストエフォート
処理と、(b) daemon-restart 境界での credential file rehydration という
一度きり・must-succeed の処理、という異なる失敗許容度の 2 責務が同居して
おり、「token materialization が実際に成功したか」を追跡する owner が
どこにもいない (SSOT 不在 / 契約の暗黙化)。ただし、今回の 1 回の書き込み
失敗が具体的に何 (boot-time transient / gcloud 同時実行競合 / b932902e の
env・umask 変更による permission regression 等) に起因するかは未確定
(confidence: low) — daemon の実 slog と `docker inspect` へのアクセスが
必要で、本 investigation では取得できなかった。詳細は investigation-2.json
の `rca`/`hypotheses`/`responsibility_checks` を参照。

## 修正
未着手 (本調査は investigation のみ。修正は別 workflow で承認後に実施)。

## テストを書けなかった理由
再現には (1) 実 systemd 経由の daemon 再起動, (2) 実 docker で稼働継続する
devcontainer, (3) 実 `gcloud` CLI とその認証状態, の 3 つの実環境依存が
同時に必要であり、fake/mock では「daemon 再起動境界を実際に跨いで
in-memory 状態がリセットされる」という本質的条件を再現できない
(fake はプロセスを再起動しない)。gcloudcli 側の単体テスト
(spec_test.go) は `tokenTargets` を直接操作して単体検証できるが、それは
「エラーが握りつぶされ、以後の呼び出しがガードで早期returnする」という
契約レベルの欠陥を、restart 境界という文脈込みで再現するものではない。

## Validation 方法 (テスト代替)
- 静的検証: 本調査で `container.ProjectRunHash` (credproxy) と
  `agentlaunch.ProjectRunDir` (agent-grid) の hash 実装が byte-identical
  であることを確認済み (mount path 不一致は今回のトリガーではない)。
- 手動再現: 上記再現手順を実機 (systemd + docker + 実 gcloud 認証) で
  実施し、daemon boot ログに
  `gcloudcli: initial token write failed` (spec.go:216) または
  `gcloudcli: metadata server start failed` (spec.go:158) の WARN が
  再起動タイミングと相関して出ていないか確認する。
- 将来のテスト投資候補: `credproxy` 側に
  「`ensureMetadataServer` の初回 `writeTokenFile` が失敗したら
  `tokenTargets` に未成功フラグを残し、次回 `ContainerSpec` 呼び出しで
  再試行する」契約テストを追加し、あわせて agent-grid 側に
  FakeVsReal トリプル (fake gcloudcli, real gcloud CLI backstop,
  invariant-naming contract test) を新設する。
