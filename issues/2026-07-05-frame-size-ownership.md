# frame size (cols/rows) の所有権整理 — spawn 時 size hint 未配線と死んだ size API

- 作成日: 2026-07-05
- 対象 layer: `client/` (runtime, state, driver, proto), `platform/termvt`, `server/web`
- 現状: 未着手 (本 issue は調査済み・実装待ち)
- 関連 issue: `issues/2026-07-02-vt-emulator-insertlinearea-panic.md` (**別物として扱う**。あちらは上流 lib の bounds bug、こちらは size の所有権とデータフローの整理。依存関係なし、独立に着手可能)

## TL;DR

「server に width/height は必要か? client 側で持っていれば良いのでは?」という問いを検証した結果:

- **server 側の size 保持は 2 点で不可欠** (kernel pty winsize / server-side emulator grid)。除去は不可能
- ただし正しいモデルは「**client が size を決め (authority)、server は kernel と emulator に追従させる (enforcement point)**」であり、これに反する歪みが 3 つ実在する:
  1. **spawn 時の size hint (FR-022) が未配線** — `LaunchOptions.Cols/Rows` は定義とコメントだけあって消費箇所ゼロ。spawn は常に 80×24 → browser 初回 fit で即 resize
  2. **`FrameSize` backend API に production 呼び出しがない** (テストのみ)
  3. tap の 1×1 emulator が size 不要なのに full emulator を持つ (→ これは VT issue の第 2 段スコープであり**本 issue では扱わない**)

本 issue のスコープは 1 と 2。

## 検証済みの事実 (2026-07-05 調査、再検証不要)

### server 側 size が不可欠な理由

1. **kernel pty winsize**: child TUI (codex/claude) は `TIOCGWINSZ` でサイズを読み `SIGWINCH` で再描画する。winsize は kernel の per-pty state で、pty を所有する server プロセスしか設定できない。browser から kernel に届く経路は存在しない。設定点: `src/platform/termvt/session.go:79` (`pty.StartWithSize`) / `session_deps.go:97` (`SetSize`)
2. **emulator grid**: ANSI 解釈 (折返し / カーソルアドレッシング / scroll region) はサイズ依存。emulator は reattach seed・cold-start 復元・`CaptureFrame` のために存在し、child が描画したのと同じ size でないと解釈が壊れる
3. **`CaptureFrame` に production 呼び出しがある**: `src/client/runtime/proto_bridge.go:319`, `interpret.go:219,403`。orchestrator / AI が **browser 不在時に** frame の画面を読む。「client (browser) が持っていれば良い」の直接の反例
4. **multi-viewer reconcile**: kernel winsize は 1 つ。複数 viewer が別サイズで attach する場合、誰かが 1 値に集約する必要がある。size は per-view ではなく per-pty の状態 (tmux/screen が server 側に size を持つのと同じ理由)

### 「client-only (emulator 廃止 + raw log replay)」が成立しない理由

- log trim (scrollback cap 相当) には「どの行が画面外に確定したか」の判断 = 解釈 = emulator が必要
- resize をまたぐ replay は VT issue と同型の解釈問題を client 側に移すだけ
- `CaptureFrame` (headless 消費者) が成立しない

### 歪み 1: spawn size hint 未配線 (本 issue の主対象)

- `src/client/state/driver_iface.go:315-318` — `LaunchOptions.Cols/Rows` に「The runtime bridges these to termvt.Spec on session launch (β scope)」とコメントがあるが、**`.Cols`/`.Rows` を読むコードが repo 内に存在しない** (grep 済み)
- `src/client/runtime/pty_backend.go:61,104` — `termvt.Spec` 生成時に Cols/Rows を渡していない → `session.go:271` `normalizeSize` が 80×24 に defaults
- 結果: すべての frame は 80×24 で spawn し、browser の初回 `fit.fit()` (`TerminalPane.tsx:131`) → `CmdSurfaceResize` で即 resize される
- この「hardcoded 初期値 → 直後の resize」の dance は、VT issue の crash window (80×24 → 64 → 63 の揺れ) を広げた一因でもある。spawn 時から client の size を使えば mid-flight resize が 1 回減る (VT 側は fork で修正済みなので、これは crash 対策ではなく構造整理 + defense-in-depth)

