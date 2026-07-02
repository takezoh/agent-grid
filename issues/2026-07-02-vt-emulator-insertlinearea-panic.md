# server プロセスが VT emulator の `InsertLineArea` bounds bug で panic して死ぬ

- 作成日: 2026-07-02
- 対象 process: `agent-reactor-server`
- 関係 module: `src/platform/termvt`, `github.com/charmbracelet/ultraviolet`, `github.com/charmbracelet/x/vt`
- 現状: 未修正 (crash パスは残存、trigger が引かれないだけで動いている)
- 関連コメント: `src/client/runtime/tap_manager.go:168` に同種バグの既知記載あり

## TL;DR

`platform/termvt.Session` の PTY chunk 処理中に、goroutine panic (`runtime error: index out of range [63] with length 63`) が発生し `Restart=on-failure` により server プロセスが exit → 自動再起動する。原因は上流 2 lib の防御不足の噛み合わせ:

1. `ultraviolet.Buffer.InsertLineArea` が `area.Max.Y > b.Height()` を弾かない (`buffer.go:462`)
2. `x/vt` の DECSTBM handler / `Screen.setVerticalMargins` が explicit `bottom` を screen height に clamp しない (`handlers.go:862`, `screen.go:143`)

これらが単独では防御網になるはずが、両方素通しなので `Screen.scroll.Max.Y > buf.Height()` の状態が成立し、その状態で ESC M (reverse index) が来ると `ScrollDown → InsertLine → InsertLineArea` の out-of-range 経路を踏む。

Session actor 側にも `defer recover()` が無いため、goroutine panic がプロセス全体を殺す。frametap 側 (`src/client/runtime/tap_manager.go:170 feedSafe`) は同じバグに対して既に recover 済みだが、主経路の `session_actor.go:processChunk` は無防備。

## Symptoms

### Panic 実例 (2 回、両方同一 stack)

```
panic: runtime error: index out of range [63] with length 63

github.com/charmbracelet/ultraviolet.(*Buffer).InsertLineArea
    /pkg/mod/github.com/charmbracelet/ultraviolet@v0.0.0-20260303162955-0b88c25f3fff/buffer.go:476
github.com/charmbracelet/ultraviolet.(*RenderBuffer).InsertLineArea
    /pkg/mod/github.com/charmbracelet/ultraviolet@v0.0.0-20260303162955-0b88c25f3fff/buffer.go:731
github.com/charmbracelet/x/vt.(*Screen).InsertLine
    /pkg/mod/github.com/charmbracelet/x/vt@v0.0.0-20260615091924-bb3af1bbe712/screen.go:334
github.com/charmbracelet/x/vt.(*Screen).ScrollDown                     screen.go:313
github.com/charmbracelet/x/vt.(*Emulator).reverseIndex                 cc.go:50   (ESC M = 0x4d)
github.com/charmbracelet/x/vt.(*Emulator).Write                        emulator.go:276
github.com/takezoh/agent-reactor/platform/termvt.(*Session).processChunk  session_actor.go:179
github.com/takezoh/agent-reactor/platform/termvt.(*Session).mainLoop     session_actor.go:166
```

引数レジスタから読み取れる panic 時の状態:
- `Buffer.InsertLineArea(y=0, n=1, cell=nil, area={Min:(0,0), Max:(80,64)})`
- `buffer.Height() = 63`, `area.Max.Y = 64` → `b.Lines[63]` at line 476 で index out of range
- panic 時 chunk_len: 0x356 (854B) と 0x591 (1425B)
- session goroutine 起動時の cols/rows: `mainLoop(0x50, 0x18) = 80×24` (**初期化時の引数**、直近の buffer size を示さない)

### systemd 側の観測

```
Jul 01 15:58:21 agent-reactor-server.service: Main process exited, code=exited, status=2/INVALIDARGUMENT
Jul 01 15:58:23 Scheduled restart job, restart counter is at 1.
Jul 01 16:07:37 Main process exited, code=exited, status=2/INVALIDARGUMENT
Jul 01 16:07:39 Scheduled restart job, restart counter is at 2.
```

`agent-reactor-web.service` は `BindsTo=agent-reactor-server.service` により停止するが、web の exit code は 0 なので `Restart=on-failure` に該当せず、web は **inactive のまま放置** される。手動 `systemctl --user start agent-reactor-web.service` が必要。これは crash とは独立の 2 次被害。

## 検証済みの証拠

