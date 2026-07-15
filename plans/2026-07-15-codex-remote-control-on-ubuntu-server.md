# Codex Remote Control on Ubuntu Server

Date: 2026-07-15
Status: Verified operational guide

## Goal

Ubuntu Server 上で動く Codex セッションを、ChatGPT モバイルアプリの
Remote 画面から確認・操作できるようにする。

対象には次の両方を含む。

- host-direct で起動した Codex セッション
- agent-grid が devcontainer 内で起動した Codex セッション

## Verified behavior

2026-07-15、Ubuntu Server の host 上で次を実行して動作を確認した。

```sh
codex remote-control start
codex remote-control pair
```

ペアリング後、ChatGPT モバイルアプリから次を確認できた。

- host 上で起動した複数の Codex セッションが一覧に表示される。
- agent-grid の devcontainer 内で動くセッションも表示される。
- モバイルから既存セッションへメッセージを送り、そのセッションを継続できる。

したがって、Remote Control は **host 単位で一度起動する**。devcontainer
ごとに `codex remote-control start` や `pair` を実行する必要はない。

概念上の構成は次のとおり。

```text
ChatGPT mobile
  ↕ secure relay
Ubuntu Server: Remote Control daemon
  ├─ host-direct Codex sessions
  └─ agent-grid sessions
       └─ devcontainer 内の Codex TUI / App Server
```

Remote Control は公開 TCP listener を要求しない。App Server の socket や
WebSocket listener をインターネットへ直接公開しないこと。

## Command roles

現行 Codex CLI では、以下のコマンドを区別する。

| Command | Role |
|---|---|
| `codex remote-control` | Remote Control 有効な App Server をフォアグラウンド実行する。 |
| `codex remote-control start` | 組み込みの管理対象 App Server daemon を Remote Control 有効で起動する。既に起動済みなら再利用する。 |
| `codex remote-control stop` | 管理対象 daemon を停止する。 |
| `codex remote-control pair` | 短命なペアリングコードを発行する。 |
| `codex app-server daemon bootstrap --remote-control` | daemon 設定、App Server、standalone 更新ループをまとめて起動する。SSH host の常設運用向け。 |
| `codex app-server daemon enable-remote-control` | 今後の起動設定へ Remote Control 有効状態を保存し、起動中なら反映する。 |
| `codex app-server daemon version` | CLI と実行中 App Server のバージョンを JSON で表示する。 |

`remote_control` は独立した feature flag として有効化するものではない。
現行版で `codex features enable remote_control` をセットアップ手順に入れない。

Sources:

