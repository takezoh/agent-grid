# Repro: Windows Service logon failure after successful registration (2026-07-26 / take)

## 症状
`register-wsl-host-service.ps1 -Action Install` は `New-Service` まで成功する（サービス `AgentGridWSLHost` が `Stopped` 状態で作成される）が、直後の `Start-Service` が失敗し、System イベントログ (Service Control Manager, Event ID 7038) に「The user name or password is incorrect.」が記録される。

## 環境 (必須)
- OS: Windows 10/11 build 10.0.26100.8894
- Runtime: Windows PowerShell 5.1.26100.8894 (elevated, `Start-Process -Verb RunAs -Wait` 経由で `register-wsl-host-service.bat` から起動)
- Version / branch / commit: `main` @ 6e0d3e1e + 作業ツリーの未コミット変更 (`deploy/windows/register-wsl-host-service.ps1`, `deploy/windows/register-wsl-host-service.bat`, `docs/note/note-20260726-user-windows-wsl-service.md`, `services/internal/harnesspolicy/windows_service_deploy_test.go`, `services/platform/wslhost/acl_windows.go`)
- 依存: なし (Windows 標準の `sc.exe` / `secedit.exe` / `New-Service` cmdlet のみ)
- その他: 対象アカウントは local account `DESKTOP-PJNFA6R\takeg` (SID `S-1-5-21-4034758344-1153060747-2724949352-1001`)。同一デバッグセッション内でこの `.ps1` に対し ACL 識別子解決・New-Service 用アカウント名の完全修飾化・`SeServiceLogonRight` の自動付与、の3件の修正を先に適用済み。

## 再現手順 (必須)
1. 非管理者シェルから `deploy/windows/register-wsl-host-service.bat -Action Install -SupervisorExe 'C:\Program Files\AgentGrid\agent-grid-wsl-supervisor.exe' -Distribution Ubuntu-22.04 -LinuxUser take -ServerExecutable /home/take/.local/bin/agent-grid-server -DataDir /home/take/.agent-grid -Addr 127.0.0.1:8443 -TLSMode insecure` を実行する。
2. `.bat` が `Start-Process -Verb RunAs -Wait` で自己昇格し、`Get-Credential -UserName 'takeg'` の GUI ダイアログが表示される。
3. ダイアログでパスワードを入力し OK を押す。
4. 登録ログに `registered generation=... identity=DESKTOP-PJNFA6R\takeg distribution=Ubuntu-22.04` が出力され、`Get-Service AgentGridWSLHost` が `Status=Stopped` を返す（`New-Service` 自体は成功）。
5. スクリプトが続けて実行する `Start-Service -Name AgentGridWSLHost` が失敗する。
6. `Get-WinEvent -FilterHashtable @{LogName='System'; ProviderName='Service Control Manager'}` を確認すると、Event ID 7038、本文 "The AgentGridWSLHost service was unable to log on as .\takeg with the currently configured password due to the following error: The user name or password is incorrect." が記録されている。

## 発生頻度
不明（bug-report に記載された1回の観測のみ。反復試行の頻度データはない）。

## 期待 / 実際
期待: `Start-Service` が正常にサービスを起動する。
実際: `Start-Service` が失敗し、SCM が Event 7038 で「ユーザー名またはパスワードが正しくありません」(`ERROR_LOGON_FAILURE` 相当) を報告する。

## 特定した原因 (RCA) [2026-07-26 再調査で更新]
investigation-2.json 参照 (`/home/take/.dev-skills/debug/windows-service-logon-failure/investigation-2.json`)。要約 (更新版): conductor が実機で採取した2件の live 観測により分類が変わった。(1) `Get-LocalUser takeg` は `PrincipalSource=MicrosoftAccount, PasswordRequired=True, Enabled=True` を返し、対象アカウントが Microsoft アカウント連携アカウントであることを確認した。MSA 連携アカウントはパスワードベースの `LOGON32_LOGON_SERVICE` と互換性がないことが文書化された既知の Windows 制限であり、操作者が正しい (現在の) サインインパスワードを入力しても、SCM の `LogonUser` 検証に使われるローカル検証可能な NTLM/Kerberos secret と一致しない可能性がある。(2) 時刻/Id でスコープした `Get-WinEvent` は2件の独立した新鮮な 7038 (RecordId=274913 @ 14:26:55 / RecordId=274929 @ 14:37:16) を返し、これは「同一イベントの重複読み取り」ではなく、2回の独立した (別々に再入力された) パスワードそれぞれで発生した新鮮な失敗であることを示す。この2点により、空パスワード拒否 (`PasswordRequired=True` により排除) と「operator のタイプミス」(2回の独立した新鮮な失敗を説明するには不自然な偶然の重複タイプミスを要求する) の両方より、MSA 連携アカウント不整合が主要因として支持される。加えて、`register-wsl-host-service.ps1:167-198` の `ShouldProcess` 本体に `try/catch/finally` が一切なく、`Start-Service` 失敗時にサービスが前 run の credential を保持したまま残存する構造的欠陥を静的読解で確認した (今回観測された2件の7038の直接原因ではないが、独立した実在の欠陥として記録)。分類は `usage_or_environment` から `true_bug` へ変更 (`responsibility_checks.gap=true` へ変更、`implicit_contract=true` は維持)。