### Phase 1: 上流 bounds bug の直接再現

scratchpad の Go module (`/tmp/claude-.../scratchpad/vt-repro/phase1_bounds_test.go`) で以下を証明:

```go
b := uv.NewBuffer(80, 63)               // Height=63
area := uv.Rect(0, 0, 80, 64)           // Max.Y=64 (height を 1 超える)
b.InsertLineArea(0, 1, nil, area)       // → panic: index out of range [63] with length 63
```

対照実験:

```go
b := uv.NewBuffer(80, 63)
area := uv.Rect(0, 0, 80, 63)           // Max.Y = Height
b.InsertLineArea(0, 1, nil, area)       // → panic せず
```

`Buffer.InsertLineArea` (buffer.go:462) の現状ガード:

```go
if n <= 0 || y < area.Min.Y || y >= area.Max.Y || y >= b.Height() {
    return
}
```

`y >= b.Height()` は check しているが `area.Max.Y > b.Height()` は check していない。以降の copy loop:

```go
for i := area.Max.Y - 1; i >= y+n; i-- {
    for x := area.Min.X; x < area.Max.X; x++ {
        b.Lines[i][x] = b.Lines[i-n][x]     // OOB when i >= len(b.Lines)
    }
}
```

上流 HEAD (2026-06-22 `f39628c8`) の同ファイルも同じコードで未修正。`buffer.go` に対する commit 履歴は `2026-04-28: fix(buffer): ensure RenderBuffer marks lines as touched...` のみ (別修正)。

### Phase 2: CLI 起動 escape の実測

docker exec + `script(1)` で codex を worktree cwd / main repo cwd で起動、初期化 PTY を capture (`/tmp/claude-.../scratchpad/vt-repro/probe-{worktree,main}.log`)。両方に:

```
\x1b[1;24r          DECSTBM: 上端 1、下端 24
\x1b[1;1H           CUP (1,1)
\x1bM × 11〜13      RI (Reverse Index) 連発
\x1b[r              DECSTBM reset
\x1b[1;10r or [1;13r  DECSTBM 再設定
```

この `ESC M` 連発は `Emulator.reverseIndex()` (cc.go:46) を叩き、cursor が scroll region 上端にいると `Screen.ScrollDown(1)` → `Screen.InsertLine` → 上流バグ経路。

差分は window title (`agent-grid` vs `unified-gazelle`) と reverse-index 回数 (13 vs 11)。**両方に同じバグ経路の入力が含まれる**。

### Phase 3: 静的パラメータではない (静的トリガー仮説の反証)

capture bytes を standalone emulator に流した対照:

| 対象 emulator | 入力 | 結果 |
|---|---|---|
| `vt.NewEmulator(1, 1)` | worktree capture | panic (`index out of range [22] with length 1`) |
| `vt.NewEmulator(1, 1)` | main capture | panic (同上) |
| `vt.NewEmulator(80, 24)` + `SetScrollbackSize(10000)` | worktree capture | **panic せず** |
| `vt.NewEmulator(80, 24)` + `SetScrollbackSize(10000)` | main capture | **panic せず** |

1×1 emulator は tap_manager 側 (`vt.New(1, 1)` = `feedSafe` 保護あり) と同型で、コメント通り必ず panic する。しかし production Session と同じ **80×24 で captured bytes を replay しても panic しない** — 静的なバイト列だけでは production の panic を再現できない。

### crash 相関の反証

同じ静的パラメータ (`exec codex --dangerously-bypass-approvals-and-sandbox -C .../worktrees/<name>`、resume 引数なし、adopted CLI-created thread の直後) で:

| 時刻 | worktree | resume | panic |
|---|---|---|---|
| 07-01 15:58:20 | feasible-flounder | なし | ✅ (1s 後) |
| 07-01 16:07:15 | (main repo) | あり | ❌ |
| 07-01 16:07:37 | unified-gazelle | なし | ✅ (0.4s 後) |
| 07-02 01:47:50 | profound-goblin | **なし** | ❌ (40s+ 稼働継続) |

07-02 の profound-goblin は crash 2/2 と全く同じ静的条件だが panic しなかった。**静的パラメータ (cwd / cmd / resume 有無) だけでは crash 発生を予測できない → 動的 race**。

