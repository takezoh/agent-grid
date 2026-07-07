---
id: spec-20260706-frame-messaging
kind: spec
title: Same-session frame messaging
status: draft
created: '2026-07-06'
tags:
- mcp
- frame-messaging
owners: []
functional_requirements:
- id: FR-001
  statement: agent frame は MCP tool 経由で、同一 session 内の claude/codex driver frame を列挙できなければならない
  priority: must
  rationale: 通信可能範囲を session 境界内に限定し、agent に不要な cross-session 情報を見せないため
- id: FR-002
  statement: agent frame は MCP tool 経由で、同一 session 内の target frame inbox に message
    を保存できなければならない
  priority: must
  rationale: prompt 注入より安全な既定経路として、daemon 管理下の durable message を提供するため
- id: FR-003
  statement: system は source identity を frame-scoped token または daemon binding から解決し、tool
    input で sourceSessionId/sourceFrameId を受け取ってはならない
  priority: must
  rationale: agent が source frame を偽装できる設計を避けるため
- id: FR-004
  statement: system は self-target と session 外 target を daemon 側 hard gate で拒否しなければならない
  priority: must
  rationale: no-op loop と cross-session 操作を policy 以前の境界違反として扱うため
- id: FR-005
  statement: agent frame は同一 session 内の message/reply metadata と本文を read できなければならないが、raw
    transcript を read 経路で取得できてはならない
  priority: must
  rationale: frame messaging と transcript inspection の権限境界を分離するため
- id: FR-006
  statement: agent frame は message または delivery に対する reply を作成でき、reply は finalAnswer/source/confidence/resolution
    を表現できなければならない
  priority: must
  rationale: response collection を raw transcript ではなく明示 reply contract に寄せるため
- id: FR-007
  statement: Phase 1 は inbox-only とし、deliver_prompt、turn/start、PTY submit、waitForResponse
    を提供してはならない
  priority: must
  rationale: 安全な durable messaging と強い agent 操作を分け、検証面を限定するため
- id: FR-008
  statement: Web UI は TERMINAL 以外の MESSAGES surface で inbox/reply status を確認できなければならない
  priority: must
  rationale: 人間が daemon 管理下の frame communication を監査・確認できる surface が必要なため
- id: FR-009
  statement: message/reply は session snapshot とは別の session-scoped store に永続化され、daemon
    restart 後に復元できなければならない
  priority: must
  rationale: session runtime state と user-visible communication data の寿命を分離するため
- id: FR-010
  statement: audit log は tool name、source/target、gate decision、reason、body hash を記録し、本文保存は既定で行ってはならない
  priority: must
  rationale: 監査可能性と本文漏洩面の最小化を両立するため
- id: FR-011
  statement: prompt delivery は inbox message と別 capability として扱われ、同一 session 内でも既定
    deny でなければならない
  priority: should
  rationale: target agent の turn を操作する強い権限を通常 message と混同しないため
- id: FR-012
  statement: Codex prompt delivery を実装する場合、app-server の turn/start と structured notification
    を正経路にし、VT snapshot を response source にしてはならない
  priority: should
  rationale: Codex は app-server thread が authoritative source であり、terminal 表示解析は protocol
    として不安定なため
- id: FR-013
  statement: Claude prompt delivery を実装する場合、sanitized text のみを daemon が PTY surface
    input に書き込み、idle gate と human approval policy を通さなければならない
  priority: could
  rationale: Claude は PTY 管理だが、raw control bytes と状態不明時 submit は安全でないため
non_functional_requirements:
- id: NFR-001
  type: security
  criteria: source identity は tool input から復元されず、same-session/self-target gate は MCP
    adapter ではなく daemon broker で強制される
  measurement: spoofing input を受け付けない schema test と broker gate matrix test が green
- id: NFR-002
  type: reliability
  criteria: daemon restart 後、messages.jsonl から message/reply/read state が復元され、incomplete
    state を成功推測しない
  measurement: restart recovery test が green
- id: NFR-003
  type: security
  criteria: read API と audit log は raw transcript を返さず、audit 本文保存は明示設定なしでは無効
  measurement: API response/audit fixture に transcript body が含まれないことを test で検証
- id: NFR-004
  type: maintainability
  criteria: persistence wire 型は stdlib-only とし、client/orchestrator/platform の import
    方向制約を破らない
  measurement: make lint と go test が green
