# Plan - Agent frame 間コミュニケーション MCP

- **作成日**: 2026-06-29
- **ステータス**: draft (検証済み仮説の設計化 / 未実装)
- **影響範囲**: `client/state`, `client/runtime`, `client/runtime/subsystem/stream`, `platform/mcpproxy` もしくは新規 MCP server、`server/web`、`client/web` の session view / tabs
- **目的**: agent-grid がホストする同一 session 内の agent frame 同士が、session 境界と監査の下で message / reply を配送できる MCP tool を提供する。

## 0. 今回の設計判断

この plan の成功条件は「別 session へ入力できること」ではなく、**依頼、配送、応答、監査の一連の通信が daemon 管理下で安全に完結すること**とする。したがって CLI の stdin/PTY 注入だけを実装しても不十分で、target agent のレスポンスをどの surface から受けるかを同時に設計対象に含める。

決定:

1. **応答取得は VT 画面解析を正経路にしない**
   - Codex は app-server の `item/agentMessage/delta` / `turn/completed` を正経路にする。既存の stream backend は `turnAssistant` / `lastAssistant` を保持しており、構造化 event から response を取れる。
   - Claude は transcript / hook event / driver state を正経路にする。既存の `client/lib/claude/transcript` と driver の `RecentTurns` / `StatusLine` を使う。
   - VT snapshot / terminal tail は UI 表示と最終手段の診断用に限定する。ANSI、再描画、spinner、partial line、alt-screen の影響を受けるため、protocol としては不安定。

2. **初期実装では専用 worker を置かない**
   - Inbox delivery は daemon state 更新で完結するため worker は不要。
   - Prompt delivery は target frame を所有する runtime / subsystem が同期的に gate し、受理結果を返す。
   - 非同期監視が必要なのは「delivery の受理」ではなく「target の応答完了待ち」であり、これは message correlation と既存 event fanout で扱う。独立 worker は Phase 2/3 で response timeout、retry、approval 待ち queue が必要になった時点で追加する。

3. **外部 MCP server として daemon 外に置かず、daemon 内 broker を authority にする**
   - MCP tool は agent から呼ぶ入口であり、権限判定、target state 判定、配送、audit は daemon 内で強制する。
   - MCP process を別にしても source frame 認証と target runtime への安全なアクセスは daemon が必要になるため、外部 MCP は thin adapter に留める。

4. **Claude への MCP 注入は `.mcp.json` overlay / `mcp-exec` を優先する**
   - Claude Code は MCP server を `.mcp.json` や `claude mcp add` / `add-json` で構成できる。
   - agent-grid には既に `platform/mcpproxy` が workspace の `.mcp.json` overlay を生成し、`server mcp-exec <alias>` で host MCP へ stdio relay する仕組みがある。
   - したがって本機能も Claude CLI 引数へ都度注入するより、agent-grid 管理 alias を overlay に追加する方が既存構造に合う。

5. **Codex には app-server tool として持たせる**
   - Codex の公開面として MCP 設定は `config.toml` にあるが、agent-grid 管理 session では app-server thread に `dynamicTools` を渡す経路が既にある。
   - Frame messaging tool は Codex app-server の `dynamicTools` / `item/tool/call` 経路で daemon に戻す。これにより source frame を app-server binding から解決しやすく、Codex の user config を書き換えない。
   - Project/global の `.codex/config.toml` 注入は user config 変更を伴うため、初期実装では避ける。

6. **初期 scope は同一 session 内の frame 間通信だけにする**
   - 通信可能な target は source frame と同じ session に属する frame のみに限定する。
   - 同一 session 内の frame 間 message / read / reply は allow/deny で制限しない。
   - session をまたぐ Cross Session 通信は初期実装では提供しない。将来 allow/deny policy で追加する余地は残すが、この plan からは外す。
   - これにより cross-project / cross-session approval、session-level allowlist、orchestrator cohort policy は初期実装では不要になる。

7. **Web UI は View 拡張が必要**
   - 現状の Web UI は terminal tab を主表示面としているため、inbox / reply / delivery status を表示・操作する surface がない。
   - Phase 1 でも message を人間が確認できる必要があるため、session view に `TERMINAL` 以外の tab / panel を追加する。

参考にした公開仕様:

- Claude Code MCP: `https://docs.anthropic.com/en/docs/claude-code/mcp`
- Codex MCP / app-server: `https://developers.openai.com/codex/mcp`、`https://developers.openai.com/codex/app-server`

## 1. 前提と検証結果

この plan でいう「起動中の対話セッション」は、任意の外部 terminal で動く CLI ではなく、**agent-grid が起動し、agent-grid daemon が管理している session / frame** を指す。初期実装で通信対象にできるのは、同一 session に属する frame のみとする。

用語:

