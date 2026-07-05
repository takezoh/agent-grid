---
id: adr-20260705-metadata-source-priority
kind: adr
title: Metadata source priority for Claude and Codex session state
status: accepted
created: '2026-07-05'
decision_makers:
- unknown
tags: []
owners: []
relations: []
source_paths: []
summary: Defines authoritative and fallback sources for model and effort metadata
  across Claude and Codex sessions.
updated: '2026-07-05'
---

# Metadata source priority for Claude and Codex session state

Status: Accepted

## Context

`model` と `effort` / `reasoning` metadata は、session card、session header、cold start
復元、再 launch の attach command にまで伝播する。したがって、どの source を正本として
扱うかが曖昧だと、次の破綻が起きる。

1. 実行中は正しく見えても、daemon restart や transcript 再解析で古い metadata が復活する。
2. launch command parse や transcript parse が、より新しい live event を上書きする。
3. `null` による clear を表現できず、既定値へ戻した設定が stale metadata として残る。
4. Claude/Codex で protocol shape が異なるのに、同じ source policy で扱って drift を招く。

今回の metadata は UI 表示専用ではなく、cold start 復元と attach command の入力にも
使われるため、「最後に見えた値」ではなく「どの source を authoritative source とするか」を
driver ごとに固定する必要がある。

## Decision

### 1. Claude は hook を authoritative source とする

- Claude driver における live metadata の正本は、**受理済み hook** とする。
- `SessionStart` hook は初期値 source である。
- ただし `SessionStart` のみを特別扱いせず、順序チェックを通過した以後の hook payload に
  `model` または `effort` が含まれる場合は、その都度 state を更新してよい。
- out-of-order として棄却した hook は metadata を更新してはならない。
- transcript は **hook 未着時の cold/warm restore fallback** に限定し、hook で既に確定した
  metadata を transcript が上書きしてはならない。

### 2. Codex は app-server event を authoritative source とする

- Codex driver における live metadata の正本は、**`thread/settings/updated`** とする。
- Codex が表現するのは概念としては **reasoning effort** だが、wire field 名は source によって
  `effort` または `reasoning_effort` で揺れる。driver 内では同一の reasoning metadata として扱い、
  decode 層だけが field 名差分を吸収する。
- `thread/settings/updated` を受理した後は、その thread の metadata は authoritative 済みとみなし、
  transcript parse や launch command parse がそれを上書きしてはならない。
- `thread/settings/updated` が `null` を送る場合、これは「更新なし」ではなく **explicit clear** として扱う。

### 3. transcript は両 driver とも fallback source に限定する

- transcript は **authoritative source が未取得のときだけ** metadata を供給できる。
- transcript 由来 metadata は、live event/hook が一度でもその field を authoritative に確定した後は、
  その field を上書きできない。
- ただし transcript parser 自体は `null` / clear を表現できなければならない。fallback source であっても、
  authoritative source が未取得の古い session を復元する際に stale metadata を残さないためである。

### 4. cold start 復元の優先順位を固定する

driver state の `model` / `effort` / `reasoning` を復元するときの source priority は以下とする。

#### Claude

1. persisted driver-specific metadata
2. 受理済み hook
3. transcript fallback
4. launch command parse fallback

#### Codex

1. persisted driver-specific metadata
2. `thread/settings/updated`
3. transcript fallback
4. launch command parse fallback

launch command parse は、あくまで「まだ session metadata が何も無い create/cold-start の初期 seed」
に限定する。より新しい authoritative source を上書きしてはならない。

### 5. clear を表現できる state を保持する

- driver は少なくとも field ごとに `unset / set(value) / cleared` を識別できなければならない。
- 実装上は `value + set-bit`、または同等の tri-state で表現する。
- 空文字は「未設定」ではなく「clear 済み value」と衝突し得るため、value 単体での判定は禁止する。

## Consequences

- positive: Claude/Codex とも、実行中 metadata、cold start 復元、attach command 再注入が同じ source policy に従う。
- positive: `thread/settings/updated: null` や hook 由来 clear が stale metadata に負けて復活する事故を防げる。
- positive: transcript parser は残すが、source of truth ではなく recovery backstop として位置づけられる。
- negative: driver 実装は「値」だけでなく「その値が authoritative か」を追跡する必要があり、state と reducer が少し複雑になる。
- negative: Codex の protocol field 名 (`effort`, `reasoning_effort`) と driver 内部表現の差を decode 層で管理する必要がある。

## Alternatives considered

### transcript を常に最新 source とみなす

却下。実際の session 中に受け取った `thread/settings/updated` や hook を、遅延した transcript parse が
巻き戻す。clear が表現できない限り stale metadata resurrect の温床になる。

### launch command parse を復元 source として優先する

却下。launch command は起動時点の seed に過ぎず、セッション中の `/model` / `/reasoning` / settings update
を反映できない。復元 source として authoritative 扱いするのは誤りである。

### Claude/Codex 共通の一律 source policy にする

却下。Claude は hook relay が正本、Codex は app-server event が正本であり、protocol surface が異なる。
共通化すると、どちらか一方の drift をもう片方へ持ち込む。


{% transition from="proposed" to="accepted" date="2026-07-05" %}
Codify authoritative and fallback metadata sources for Claude and Codex session state.
{% /transition %}