acceptance:
- id: AC-001
  given: 同一 session 内に source frame と target frame が存在する
  when: source が agent_frames.send_message を呼ぶ
  then: target inbox に message が保存され、source/target が agent_frames.read で確認できる
  requirement_refs:
  - FR-002
  - FR-005
- id: AC-002
  given: source が自分自身または別 session の frame を target に指定する
  when: send_message を呼ぶ
  then: broker は reject し、audit に decision と reason を記録する
  requirement_refs:
  - FR-004
  - FR-010
- id: AC-003
  given: tool input に sourceSessionId/sourceFrameId 相当の偽装 field が含まれる
  when: MCP schema validation または broker invocation が実行される
  then: source identity は input から採用されず、schema は不要 field を受け付けない
  requirement_refs:
  - FR-003
  - NFR-001
- id: AC-004
  given: message を受信した target frame
  when: target が agent_frames.reply を呼ぶ
  then: reply が message に紐付き、source が read で finalAnswer と resolution を取得できる
  requirement_refs:
  - FR-006
- id: AC-005
  given: message/reply が保存済みの session
  when: daemon を restart する
  then: messages.jsonl から inbox/reply/read state が復元される
  requirement_refs:
  - FR-009
  - NFR-002
- id: AC-006
  given: active session に frame message または reply が存在する
  when: Web UI で session view を開く
  then: TERMINAL tab を維持したまま MESSAGES surface で inbox/reply status を確認できる
  requirement_refs:
  - FR-008
- id: AC-007
  given: Phase 1 build
  when: agent_frames deliver_prompt 相当の操作を探す
  then: prompt delivery tool は公開されず、turn/start または PTY submit は実行されない
  requirement_refs:
  - FR-007
  - FR-011
- id: AC-008
  given: audit record と read API response
  when: message body と transcript を含む session で操作する
  then: audit は body hash と metadata のみを持ち、read API は raw transcript を返さない
  requirement_refs:
  - FR-005
  - FR-010
  - NFR-003
relations:
- {type: implementedBy, target: plan-20260706-frame-messaging}
- {type: referencedBy, target: note-20260706-inter-session-mcp-original-plan}
source_paths:
- src/client/state/
- src/client/runtime/
- src/client/runtime/subsystem/stream/
- src/platform/mcpproxy/
- src/server/web/
- src/client/web/
methodology: baseline
summary: 同一 session 内の agent frame 間で message/reply を配送し、prompt delivery を別権限として扱う
  frame messaging MCP の仕様
---

# Spec — Same-session frame messaging

## Overview

この spec は、agent-grid が管理する同一 session 内の agent frame 同士が、daemon の session 境界・監査・永続化の下で message と reply をやり取りする MCP surface を定義する。

成功条件は「別 frame の terminal に文字列を入れること」ではない。依頼、保存、既読、返信、監査、Web での確認が daemon 管理下で完結することを成功条件にする。prompt delivery は target agent の turn を操作する強い capability なので、inbox message とは別仕様として扱い、Phase 1 では提供しない。

## Scope

対象は agent-grid daemon が管理する `session` と、その session に属する `frame` だけである。任意の外部 terminal、tmux、shell、または `/dev/pts` への後付け注入は扱わない。

`frame` は frame messaging の authority であり、MCP token、inbox、既読、reply、audit の単位になる。`thread` や `turn` は driver-specific な実行状態であり、宛先 identity には使わない。

## Requirements

{% req id="FR-001" %}
**同一 session frame listing** — `agent_frames.list` は source frame と同じ session に属する `claude` / `codex` driver frame だけを返す。Cross Session target は返さない。self frame は表示してよいが、送信可能 capability は false として扱う。
{% /req %}

{% req id="FR-002" %}
**inbox message** — `agent_frames.send_message` は `targetFrameId`、`topic`、`body`、`priority` を受け取り、target frame の durable inbox に `FrameMessage` を保存する。これは prompt 注入ではない。
{% /req %}

{% req id="FR-003" %}
**source identity** — source session/frame は frame-scoped token または app-server binding から daemon が解決する。tool input に `sourceSessionId` / `sourceFrameId` を持たせない。
{% /req %}

{% req id="FR-004" %}
**hard gate** — daemon broker は self-target と session 外 target を拒否する。同一 session 内の inbox message/read/reply には allow/deny policy を設けない。
{% /req %}

