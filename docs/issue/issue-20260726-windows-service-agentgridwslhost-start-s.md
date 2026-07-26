---
id: issue-20260726-windows-service-agentgridwslhost-start-s
kind: issue
title: Windows Service (AgentGridWSLHost) Start-Service fails to log on under Microsoft-account-linked
  local account
status: open
created: '2026-07-26'
issue_type: bug
tags: []
owners: []
relations: []
subject_paths:
- deploy/windows/register-wsl-host-service.ps1
- deploy/windows/register-wsl-host-service.bat
- docs/bugs/windows-service-logon-failure.md
---

## 症状 / 機会

`register-wsl-host-service.ps1 -Action Install` は `New-Service` まで成功する（`identity=DESKTOP-PJNFA6R\takeg` でサービス `AgentGridWSLHost` が `Stopped` 状態で作成される）が、続く `Start-Service` が一貫して失敗する。対象アカウント `takeg` は `Get-LocalUser` 上はローカルアカウントだが `PrincipalSource=MicrosoftAccount`（Microsoft アカウント連携）。

## 証拠

System イベントログ (Service Control Manager, Event ID 7038):
```
The AgentGridWSLHost service was unable to log on as .\takeg with the currently configured password due to the following error:
The user name or password is incorrect.
```

同一パスワードで試したが解決しなかった組み合わせ (いずれも同一エラー):
- 通常のサインインパスワード (複数回、別々のタイミング)
- 同一パスワードでの interactive logon (`Start-Process -Credential`) — service logon 固有の問題ではないことを示す
- 「このデバイスでは Microsoft アカウントに Windows Hello サインインのみ許可する」を Off にした後
- 2段階認証用アプリパスワード (クラウド側の別認証経路であり誤った仮説だった可能性が高い)
- ロック画面でパスワード入力して解除した直後 (CloudAP 経由の検証で、ローカル SAM の NTLM ハッシュ更新には寄与しない可能性)

`/debug` skill (dev:debug-investigator ⇄ dev:debug-causal-critic、2周) + 実機ライブ診断で調査。詳細な調査記録: `docs/bugs/windows-service-logon-failure.md`。

同一セッションで副次的な実バグは特定・修正済み (commit `dfb9964c`, GitHub Issue https://github.com/takezoh/agent-grid/issues/31 にも記録): `.bat` ランチャーの引数転送バグ、fire-and-forget な `ShellExecute` 昇格、`icacls` の ID 解決失敗時のエラーメッセージ不明瞭、`New-Service` のベアなアカウント名拒否、`SeServiceLogonRight` 未付与、`secedit` の UTF-16LE エンコーディング未対応。これらは根本原因ではなく、調査の過程で見つかった別問題。

## 着手に必要な文脈

未検証の次の一手候補: 設定 > アカウント > サインイン オプション > パスワード > 「変更」を明示的に通し (ロック画面解除ではなく)、その後**サインアウト→サインイン**してから再試行する。Task Scheduler で同種の症状報告があり、パスワード変更後のサインアウト/サインインで解消したという複数の非公式報告がある一方、Microsoft 公式ドキュメントはこの MSA 連携ローカルアカウント × サービスログオンの組み合わせを明示的に扱っていない。

それでも解決しない場合、MSA 連携アカウントを Windows Service アカウントとして使うことの可否を確定情報として得る手段がない (公式ドキュメントに明記なし)。

恒久対応候補: `register-wsl-host-service.ps1` の `Get-Credential` のユーザー名欄が、同一セッション中に4回連続でメールアドレスへの誤入力を誘発した。ユーザー名欄を廃止しパスワードのみ入力する形への変更を一度実装したが、ユーザーの `戻して` 指示により revert 済み。再検討の価値がある。
