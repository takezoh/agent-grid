# Agent Grid Terminal CLI Client Plan

- **作成日**: 2026-08-12
- **ステータス**: implementation-ready plan
- **対象**: `clients/cli`（新規）、必要最小限の public client library / shared config integration
- **非対象**: Grid Server の PTY/session/multiplexing 再実装、terminal emulator、TUI、server-side interaction policy の追加
- **関連**:
  - [plan-20260723-native-clients.md](./plan-20260723-native-clients.md)
  - [ARCHITECTURE.md](../ARCHITECTURE.md)
  - [protocol/openapi.yaml](../protocol/openapi.yaml)
  - [clients/README.md](../clients/README.md)

## 1. Purpose

Agent Grid に、Claude Code CLI / Codex CLI の server-side session を通常のローカル terminal から直接操作できる **terminal-first CLI client** を追加する。

目標 UX は tmux に近い。

```text
$ ag ls
$ ag attach <session>
    ↓
Claude Code / Codex の既存 PTY に接続
    ↓
通常の terminal と同様に対話
    ↓
detach / terminal close / network disconnect
    ↓
server-side session と agent process は継続
    ↓
$ ag attach <session>
    ↓
同じ session に再接続
```

CLI は agent process をローカルで spawn して server に接続するラッパーではない。**Agent Grid server-side execution plane が所有する session / PTY / agent process に attach する client** である。

この CLI により、Agent Grid の UI surface を次のように分業できる。

- Grid / workspace UI: 複数 agent/session の俯瞰、supervision、ファイル/diff/Markdown 等
- CLI: 1 session に入り、Claude Code / Codex を terminal-native に操作する
- automation: session discovery / attach target resolution / machine-readable state access

## 2. Desired outcomes

1. Claude Code / Codex session を Agent Grid 内で稼働させたまま、任意の terminal から attach / detach / reattach できる。
2. CLI を閉じても session / PTY / agent process を終了しない。
3. Claude Code / Codex から見て通常の interactive TTY と同等に、入力、ANSI output、terminal resize が動作する。
4. CLI は Grid Server の既存 terminal/session semantics をそのまま利用し、client 独自の PTY state、multiplexing policy、replay system を持たない。
5. browser / desktop workspace と CLI が同じ server-side session に同時に接続できる。追加の controller/observer/takeover 制御は導入しない。
6. CLI は server internal package に依存せず、public wire contract の client として成立する。
7. Windows / WSL、Linux、macOS から単一バイナリに近い運用で利用できる。

## 3. Current state: already implemented by the server

本計画で重要なのは、以下を CLI の新規実装項目と混同しないことである。

### 3.1 Session / PTY / agent process ownership

既存 native-client plan が定義している通り、Server / execution plane が source of truth であり、long-running session、PTY、agent process を client が所有しない。

CLI はこの ownership model を変更しない。

### 3.2 Surface subscribe / unsubscribe

`services/server/api/gateway.go` の `AttachWS` / `Attacher` は既に session surface の subscribe / unsubscribe を実装している。

接続時に client が terminal geometry を宣言し、server-side surface subscribe が開始される。切断時は surface unsubscribe されるが、これは **attach subscriber の解除**であり session termination ではない。

### 3.3 Initial terminal state / snapshot delivery

現在の `DaemonAdapter.SubscribeSurface` は event subscriber を先に登録してから `CmdSurfaceSubscribe` を送る。コードコメントにある通り、これは `termvt` が new subscriber に送る initial screen snapshot を落とさないための既存設計である。

したがって CLI の reattach 対応のために、新しい terminal replay buffer / snapshot store を実装しない。

CLI は subscribe 後に server から届く既存 surface output をそのまま terminal に流す。

### 3.4 Input forwarding

`AttachWS` の inbound path は `k:"i"` を `CmdSurfaceWriteRaw` に変換し、server-side PTY/session に入力を転送する。

CLI 側で必要なのは local stdin bytes を既存 inbound wire format に encode して送ることだけである。

### 3.5 Terminal resize

既存 inbound path は `k:"r"` と `cols` / `rows` を受け、validation 後に `CmdSurfaceResize` を実行する。

CLI 側では local terminal size の initial declaration と resize event を既存 message に変換するだけでよい。

### 3.6 Multiplexing