- `session`: agent-grid runtime / UI 上の grouping。Cross Session 通信は初期実装では扱わない。
- `frame`: frame messaging の通信主体。MCP token、inbox、既読、audit は frame を authority にする。
- `thread` / `turn`: Codex app-server など driver-specific な実行状態。frame messaging の宛先 identity には使わない。
- `message`: 同一 session 内 frame の inbox に保存される durable object。
- `delivery`: prompt delivery の試行単位。Phase 1 の inbox-only では作らない。
- `reply`: message / delivery に対する response object。`finalAnswer` を含み得る。

検証済みの事実:

1. **Codex session は app-server 管理**
   - Codex driver は `LaunchSubsystemStream` を選ぶ。
   - stream backend が `codex app-server` を起動し、`thread/start` / `thread/resume` で app-server thread を bind する。
   - 表示用 frame は `codex --remote unix://<sock> ...` で同じ app-server に接続する。
   - したがって Codex への prompt 配送は PTY 注入ではなく app-server API を使うべき。

2. **Codex app-server API**
   - `turn/start` は idle thread に新しい user input を送る。
   - `turn/steer` は active turn に追加入力を送る。
   - 現行 schema の入力形式は `message` ではなく `input: [{ "type": "text", "text": "..." }]`。
   - `turn/steer` は `expectedTurnId` を必須 precondition とする。
   - agent response は PTY ではなく `item/agentMessage/delta` と `turn/completed` notification から取る。

3. **Claude session は PTY 管理**
   - Claude Code の `--remote-control` は今回の要件とは別物として扱う。
   - agent-grid 管理下の Claude は PTY master を daemon が保持するため、配送は `termvt.Session.WriteInput` 相当の surface input 経路を使う。
   - 単体検証では、PTY master に文字列を書いた場合 Claude の対話入力欄に反映された。
   - agent response は VT snapshot ではなく Claude transcript / hook event / driver state から取る。

4. **任意外部プロセスへの後付け注入は対象外**
   - `/dev/pts/N` の slave へ write しても stdin 注入にはならない。
   - `TIOCSTI` は環境依存で、この環境では失敗した。
   - 本 plan は agent-grid が master / app-server connection を保持している同一 session 内 frame のみを対象にする。

## 2. 目的

agent が同一 session 内の他 frame に安全に依頼・返信・引き継ぎできる仕組みを提供する。

達成したいこと:

1. agent frame から MCP tool 経由で、同一 session 内の target frame に message を送れる。
2. 必要な場合だけ、message を target agent の prompt として配送できる。これは frame messaging とは別の強い操作として扱う。
3. Codex と Claude の配送実装差を daemon 側で吸収する。
4. hard gate により、agent が勝手に session 外 frame を操作したり user / system を偽装したりできないようにする。
5. 全配送を監査可能にする。

## 3. 非目的

- 任意の外部 terminal / tmux / shell 上の CLI へ後付け入力する。
- agent が raw PTY bytes や任意 control sequence を送れるようにする。
- broadcast / swarm coordination を初期実装で提供する。
- agent が target frame の full transcript を自由に読む。
- human approval や policy を prompt instruction だけで代替する。
- session をまたぐ通信を提供する。
- project / cohort / explicit pair allowlist による Cross Session 通信を初期実装で提供しない。

## 4. 基本モデル

MCP server は「直接注入器」ではなく、agent-grid daemon の **frame messaging broker** への入口にする。

既定経路は inbox delivery:

```text
source agent
  -> MCP tool
  -> agent-grid daemon broker
  -> target frame inbox
  -> target agent / UI が読む
```

例外経路として prompt delivery:

```text
source agent
  -> MCP tool
  -> daemon hard gate
  -> driver-specific delivery
       Codex: app-server turn/start or turn/steer
       Claude: PTY surface input
```

prompt delivery は強い権限を要求し、default deny とする。

response collection は delivery と分離する:

```text
target agent response
  -> driver-specific structured source
       Codex: app-server notifications
       Claude: transcript / hook events
  -> daemon response correlator
  -> source frame inbox / MCP tool result polling
```

MCP tool の同期戻り値は「配送が accepted / rejected されたか」までを返す。agent の実回答は、短時間で完了した場合を除き `messageId` / `deliveryId` を返し、source agent が `agent_frames.read` で読む。長時間 blocking する MCP call は、tool timeout と agent の turn lifecycle を不安定にするため初期実装では避ける。

## 5. MCP Tool 案

### 5.1 `agent_frames.list`

通信可能なのは、source frame と同一 session 内の `claude` / `codex` driver frame のみとする。session をまたぐ Cross Session target は返さない。同一 session 内の frame は allow/deny で絞り込まないが、driver が agent ではない frame は対象外にする。

返す情報:

- `sessionId`
- `frameId`
- `driver`: `codex` / `claude`
- `project`
- `status`
- `capabilities`: `inbox`, `promptStart`, `promptSteer`, `ptySubmit` など
- `scopeSummary`: `same_session_only` など、agent に見せてよい範囲の概要

### 5.2 `agent_frames.read`

target frame の公開 inbox / summary / status を読む。

原則:

- raw transcript は返さない。
- `messageId` 単位で既読管理する。
- 同一 session 内 frame の inbox / reply metadata を返す。
- prompt delivery の response がある場合は、correlation された assistant response / final status を返す。

