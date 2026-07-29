---
id: note-20260729-github-oauth-app-operations
kind: note
title: GitHub OAuth App operations
status: published
created: '2026-07-29'
tags:
- github
- oauth
- operations
owners: []
relations:
- {type: references, target: adr-20260728-client-host-github-device-flow}
- {type: originatedFrom, target: change-20260728-github-repository-sessions}
source_paths:
- clients/config/github.schema.json
- clients/workspace/src/main/github-host.ts
- clients/web-host/internal/githubhost/githubhost.go
summary: Operate the product GitHub OAuth App registration, inject its public client
  ID consistently, validate Device Flow before release, and rotate or recover without
  exposing credentials.
updated: '2026-07-29'
---

## Summary

Agent Grid の GitHub account と repository catalog は、製品所有の GitHub
OAuth App と OAuth 2.0 Device Flow を使用する。利用者が client ID や
client secret を Settings へ入力する方式ではない。製品運用者は OAuth App
registration、公開 client ID の製品組み込み、release 前の実 API 検証、
rotation、障害対応を所有する。

2026-07-29 現在、source と配布済み Workspace には
`Ov23liAgentGridDeviceFlow` が固定されているが、この identity は GitHub の
device-code endpoint から拒否される。Settings の **Connect** と New Session
の **Sign in to GitHub** は同じ authority を使用するため、正式な registration
を設定した build を配布するまでどちらも利用できない。

{% adr-ref id="adr-20260728-client-host-github-device-flow" /%}

## Ownership

Production registration には個人所有ではなく、継続的に管理できる GitHub
Organization 所有を推奨する。release 前に次の役割を実名または team 名で
割り当てる。

| Role | Responsibility |
|---|---|
| Registration owner | OAuth App の作成、Device Flow 設定、移管、削除 |
| Release owner | 正式 client ID の組み込み、実 API gate、配布物の照合 |
| Security owner | scope、credential handling、incident、rotation の承認 |

client ID は公開 identifier であり secret ではない。client secret はこの
Device Flow 実装では不要であり、repository、設定ファイル、build log、
application log のどこにも保存しない。access token、refresh token、
device code、user code も運用記録へ貼り付けない。

## Create the production OAuth App

GitHub の
[Creating an OAuth app](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app)
に従う。

1. 所有 Organization の **Settings → Developer settings → OAuth apps** を開く。
2. **New OAuth App** を選択する。
3. Application name と公開可能な Homepage URL を設定する。
4. Authorization callback URL には Organization が管理する HTTPS URL を設定する。
   Agent Grid の現行 Device Flow は callback を使用しないが、GitHub の
   registration form では値が必要になる。
5. **Enable Device Flow** を有効にする。
6. registration を作成し、表示された client ID を release owner へ渡す。
7. owner、registration URL、作成日、Device Flow の有効状態を
   Organization の非公開運用台帳へ記録する。client secret は生成・共有しない。

GitHub の
[Device Flow documentation](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow)
では、利用前に app settings で Device Flow を有効にすることが必須である。

## Product configuration

Canonical contract は [github.schema.json](../../clients/config/github.schema.json)
で、値は次を満たす。

```json
{
  "schema_version": 1,
  "client_id": "<GitHub が発行した公開 client ID>",
  "scopes": ["read:user", "repo"]
}
```

`repo` scope は private repository を catalog と clone の対象に含めるために
必要である。公開 repository のみに機能を縮小する仕様変更なしに scope を
削除しない。

現行実装には production client ID を外部設定から注入する完成済み経路がない。
したがって、正式 client ID の取得後は次の実装変更と再配布が必要である。

1. Workspace と Web Host が読む production OAuth config の SSOT を定める。
2. build/package 時に client ID を必須入力とし、空値、fixture 値、既知の
   placeholder を fail closed で拒否する。
3. Workspace、Web Host、config fixture が同一 identity と scope を使うことを
   contract test で固定する。
4. 配布物に組み込まれた client ID を release gate が抽出し、入力値と照合する。

手作業で
[github-host.ts](../../clients/workspace/src/main/github-host.ts) と
[githubhost.go](../../clients/web-host/internal/githubhost/githubhost.go) の literal
だけを書き換えて release してはならない。設定の再分散と検証漏れが再発する。

## Release validation

### T0–T2: deterministic gates

- config schema が client secret と token fields を拒否する。
- Web Host と Workspace の account projection が同じ contract を満たす。
- fake は GitHub の成功・pending・denied・expired・slow-down・invalid
  responses を再現する。
- placeholder、空値、Web/Workspace identity 不一致で build または package を
 失敗させる。

### T3: real authority gate

正式 client ID を明示して、両 host の既存 FakeVsReal backstop を実行する。

```sh
AG_E2E_GITHUB_OAUTH_CLIENT_ID=<client-id> \
  npm --prefix clients/workspace test -- github-host.test.ts -t FakeVsRealGitHubAuth

cd clients/web-host
AG_E2E_GITHUB_OAUTH_CLIENT_ID=<client-id> \
  go test ./internal/githubhost -run TestFakeVsRealGitHubAuth
```

合格条件は、GitHub が non-empty `device_code`、`user_code`、
`verification_uri`、正の `expires_in` と `interval` を返すこと。テスト出力を
CI artifact として保存する場合も、code 値は redact する。real authority が
失敗した場合は fake や assertion を弱めず、registration または fake を直す。

### Packaged Windows smoke

1. Release build を Windows へインストールする。
2. Settings の GitHub **Connect** で verification URI と user code が表示される。
3. 同じ transaction を cancel または expire させ、typed recovery が表示される。
4. New Session の **Sign in to GitHub** でも同じ authorizing state へ遷移する。
5. GitHub で承認し、repository catalog が取得できる。
6. private repository が見えることと、Projects source が失敗の影響を受けないことを確認する。

## Rotation

GitHub OAuth Appの client ID は同じ registration のまま通常ローテーション
できない。registration を置き換える場合は app identity の migration として扱う。

1. 新しい Organization-owned OAuth App を作成して Device Flow を有効にする。
2. 新 client ID で T0–T3 と packaged smoke を完了する。
3. 新 identity を組み込んだ全 host を同一 release で配布する。
4. 既存 token は旧 app identity に属するものとして破棄し、利用者へ再認証を求める。
5. rollback window 終了後に旧 registration を無効化または削除する。
6. 運用台帳、release evidence、incident record を更新する。

## Failure response

| Observation | Operator action |
|---|---|
| Device-code start が 404 または credential error | 配布物の client ID、registration の存在、所有者、Device Flow 設定を照合する |
| `device_flow_disabled` | OAuth App settings で Device Flow を有効化し、T3を再実行する |
| `authorization_pending` | 正常。GitHub が返した polling interval を守って継続する |
| `slow_down` | polling interval を GitHub の指示どおり増やす |
| `expired_token` | transaction を破棄し、利用者に再開始を提示する |
| token取得後にrepository取得が403 | scope、Organization policy、SSO authorization、利用者権限を確認する |

GitHub 障害時も Projects source は利用可能なまま維持する。token や code を
診断ログへ追加してはならない。

## Release checklist

- [ ] Organization-owned OAuth App と担当 owner が存在する
- [ ] Device Flow が有効
- [ ] 正式 client ID が SSOT に設定され、placeholder guard が通る
- [ ] Workspace、Web Host、fixture、配布物の identity が一致
- [ ] T0–T2 が成功
- [ ] Workspace と Web Host の T3 が成功
- [ ] Packaged Windows smoke が成功
- [ ] token/codeを含まないrelease evidenceを保存
- [ ] rotation と incident の担当者が確定
