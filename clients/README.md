# clients/

ユーザー向けクライアントの置き場。実行プレーン (`src/`, Go module) とは独立に、各クライアントが自分のツールチェーン (npm / dotnet / Xcode / Gradle) でビルド・テストする。

すべてのクライアントはデーモンの**公開契約** (`protocol/` + `contracts/`) 経由でのみ通信する。特権的な裏口 API を持つクライアントを作らない。

| ディレクトリ | 中身 | 状態 |
|---|---|---|
| `ui/` | 共有 SPA (React/TS/xterm.js)。ブラウザと Electron workspace の両方が使う UI 資産。配信機は `src/uihost` (`cmd/uihost`) | 移設済み (旧 `src/client/web`) |
| `windows-shell/` | 常駐ネイティブシェル (C# / WinUI 3): パネル・通知・deep link・デーモン監督 | 未着工 (native-clients plan Phase 2 S1) |
| `workspace/` | セッションウィンドウホスト (Electron / TS)。SPA を hosted モードで表示 | 未着工 (Phase 2 S4) |
| `apple/` | Swift package + iOS / macOS ターゲット | 未着工 (Phase 3/4) |
| `android/` | Kotlin / Jetpack Compose | 未着工 (Phase 4/5) |

ここに置かないもの: デーモン・ゲートウェイ・orchestrator (→ `src/`)、スキーマ正本 (→ `protocol/`)、契約文書 (→ `contracts/`)。生成クライアントコードは各クライアント配下 (例: `ui/src/gen/`) に置き、正本→生成の一方向を守る。

ブラウザクライアントは専用ディレクトリを持たない: 実体は「`uihost` の origin をブラウザで直接開く」配信モードであり、固有コードは `src/uihost` そのもの。

設計文書: [plans/plan-20260723-native-clients.md](../plans/plan-20260723-native-clients.md) · [plans/plan-20260723-windows-shell-design.md](../plans/plan-20260723-windows-shell-design.md) · [plans/plan-20260723-repo-structure.md](../plans/plan-20260723-repo-structure.md)