### 5.3 `agent_frames.send_message`

target frame の inbox に message を追加する。

入力案:

```json
{
  "targetFrameId": "frame-b",
  "topic": "review-api",
  "body": "直近の API 設計を確認してください",
  "priority": "normal"
}
```

これは prompt 注入ではない。初期実装の安全な既定経路とする。

### 5.4 `agent_frames.deliver_prompt`

target frame に prompt として配送する。

入力案:

```json
{
  "targetFrameId": "frame-b",
  "body": "直近の差分をレビューし、懸念点だけ返してください",
  "delivery": "prompt",
  "submit": true
}
```

戻り値案:

```json
{
  "accepted": false,
  "reason": "target_not_idle",
  "fallbackMessageId": "msg-123"
}
```

失敗時に inbox fallback するかは policy で決める。無断 fallback は避け、tool 入力または policy に明示する。

同期戻り値は delivery decision と correlation id までを基本にする:

```json
{
  "accepted": true,
  "deliveryId": "deliv-123",
  "targetMessageId": "msg-456",
  "responseMode": "poll"
}
```

`waitForResponse` は将来拡張に留める。初期実装では提供せず、caller は `deliveryId` を使って `read` polling に切り替える。

### 5.5 `agent_frames.reply`

受信 message に対する reply を作る。

入力:

- `messageId`
- `body`
- `resolution`: `answered` / `declined` / `needs_info`

### 5.6 `agent_frames.request_handoff`

作業移譲を提案する。初期実装では direct prompt delivery ではなく inbox message として扱う。

## 6. Hard Gate

hard gate は MCP server ではなく daemon 側で強制する。agent が tool input で偽装できない情報を authority とする。

### 6.1 Source Frame 認証

- MCP request の認証 token から source session / frame を解決する。
- `sourceSessionId` / `sourceFrameId` は tool input で受け取らない。
- container / frame に配布する token は frame-scoped にする。

### 6.2 Target Scope Gate

- target は source と同じ session に属する frame に限定する。
- session が異なる target は、allow/deny 設定を見る前に reject する。
- 初期実装では Cross Session allow/deny を提供しない。
- 同一 session 内の `inbox` message / read / reply は allow/deny で制限しない。
- `prompt` delivery は message ではなく target agent 操作なので、同一 session 内でも別 gate とする。初期値は deny。

### 6.3 Direction Policy

通信方向を `sourceFrame -> targetFrame` で評価する。

初期実装:

- same session only
- source frame 自身への送信は no-op 価値がないため reject
- session 外 target は reject
- same-session frame messaging は allow/deny なし

将来候補:

- explicit pair allowlist
- user-approved once / always
- orchestrator-created cohort
- cross-session / cross-project allowlist

### 6.4 Delivery Mode Policy

配送 mode ごとに gate 強度を分ける。

| Mode | 既定 | Gate |
|---|---|---|
| `inbox` | 同一 session 内は無制限 | same-session + self-target reject |
| `prompt_start` | deny | target idle + prompt permission |
| `prompt_steer` | deny | active turn id match + steer permission |
| `pty_submit` | deny | target idle + human approval 推奨 |

### 6.5 Target State Gate

Codex:

- `threadStatus == idle` なら `turn/start`。
- active turn があり、`activeTurnID` が記録されていて steerable なら `turn/steer`。
- `turn/steer` では `expectedTurnId` に daemon が保持する `activeTurnID` を入れる。
- app-server が `activeTurnNotSteerable` や `no active turn to steer` を返した場合は reject として扱う。

Claude:

- target が input-ready であること。
- input-ready は hook-derived state が `waiting` / `idle`、または OSC 133 prompt event 由来の waiting / idle 相当であることを正とする。
- `running`、`pending`、`stopped`、unknown は reject。
- hook と OSC が矛盾する場合は `unknown` として reject。
- terminal tail / VT snapshot は input-ready の positive signal に使わない。

### 6.6 Human Approval Gate

以下は human approval を要求できる policy にする。

- 初回 `source -> target`
- cross-project delivery
- prompt delivery
- active turn steer
- Claude PTY submit
- high priority / large payload

承認は daemon state に記録し、agent の prompt instruction で代替しない。

### 6.7 Provenance Envelope

prompt delivery では daemon が必ず envelope を付ける。source agent は user / system を偽装できない。

例:

```text
[agent-grid frame message]
from: sess-a
to: sess-b
message-id: msg-123
delivery: prompt
---
<sender body>
```

Codex app-server delivery でも Claude PTY delivery でも同じ envelope を入れる。

### 6.8 Sanitization

- MCP から raw control bytes は受けない。
- `body` は UTF-8 text として扱う。
- NUL、OSC、CSI、terminal control sequence は拒否または escape する。
- Claude PTY delivery で Enter を送る場合も daemon が `\r` を付与する。agent が任意 control sequence を送れないようにする。

### 6.9 Size / Fanout Guard