Grid Server の lifecycle WebSocket は既に 1 connection 上で lifecycle traffic と複数 session の surface traffic を multiplex できる。per-session `AttachWS` も既存 surface subscribe semantics を利用する。

**CLI のための controller / observer / takeover ownership policy は追加しない。**

複数 client が同じ session に attach する場合の semantics は既存 server の subscriber/multiplexing semantics に従う。CLI はその上に独自制御を重ねない。

### 3.7 Authentication / WS ticket / protocol version

public client contract には bearer token、single-use WS ticket、client instance ID、protocol version/capability の既存仕組みがある。

CLI は既存方式を利用し、CLI 用の認証方式や別ポートを新設しない。

### 3.8 Lifecycle state and control

現在の public client surface は REST だけでは完結していない。`protocol/openapi.yaml` は REST binding annex であり、normative message types は `protocol/*.schema.json`、live state/control は lifecycle WebSocket contract が担う。

CLI 実装時に、古い/仮想的な `POST /api/sessions` などを新規 API として前提にしない。browser/workspace が現在利用する lifecycle desired/state contract を inventory し、同じ public contract を利用する。

## 4. Scope

### 4.1 v1 commands

CLI の primary job は terminal attach であり、command surface は小さく保つ。

```text
ag ls
ag attach <session>
ag new ...
ag kill <session>
```

必要なら discoverability のため long form を提供する。

```text
agent-grid ls
agent-grid attach <session>
agent-grid new ...
agent-grid kill <session>
```

`ag` を binary name にするか alias にするかは packaging 時に決定する。内部 command/API design は `agent-grid` を canonical name としてよい。

### 4.2 `ls`

目的: attach target を素早く見つける。

最低限表示する候補:

- session ID / short ID
- display name（存在する場合）
- agent kind / model（current SessionInfo に存在する範囲）
- host/server identity
- cwd / worktree（wire state に存在する範囲）
- lifecycle state

原則: **server が現在公開している SessionInfo / lifecycle state だけを表示する。CLI の都合で server-side derived state を新設しない。**

オプション:

```text
ag ls --json
```

`--json` は scripting / agent automation のため v1 に含める。

### 4.3 `attach <session>`

CLI の中心機能。

```text
ag attach abc123
```

session selector は v1 では次を優先する。

1. exact session ID
2. server contract に stable name が既に存在する場合のみ exact name
3. unique short-ID prefix

曖昧な selector は自動選択せず、候補を表示して non-zero exit する。

attach 後は local terminal を raw mode にして stdin/stdout と server surface stream を bridge する。

### 4.4 `new`

新しい Claude Code / Codex session を作成し、成功後に原則その session へ attach する。

例として目指す UX:

```text
ag new --agent claude --cwd ~/src/foo
ag new --agent codex --cwd ~/src/foo
ag new --agent claude --cwd ~/src/foo --no-attach
```

ただし CLI 固有の session-create model を新設しない。**現在の browser/workspace が lifecycle desired contract へ送る session creation intent と同じ wire model を使う。**

実装前の Phase 0 で current lifecycle desired message と UI-side create flow を確定し、CLI flags はその public fields への薄い mapping とする。

Claude/Codex 用 executable/argv/profile の決定が server 側に既にある場合、CLI はそれを再定義しない。

### 4.5 `kill <session>`

既存 lifecycle desired/control contract で session termination を明示的に要求する command。

重要:

- `attach` の終了、Ctrl-C、network failure は `kill` と同義にしない。
- session termination は `ag kill` のような明示 action のみ。
- server が現在持つ graceful/force semantics があればそのまま利用する。

## 5. Explicit non-goals

v1 では以下を実装しない。

### 5.1 Multiplexing policy

実装しないもの:

- controller / observer role
- input ownership lock
- `--takeover`
- exclusive attach
- CLI-side subscriber coordination

既存 server semantics に従う。

### 5.2 PTY ownership / process supervision

CLI は以下を行わない。

- PTY allocate
- Claude / Codex process の local spawn
- server-side process group management
- session persistence
- output fan-out
- reconnect snapshot generation

すべて execution plane の責務である。

### 5.3 Terminal emulator

CLI は ANSI/xterm state を parse/render しない。

ローカルの Windows Terminal / WezTerm / iTerm2 / Terminal.app 等を renderer とし、server surface output を可能な限り opaque に stdout へ渡す。