なお 07-01 16:07:39 → 07-02 01:25 の間、server binary は差し替わっているが、`git diff src/client/driver/{codex_event,codex_resume}.go src/client/runtime/bootstrap.go` の中身は observability ログ追加のみ (`logCodexIdentityCaptured` + `bootstrap: deleted unrecoverable snapshot` / `bootstrap: dropping stopped frame on cold start` の Info log)。**crash パスに影響するロジック変更なし**。

## Root cause 分析 (codex MCP による考察を含む)

### `buffer.Height() = 63` はどこから来るか

`NewSession` (`src/platform/termvt/session.go:71`) は 80×24 で `vt.NewEmulator` を作り、`SetScrollbackSize(10000)` は main screen の scrollback cap だけを変える (visible height は不変)。alt-screen 切替も size を変えない。`Screen.Resize` (`x/vt/screen.go:73`) は `s.scroll = s.buf.Bounds()` で常に scroll region を bounds と同期させる。

したがって Go 側の Session actor 内で `buf.Height()` が 63 に落ち込む純粋な race は見えない。63 の由来として最有力は **browser 側の xterm.js FitAddon** による resize と考察できる:

- `src/client/web/src/components/TerminalPane.tsx:131` mount 直後に `fit.fit()` を実行
- `src/client/web/src/components/TerminalPane.tsx:180` `term.onResize` が daemon に `{k:"r"}` を送る
- `src/server/web/gateway.go:311` `CmdSurfaceResize` が受け取り、最終的に `sess.Resize` (`src/platform/termvt/session.go:175`) を呼ぶ

panic stack の `mainLoop(80, 24)` は **初期化時の goroutine 引数**にすぎない (`s.mainLoop(cols, rows)` の cols/rows は関数ローカル、以降の Resize は emulator 内部を書き換えるが goroutine 引数は不変)。

### race のシナリオ

以下が現状の情報と整合する最も筋の通る sequence:

1. browser attach で session が **80×64** などに resize される
2. child (codex TUI) がその rows を元に `\x1b[1;64r` (DECSTBM with explicit bottom = 64) を送る
3. browser の再 fit (ResizeObserver / window resize) で session が **80×63** に縮む
4. child 側の描画 or `ESC M` (reverse index) が続く
5. `x/vt` 側で `Screen.scroll.Max.Y = 64` を保持したまま `buf.Height() = 63` の状態で `InsertLineArea` が呼ばれて panic

ここで **`x/vt` の DECSTBM handler が explicit `bottom` を clamp していない** ことが重要な下地。no-param reset (`\x1b[r`) だけなら `bottom = e.Height()` で安全だが、`\x1b[1;64r` のような explicit bottom は無防備に scroll region に格納される。その後の Screen.Resize は `s.scroll = s.buf.Bounds()` で修正するが、Resize と DECSTBM の**間**に ESC M が挟まる時間窓で panic が確定する。

Session actor は `em.Write` と `em.Resize` を同一 goroutine で serialize しているので Go race ではない。純粋に **child TUI が sent 側から出す DECSTBM の下端が、その時点の Emulator height を超えている** ことが直接原因。

### 「時々」の動的要因

同じ静的条件で結果が反転するのは以下の interleave 差:

- browser がその session に早く subscribe したか
- `fit.fit()` が何回走ったか (初回 fit + ResizeObserver + window resize で 64→63 の揺れは十分あり得る)
- codex startup TUI が DECSTBM / RI を吐いたタイミング
- **resumed session は startup の full-screen redraw が弱い**、または attach 時点で既に初期化を抜けているため crash 経路を通りにくい (= 「resume の方が落ちにくい」相関の合理的説明)

### frametap 側との対比

`src/client/runtime/tap_manager.go:168`:

> ```
> // vt.New(1,1) panics on ESC M / CSI M / DECRC via InsertLineArea out-of-bounds;
> // emulator handler state remains valid after the panic so the next chunk is safe.
> ```

同じバグ経路は既知で、frametap の `feedSafe` (line 170) は `defer recover()` で chunk drop する。主経路の `platform/termvt/session_actor.go processChunk` (line 177) には同種 recover が存在せず、goroutine panic がプロセス全体を殺す。

## 修正方針

### 三層防御 (推奨)