- 同一 session 内 frame messaging には allow/deny や rate limit を設けない。
- message size 上限は wire / persistence 保護のために設ける。
- fanout / broadcast は初期実装では禁止。

### 6.10 Audit Log

記録するもの:

- timestamp
- source session / frame
- target session / frame
- tool name
- delivery mode
- gate decision
- reason
- message hash
- human approval id

本文保存は設定で選択する。default は hash + metadata のみが望ましい。

## 7. Driver-specific Delivery

### 7.1 Codex

Codex は stream backend が app-server connection と thread binding を持つため、ここに delivery method を追加する。

必要な内部情報:

- `frameBinding.threadID`
- `frameBinding.activeTurnID`
- `frameBinding.threadStatus`
- `frameBinding.waitApproval`

配送:

- idle: `turn/start`
- running: `turn/steer`

応答取得:

- `item/agentMessage/delta` を `deliveryId` に紐付く streaming response として蓄積する。
- `turn/completed` を final status として扱い、`lastAssistant` を source frame の inbox reply に反映する。
- app-server が返す request error は delivery rejection として audit に残す。

注意:

- 現在の `codexclient.StartTurn` は現行 schema とずれている可能性があるため、`input` 配列形式へ更新する。
- `turn/steer` client helper を追加する。
- app-server response / error を gate result として上位へ返す。
- Codex に frame messaging tool を見せる経路は `dynamicTools` を第一候補にする。Codex の `config.toml` / MCP 設定を書き換える経路は初期実装の対象外にする。

### 7.2 Claude

Claude は PTY surface input を使う。

配送:

- `submit=false`: text を入力欄に挿入するだけ。
- `submit=true`: text + Enter を送る。

初期実装では `submit=true` のみでもよいが、誤送信のリスクがあるため `human approval` と `target idle` gate を必須にする。

応答取得:

- transcript tracker が観測する assistant turn を `deliveryId` に紐付ける。
- hook event / driver status で turn start / completion を補助判定する。
- VT snapshot は response 抽出には使わない。status unknown の補助診断に限る。

MCP 注入:

- Claude 側 agent に frame messaging tool を見せる経路は `.mcp.json` overlay と `mcp-exec` broker を第一候補にする。
- CLI 引数での one-shot 注入は初期実装では使わない。必要なら対象 Claude Code version で別途検証する。agent-grid では workspace overlay の方が shared container / project mount と整合する。

## 7.3 Response Correlation

prompt delivery は必ず `deliveryId` を発行し、daemon が付与する provenance envelope に含める。

### 7.3.1 Final Answer Semantics

frame messaging response で返す `finalAnswer` は「target agent が source agent に返す成果物」であり、transcript 上の全 assistant text ではない。進捗説明、preamble、tool-use narration、thinking、subagent の中間出力を final answer に混ぜない。

優先順位:

1. **明示 reply**
   - target agent が `agent_frames.reply` を呼んだ内容を final answer とする。
   - prompt delivery でも、envelope 内で「完了時は `agent_frames.reply` で `deliveryId` に reply する」ことを指示する。
   - これが唯一の high-confidence final answer。

2. **Codex app-server の final phase**
   - Codex は app-server event / turn item に `phase=final_answer` を持てる。
   - `turn/completed` の `turn.items` に `agentMessage` + `phase=final_answer` があれば、それを final answer とする。
   - `item/agentMessage/delta` は streaming 表示用であり、単独では final 判定に使わない。
   - `phase=commentary` や reasoning / tool item は除外する。

3. **driver-specific heuristic fallback**
   - Claude transcript には thinking や途中 narration が含まれるため、transcript だけでは厳密な final answer は確定できない。
   - fallback では correlated user entry 以降、target が idle / completion 相当になった時点の最後の `KindAssistantText` のみを候補にする。
   - `KindAssistantThinking`、`KindToolUse`、`KindToolResult`、system / attachment / subagent inline output は除外する。
   - fallback result は `confidence=heuristic` とし、明示 reply と同格に扱わない。

MCP/API の response shape には、少なくとも以下を持たせる:

```json
{
  "deliveryId": "deliv-123",
  "status": "completed",
  "finalAnswer": "調査結果...",
  "answerSource": "explicit_reply",
  "confidence": "high"
}
```

`answerSource` 候補:

- `explicit_reply`
- `codex_final_answer`
- `driver_heuristic`
- `none`

Codex:

- `turn/start` / `turn/steer` の戻り値または直後の `turn/started` から `turnID` を確定する。
- `frameBinding.activeTurnID` と `deliveryId` を対応付ける。
- `item/agentMessage/delta` / `turn/completed` の `turnID` が一致するものだけを response 候補とする。
- final answer は `turn/completed` の `phase=final_answer` を優先し、なければ既存 backend と同じく最後の agent message を `driver_heuristic` として扱う。

Claude:

- prompt envelope 内の `deliveryId` と直後の transcript user entry を対応付ける。
- その user entry 以降、次の assistant text / completion 相当を response 候補とする。
- final answer は `agent_frames.reply` を優先する。transcript からは最後の `KindAssistantText` だけを heuristic fallback として採用する。
- Claude の transcript で delivery marker が見つからない場合は `response_status=unknown` とし、VT から推測して成功扱いにしない。