## 修正 (fixer 段で追記)
(このスキル呼び出しは investigate のみ。修正は未適用。分類は `true_bug` — investigation-2.json の blast_radius に修正候補 (L71-82 へのアカウント種別事前検証、L167-198 への rollback 追加) を記録済み。recurrence を「同一契約の4回目」と判定したため、fixer/user へは構造的選択肢の提示を推奨し、単純な5回目のローカルパッチのみでの着地は推奨しない。)

## テストを書けなかった理由
症状は実機 Windows の Service Control Manager によるライブのログオン検証 (`LogonUser` 経由の認証) にのみ現れ、リポジトリの Go/PowerShell テストハーネスは Windows SCM のライブ認証をシミュレートできない (fake も存在しない)。今回主要因と判定した「アカウントが MSA 連携かどうか」はこの実機固有の環境事実であり、モックによる red test では実際の欠陥を証明できない (no-fabricated-tests 方針に抵触する)。

## Validation 方法 (テスト代替)
- 静的検証: `Assert-InstallParameters` / `New-AgentGridService` のコードパスを読み、`$Credential.Password` (SecureString) を変換・複製・再構築する経路が存在しないことを再確認済み。
- live 観測: `Get-LocalUser takeg | Select-Object Name,PrincipalSource,PasswordRequired,Enabled` → `PrincipalSource=MicrosoftAccount, PasswordRequired=True`。`Get-WinEvent -FilterHashtable @{LogName='System';ProviderName='Service Control Manager';Id=7038;StartTime=(Get-Date).AddHours(-2)}` → 2件の独立した RecordId (274913, 274929)。
- 既存の contract test `services/internal/harnesspolicy/windows_service_deploy_test.go::TestWindowsServiceRegistrationAssets` はスクリプト本文の文字列 pin のみを行い、ライブのログオン成否やアカウント種別検証・rollback 挙動は検証しない — investigation-2.json の harness_gaps に記録済み。
- **決定的観測1 (2周目 causal-critic の指摘により実施)**: 同じ資格情報で service logon とは別の logon type (`Start-Process -Credential -FilePath cmd.exe -ArgumentList '/c','exit'`, interactive logon 相当) を試したところ、**同一の「The user name or password is incorrect.」で失敗**した。これは「MSA 連携アカウントは `LOGON32_LOGON_SERVICE` に特有に非互換」という investigation-2.json の主要因仮説を **反証** する (真に service logon 固有の非互換なら interactive logon は成功するはず)。investigation-2.json の RCA はこの観測により更新が必要 (未反映)。
- **決定的観測2 (conductor が Web 検索と実機の設定画面キャプチャで確認、最終確定)**: Microsoft 公式情報では「MSA のパスワードはローカル認証にも通常使える (protector 機構経由)」とあり、「MSA は原理的に LogonUser 系 API を使えない」という一般化は不正確と判明。代わりに、実機の `設定 > アカウント > サインイン オプション > 追加設定` を確認したところ、**「このデバイスでは Microsoft アカウントに Windows Hello サインインのみ許可する (推奨)」が On** であることを確認した (ユーザーがスクリーンショットで確認、2026-07-26)。この設定はデバイス全体でパスワードベースのサインインを無効化するため、`LogonUser` 経由の認証 (service logon・interactive logon 問わず) が正しいパスワードを入力しても一律に失敗する。観測1・観測2 を突き合わせることで根本原因が確定した。

## 最終確定した根本原因 (2026-07-26)
`takeg` は Microsoft アカウント連携のローカルアカウントであり、このデバイスには「Microsoft アカウントに Windows Hello サインインのみ許可する」ポリシーが有効になっている。このポリシーはパスワードベースの `LogonUser` 認証をデバイス全体で無効化するため、`register-wsl-host-service.ps1` が要求するパスワード資格情報方式のサービスアカウントとして `takeg` を使うことは、コードの修正では解決できない環境制約である。
分類: `usage_or_environment`(環境制約により、この特定アカウントでは達成不可能な要求) — ただし `register-wsl-host-service.ps1:167-198` の rollback 欠如 (try/catch/finally 不在) は独立した実在の副次的欠陥として別途記録する。
対処: (a) 該当ポリシーを無効化してこのデバイスの MSA アカウント全体でパスワードサインインを許可する (セキュリティ上のトレードオフを伴うため非推奨)、または (b) MSA と無関係な専用ローカルアカウントを新規作成し、そのアカウントを WSL distribution 所有者兼サービスアカウントとして使う (推奨、ドキュメントの元々の例 `agentgrid` もこの想定)。