### 5.4 TUI / grid UI

Bubble Tea 等による full-screen session manager は v1 に入れない。

`ls` + `attach` の Unix-style CLI の方が既存 Grid UI と責務が重ならない。

将来 `ag top` のような TUI を追加する場合も別計画とする。

### 5.5 New server authentication / remote transport design

CLI のために auth、TLS、multi-host gateway、remote discovery を再設計しない。

existing client topology / gateway plan に従う。

### 5.6 General-purpose remote shell

v1 の目的は Claude Code / Codex session への terminal access である。arbitrary shell/process launcher への一般化は、この CLI の成立条件にしない。

## 6. Architecture

```text
Local terminal
   │
   │ stdin / stdout / terminal size
   ▼
clients/cli (Go)
   │
   ├── config / auth bootstrap
   ├── lifecycle client
   └── terminal surface client
           │
           │ public HTTP / WebSocket wire contract
           ▼
Agent Grid Server
   │
   ├── lifecycle/session authority       [existing]
   ├── multiplexing/subscriptions        [existing]
   ├── terminal snapshot/state           [existing]
   ├── input/resize routing              [existing]
   └── PTY / agent process ownership     [existing execution plane]
           │
           ▼
      Claude Code / Codex
```

### Boundary rule

`clients/cli` may share **public protocol/config/client helpers**, but must not import server internal domain/runtime packages simply because both are Go.

Bad:

```text
clients/cli -> services/host/state internals
clients/cli -> services/server/api implementation types
```

Good:

```text
clients/cli -> clients-side public wire models/helpers
            -> HTTP/WS
            -> server
```

This protects remote-host operation and keeps CLI behavior honest as another Agent Grid client.

## 7. Why Go

The existing native-client plan already places the CLI on Go. This terminal-first plan keeps that decision.

The reason is stronger for an interactive attach client than for a generic REST CLI:

- single executable distribution
- raw terminal mode
- Unix signal / resize handling
- straightforward bidirectional byte streaming
- low startup overhead
- Windows/Linux/macOS builds
- no Node runtime requirement just to attach to a terminal session

Do not use Go as an excuse to bypass the public contract through internal package imports.

## 8. Proposed repository layout

Start small; do not create a broad SDK abstraction before actual reuse is visible.

```text
clients/
└── cli/
    ├── README.md
    ├── cmd/
    │   └── agent-grid/
    │       └── main.go
    └── internal/
        ├── app/
        │   ├── root.go
        │   ├── list.go
        │   ├── attach.go
        │   ├── new.go
        │   └── kill.go
        ├── gateway/
        │   ├── client.go
        │   ├── auth.go
        │   ├── lifecycle.go
        │   ├── surface.go
        │   └── wire.go
        ├── terminal/
        │   ├── terminal.go
        │   ├── raw_unix.go
        │   ├── raw_windows.go
        │   ├── resize_unix.go
        │   └── resize_windows.go
        ├── selector/
        │   └── session.go
        └── output/
            └── output.go
```

Exact OS file split は implementation spike 後に調整してよい。

### Shared Go client library extraction rule

最初から `clients/sdk/go` を大きく設計しない。

まず `clients/cli/internal/gateway` に public contract adapter を置き、次のどちらかが成立した時点で `clients/sdk/go` へ抽出する。

- 他の Go client / automation consumer が実際に必要になった
- CLI 実装前から repository の current client architecture が Go SDK を正式 shared surface と定義している

これにより SDK design 自体が CLI の blocker になるのを防ぐ。

## 9. Terminal attach protocol implementation

### 9.1 Connection setup

`attach` は概ね次の順序で動作する。

```text
1. load client/gateway config
2. resolve target server/host
3. resolve session selector -> stable session ID
4. perform existing auth / WS-ticket bootstrap
5. read current local terminal size
6. open existing terminal surface WS
7. send required initial geometry frame
8. enter local raw mode
9. run stdin -> WS and WS -> stdout pumps
10. forward resize notifications
11. detach / disconnect
12. restore local terminal state unconditionally
```

実際の endpoint URL、ticket query/header、frame envelope は現在の browser/workspace implementation と gateway tests を source of truth としてコピーする。新しい CLI-only protocol variation は作らない。