Source session への返却:

- target response は source frame inbox に `replyTo: deliveryId` として保存する。
- MCP caller は `agent_frames.read` で response を取得する。
- 生 transcript 全体は返さず、correlation された final answer / response candidate と metadata のみに制限する。

## 8. 状態と Wire 型

追加候補:

- `FrameMessage`
- `FrameDeliveryRequest`
- `FrameDeliveryDecision`
- `FrameMessagingPolicy`
- `FrameMessagingAuditRecord`

永続化:

- inbox / reply は session snapshot とは別 store にする。
- audit log は別 append-only store にする。
- persistence 型は stdlib-only を維持する。

## 8.1 実装設計前の仕様決定ポイントと Recommend

採用済み決定:

- self-target は reject する。無制限 messaging の例外ではなく、no-op / loop 防止の整合性チェックとして扱う。
- `agent_frames.read` は同一 session 内の全 frame message / reply を読める。raw transcript は読めない。
- Phase 1 は `targetFrameId` 必須。broadcast / all-frames は実装しない。
- `agent_frames.list` は同一 session 内の `claude` / `codex` driver frame に限定する。
- message / delivery の宛先は作成時点の `frameId` に固定する。target frame が消えたら `target_lost` とし、再接続や復活推測はしない。
- inbox / reply は session snapshot とは別 store に保存する。audit も別 append-only store にする。
- 既読は Phase 1 では frame 単位のみ。Web 既読と agent 既読は分けない。
- 1 message に複数 reply を許す。`resolution` は message thread の最新 state として扱う。
- Web UI は Phase 1 では閲覧 + status 確認のみ。UI からの送信は後回しにする。
- prompt delivery は frame messaging とは別 capability。Phase 1 では実装しない。Phase 2 の Codex `prompt_start` も明示 opt-in まで実験扱いにする。
- MCP tool 名は `agent_frames.*` で確定する。

### 8.1.1 Wire Model

Point:

- `message`、`delivery`、`reply`、`finalAnswer` の境界を曖昧にすると、UI、audit、MCP result、retry の責務が混ざる。
- Inbox-only と prompt delivery を同じ操作に見せると、安全な message と強い権限が必要な prompt injection の区別が崩れる。

Recommend:

- `FrameMessage` は inbox に保存される durable object とする。
- `FrameDelivery` は prompt delivery の試行単位とする。`messageId` を起点にしてもよいが、prompt 注入、state gate、response correlation、timeout は `deliveryId` で管理する。
- `FrameReply` は `messageId` または `deliveryId` に紐付く response object とする。
- `finalAnswer` は `FrameReply` の field とし、raw transcript から独立させる。

### 8.1.2 State Machine

Point:

- delivery の同期戻り値と、target agent の実応答完了は別 lifecycle。
- `accepted` だけでは、配送済みなのか、agent が読んだのか、回答が終わったのかを表せない。

Recommend:

- delivery status は以下に固定する。

```text
created -> gated -> accepted -> started -> completed
                         |          |          |
                         |          |          -> failed
                         |          -> timed_out / target_lost / unknown
                         -> rejected
```

- MCP call の直接戻り値は原則 `accepted` / `rejected` と `deliveryId` までにする。
- `completed` は `agent_frames.read` で取得する。
- daemon restart 後に復元できない in-flight delivery は `unknown` に遷移させ、成功推測しない。

### 8.1.3 Reply Contract

Point:

- Claude transcript には thinking / tool narration / subagent output が含まれるため、transcript から最終返答を厳密に確定できない。
- Codex app-server は `phase=final_answer` を持てるが、すべての経路で常に存在するとは限らない。

Recommend:

- すべての prompt envelope に「完了時は `agent_frames.reply` で `deliveryId` に reply する」を含める。
- `agent_frames.reply` を high-confidence final answer の唯一の正規経路にする。
- Codex の `phase=final_answer` は `answerSource=codex_final_answer` として accepted fallback にする。
- Claude transcript fallback は `answerSource=driver_heuristic` / `confidence=heuristic` に限定する。

### 8.1.4 Policy Matrix

Point:

- `read`、`send_message`、`deliver_prompt`、`reply` は危険度が違う。
- 同一 session 内の frame messaging は制限しないが、Cross Session は scope 外として常に拒否する。
- prompt delivery は message ではなく target agent 操作なので、frame messaging とは別 gate にする。

Recommend:

| Capability | Default | Recommend |
|---|---|---|
| `list` | unrestricted same-session | 同一 session 内 frame を返す。Cross Session は返さない |
| `read` | unrestricted same-session | 同一 session 内 frame の inbox / reply を読める |
| `send_message` | unrestricted same-session | 同一 session 内 frame へ制限なく送れる |
| `reply` | unrestricted same-session | 同一 session 内 frame の message / delivery に返信できる |
| `deliver_prompt_start` | deny | Phase 2 以降、idle + explicit allow + audit |
| `deliver_prompt_steer` | deny | 初期実装では実装しない |
| `read_target_status` | filtered | idle/running/unknown と capability のみ |
| `read_transcript` | deny | raw transcript は公開しない |

