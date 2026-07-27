# clients/

ユーザー向けクライアントの置き場。実行プレーン (`src/`, Go module) とは独立に、各クライアントが自分のツールチェーン (npm / dotnet / Xcode / Gradle) でビルド・テストする。

すべてのクライアントはデーモンの**公開契約** (`protocol/` + `contracts/`) 経由でのみ通信する。特権的な裏口 API を持つクライアントを作らない。

| ディレクトリ | 中身 | 状態 |
|---|---|---|
| `ui/` | 共有 SPA (React/TS/xterm.js)。Web host と Electron workspace の両方が使う UI 資産 | client |
| `web-host/` | SPA の配信と、設定済み server fleet への server-qualified HTTP/WS transport を所有する独立 Go module | client host |
| `config/` | 全 client host が読む `servers.json` schema/fixture の正本 | shared client contract |
| `windows-shell/` | 常駐ネイティブシェル (C# / WinUI 3): パネル・通知・deep link・デーモン監督 | Phase 2 Core/Platform 着工済み (`dotnet test`; WinUI host は Windows 開発機) |
| `workspace/` | セッションウィンドウホスト (Electron / TS)。SPA を hosted モードで表示 | Phase 2 main/preload 骨格 + vitest 着工済み |
| `apple/` | Swift package + iOS / macOS ターゲット | 未着工 (Phase 3/4) |
| `android/` | Kotlin / Jetpack Compose | 未着工 (Phase 4/5) |

ここに置かないもの: デーモン・ゲートウェイ・orchestrator (→ `src/`)、スキーマ正本 (→ `protocol/`)、契約文書 (→ `contracts/`)。生成クライアントコードは各クライアント配下 (例: `ui/src/gen/`) に置き、正本→生成の一方向を守る。

Web と Electron は対等な client host である。両者は同じ複数-server 設定、server-qualified identity、partial-failure semantics を提供し、相違は Web の same-origin HTTP/WS と Electron の preload/IPC という delivery transport に限る。Web を単一 server とみなすこと、Electron にだけ fleet 処理を持たせること、暗黙の default server を合成することは禁止する。

設計文書: [plans/plan-20260723-native-clients.md](../plans/plan-20260723-native-clients.md) · [plans/plan-20260723-windows-shell-design.md](../plans/plan-20260723-windows-shell-design.md) · [plans/plan-20260723-repo-structure.md](../plans/plan-20260723-repo-structure.md)