### 9.2 Output path

```text
server surface output
    ↓
WS frame decode (existing wire envelope only)
    ↓
terminal payload
    ↓
os.Stdout.Write
```

CLI が terminal escape sequence を理解する必要はない。

注意: existing server output frame が raw binary bytes そのものではなく asciicast/event envelope なら、**envelope だけ decode して payload を stdout に書く**。wire representation を勝手に raw-binary protocol へ変更しない。

### 9.3 Input path

```text
local stdin in raw mode
    ↓
read bytes
    ↓
encode existing k:"i" surface input message
    ↓
WebSocket
    ↓
existing server WriteRaw path
```

UTF-8 text typing、control keys、escape sequences、paste が現在の browser surface contract の範囲で透過的に動くことを確認する。

### 9.4 Resize

attach 開始時に必須 initial geometry を送る。

attach 中は terminal resize を検出し、既存 `k:"r"` message に current cols/rows を載せる。

resize event の burst は client-side で過度に複雑な queue を持たず、必要であれば latest-value debounce/coalescing のみ行う。

### 9.5 Detach

CLI-specific detach shortcut は **local transport detach** だけを行う。

候補 default:

```text
Ctrl-] 
```

または tmux-like two-stroke sequence。

ただし Claude Code / Codex 自身が使う key chord と衝突しないことを実機確認してから final default を決める。shortcut は config/flag で変更可能にできる設計にする。

Detach action:

1. stop accepting local stdin
2. close surface WS normally
3. restore terminal mode
4. return shell prompt

**session delete/stop command は送らない。**

### 9.6 Ctrl-C semantics

raw attach 中の Ctrl-C は原則 local CLI process の SIGINT として奪わず、`0x03` input として remote PTY/agent に送る。

つまり Claude/Codex の current operation interrupt に使えることを優先する。

CLI 自体から detach する操作は専用 detach shortcut を用いる。

ただし OS / Go terminal implementation 上 Ctrl-C が signal として到達する platform がある場合は、raw-mode behavior を integration test して合わせる。

### 9.7 Cleanup safety

terminal raw mode を壊したまま shell に戻すのが最も避けるべき failure である。

restore は以下すべてで保証する。

- normal detach
- WebSocket close
- server disconnect
- auth/protocol failure after raw-mode transition
- local process signal
- stdin EOF
- panic boundary（可能な範囲）

raw mode への移行は WS/session resolution 完了後まで遅らせ、失敗範囲を小さくする。

## 10. Config and topology

CLI-only config format を作らない。

`clients/config` にある canonical client topology/config を inventory し、Go から直接 reuse できなければ **同じ schema / precedence** を Go 側で実装する。

最低限必要な input:

- target gateway/server
- auth token or token source
- host/server selector where applicable
- protocol/client identity bootstrap inputs

CLI flags は temporary override とし、repository-wide config と競合する第2の設定体系にしない。

推奨 precedence:

```text
explicit CLI flag
> environment override
> canonical client config
> documented local default
```

ただし既存 `clients/config` に canonical precedence が既に定義されている場合はそれを優先する。

## 11. Lifecycle commands and public contract audit

terminal attach は current `AttachWS` が明確だが、`new` / `kill` / `ls` は current lifecycle WS contract を正しく利用する必要がある。

実装着手時の最初の作業を **server implementation ではなく contract inventory** とする。

確認対象:

- browser/workspace の lifecycle WS connection bootstrap
- WS ticket mint/use flow
- hello frame (`k:"h"`) の SessionInfo
- view-update / lifecycle outcome message
- lifecycle desired inbound command
- create desired state の fields
- terminate desired state の fields
- acknowledgement / outcome correlation
- session selector に使える stable identifiers
- multi-host config から target gateway までの resolution

成果物は CLI の `gateway/wire.go` で ad-hoc copy を増殖させるのではなく、既存 `protocol/*.schema.json` と current protocol constants を基準にする。

`protocol/openapi.yaml` は REST binding annex なので、session lifecycle のためだけに REST endpoint を新設することをデフォルト解としない。

## 12. Error model and exit codes

CLI は human use と automation の両方で予測可能にする。

候補:

```text
0  success / normal detach
1  generic failure
2  CLI usage / invalid selector syntax
3  target/session not found
4  ambiguous session selector
5  auth / compatibility failure
6  transport/server disconnected before attach established
```