- `list/read/send_message/reply` は同一 session 内で隠さない。
- `deliver_prompt` は同一 session 内でも default deny。権限がない source には、可能なら tool 自体を出さない。

### 8.1.5 Identity Scope

Point:

- `sourceSessionId` / `sourceFrameId` を tool input に含めると偽装できる。
- session 単位で target を指定すると、同一 session 内の複数 frame / thread の権限境界が曖昧になる。

Recommend:

- source identity は daemon が発行した token から解決し、tool input では受け取らない。
- token は frame-scoped を推奨する。UI 表示や audit では session と frame の両方を記録する。
- target は `targetFrameId` を必須にする。session-level target alias は初期実装では提供しない。

### 8.1.6 Claude Idle Gate

Point:

- Claude PTY は app-server のような authoritative turn state を持たない。
- VT tail だけで idle 判定すると誤送信しやすい。

Recommend:

- Phase 3 まで Claude prompt delivery は実装しない。
- 実装時は hook-derived state または OSC 133 prompt-derived state が `waiting` / `idle` 相当なら input-ready とする。
- `permission_prompt` / approval pending / running / stopped / unknown は reject。
- hook と OSC の freshness は last prompt submission / delivery attempt より後であることを要求する。
- 判定が割れた場合は `unknown` として reject する。
- terminal tail / VT snapshot は input-ready の positive signal に使わない。

### 8.1.7 Persistence / Audit

Point:

- inbox は user-visible data、audit は security data で保持要件が違う。
- 本文を audit に保存すると機密漏洩面が増える。

Recommend:

- inbox/reply 本文は durable store に保存する。
- audit は default で metadata + body hash のみ保存する。
- body 保存 audit は明示設定がある場合だけにする。
- append-only audit file とし、delivery state の現在値とは分離する。

### 8.1.8 Fallback

Point:

- prompt delivery 失敗時に inbox fallback すると、agent が意図しない形で情報を送る可能性がある。

Recommend:

- 無断 fallback は禁止する。
- `fallback: "none" | "inbox_on_reject"` のように tool input または policy で明示する。
- default は `none`。

### 8.1.9 Claude Transcript Fallback

Point:

- Claude transcript は driver の観測 source として有用だが、delivery marker / envelope が実 transcript 上で常に安定する保証はない。
- transcript には thinking、tool narration、subagent inline output、途中説明が含まれるため、最終返答の正規 source にすると誤判定しやすい。

Recommend:

- Claude の high-confidence final answer は `agent_frames.reply` のみとする。
- delivery envelope には `deliveryId` と `reply` instruction を入れるが、marker が transcript に残ることを成功条件にしない。
- transcript marker が見つかる場合だけ、correlated user entry 以降の最後の `KindAssistantText` を `answerSource=driver_heuristic`, `confidence=heuristic` の fallback reply として保存する。
- marker が見つからない、または hook / transcript の順序が矛盾する場合は `response_status=unknown` とする。VT snapshot から成功推測しない。
- Phase 3 着手前に fake transcript と実 transcript recording で、envelope marker が user entry として観測できるケース / 欠落するケースを contract test 化する。

### 8.1.10 waitForResponse

Point:

- MCP client / agent runtime の tool timeout は caller 側に依存し、daemon から安全に制御できない。
- target agent の turn は長時間化し得るため、prompt delivery tool を response 完了まで blocking すると caller turn と target turn の両方が不安定になる。

Recommend:

- Phase 1 / Phase 2 では `waitForResponse` を提供しない。
- `agent_frames.deliver_prompt` の同期戻り値は `accepted` / `rejected`、`deliveryId`、`responseMode="poll"` までとする。
- response 取得の正経路は `agent_frames.read` polling と Web UI の delivery status 表示にする。
- 将来 `waitForResponse` を追加する場合も default off とし、上限は短い server-side timeout に固定する。timeout 時は失敗扱いではなく `deliveryId` と `responseMode="poll"` を返す。
- MCP client 側 timeout より短い上限を daemon 側で強制し、caller supplied timeout が上限を超える場合は clamp する。

### 8.1.11 Phase Cut

Point:

- 最初から prompt delivery、approval UI、response waiting、Claude PTY を入れると検証面が広すぎる。

Recommend:

- Phase 1 は inbox-only + reply contract + audit + same-session scope gate まで。
- Phase 2 は Codex `prompt_start` のみ。`turn/steer` は別 Phase にする。
- Phase 3 は Claude prompt delivery。idle gate と transcript fallback の実機検証が揃うまで入れない。
- `waitForResponse` は Phase 1/2 では入れず、polling を正経路にする。
- Cross Session allow/deny はどの Phase にも含めない。別 plan で扱う。

## 8.2 Web UI / View 変更

現状:

