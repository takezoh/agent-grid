---
id: adr-20260706-frame-messaging-structured-response-sources
kind: adr
title: Frame messaging responses use structured driver sources, not VT snapshots
status: accepted
created: '2026-07-06'
decision_makers:
- unknown
tags:
- mcp
- frame-messaging
- driver
owners: []
relations:
- {type: partOf, target: change-20260706-frame-messaging}
source_paths:
- src/client/runtime/subsystem/stream/
- src/client/lib/claude/
- src/platform/termvt/
summary: Codex は app-server notifications、Claude は transcript/hook/driver state を
  response source とし、VT snapshot は protocol source にしない
updated: '2026-07-06'
---

# Frame messaging responses use structured driver sources, not VT snapshots

## Context

{% context %}
prompt delivery を将来実装する場合、delivery accepted/rejected と target agent の実回答 completion は別 lifecycle になる。response を VT snapshot や terminal tail から抽出すると、ANSI、再描画、spinner、partial line、alt-screen、thinking/tool narration の混入により protocol として不安定になる。

Codex は agent-grid の stream backend が `codex app-server` connection と thread binding を保持している。Claude は PTY 管理だが、transcript tracker、hook event、driver state が存在する。
{% /context %}

## Decision

{% decision %}
response collection の正経路に VT snapshot / terminal tail を使わない。

Codex は app-server の structured notification を使う。`turn/start` / `turn/steer` の結果または `turn/started` から `turnID` を確定し、`item/agentMessage/delta` は streaming candidate、`turn/completed` は final status として扱う。`phase=final_answer` があれば `answerSource=codex_final_answer`、なければ最後の agent message を `driver_heuristic` とする。

Claude は transcript / hook event / driver state を使う。high-confidence final answer は `agent_frames.reply` のみとし、transcript fallback は correlated user entry 以降の最後の assistant text に限定して `confidence=heuristic` とする。delivery marker が見つからない場合や state が矛盾する場合は success を推測せず `unknown` とする。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
terminal 表示の偶然に依存せず、driver ごとの authoritative source に基づいて response を保存できる。raw transcript を MCP read に流さない境界も維持できる。
{% /consequence %}

{% consequence kind="negative" %}
driver-specific correlation 実装が必要になる。Claude は Codex より final answer の確度が低く、明示 reply contract に寄せる必要がある。
{% /consequence %}

{% consequence kind="neutral" %}
VT snapshot は UI 表示と診断用には引き続き使えるが、delivery success や final answer の source にはしない。
{% /consequence %}

## Alternatives

- **VT snapshot / terminal tail を parse する** — 却下。表示最適化や terminal state に左右され、protocol source として安定しない。
- **MCP call を target response 完了まで blocking する** — 却下。tool timeout と caller/target turn lifecycle が不安定になる。同期戻り値は accepted/rejected と correlation id までにする。
- **Claude transcript を high-confidence final answer とする** — 却下。thinking、tool narration、subagent output、途中説明が混ざるため、明示 reply と同格には扱わない。


{% transition from="proposed" to="accepted" date="2026-07-06" %}
Decision extracted from promoted inter-session MCP plan
{% /transition %}
