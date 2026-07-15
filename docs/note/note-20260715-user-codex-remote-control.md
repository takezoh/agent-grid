---
id: note-20260715-user-codex-remote-control
kind: note
title: Codex Remote Control
status: published
created: '2026-07-15'
updated: '2026-07-15'
tags:
- user
- codex
- remote-control
owners: []
relations:
- {type: referencedBy, target: note-20260624-docs-overview}
- {type: referencedBy, target: note-20260624-user-overview}
source_paths:
- Makefile
- deploy/systemd/codex-remote-control.service
topic: user
summary: Host-scoped Codex Remote Control setup, systemd operation, pairing lifecycle,
  and devcontainer session behavior.
---

# Codex Remote Control

Codex Remote Control を Ubuntu Server のホスト単位で常設し、ChatGPT
モバイルアプリから Codex セッションを確認・操作するための運用ガイドです。

## 運用単位

Remote Control daemon は、Codex セッションごとではなく **ホストで一つ**起動します。
Agent Grid の動作検証では、ホストをペアリングすることで、そのホスト上の次の
セッションをモバイルから確認し、メッセージを送信できました。

- ホスト上で直接起動した Codex セッション
- Agent Grid が devcontainer 内で起動した Codex セッション

devcontainer の `postCreateCommand` やセッション起動コマンドから
`codex remote-control start` を実行しないでください。devcontainer ごとの daemon
やペアリングは不要です。

## 前提

- Linux の systemd user manager を利用できること
- ホストの実行ユーザーで Codex に ChatGPT 認証済みであること
- モバイル側とホスト側で同じ ChatGPT account/workspace を使用すること
- 公式 installer が管理する standalone Codex が次にあること

```text
~/.codex/packages/standalone/current/codex
```

daemon lifecycle は、npm、mise、Homebrew など別経路の `codex` ではなく、この
managed standalone binary を使用します。

## systemd で起動する

リポジトリのルートで次を実行します。

```sh
make install-codex-remote-control-systemd
```

このターゲットは managed standalone binary の存在を確認してから、次を行います。

1. `codex-remote-control.service` を `~/.config/systemd/user/` へ配置する。
2. user manager を `daemon-reload` する。
3. service を `enable --now` で有効化・起動する。

service は boot ごとに次を実行します。

```sh
codex app-server daemon bootstrap --remote-control
```

`bootstrap` は Remote Control 対応 App Server daemon と standalone updater loop を
起動します。service 停止時は `codex app-server daemon stop` を実行します。

ログインしていない間もホスト起動時から user service を動かす場合は、linger を
一度有効化します。

```sh
loginctl enable-linger "$USER"
```

## モバイル端末をペアリングする

daemon の起動後、ホストで次を手動実行します。

```sh
codex remote-control pair
```

表示された pairing code を ChatGPT モバイルアプリの Remote Control
セットアップ画面へ入力します。コードは短命なので、入力前に期限切れになった場合は
`pair` を再実行してください。

### ペアリングが必要な場合

- このホストへ端末を初めて接続するとき
- 別のモバイル端末を追加するとき
- 別のホストを追加するとき
- 以前のペアリングを解除・失効したあと、再接続するとき

### 再ペアリングが不要な場合

- Remote Control daemon の再起動
- 通常のホスト再起動
- Agent Grid の再起動
- devcontainer の再作成・再起動
- 新しい Codex セッションの開始

ペアリングは daemon の起動とは別の操作です。`codex remote-control pair` を
systemd unit に組み込まず、新しいペアリングが必要なときだけ実行してください。

## 状態確認と操作

```sh
systemctl --user status codex-remote-control.service
journalctl --user -u codex-remote-control.service -b
codex app-server daemon version
```

service の再起動と停止:

```sh
systemctl --user restart codex-remote-control.service
systemctl --user stop codex-remote-control.service
```

daemon の停止は、成立済みの device pairing の削除を意味しません。

## トラブルシューティング

### Managed standalone Codex がない

次が executable であることを確認します。

```sh
test -x "$HOME/.codex/packages/standalone/current/codex"
```

存在しない場合は公式 Codex installer で managed standalone 版を導入してから、
Make ターゲットを再実行します。

### モバイルにホストまたはセッションが表示されない

次を確認します。

1. `codex app-server daemon version` が実行中 daemon の情報を返す。
2. ホストがネットワークへ接続され、sleep していない。
3. ホストとモバイルが同じ ChatGPT account/workspace を使用している。
4. workspace policy で Remote Control が禁止されていない。
5. pairing code の期限内にセットアップを完了した。
6. 対象セッションと、その devcontainer がまだ動作している。

## セキュリティ

Remote Control のために App Server の Unix socket や WebSocket listener を public
network へ公開しないでください。Agent Grid の web UI や SSH を別途公開する場合は、
それぞれの認証と暗号化を独立して設定します。

## 関連資料

- [Codex CLI command reference](https://learn.chatgpt.com/docs/developer-commands?surface=cli#cli-codex-remote-control)
- [OpenAI Codex app-server-daemon README](https://github.com/openai/codex/blob/main/codex-rs/app-server-daemon/README.md)
- [ChatGPT Remote connections](https://learn.chatgpt.com/docs/remote-connections)