`LaunchOptions` の現在の流れ (Cols/Rows は途中で誰も読まないまま落ちる):

```
web API (server/web/mux.go:408 req.Cols/Rows)
  → proto/sessions/client.go:33 CreateSession(options)
  → driver PrepareLaunch/PrepareCreate (generic.go / codex.go / claude*.go / shell.go / gemini.go)
  → (ここで Options.Cols/Rows がどこにも転写されない)
  → EffFrameSpawn → pty_backend.go SpawnFrame → termvt.Spec{Cols/Rows なし}
```

### 歪み 2: `FrameSize` API が死んでいる

- 定義: `src/client/runtime/backends.go:65-66` (FrameInspect interface), 実装: `pty_backend.go:176`
- production 呼び出しゼロ (テスト `pty_backend_test.go` のみ)。grep 済み

### 健全な点 (変更不要)

- state 層 (reducer) は size を保持せず event→effect の pass-through (`reduce_surface.go:134`)。server 内の size の置き場は termvt session actor (`session_actor.go:22-23`) + kernel の 1 箇所ずつで、重複管理なし

## 修正スコープ

### A. spawn size hint の配線 (FR-022 の実装完了)

`LaunchOptions.Cols/Rows` を `termvt.Spec.Cols/Rows` まで届ける:

1. web API (`mux.go` の create request) → `LaunchOptions` は既に配線済み
2. driver の `PrepareLaunch`/`PrepareCreate` → `LaunchPlan`/`CreateLaunch` に size を通すか、driver を素通りさせて runtime 側で直接 Spec に渡すかは実装時に判断 (driver は size に関心がないはずなので、素通り経路が筋が良い可能性が高い。state 層の spawn effect に載せる案も含めて要トレース)
3. `pty_backend.go:61,104` の `termvt.Spec` に Cols/Rows を設定
4. hint 欠落時 (API が size を送らない場合) は現行どおり `normalizeSize` の 80×24 に fallback
5. `normalizeSize` の clamp (`maxDim`) が hint にも適用されることを確認 (悪意ある client 対策は既存の仕組みに乗る)

テスト: spawn 時に hint が pty winsize と emulator grid の両方に反映されること (fakeEmulator/fakePTY で観測可能)。hint なしで 80×24 に fallback すること。

### B. `FrameSize` API の処遇決定

選択肢: (a) 削除 (呼び出しゼロの query surface を消す)、(b) 将来の消費者 (WORKFLOW.md 系?) を見込んで残す。実装セッションで `git log` から導入意図を確認して判断。削除する場合 `backends.go` の FrameInspect interface / noopBackend / fake も併せて整理。

### スコープ外 (混ぜないこと)

- **tap の 1×1 emulator 撤去・`Emulator` interface 分割**: VT issue の第 2 段 (別 issue 予定) の領分
- **VT bounds bug 対応**: `issues/2026-07-02-vt-emulator-insertlinearea-panic.md` で修正済み。`forks/` に触る必要はない
- **multi-viewer の size reconcile ポリシー** (現状 last-writer-wins): 実害の報告がない限り現状維持

## 関連 file 参照

- `src/client/state/driver_iface.go:315` — LaunchOptions.Cols/Rows (未消費 hint)
- `src/client/runtime/pty_backend.go:61,104` — termvt.Spec 生成 (size 渡し漏れ)
- `src/platform/termvt/session.go:22-27,71-87,264-272` — Spec / NewSession / normalizeSize
- `src/client/runtime/backends.go:65-66` + `pty_backend.go:176` — FrameSize (死 API)
- `src/server/web/mux.go:127-128,408-409` — web API の cols/rows 受け口
- `src/server/web/gateway.go:311-323,489-503` — resize 経路 (参考: 配線済みの側)
- `src/client/web/src/components/TerminalPane.tsx:131,180` — browser 側 fit / onResize 起点