細かすぎる exit-code taxonomy は避けるが、`not found` / `ambiguous` / auth failure は scripting 上価値が高い。

attach 後の server disconnect は reason を stderr に簡潔に出し、必ず terminal restore して終了する。

## 13. Machine-readable behavior

`ag ls --json` は v1 に含める。

JSON は table-rendered data を reverse parse させず、public session state を安定した CLI output DTO として返す。

interactive `attach` 自体に JSON mode は不要。

`new --no-attach` は成功時に session ID を machine-readable に取得できる option を持たせてもよい。

例:

```text
ag new --agent claude --cwd . --no-attach --json
```

これにより shell script / Codex / Claude 等から Agent Grid session を作成して後続処理へ ID を渡せる。

## 14. Implementation phases

### Phase 0 — Current contract pin-down

目的: server に既にあるものを再実装しないための短い implementation audit。

Tasks:

- current browser/workspace lifecycle WS client path を trace
- terminal AttachWS route / auth / ticket URL を確定
- surface output envelope と snapshot delivery behavior を fixture 化
- lifecycle desired create/terminate wire messages を確定
- SessionInfo fields と selector rule を確定
- `clients/config` integration point を確定
- Claude Code / Codex の現在の server-side creation path を確認

Exit:

- CLI command -> current public wire operation の mapping table がコードコメント/README で確定
- server modification が必要な gap と、単なる client implementation gap が分離されている

### Phase 1 — CLI skeleton + discovery

Implement:

- Go CLI entry point
- config loading
- auth / WS ticket bootstrap
- lifecycle connection
- `ag ls`
- selector package
- `--json`

Exit:

- live Grid Server の sessions を CLI で安定して列挙できる
- remote/public client boundary だけを通っている

### Phase 2 — Read-only attach

まず output side を成立させる。

Implement:

- per-session surface WS attach
- initial geometry
- existing initial snapshot/output decode
- stdout rendering
- clean disconnect/restore infrastructure

Exit:

- running Claude Code / Codex session に途中から attach し、現在画面と subsequent output を terminal で確認できる
- browser/workspace と同時 attach しても既存 server semantics のまま動く

### Phase 3 — Interactive terminal

Implement:

- raw terminal mode
- stdin -> `k:"i"`
- resize -> `k:"r"`
- detach key handling
- Ctrl-C passthrough
- robust terminal restore

Exit:

- Claude Code CLI / Codex CLI の主要 interactive flow を local terminal と同じ感覚で操作できる
- detach 後も session survives
- reattach して同じ session を継続できる

### Phase 4 — Session lifecycle commands

Implement:

- `ag new`
- `ag kill`
- optional `--no-attach`
- lifecycle outcome correlation/error reporting

Exit:

```text
ag new --agent claude --cwd <repo>
```

から server-side session create -> terminal attach まで一連で動く。

### Phase 5 — Cross-platform hardening

Test/fix:

- Linux native
- Windows terminal client -> local/WSL-hosted Grid Server
- macOS
- terminal resize behavior
- UTF-8 / IME-generated terminal input
- paste / bracketed paste where surfaced by local terminal
- abnormal disconnects
- protocol-version mismatch

Exit:

- daily-use CLI として raw terminal corruption を起こさない
- Windows/WSL を primary path として Claude/Codex の実運用が可能

## 15. Testing strategy

### 15.1 Unit tests

- session selector exact / prefix / ambiguous
- lifecycle frame parsing
- surface frame envelope parsing
- input frame encoding
- resize frame encoding
- JSON output stability
- config precedence
- exit-code mapping

### 15.2 Fake WebSocket integration tests

- first frame sends valid geometry
- initial surface snapshot is written to stdout
- subsequent output ordering
- stdin becomes input wire messages
- resize event produces current dimensions
- detach closes subscription without lifecycle termination request
- server close restores terminal
- malformed frame does not corrupt local terminal state

### 15.3 Live server integration tests

Use actual Grid Server and real session where practical.

Critical cases:

1. create Claude session -> attach -> type prompt -> receive output
2. detach -> verify session still running -> reattach
3. same for Codex
4. browser/workspace attached simultaneously with CLI
5. close terminal window/network -> verify session survival
6. resize repeatedly while Claude/Codex TUI is active
7. Ctrl-C interrupts remote agent operation rather than killing `ag`
8. kill command terminates only when explicitly invoked