- `state/view.View` は `LogTabs` を持つ。
- Web UI の `MainTabs` は synthetic `TERMINAL` tab と driver-provided `log_tabs` を表示する。
- `LogTabs` は transcript / event-log のような append-only file 表示を前提にしており、inbox / reply / delivery status のような stateful object 表示には向かない。

Recommend:

- `View` に frame messaging 用の summary payload を追加する。
  - 未読件数
  - 最新 message / reply の preview
  - pending delivery count
  - last delivery status
- Web UI は `TERMINAL` と `LogTabs` だけでなく、stateful panel を持てる tab model に拡張する。
- Phase 1 の最小 UI は `MESSAGES` tab / panel とする。
- `MESSAGES` panel は source / target frame、topic、body preview、reply status、finalAnswer preview を表示する。
- raw transcript は表示しない。必要なら既存 `TRANSCRIPT` tab へ明示的に移動する。
- message 作成 UI は初期実装では必須にしない。Phase 1 では「受信・返信・状態確認」ができればよい。

完了条件:

- active session に sibling frame message がある場合、Web UI に `MESSAGES` tab / panel が表示される。
- `TERMINAL` tab は従来どおり常時利用できる。
- `TRANSCRIPT` / `EVENTS` など既存 `LogTabs` は regression なく表示される。
- `MESSAGES` は log file tailing ではなく daemon snapshot / event update から描画される。

## 9. 実装フェーズ

### Phase 1: Inbox-only

- MCP tools: `list`, `send_message`, `read`, `reply`
- Claude 向けには `.mcp.json` overlay / `mcp-exec` alias として露出する。
- Codex 向けには app-server `dynamicTools` として露出する。
- direct prompt delivery なし。
- same-session scope gate、self-target reject、audit log の最小実装。
- 同一 session 内 frame messaging の allow/deny は実装しない。
- Web UI に inbox / replies を確認する View surface を追加する。

提案仕様:

- Store:
  - `messages.jsonl`: append-only。`FrameMessageCreated` / `FrameReplyCreated` / `FrameMessageResolutionUpdated` を記録する。
  - `message_state.json`: compaction 用の現在値。daemon 起動時は `messages.jsonl` から復元できることを正とし、`message_state.json` は最適化扱いにする。
  - 配置は session-scoped store とし、session snapshot には含めない。
- Wire schema:
  - `FrameMessage`: `id`, `sessionId`, `sourceFrameId`, `targetFrameId`, `topic`, `body`, `priority`, `createdAt`, `readByFrameIds`, `resolution`, `replyIds`
  - `FrameReply`: `id`, `sessionId`, `messageId`, `deliveryId`, `sourceFrameId`, `targetFrameId`, `body`, `finalAnswer`, `answerSource`, `confidence`, `resolution`, `createdAt`
  - `FrameMessagingSummary`: `unreadCount`, `latestMessagePreview`, `latestReplyPreview`, `pendingDeliveryCount`, `lastDeliveryStatus`
- MCP:
  - `agent_frames.list`: source と同一 session かつ driver が `claude` / `codex` の frame を返す。self frame は返してよいが `capabilities.inbox=false` とし、send 時は reject する。
  - `agent_frames.send_message`: `targetFrameId`, `topic`, `body`, `priority` を受ける。target が同一 session の `claude` / `codex` frame でなければ reject。
  - `agent_frames.read`: 同一 session 内の全 frame message / reply を読める。raw transcript は返さない。read は caller frame の既読を更新する。
  - `agent_frames.reply`: `messageId` または `deliveryId` に返信する。Phase 1 では `messageId` 必須、`deliveryId` は Phase 2 で使う。
- Source auth:
  - frame-scoped token を MCP bridge / dynamic tool invocation に紐付ける。
  - tool input の `sourceSessionId` / `sourceFrameId` は無視ではなく schema 上受け取らない。
- Web:
  - `View` には `FrameMessagingSummary` のみ載せる。
  - message 一覧と本文は別 API: `GET /api/sessions/{sessionId}/messages`。
  - Phase 1 の Web UI は `MESSAGES` tab で閲覧と既読更新だけ行う。送信 UI は作らない。

完了条件:

- source session / frame を token から解決できる。
- 同一 session 内の target frame に message が届く。
- self-target と session 外 target は拒否される。
- 同一 session 内 target について allow/deny 設定なしで送受信できる。
- audit record が残る。
- source agent が `read` で inbox reply を取得できる。
- Web UI で `TERMINAL` 以外の tab / panel から inbox と reply status を確認できる。
- daemon restart 後も `messages.jsonl` から message / reply が復元される。

### Phase 2: Codex app-server prompt delivery

- Codex stream backend に delivery method を追加。
- `turn/start` helper を現行 schema で実装。
- target state gate を実装。
- app-server notification から response correlation を実装。

提案仕様:

- Scope:
  - Phase 2 は Codex `prompt_start` のみ。
  - `turn/steer`、Claude PTY delivery、Cross Session delivery は実装しない。
  - Human approval UX は未解決のまま残す。Phase 2 の有効化は config / feature flag / developer-only opt-in に限定する。