{% req id="FR-005" %}
**read boundary** — `agent_frames.read` は同一 session 内の inbox/reply/status を返す。raw transcript、terminal tail、VT snapshot は返さない。
{% /req %}

{% req id="FR-006" %}
**reply contract** — `agent_frames.reply` は message または delivery に対する `FrameReply` を作る。`finalAnswer` は明示 reply を high-confidence source とし、driver heuristic と区別する。
{% /req %}

{% req id="FR-007" %}
**Phase 1 inbox-only** — Phase 1 は `list`、`send_message`、`read`、`reply` だけを公開する。`deliver_prompt`、Codex `turn/start`、Claude PTY submit、`waitForResponse` は提供しない。
{% /req %}

{% req id="FR-008" %}
**Web surface** — session view は `TERMINAL` を維持しつつ、`MESSAGES` surface で inbox、reply status、latest preview を確認できる。
{% /req %}

{% req id="FR-009" %}
**persistence** — message/reply は session snapshot と別の session-scoped store に保存する。`messages.jsonl` を復元の正本にし、compaction state は最適化扱いにする。
{% /req %}

{% req id="FR-010" %}
**audit** — audit は append-only とし、timestamp、source/target、tool name、delivery mode、gate decision、reason、body hash を記録する。本文保存は明示設定なしでは行わない。
{% /req %}

{% req id="FR-011" %}
**prompt delivery separation** — prompt delivery は message ではなく target agent 操作として扱う。同一 session 内でも default deny とし、inbox fallback は tool input または policy で明示された場合だけ許可する。
{% /req %}

{% req id="FR-012" %}
**Codex delivery posture** — Codex prompt delivery を実装する場合、app-server `turn/start` と `turn/completed` / `agentMessage` notification を正経路にする。VT snapshot は response source にしない。
{% /req %}

{% req id="FR-013" %}
**Claude delivery posture** — Claude prompt delivery を実装する場合、hook/OSC derived idle gate と human approval policy を通し、daemon が sanitized text だけを PTY surface input に書く。
{% /req %}

## Non-Goals

- session をまたぐ communication、project/cohort allowlist、broadcast/swarm coordination。
- 任意外部プロセスや `/dev/pts/N` への後付け stdin injection。
- raw PTY bytes、OSC、CSI、任意 control sequence の agent からの送信。
- target frame の full transcript を MCP read で読むこと。
- prompt instruction だけで human approval や daemon gate を代替すること。

## Wire Model

`FrameMessage` は durable inbox object であり、`id`、`sessionId`、`sourceFrameId`、`targetFrameId`、`topic`、`body`、`priority`、`createdAt`、`readByFrameIds`、`resolution`、`replyIds` を持つ。

`FrameReply` は `messageId` または将来の `deliveryId` に紐付く response object であり、`body`、`finalAnswer`、`answerSource`、`confidence`、`resolution` を持つ。

`FrameMessagingSummary` は View に載せる軽量 payload であり、`unreadCount`、`latestMessagePreview`、`latestReplyPreview`、`pendingDeliveryCount`、`lastDeliveryStatus` を持つ。message 一覧と本文は Web API で取得する。

## Acceptance

{% acceptance id="AC-001" %}
同一 session 内の source frame から target frame に `send_message` したとき、target inbox に message が保存され、`read` で確認できる。
{% /acceptance %}

{% acceptance id="AC-002" %}
self-target と session 外 target は reject され、audit に decision と reason が残る。
{% /acceptance %}

{% acceptance id="AC-003" %}
source identity 偽装 field は schema で受け付けず、broker は token/binding 由来の source だけを使う。
{% /acceptance %}

{% acceptance id="AC-004" %}
`reply` により message に紐付く finalAnswer と resolution が作られ、source frame が `read` で取得できる。
{% /acceptance %}

{% acceptance id="AC-005" %}
daemon restart 後も `messages.jsonl` から message/reply/read state が復元される。
{% /acceptance %}

{% acceptance id="AC-006" %}
Web UI の session view で `TERMINAL` と既存 `LogTabs` を壊さず、`MESSAGES` surface から inbox/reply status を確認できる。
{% /acceptance %}

{% acceptance id="AC-007" %}
Phase 1 build では prompt delivery tool が公開されず、Codex `turn/start` と Claude PTY submit は実行されない。
{% /acceptance %}

{% acceptance id="AC-008" %}
audit と read API は raw transcript を含まず、audit 本文保存は明示設定なしで無効である。
{% /acceptance %}