### 15.4 Platform tests

Primary:

- Windows client terminal + WSL-hosted/local Grid Server

Secondary:

- Linux
- macOS

Windows native console behavior must not be inferred from Unix SIGWINCH/raw-mode implementation; test actual Windows Terminal path.

## 16. Acceptance criteria

The CLI client is v1-complete when all of the following hold.

### Terminal use

- `ag attach <session>` opens an existing Claude Code or Codex surface in the invoking terminal.
- ANSI/cursor/full-screen behavior is rendered by the local terminal without CLI-side emulation.
- keyboard input reaches the existing server-side PTY.
- terminal resize reaches the existing server resize path.
- Ctrl-C can be used by the remote Claude/Codex CLI.
- a dedicated local detach operation returns to the shell.

### Session continuity

- detach never implicitly terminates the session.
- CLI crash/network disconnect does not intentionally issue termination.
- reattach gets the existing server-provided terminal state/snapshot and continues the session.
- multiple clients can attach according to existing server behavior; CLI introduces no ownership policy.

### Lifecycle

- `ag ls` discovers live sessions from the existing lifecycle contract.
- `ag new` uses the existing lifecycle desired/create path and can immediately attach.
- `ag kill` is the explicit termination action.

### Architecture

- CLI does not allocate or own PTYs.
- CLI does not spawn Claude/Codex locally as the authoritative agent process.
- CLI does not implement terminal multiplexing.
- CLI does not implement terminal state/replay storage.
- CLI does not depend on `services/*` internal Go APIs for remote client behavior.
- no CLI-only server endpoint/protocol is added unless Phase 0 identifies a concrete missing public-contract capability.

## 17. Server-change rule

During implementation, a server change is acceptable only if all three are true:

1. the required capability is not already present in the execution plane/server;
2. the capability cannot be expressed through the current public client contract;
3. browser/workspace and CLI would benefit from the same public-contract fix.

If this occurs, record it as a small prerequisite contract change rather than silently moving execution responsibility into the CLI.

Examples of **not** acceptable reasons for a server change:

- easier to import an internal type from Go
- CLI wants a different terminal framing solely for convenience
- CLI wants its own controller/observer semantics
- duplicate REST endpoint is easier than using the existing lifecycle WS

## 18. Risks and mitigations

### Terminal state corruption on client failure

**Risk:** raw mode survives after abnormal CLI termination.

**Mitigation:** minimal raw-mode critical section, deferred restore, signal cleanup, real terminal integration tests.

### Wire-contract duplication

**Risk:** Go CLI manually redefines lifecycle/surface frames and drifts from browser/server.

**Mitigation:** derive models/constants from protocol SoT where practical; keep gateway adapter narrow; add compatibility fixtures against current server frames.

### Accidental session termination on detach

**Risk:** generic context cancellation is incorrectly mapped to lifecycle stop.

**Mitigation:** transport unsubscribe/close and session termination are separate code paths and APIs; test absence of terminate command on every attach shutdown path.

### Windows terminal differences

**Risk:** Unix raw-mode assumptions fail on Windows console APIs.

**Mitigation:** isolate terminal platform adapter; make Windows/WSL path a first-class Phase 5 target rather than a late portability check.

### Scope creep into a second Grid UI

**Risk:** CLI grows dashboards, approvals UI, TUI panes, session orchestration before terminal attach is excellent.

**Mitigation:** v1 success is `new / ls / attach / kill`; everything else requires a separate outcome-driven plan.

## 19. Recommended implementation order

The shortest path to useful value is:

```text
contract audit
  -> ls
  -> read-only attach
  -> interactive attach + detach + resize
  -> new + immediate attach
  -> kill
  -> cross-platform hardening
```

Do **not** begin with a broad Go SDK, TUI, new server endpoint set, or multiplexing design.

The first milestone worth dogfooding is simpler:

> From Windows Terminal or another local terminal, run `ag attach <existing-session>` and use the already-running Claude Code/Codex session as naturally as if it had been launched in that terminal, then detach and reattach without affecting the agent process.

Once that is solid, `ag new` turns the CLI into a full terminal entry point for Agent Grid rather than merely an attach utility.
