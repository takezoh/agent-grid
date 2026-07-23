# Windows Shell + Electron Workspace 設計

- **作成日**: 2026-07-23
- **ブランチ**: `claude/native-clients-plan-review-2sjfs5`
- **ステータス**: draft (設計レビュー段階)
- **位置づけ**: [plan-20260723-native-clients.md](./plan-20260723-native-clients.md) Phase 2 (Windows デスクトップアプリ縦切りスライス) の詳細設計
- **影響範囲**: 新規 `clients/windows-shell/` (C#)、新規 `clients/workspace/` (Electron/TS)、`src/client/web` の hosted モード、配布/署名/更新

## 0. 構成の要約

Windows デスクトップは **2 コンポーネント + デーモン** で構成する。

```text
┌─ Agent Grid (単一インストーラ) ─────────────────────────────────┐
│                                                                  │
│  Shell (ネイティブ C# / WinUI 3)          常駐・軽量・即応       │
│  ├── 常駐パネル (トレイフライアウト + 上端フローティングバー)    │
│  ├── トースト通知 (承認/回答ボタン付き)                          │
│  ├── agent-grid:// deep link の登録先・ルータ                    │
│  ├── グローバルショートカット                                    │
│  ├── ジャンプバック (外部窓アクティベーション)                   │
│  └── デーモン監督 (起動・採用・ヘルス・更新時入替)               │
│                                                                  │
│  Workspace (Electron / TS)                オンデマンド起動       │
│  ├── セッションウィンドウ群 (1 セッション = 1 窓、再利用・復元)  │
│  └── 中身 = 既存 SPA (hosted モード): ターミナル / diff /        │
│      Markdown / ファイル                                         │
│                                                                  │
│  server.exe (既存 Go デーモン)            真実の源               │
│  └── 127.0.0.1:<port> で REST/WS 公開 (公開契約)                 │
└──────────────────────────────────────────────────────────────────┘
```

- **Shell と Workspace は対等なクライアント**。どちらもデーモンの公開契約 (REST/WS + bearer token) だけで話す。特権的裏口なし。
- **Shell は常駐**(ログイン時自動起動)、**Workspace はオンデマンド**(パネル/通知/deep link から起動・アクティベート)。
- **Electron はターミナル/ワークスペース側**に限定する。パネル・通知・deep link・デーモン監督は Shell (ネイティブ) が持ち、Electron は窓とレンダリングに集中する。

### なぜこの分担か

| 責務 | 担当 | 理由 |
|---|---|---|
| 常駐パネル | Shell (native) | 画面に常時出る面。非アクティベート窓 (`WS_EX_NOACTIVATE`)、Mica/Acrylic、フォーカス規律、メモリ (常駐が Electron だと待機だけで 150MB+)。デザイン品質 = 製品価値の場所 |
| トースト通知 | Shell | Windows App SDK `AppNotification` はボタン付きトースト (承認/拒否をトーストから直接) と COM 再活性化をネイティブに扱える |
| ターミナル/ワークスペース | Workspace (Electron) | xterm.js + Electron は VS Code が証明した業界標準構成。IME・キーボード・マルチウィンドウ管理・カスタムタイトルバーが TS で完結し、既存 SPA (React/TS) と言語・ツールチェーンを共有。Playwright for Electron でテスト可能 |
| デーモン監督 | Shell | 常駐している方が持つ。Workspace が閉じてもセッションは生きる、の実装点 |

## 1. 技術選定 (AGENTS.md ライブラリ選定規約に基づく比較)

### 1.1 Workspace ホスト: Electron vs WebView2 島 vs ブラウザ継続

| 候補 | 利点 | 欠点 |
|---|---|---|
| **Electron (採用)** | xterm.js 実績 (VS Code/Cursor)、マルチウィンドウ・タイトルバー・メニューを TS で所有、`focusable`/`alwaysOnTop` 等の窓制御 API が成熟、Playwright 公式サポート、SPA と同一言語 | 配布サイズ (~100MB)、窓ごとではなくアプリで ~150-250MB、Chromium 追従更新 |
| WinUI 3 + WebView2 島 | ランタイム同梱不要 (Evergreen)、シェルと同一アプリに統合可能 | マルチウィンドウ+WebView 管理を C# で書く量が多い、xterm.js/IME まわりの実績が Electron 比で薄い、web↔native ブリッジの型付けを自前整備 |
| ブラウザ継続 | 追加実装ゼロ | タブ埋没・窓規律・deep link 所有が実現できない (本プランの動機そのもの) |

**判断**: ターミナルが中核のワークスペースでは Electron の実績と TS 完結が勝つ。サイズ/メモリは Workspace がオンデマンド起動である (常駐しない) ことで許容する。

### 1.2 Shell スタック: WinUI 3 vs WPF vs Electron 常駐

| 候補 | 利点 | 欠点 |
|---|---|---|
| **WinUI 3 / Windows App SDK (採用)** | `AppNotification` (ボタン付きトースト + 再活性化)、Mica/Acrylic が一級、MSIX/protocol activation が素直、現行世代 API | トレイ公式 API なし (下記ライブラリで補完)、Win32 interop (`WS_EX_NOACTIVATE`) は HWND 直叩き |
| WPF | 枯れている、トレイ/レイヤード窓の情報が豊富 | 素材 (Mica 等)・通知が古い世代、新規採用の魅力薄 |
| Electron 常駐 | 言語統一 | 常駐で 150MB+、非アクティベートやトースト再活性化はどのみちネイティブ補助が要る。ユーザー決定によりターミナル側限定 |

補完ライブラリ: トレイは `H.NotifyIcon.WinUI` (MIT、メンテ活発) を第一候補、代替は Win32 `Shell_NotifyIcon` 直実装 (依存ゼロだが実装量増)。

### 1.3 Electron 側の主要依存

- ビルド/配布: `electron-builder` (NSIS target)。代替 `electron-forge` — updater 連携と NSIS 実績で builder を採用。
- 更新: `electron-updater`。ただし §6 の通り**バンドル一体更新は Shell が主導**し、electron-updater は使わない方向を第一案とする (二重更新経路を作らない)。
- Shell↔Workspace IPC: Node 標準 `net` (named pipe) + JSON Lines。フレームワーク不要のため追加依存なし (stdlib 優先の原則)。

## 2. デーモン接続 (両コンポーネント共通)

現行実装 (`src/cmd/server`) をそのまま使う。ゲートウェイは `-addr` (既定 `:8443`)、bearer token (`-token-file` で永続化可、`0600`)、ブラウザ WS 用の短命チケット (`/ws?ticket=`, 30s/単回) を持つ。

- Shell がデーモンを spawn する際の引数: `server.exe -addr 127.0.0.1:<port> -token-file %APPDATA%\agent-grid\gateway-token`。**`127.0.0.1` bind を明示する** (既定 `:8443` は全インターフェース bind のため、同梱運用では必ず loopback を渡す)。
- port は設定ファイル (`%APPDATA%\agent-grid\shell.json`) で固定 (既定 8443)。衝突時は次候補を試し、実際の port を state ファイルに書いて Workspace と共有する。
- **認証**: Shell / Workspace main はともに token-file を読み、`Authorization: Bearer` ヘッダで REST を叩く。WS は native からはヘッダ付与で接続できるためチケット不要の想定だが、ゲートウェイの WS 認証がチケット専用だった場合は「REST でチケット mint → `/ws?ticket=`」のブラウザと同じ 2 段で接続する (実装時に確認、どちらでも公開契約の範囲)。
- **採用 (adopt)**: 起動時に port の疎通 + `/api/sessions` を token で確認し、生きていれば spawn せず採用。token-file が無い/不一致なら異常系としてユーザーに提示 (勝手に kill しない)。
- **多重防止**: デーモン側の socket/lock (既存) に任せ、Shell は「spawn が即死したら採用を試みる」だけの単純リトライにする。

## 3. Shell 設計 (C# / WinUI 3)

### 3.1 プロセス構成とモジュール

単一プロセス。UI スレッド + バックグラウンドサービス群。

```text
clients/windows-shell/
├── AgentGrid.Shell/              # WinUI 3 app (エントリ、DI 組み立て)
│   ├── Panel/                    #   パネル UI (XAML)
│   └── TrayIcon/                 #   H.NotifyIcon 統合
├── AgentGrid.Shell.Core/         # UI 非依存ロジック (テスト対象の中心)
│   ├── GatewayClient/            #   生成 C# クライアント (契約層) の薄い所有
│   ├── SupervisionState/         #   セッション/承認/質問の view state (pure)
│   ├── DaemonSupervisor/         #   spawn/adopt/health/swap の状態機械 (pure) + I/O 殻
│   ├── DeepLinkRouter/           #   agent-grid:// → 行き先決定 (pure)
│   └── WorkspaceLauncher/        #   Workspace 起動/転送 (named pipe client)
└── AgentGrid.Shell.Platform/     # Win32 interop (NOACTIVATE, SetForegroundWindow, toast COM)
```

`SupervisionState` / `DaemonSupervisor` / `DeepLinkRouter` は **入力→出力の純粋関数 + 薄い I/O 殻**で書く (ARCHITECTURE.md の FC/IS 原則を C# 側にも適用)。デーモンの WS イベントは 1 本の受信ループが `SupervisionState.Reduce` に流し、UI はその snapshot を購読する。

### 3.2 パネル (トレイフライアウト + 上端フローティングバー)

- **形態**: 常時表示の細いバー (上端中央、`WS_EX_NOACTIVATE | WS_EX_TOOLWINDOW`、タスクバー非表示、全仮想デスクトップ表示)。クリック/ホットキーで下方向に展開。トレイアイコンからも同内容のフライアウト。
- **フォーカス 2 モード**:
  - **glance** (既定): 非アクティベート。マウスクリック (承認/拒否ボタン、ジャンプバック) はフォーカスを奪わず処理する。
  - **engage**: テキスト入力 (質問回答、短い指示) が必要な瞬間だけ `WS_EX_NOACTIVATE` を外してフォーカスを取り、確定/Esc で元の窓へフォーカスを返す。**返す先を覚えて返す** (`GetForegroundWindow` を engage 前に記録) のを契約とする。
- **表示内容**: セッション状態の要約 (running/waiting/failed/done のカウント + 直近)、承認キュー、質問キュー、各項目に [Approve][Deny] / [回答…] / [Jump back] / [Open]。
- **素材**: Acrylic (バーは半透明)、ダーク/ライト OS 追従。アニメーションは Composition API、目標 60fps・展開 150ms 級。

### 3.3 通知

- Windows App SDK `AppNotificationManager`。承認要求トーストに **[Approve] [Deny] ボタン**、質問トーストに inline textbox (Windows トーストの `<input>`) を付け、**トーストだけで往復完結**を第一動線にする。
- トーストのボタン押下はバックグラウンド COM 活性化で Shell に届く → GatewayClient 経由で公開契約の承認 API を叩く。Shell が落ちていれば起動される (MSIX の notification activation)。
- 本文タップは deep link 相当 (`DeepLinkRouter` に合流)。

### 3.4 Deep link とルーティング

- `agent-grid://` は **Shell が唯一の登録先** (MSIX manifest の protocol activation)。
- ルール (native-clients plan の窓規律に対応):
  - `agent-grid://approval/<id>` `…/question/<id>` → パネルを展開して該当項目へ (窓は開かない)
  - `agent-grid://session/<id>` → `WorkspaceLauncher` へ転送 (Workspace の該当窓をアクティベート、なければ起動)
  - `agent-grid://session/<id>/jump` → ジャンプバック (外部窓アクティベーション)
- ブラウザには一切フォールバックしない。

### 3.5 ジャンプバック

- `JumpBackService`: セッション → 外部ターゲット (Windows Terminal のタブ、VS Code 窓、WSL、UE/Blender) のアクティベーション。
- 実装は best-effort の段階制: (1) HWND 既知なら `SetForegroundWindow` (フォアグラウンド権限は `AllowSetForegroundWindow` / thread-input attach で確保)、(2) プロセス名 + タイトルマッチ、(3) 失敗時はパネルに「見つからない」を正直に出す (fabricated fallback 禁止の原則)。
- セッション ↔ ターゲットの対応はデーモン側メタデータ (起動時の cwd/コマンド) から推定し、`AgentGrid.Shell.Platform` に隔離。**契約層にはこの概念を漏らさない** (OS 統合はサーバー意味論を変えない、の実践)。

### 3.6 デーモン監督

状態機械 (pure): `NotRunning → Spawning → Healthy → Degraded → Swapping` + `Adopted`。

- 起動: adopt 試行 → 失敗なら spawn。ヘルスは `/api/sessions` 応答 (5s 間隔、backoff 付き)。
- **更新時入替 (swap)**: 新バイナリ配置 → 新旧のバージョン/契約互換を確認 → デーモンへ graceful shutdown 要求 → セッション永続化 (既存の restart 耐性 — c1d4157 で Codex セッションは restart を跨いで保持される — に乗る) → 新バイナリ起動 → Workspace/パネルは WS 再接続 (ADR-0025 の backfill 経路)。**実行中セッションが生きて戻ること**をこのフローの受け入れ条件にする。
- Shell 終了 (明示 Quit) でもデーモンは殺さない。「デーモンも停止して終了」は別メニュー項目として明示的に分ける。

## 4. Workspace 設計 (Electron / TS)

### 4.1 プロセス構成

```text
clients/workspace/
├── src/main/                     # Electron main
│   ├── window-registry.ts        #   session → BrowserWindow (1:1、再利用・復元)
│   ├── control-endpoint.ts       #   named pipe server (Shell からの命令受け口)
│   ├── daemon-config.ts          #   port/token の解決 (state ファイル読取)
│   └── app-menu.ts / titlebar    #   ネイティブメニュー、カスタムタイトルバー
├── src/preload/                  # contextBridge (typed、最小 API)
└── package.json                  # SPA は成果物参照 (下記 4.3)
```

- セキュリティ姿勢: `contextIsolation: true`、`nodeIntegration: false`、`sandbox: true`。preload が公開するのは `{ windowControls, hostedModeInfo (port/token/sessionId), jumpBack要求のShell転送 }` のみ。SPA から Node API には一切触れない。
- 単一インスタンス (`requestSingleInstanceLock`)。二重起動は既存インスタンスへの命令に変換。

### 4.2 ウィンドウ規律の実装

`window-registry.ts` が唯一の窓生成点。

- `openSession(id)`: 既存窓があれば `focus()`、なければ生成。**このモジュール以外から `new BrowserWindow` を呼ばない** (lint で禁止)。
- 窓レイアウト (bounds、session→窓、モニタ) を `%APPDATA%\agent-grid\workspace-state.json` に保存し、起動時に復元。存在しないセッションの窓は復元しない (デーモンに問い合わせてから復元)。
- 窓 close = ビューを畳むだけ。全窓 close でもアプリは終了せず常駐もしない — main は control-endpoint を残して待機し、無窓 5 分で自然終了 (Shell からいつでも再起動できるため)。

### 4.3 SPA の hosted モード

SPA (`src/client/web`) は**デーモンのゲートウェイから配信されている現行構成のまま** `http://127.0.0.1:<port>/?hosted=1&session=<id>` を BrowserWindow にロードする (ゲートウェイが SPA を配信していない場合はビルド成果物を Electron 同梱で `file://` ロードし、API base を注入する — 実装時にゲートウェイの静的配信有無を確認して片方に決める)。

hosted モードで SPA に入る変更 (mode flag で分岐、ブラウザ向けは不変):

1. **1 窓 1 セッションビュー**: タブ UI を出さず、指定セッションのワークスペースだけを描く。
2. **認証スキップ**: token は preload 経由で注入 (`hostedModeInfo`)。ブラウザの token 入力 UI を出さない。
3. **脱ブラウザ化**: ページ内ナビゲーション排除、ブラウザスクロールバー→OS 風、コンテキストメニューはネイティブ (`webContents` の menu へ委譲)、タイトルバードラッグ領域 (`-webkit-app-region`)。
4. **キーボード/IME**: アプリショートカットは main の `Menu` アクセラレータで所有し、xterm.js への素通しを既定にする (ブラウザのショートカット衝突が消えるのが hosted の利点)。
5. **テーマ**: `nativeTheme` 追従。

### 4.4 Shell↔Workspace IPC

- named pipe `\\.\pipe\agent-grid-workspace` 上の JSON Lines。コマンドは最小: `{op:"openSession", id}` / `{op:"activate"}` / `{op:"quit"}`。応答は `{ok}` / `{error}`。
- Workspace が起動していなければ Shell が exe を spawn → pipe 接続リトライ。
- **ドメインデータはこの pipe に流さない** (セッション状態や承認は各自がデーモンから取る)。pipe は「窓を出せ」だけの制御線。gateway no-domain 原則のミニチュア。

## 5. 責務境界のまとめ

| 関心事 | Shell | Workspace | デーモン |
|---|---|---|---|
| セッション/承認/質問の真実 | — | — | ✅ |
| 承認/回答の実行 | ✅ (パネル/トースト) | ✅ (SPA 内) | 受理・裁定 (二重応答は契約の conflict-resolution) |
| 窓の生成・再利用・復元 | — | ✅ WindowRegistry | — |
| deep link 登録・ルート | ✅ | 受動 (pipe 経由) | — |
| 通知 | ✅ | — | イベント発出のみ |
| デーモン lifecycle | ✅ | — (接続するだけ) | — |
| ジャンプバック | ✅ | 要求を Shell へ転送 | メタデータ提供 |

二重応答 (トーストと SPA で同じ承認に同時応答) は**クライアント側で防がない**。契約層の approval-contract (単回裁定、敗者には resolved-by-other が返る) に委ね、両 UI は結果を表示するだけにする。

## 6. 配布と更新

- **単一インストーラ**: `AgentGridSetup.exe` (NSIS) が Shell (MSIX or 素の exe)、Workspace (Electron)、`server.exe`、web アセットを一括導入。MSIX 単独案は Electron 側の書込み挙動と相性を検証してから (未決事項 §9)。
- **署名**: コード署名証明書 1 枚で 3 実行体 (installer / shell / workspace) と server.exe に署名。
- **更新は Shell が単独で主導**: 更新チェック → バンドル DL → 検証 → Workspace へ `quit` → デーモン swap (§3.6) → 自身の再起動。electron-updater は使わず**更新経路を 1 本**に保つ。
- **自動起動**: Shell のみスタートアップ登録。Workspace とデーモンは Shell が起こす。

## 7. テスト戦略

| 対象 | 手法 |
|---|---|
| Shell Core (SupervisionState / DaemonSupervisor / DeepLinkRouter) | xUnit。pure 状態機械としてイベント列→状態/効果を検証 (FC/IS なので殻なしで回る) |
| DaemonSupervisor 統合 | fake `server.exe` (即応答する小さな Go バイナリ、`src/server/web/testsupport/fakeagents` の流儀を踏襲) に対する spawn/adopt/swap |
| Workspace main (window-registry, control-endpoint) | vitest (unit) + **Playwright for Electron** で窓規律 e2e: 再アクティベートで窓が増えない、復元、pipe 経由 openSession |
| hosted モード SPA | 既存 `src/client/web` の unit + Playwright に hosted フラグの分岐ケースを追加 (`e2e/support/fake-backend.ts` 再利用) |
| 契約適合 | Shell は生成 C# クライアント、Workspace/SPA は生成 TS クライアント経由に限定し、記録済みシナリオ (native-clients plan Phase 1) を両方に流す |
| トースト/COM 活性化・NOACTIVATE 実挙動 | 自動化困難につき手動チェックリスト + スクリーンショット成果物 (T3 相当、opt-in) |

## 8. 実装順 (Phase 2 内スライス)

1. **S1 — 接続と監督**: Shell 骨格 + DaemonSupervisor (spawn/adopt/health) + トレイ常駐。UI はトレイメニューのみ。
2. **S2 — パネル glance**: バー + フライアウト、セッション状態表示、Jump back (Windows Terminal のみ)。
3. **S3 — 承認往復**: 承認/質問の表示と応答 (パネル + トースト)。※ Phase 0 の approval/question ドメイン完成が前提。
4. **S4 — Workspace**: Electron 骨格 + WindowRegistry + hosted モード最小 (1 窓 1 セッション、認証注入) + pipe 連携 + deep link。
5. **S5 — 製品化**: インストーラ、署名、更新フロー (デーモン swap 含む)、窓復元、脱ブラウザ化仕上げ。

S1–S2 は Phase 0/1 と並行可能 (既存 API のみで動く)。S3 が契約層との合流点。

## 9. 未決事項

1. ゲートウェイの WS 認証がヘッダ bearer を受けるか (チケット専用か) — 実装時確認。native クライアントにはヘッダ許可を足すのが素直 (公開契約の追記)。
2. ゲートウェイが SPA を静的配信しているか → hosted モードのロード元 (gateway URL vs 同梱 file://) を確定。
3. MSIX 化の範囲 (Shell のみ MSIX + 他は NSIS、の混成が現実的か)。
4. パネルのテキスト入力 (engage モード) のフォーカス返却先の端ケース (対象窓が閉じた場合など)。
5. トースト inline 回答の文字数/IME 制約 — 超えたらパネル展開へフォールバック。
6. `server.exe` の Windows ネイティブ動作検証 (現行は Unix socket 前提の箇所がある — `~/.agent-grid/server.sock`。Windows では named pipe 化または TCP loopback 専用化が要る。**これは Phase 0 で洗い出す移植項目**)。

## 10. リスク

- **`server.exe` の Windows 移植が隠れた最大工数** (§9-6)。Unix socket / pty / sandbox まわりの Windows 対応 (ConPTY、WSL 内実行への委譲) は本設計の前提であり、早期に spike する。最悪ケースの逃げ道は「デーモンは WSL 内で動かし、Shell/Workspace は Windows 側から loopback 接続」— この構成でも本設計は成立する。
- **トースト COM 活性化の複雑さ** — MSIX パッケージングと密結合。S3 で最初に通す。
- **hosted モードの「額縁の中のサイト」感** — §4.3 の 5 項目を exit 基準に含め、デザインレビューを S5 で必須化。
- **二重クライアントの応答競合** — 契約層に委ねる設計 (§5) を崩さない。クライアント側の楽観ロック等を足し始めたら設計違反。