- Gate:
  - target は同一 session 内の `codex` frame。
  - target frame は `threadID != ""` かつ `threadStatus == idle` でなければ reject。
  - target frame が approval pending / waitApproval の場合は reject。
  - source self-target は reject。
- Delivery:
  - `FrameDelivery`: `id`, `messageId`, `sessionId`, `sourceFrameId`, `targetFrameId`, `targetThreadId`, `targetTurnId`, `status`, `body`, `createdAt`, `startedAt`, `completedAt`, `error`
  - `agent_frames.deliver_prompt` は `targetFrameId`, `body`, `messageId?`, `fallback` を受ける。
  - daemon が provenance envelope を付与し、`codexclient.StartTurn(conn, threadID, startDir, []byte(envelopedBody), TurnOptions{})` を呼ぶ。
  - `fallback` default は `none`。reject 時に inbox へ落とすのは `fallback=inbox_on_reject` のときだけ。
- State:
  - delivery status は `created -> gated -> accepted -> started -> completed` を基本にする。
  - app-server request error は `failed`。
  - target frame release / target thread mismatch / daemon restart recovery failure は `target_lost` または `unknown`。
  - Phase 2 では `waitForResponse` を提供しない。caller は `agent_frames.read` で polling する。
- Correlation:
  - `turn/start` の request result または直後の `turn/started` から `turnID` を確定し、`deliveryId -> turnID` を記録する。
  - `item/agentMessage/delta` は streaming candidate として蓄積するが final 判定には使わない。
  - `turn/completed` の `turn.items` に `agentMessage` + `phase=final_answer` があれば `answerSource=codex_final_answer`, `confidence=high` の `FrameReply` を作る。
  - `phase=final_answer` がない場合は最後の agent message を `answerSource=driver_heuristic`, `confidence=heuristic` として保存する。
  - app-server `error` notification / request error は `FrameDelivery.error` と audit に残す。
- UI:
  - `MESSAGES` tab に delivery status、target frame、finalAnswer preview を表示する。
  - active TUI 側の表示反映は app-server event 由来の既存 stream backend event を使う。Phase 2 で新しい terminal injection はしない。
- Tests:
  - fake app-server に `turn/start` が `input: [{type:"text", text:<envelope>}]` で送られる。
  - target non-idle / waitApproval / self-target / non-Codex target は reject。
  - `turn/completed` の `phase=final_answer` が reply になる。
  - `commentary` のみの場合は heuristic reply になる。
  - daemon restart recovery できない in-flight delivery は success にしない。

完了条件:

- 同一 session 内の idle Codex frame に `turn/start` で prompt が届く。
- TUI 側にも配送結果が反映される。
- source frame が `deliveryId` に紐付いた assistant response を `read` で取得できる。
- Human approval UX 以外の gate / delivery / correlation / persistence / UI 表示仕様が確定している。

### Phase 3: Claude gated PTY delivery

- Claude target state 判定を実装。
- sanitized text のみ PTY に送る。
- human approval policy を接続。
- transcript / hook event から response correlation を実装。

完了条件:

- 同一 session 内の idle Claude frame にだけ prompt を submit できる。
- running / unknown 状態では拒否される。
- control sequence は拒否される。
- source frame が `deliveryId` に紐付いた assistant response を `read` で取得できる。
- transcript marker が見つからない場合は成功推測せず `response_status=unknown` になる。

### Phase 4: UI / Policy Management

- Web UI の frame messaging tab / panel を整える。
- delivery status / reply / audit viewer を表示する。
- pending approval UI。
- cross-session allow/deny UI は初期実装では作らない。

## 10. Test Plan

Unit:

- gate reducer: same-session / self-target / session外 target matrix。
- source token spoofing rejection。
- target frame scope gate。
- delivery mode policy。
- sanitization。
- message size / fanout guard。
- response correlation。
- final answer source precedence。
- 将来 `waitForResponse` を追加する場合の timeout 時 polling fallback。

Codex integration:

- fake app-server connection に `turn/start` payload が現行 schema で送られる。
- app-server error が delivery rejection に変換される。
- `item/agentMessage/delta` / `turn/completed` が `deliveryId` に紐付いて source inbox reply になる。
- `phase=final_answer` が `commentary` / reasoning / tool output より優先される。

Claude integration:

- fake PTY に sanitized prompt + Enter が書かれる。
- control sequence を含む body は拒否される。
- target non-idle では PTY write が発生しない。
- fake transcript で envelope marker 後の assistant response が `deliveryId` に紐付く。
- `thinking` / tool use / tool result / subagent inline output は final answer にならない。
- transcript fallback は `confidence=heuristic` になる。
- VT snapshot のみでは response success にしない。

MCP:

- tool schema。
- unauthorized source。
- self-target / session外 target rejection。
- audit emission。
- Claude `.mcp.json` overlay に frame messaging alias が入る。
- Codex `dynamicTools` に frame messaging tool schema が入る。

## 11. 未解決事項

1. human approval の UX を既存 approval surface と統合するか。