- [Codex CLI command reference](https://learn.chatgpt.com/docs/developer-commands?surface=cli#cli-codex-remote-control)
- [OpenAI Codex app-server-daemon README](https://github.com/openai/codex/blob/main/codex-rs/app-server-daemon/README.md)
- [ChatGPT Remote connections](https://learn.chatgpt.com/docs/remote-connections)

## Prerequisites

### Authentication

host の実行ユーザーで Codex に ChatGPT 認証しておく。

```sh
codex login
```

headless host でブラウザ callback を使えない場合は device authentication を使う。

```sh
codex login --device-auth
```

モバイル側では同じ ChatGPT account と workspace を使用する。workspace の
ポリシー、MFA、SSO、passkey が要求される場合はペアリング中に完了する。

### Standalone managed installation

組み込み daemon と `bootstrap` は、OpenAI の installer が管理する standalone
Codex を使用する。標準パスは次のとおり。

```text
$CODEX_HOME/packages/standalone/current/codex
```

`CODEX_HOME` 未指定時は通常 `~/.codex` になる。管理版がない場合は公式
installer を使用する。

```sh
curl -fsSL https://chatgpt.com/codex/install.sh | sh
```

npm、mise、Homebrew など別経路の `codex` が `PATH` の先頭にあっても、daemon
lifecycle は上記の managed path を使用する。このため systemd unit では managed
binary の絶対パスを指定する。

## Initial setup

最初に host 上で daemon を bootstrap する。

```sh
"$HOME/.codex/packages/standalone/current/codex" \
  app-server daemon bootstrap --remote-control
```

`bootstrap` は次を行う。

- Remote Control を有効にした App Server daemon の起動
- daemon 設定の `$CODEX_HOME/app-server-daemon/` への保存
- standalone Codex を定期更新する updater loop の起動

状態を確認する。

```sh
codex app-server daemon version
```

`start` は冪等であり、既にdaemonが起動している場合は新しいdaemonを重ねて
起動しない。lifecycle操作は `CODEX_HOME` 単位で直列化される。

## systemd user service

Codex の組み込みdaemonはpidfileベースでdetachする。`bootstrap` が起動する
updater loopはOS再起動を越えないため、Ubuntu Serverの起動ごとに
`bootstrap --remote-control`を再実行するsystemd user unitを置く。

`~/.config/systemd/user/codex-remote-control.service`:

```ini
[Unit]
Description=Codex Remote Control daemon bootstrap
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=%h/.codex/packages/standalone/current/codex app-server daemon bootstrap --remote-control
ExecStop=%h/.codex/packages/standalone/current/codex app-server daemon stop
RemainAfterExit=yes
TimeoutStartSec=60
TimeoutStopSec=15

[Install]
WantedBy=default.target
```

反映して起動する。

```sh
systemctl --user daemon-reload
systemctl --user enable --now codex-remote-control.service
```

SSH切断後やhost再起動後もuser serviceを起動するには、agent-gridのsystemd
運用と同様にlingerを有効化する。

```sh
loginctl enable-linger "$USER"
```

確認コマンド:

```sh
systemctl --user status codex-remote-control.service
journalctl --user -u codex-remote-control.service -b
codex app-server daemon version
```

このunitは、systemd自身がApp Serverプロセスを直接監視する構成ではない。
`bootstrap`がCodex組み込みのpidfile daemonを起動し、systemdはboot時の起動と
shutdown時の停止を担当する。常時foreground監視が必要なら、別案として
`codex remote-control`を`Type=simple`で直接監視できるが、組み込みdaemonの
updater loopは利用しない構成になる。

### Optional agent-grid ordering

Remote Control を agent-grid より先に開始したい場合は、
`agent-grid-server.service` の user drop-in に順序だけ追加する。

```ini
[Unit]
Wants=codex-remote-control.service
After=codex-remote-control.service
```

`Wants=` とすることで、Remote Control側の一時的な障害がagent-grid全体の起動を
妨げない。Remote Controlを必須条件にしたい場合だけ`Requires=`を検討する。

## Pairing a mobile device

pairingはdaemon起動とは別の操作である。新しいphoneや対応desktop deviceを
hostへ追加するときだけ実行する。

```sh
codex remote-control pair
```

表示されたmanual pairing codeをChatGPTモバイルアプリのRemote設定フローで
入力する。モバイル側のUI名はバージョンにより変わり得るが、Remote画面から
host/device追加を開始する。

機械可読な出力が必要な場合:

```sh
codex remote-control pair --json
```

JSONには少なくとも次が含まれる。

- `pairingCode`
- `manualPairingCode`
- `environmentId`
- `expiresAt`

### Pairing lifecycle

- pairing code は短命であり、期限切れ時は `pair` を再実行する。
- 成立済みのdevice pairingはdaemon再起動や通常のhost再起動では作り直さない。
- `pair` をsystemdの`ExecStart`へ入れない。bootごとに未使用コードを発行しても
  pairingの維持には寄与しない。
- deviceごと、hostごとに一度pairingする。
- ChatGPTからsign outするとRemote Controlはoffになるが、既存pairing自体は
  削除されない。sign in後にRemote Controlを再度有効化する。

## Host and devcontainer scope

agent-gridではCodex App Server本体の配置がセッションに従う。

```text
host-direct session  → host App Server
devcontainer session → devcontainer App Server
```

一方、検証済みのRemote Control運用単位はUbuntu hostである。host daemonを
pairingすると、同じhost上のagent-gridが起動したdevcontainerセッションも
モバイルから表示・継続できた。

したがって次は実施しない。

- devcontainerの`postCreateCommand`で`remote-control start`を実行する。
- Codex frameごとのpre-commandでdaemonを起動する。
- devcontainerごとにmobile deviceをpairingする。

これらはdaemonとpairing状態を不必要に複製し、host単位の運用を複雑化する。

## Operations

### Start or repair Remote Control

```sh
codex remote-control start
```

またはupdaterを含めて再bootstrapする。

```sh
codex app-server daemon bootstrap --remote-control
```

### Persist Remote Control enablement

```sh
codex app-server daemon enable-remote-control
```

起動中のmanaged daemonがある場合は、新しい設定を反映するため再起動される。

### Stop

```sh
codex remote-control stop
```

または:

```sh
codex app-server daemon stop
```

停止はdevice pairingの削除を意味しない。

### Upgrade

`bootstrap`で起動したupdater loopは公式installerを定期実行し、managed binaryが
更新された場合にApp Serverを更新する。updater loopはbootを越えないため、
systemd unitから毎boot bootstrapする。

## Troubleshooting

### `managed standalone Codex install not found`

daemon lifecycleはnpm/mise/Homebrewの実行ファイルではなくmanaged standalone
pathを要求する。公式installerを実行し、次を確認する。

```sh
test -x "$HOME/.codex/packages/standalone/current/codex"
```

### Mobile app does not show the host

次を順に確認する。

1. `codex app-server daemon version`が実行中App Serverを返す。
2. hostがネットワークへ接続され、sleepしていない。
3. mobileとhostが同じChatGPT account/workspaceを使用している。
4. workspace policyでRemote Controlが禁止されていない。
5. pairing codeの期限内にセットアップを完了した。
6. sign out後ならRemote Controlを再度有効化した。

### Mobile can see the host but a session is missing

次を確認する。

- 対象Codex TUI/sessionがhost上でまだ利用可能である。
- agent-gridが対象sessionを復元できている。
- devcontainerが停止・削除されていない。
- Codex CLI、managed daemon、mobile appを現行互換版へ更新している。

### Network exposure

Remote ControlのためにApp ServerのUnix socketやWebSocket listenerをpublic networkへ
公開しない。Remote接続は認可済みdevice向けのsecure relayを使用する。SSHや
agent-grid web UIを別途公開する場合は、それぞれの認証とTLSを独立して設定する。

## Decision summary

- Ubuntu Server上のheadless Codex Remote Controlは実動確認済み。
- `codex remote-control start`はhostで一度実行する。
- 常設運用ではmanaged standalone版と
  `app-server daemon bootstrap --remote-control`を使用する。
- systemd user unitからbootごとにbootstrapし、updater loopも復元する。
- `codex remote-control pair`は新規device追加時のみ手動実行する。
- host pairingからhost-directとdevcontainer双方のagent-gridセッションを操作できる。
- devcontainerやframeごとのRemote Control daemon起動は不要。

## Verification checklist

- [x] Installed Codex version exposes `remote-control` and `app-server daemon`.
- [x] `codex remote-control start` succeeds on the Ubuntu host.
- [x] `codex remote-control pair` pairs ChatGPT mobile without ChatGPT Desktop.
- [x] Mobile lists host-direct Codex sessions.
- [x] Mobile lists agent-grid devcontainer Codex sessions.
- [x] Mobile can send a follow-up message to a devcontainer session.
- [x] Built-in daemon lifecycle and standalone requirement are documented.
- [x] systemd boot activation is documented.
- [x] Pairing lifecycle and JSON fields are documented.
- [x] systemd unit and Makefile installation contract are covered by a static test.
- [ ] Validate boot activation on a clean host after `make install-codex-remote-control-systemd` and linger setup.