| 層 | 対象 | 内容 | 位置付け |
|---|---|---|---|
| A1 | `charmbracelet/ultraviolet` (fork + `replace` directive) | `Buffer.InsertLineArea` / `DeleteLineArea` で `area.Max.Y > b.Height()` を clamp | 上流バグの本丸 |
| A2 | `charmbracelet/x/vt` (fork + `replace` directive) | `DECSTBM` handler / `Screen.setVerticalMargins` で `bottom > e.Height()` を clamp | 攻撃面をライブラリ境界で塞ぐ |
| C | `src/platform/termvt/session_actor.go processChunk` | `defer recover()` を追加、panic 時は chunk を捨て ERROR log を出して session を継続 (あるいは EventExit で session を kill) | daemon crash だけは止める可用性ガード。terminal 内部 state が壊れる可能性は残るので A1/A2 の代替ではなく併用 |
| D | upstream PR | ultraviolet + x/vt 双方に PR を出す | 恒久解、merge されたら pin を戻す |

`A1` だけだと crash は止まるが invalid scroll region は残る。`A2` だけだと今回の経路は止まるが、library 境界の防御としては弱い。**両方入れる**のが実務的。

`web.service` の `Restart=on-failure` を `Restart=always` に変更するか、あるいは web を `Requires=` のみに変更して `BindsTo=` を外す (今回のような 2 次被害を防ぐ) 議論も別途必要。

### instrumentation 提案 (race 実測用)

crash が「時々」発生する race である以上、次に落ちた瞬間の状態を回収するのが最短。**daemon 側だけで完結する 3 点**:

1. `src/server/web/gateway.go:311` inbound resize handler で `sid/cols/rows` を Info log 追加
2. `src/platform/termvt/session_actor.go:175 processChunk` の入口で以下を DEBUG log:
   - `seq` (chunk 番号)
   - `len(chunk)`
   - hex prefix / suffix (先頭 32B + 末尾 32B)
   - cursor 位置 (`em.CursorPosition()`)
   - alt-screen 有無 (`em.IsAltScreen()`)
3. `em.Write(chunk)` を `defer recover()` で包み、panic 時に**直前 chunk の raw bytes と直近 resize 列**を必ず ERROR log に出す

より深く (`y/n/area/buffer.Height` の実測) 取りたい場合は upstream fork 側に log を入れる必要があるが、そこまでやらなくても「panic した chunk の bytes」と「その前の resize 系列」が取れれば race の詳細は再現可能。

### 運用回避 (修正投入までのしのぎ)

- crash trigger の完全回避は現状不可能 (race なので静的条件で予測できない)
- ただし相関としては **既存 session の resume のほうが panic しにくい** ため、極力 resume ベースで運用
- crash 時は自動 restart されるが web は手動起動が必要 → `systemctl --user start agent-reactor-web.service`

## 関連 file 参照

- `src/platform/termvt/session.go` — Session actor + Spec (ScrollbackLines default = 10000 via config)
- `src/platform/termvt/session_actor.go` — mainLoop / processChunk (recover 欠如)
- `src/platform/termvt/session_deps.go` — Emulator interface + `emulatorFor` (vt.NewEmulator 呼び出し)
- `src/client/runtime/tap_manager.go` — 同種バグの既知記載 + feedSafe (recover 済み経路)
- `src/client/runtime/pty_backend.go` — SpawnFrame → Manager.Create の入り口
- `src/client/web/src/components/TerminalPane.tsx` — xterm.js FitAddon による resize 起点
- `src/server/web/gateway.go` — CmdSurfaceResize から sess.Resize への橋渡し
- `~/.local/share/go/pkg/mod/github.com/charmbracelet/ultraviolet@v0.0.0-20260303162955-0b88c25f3fff/buffer.go` — 上流バグの本丸
- `~/.local/share/go/pkg/mod/github.com/charmbracelet/x/vt@v0.0.0-20260615091924-bb3af1bbe712/{cc.go,screen.go,handlers.go,emulator.go}` — DECSTBM / reverseIndex 経路
- `/home/dev/.local/state/agent-reactor/server.log.{3,4}` — 07-01 の 2 crash 実ログ
- `~/.config/systemd/user/agent-reactor-{server,web}.service` — systemd unit 定義

## 未解決事項

- 実 production の PTY chunk (crash 直前) の raw bytes 未回収 (instrumentation を投入して次の crash 待ち)
- `buf.Height() = 63` の**厳密な**由来 (browser resize の中間状態、と考察したが実測は未確認)
- alt-screen 切替 (`\x1b[?1049h`) が race に絡むかどうかは未検証
- `DeleteLineArea` 側で同種 crash が発生するかは未観測 (コード上は同じガード欠落なので理論的には可能)
