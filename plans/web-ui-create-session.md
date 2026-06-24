# Handoff: Web UI session 作成フォームの UX 修正

> Status: **未着手** (作成 2026-06-22) · Branch: `feat/tmux-free-web-server`
> 直近 backend 修正: commit `f83047d` (POST /api/sessions の 500/502 を grep-able 化 + spawn mode 追加 + `project` 絶対パス強制)
> 引き継ぎ元コンテキスト: ユーザーの incident report → backend 側修正 → frontend 側の UX 問題が露呈
> 関連: [`docs/user/web-server.md`](../docs/user/web-server.md) · [`docs/technical/web-gateway.md`](../docs/technical/web-gateway.md)

---

## 1. ゴール

`src/client/web/src/components/CreateSessionForm.tsx` が **「project (working directory) と command を入力する UI として成立していない」** 状態にある。これを「ユーザーが意図通りに session を作れる」状態に直す。

直近の backend 強化 (`isAbsoluteProjectPath` validator + 全 ErrCode の HTTP マッピング) によって、フォームの誤入力が「無音の 502 / 201-then-die」から「明示的な 400」に変わった。エラー出口が綺麗になったので、入口 (UI) を整える時。

---

## 2. 現状の事実 (証拠付き)

### 2.1 フォーム実装 (`CreateSessionForm.tsx:1-60`)

```tsx
const [title, setTitle] = useState("");

<input
  type="text"
  value={title}
  placeholder="New session title"   // ← ユーザーには「自由テキストの label」と見える
/>

body: JSON.stringify({
  project: title.trim(),            // ← 実際は project (= working directory) として送信
  command: "claude"                 // ← 完全にハードコード
})
```

### 2.2 backend 側の挙動 (commit `f83047d` 反映後)

| input | レスポンス | reason |
|---|---|---|
| `{project:"abcd", command:"claude"}` | **400** | `project_not_absolute` (gateway validator が即弾く) |
| `{project:"abcd", command:"shell"}` | **400** | 同上 |
| `{project:"/abs/path", command:"claude"}` | 201 → spawn 失敗時は 502 with `reason=daemon_internal` | `tmux spawn failed: …` (devcontainer 等) |
| `{project:"/abs/path", command:"<未登録>"}` | **422** | `unsupported` (driver 未登録) |
| `{project:"/abs/path", command:"shell"}` | 201 | 通常 |

X-Request-Id ヘッダ + body 末尾 `(request_id=…)` で server.log と相関可能。

### 2.3 既存テスト

- `CreateSessionForm.test.tsx`: 1 ケース ("posts /api/sessions and selects returned id") のみ
- form の input は単一、現在の挙動を前提にしている

---

## 3. 問題の分解

| # | 問題 | 影響 |
|---|------|------|
| P1 | `project` field の意味が UI とコード間で完全にズレている (label vs working directory) | ユーザーが任意の文字列を入れて 400 になる |
| P2 | `command` が hardcode `"claude"` で UI から選択不能 | shell / codex / その他 driver で session を作れない |
| P3 | "title" 的な session 表示名を別途持つ抽象が backend に存在しない | UI で label を出したくても project (path) しか頼れない |
| P4 | error メッセージの surface 経路: フォームは `Error("POST /api/sessions failed: <status>")` で status だけ捨てている | せっかく backend が `(request_id=…)` 付き具体的 message を返しても見えない |

---

## 4. 解決策の選択肢

### A. 最小修正 (1 PR、~1 時間)
1. placeholder を `"Project directory (absolute path, e.g. /home/me/myrepo)"` に変更
2. submit 前に client-side で `value.startsWith("/")` チェック → 即 inline error
3. error 表示は `await resp.text()` で body をそのまま見せる (request_id 含む)
4. command は引き続き `"claude"` 固定

**trade-off:** P1 の表面症状は緩和。P2/P3 は未解決。

### B. 中規模修正 (1-2 PR、~半日)
A に加えて:
1. `<select>` で command を選ばせる。選択肢は固定 hardcode (`claude` / `shell` / `codex`) か、もしくは新 endpoint `GET /api/drivers` (state 側に `RespDriverList` あり、proto 対応済み) を叩いて動的に
2. (任意) ディレクトリ選択 hint: 過去に作った session の project list (`RespSessions.Sessions[].Project`) を datalist で suggest

**trade-off:** P1/P2 解決、P3 は残る。`RespDriverList` を web 経由で取得する endpoint が無いので gateway 側に `GET /api/drivers` 追加が必要 (~30 行)。

### C. proper 修正 (複数 PR)
B に加えて:
1. `state.Session` に `Label string` field 追加 (project と独立)
2. proto `SessionInfo.Label` 追加
3. reducer + persistence 更新 (snapshot 互換に注意)
4. form を 2 input にする (Label + Project path)
5. TUI 側の card 表示も Label 優先に切り替え

**trade-off:** 全問題解決。実装範囲が広く ADR 級。

---

## 5. 推奨

**A → B → (必要なら) C の段階適用**。理由:

- A は backend に追加 work 不要、frontend 5 行程度の変更で UX 改善
- B で `GET /api/drivers` を 1 endpoint 追加するだけで command 選択が機能、driver 増えても自動追従
- C は project と display label の概念分離が必要かどうか、実利用者の声を聞いてから

---

## 6. 着手前に確認すべきこと

1. `command` の選択肢を hardcode するか動的取得 (`RespDriverList`) するか
   - 動的なら `server/web/mux.go` に `GET /api/drivers` 追加 (proto は既にある)
   - hardcode なら値の確定: 現状有効な driver は `state.Register(...)` を grep
2. デザイン原則: 既存 `App.tsx` の他 form (もし将来追加されるなら) のレイアウトと整合させるか
3. テスト方針:
   - `CreateSessionForm.test.tsx` の既存ケースを 400/422 path も追加
   - vitest で MSW モックする方針は既に確立済み (`mux_daemon_test.go` 相当の TS 版)

---

## 7. 触る files (推定)

| file | 変更 |
|---|---|
| `src/client/web/src/components/CreateSessionForm.tsx` | input 追加・修正、API 呼び出し body 拡張、error 表示 |
| `src/client/web/src/components/CreateSessionForm.test.tsx` | テストケース追加 |
| (B案以降) `src/server/web/mux.go` | `GET /api/drivers` 追加 |
| (B案以降) `src/server/web/mux_daemon_test.go` | drivers endpoint テスト |
| (C案) `src/client/state/...` | Session.Label field 追加 |
| (C案) `src/client/proto/response.go` | SessionInfo.Label 追加 |

---

## 8. 受け入れ条件 (案 A の場合)

- [ ] placeholder が絶対パスを要求する文言になっている
- [ ] 非絶対パス submit で client-side error が即表示される (backend 400 を待たずに)
- [ ] backend が 400/502 を返したら response body をそのまま画面に出す (request_id 込み)
- [ ] 既存 vitest が通る + 上記挙動の新規ケースが pass

---

## 9. 引き継ぎ補足

- backend の `isAbsoluteProjectPath` は `server/web/mux.go:43-58`。frontend 側で先回り validation するなら同じルール (`startsWith("/")`) を使う
- request_id は X-Request-Id ヘッダ + body 末尾 `(request_id=…)` の両方に入る。grep server.log で原因即特定可能
- driver list の proto 型は `proto.RespDriverList` (`client/proto/response.go`)、daemon-side reducer は `state.EventListDrivers` 対応済み
- 直近のメモリ: `~/.claude/projects/-home-dev-dev-agent-reactor-new/memory/web_gateway_isolation.md` に backend 側の文脈と修正履歴
