---
id: spec-20260715-terminal-scrollback-reflow
kind: spec
title: Terminal scrollback width-independent reattach contract
status: implemented
created: '2026-07-15'
tags:
- terminal
- scrollback
- reflow
owners: []
functional_requirements:
- id: FR-001
  statement: システムは、保持中の primary terminal history について、hard break と soft continuation、head-truncated、exact-column
    pending wrap、cell/style/cursor の意味を、すべての row mutation を通して保存しなければならない。
  priority: must
  rationale: 旧物理行の幅から意味を推測すると、異なる幅への reattach で hard newline を捏造するため。
- id: FR-002
  statement: viewer attach が geometry-pending の間、システムは、有効な cols/rows が確定するまで subscribe
    request、snapshot seed、subscriber publish を保留しなければならない。
  priority: must
- id: FR-003
  statement: 有効な geometry を持つ viewer attach を受けたとき、システムは、PTY SetSize 成功後に VT resize、opaque
    ANSI snapshot、seed enqueue、subscriber publish を一つの actor turn で commit しなければならない。
  priority: must
- id: FR-004
  statement: 異なる幅へ attach または再 resize したとき、システムは、terminal autowrap の soft continuation
    だけを現在幅で再折返しし、application が出力した CR/LF などの hard break を保持しなければならない。
  priority: must
- id: FR-005
  statement: もし attach の PTY SetSize が失敗した場合、システムは、session size、VT state、subscriber
    集合を変更してはならない。
  priority: must
- id: FR-006
  statement: もし commit 後の VT invariant または opaque snapshot 生成が失敗した場合、システムは、旧 newline
    serializer へ fallback せず session を unusable として fail fast しなければならない。
  priority: must
- id: FR-007
  statement: attach 中に transport generation が失効したとき、システムは、旧 generation の seed を破棄し、desired
    attach を最新 geometry で再試行しなければならない。
  priority: must
- id: FR-008
  statement: scrollback cap が logical line の先頭を切り落としたとき、システムは、残存 fragment を head-truncated
    として列 0 から描画し、欠落 prefix または hard break を生成してはならない。
  priority: must
- id: FR-009
  statement: システムは、reattach snapshot とその再 resize を通して SGR、trailing erased cells、wide/combining
    grapheme、cursor、EL の observable terminal state を保持しなければならない。
  priority: must
- id: FR-010
  statement: システムは、alternate-screen の内容を primary scrollback に追加せず、active screen の現在の可視状態だけを
    attach seed に含めなければならない。
  priority: must
- id: FR-011
  statement: internal terminal observer が current subscription を要求したとき、システムは、session
    geometry を変更せず現在 geometry の snapshot と live stream を返さなければならない。
  priority: must
- id: FR-012
  statement: もし viewer attach request の geometry が欠落または不正な場合、システムは、購読を成立させず既存 WebSocket
    error posture に従って警告または破棄しなければならない。
  priority: must
non_functional_requirements:
- id: NFR-001
  type: reliability
  criteria: attach の seed/live 境界で重複、欠落、順序逆転が 0 件であること。
  measurement: actor ordering contract の反復テストと Go gateway scenario。
- id: NFR-002
  type: maintainability
  criteria: hard/soft provenance と ANSI reattach rendering の terminal semantic decision
    が既存 x/vt fork 一箇所だけに存在すること。
  measurement: narrow Emulator contract と ownership invariant test。agent-grid 側 SemanticLine/CellRun
    renderer は 0 件。
- id: NFR-003
  type: compatibility
  criteria: outbound terminal output frame は xterm-compatible ANSI bytes のまま、xterm
    public write/resize API だけで描画できること。
  measurement: Playwright で rows、isWrapped、cursor、再 resize を観測する。
- id: NFR-004
  type: performance
  criteria: reattach snapshot の時間・空間計算量が bounded history と active screen の保持 cell
    数に対して O(n) であること。
  measurement: unbounded raw transcript を保持または replay しない設計レビューと cap contract test。
- id: NFR-005
  type: reliability
  criteria: 変更した外部 fork 契約を fake、invariant-naming contract、FakeVsReal の三点で常時検証すること。
  measurement: go test に含まれる T0/T2 gate。
acceptance:
- id: AC-001
  given: cols=5 で abcdefghij が terminal autowrap により二つの物理 row として保持されている
  when: cols=10 の viewer が attach する
  then: 一つの logical line として表示され、row continuation は isWrapped=true の意味を保持して再 resize
    できる
  requirement_refs:
  - FR-001
  - FR-004
  - FR-009
- id: AC-002
  given: application が abcde、LF、fghij を出力している
  when: cols=10 の viewer が attach する
  then: 二つの hard line のまま表示され、LF 境界は reflow で連結されない
  requirement_refs:
  - FR-004
- id: AC-003
  given: desired session は存在するが初回 fit geometry が未確定である
  when: WebSocket が open または reconnect する
  then: controller は geometry-bearing subscribe を送らず desired attach を保持する
  requirement_refs:
  - FR-002
  - FR-007
- id: AC-004
  given: session size が 80x24 で subscriber A が存在する
  when: 120x40 の AttachAtGeometry の PTY SetSize が失敗する
  then: size、VT state、subscriber A は変わらず新 subscriber は作られない
  requirement_refs:
  - FR-003
  - FR-005
- id: AC-005
  given: PTY SetSize が成功し VT semantic invariant が破損している
  when: AttachAtGeometry が snapshot を生成する
  then: session は unusable となり観測可能な内部エラーで全 subscriber を sever し旧 serializer を使わない
  requirement_refs:
  - FR-006
- id: AC-006
  given: current viewer attach と pty tap が同じ session を観測する
  when: pty tap が SubscribeCurrent を開始する
  then: global geometry と PTY winsize は変更されない
  requirement_refs:
  - FR-011
- id: AC-007
  given: cap eviction が soft-wrapped logical line の prefix を除去している
  when: 異なる幅へ attach する
  then: retained head fragment が列 0 から表示され、存在しない prefix と hard break は生成されない
  requirement_refs:
  - FR-008
- id: AC-008
  given: soft/hard、pending wrap、erased cells、SGR、wide/combining、cursor/EL を含む fixture
    がある
  when: snapshot を xterm に書き込み、幅を縮小して再拡大する
  then: contract matrix の rows、isWrapped、cursor、style と erase observable がすべて一致する
  requirement_refs:
  - FR-001
  - FR-004
  - FR-009
- id: AC-009
  given: attach request に geometry がないか bounds 外である
  when: gateway が request を処理する
  then: actor state と subscription は不変で、既存 error posture に従う
  requirement_refs:
  - FR-012
relations:
- {type: implementedBy, target: plan-20260715-terminal-scrollback-reflow}
- {type: referencedBy, target: adr-20260715-terminal-semantic-history-reattach}
- {type: referencedBy, target: adr-20260715-geometry-bearing-terminal-attach}
source_paths:
- src/platform/termvt/
- src/client/runtime/pty_backend.go
- src/client/runtime/pty_tap.go
- src/server/web/gateway.go
- src/client/web/src/socket/terminalSubscription.ts
- src/client/web/src/components/TerminalPane.tsx
methodology: sdd
summary: Define width-independent terminal history semantics and geometry-aware reattachment
  so past output reflows correctly across devices.
updated: '2026-07-15'
---

## Goal

狭い端末で蓄積された過去ログを、late join、reconnect、session switch でより広い端末から開いたとき、application の hard break を守りながら terminal autowrap だけを現在幅で再折返しする。表示上の近似ではなく、再 resize 可能な xterm の wrap/cursor state まで契約する。

## Requirements

{% req id="FR-001" %}VT semantic owner は row mutation の閉じた保存則を持つ。対象は write/overwrite、erase、insert/delete char、insert/delete line、LF/index、region/full scroll、resize、cap eviction、primary/alternate transition であり、新しい row mutation が未分類なら invariant contract が失敗する。{% /req %}

{% req id="FR-002" %}geometry-pending は desired controller が有効 geometry を待つ状態であり、wire subscription はまだ存在しない。{% /req %}

{% req id="FR-003" %}AttachAtGeometry は geometry を session-wide size write として actor mailbox に直列化し、prepare/commit 契約を満たす。{% /req %}

{% req id="FR-004" %}reflow 対象は terminal autowrap の soft continuation だけである。CR/LF、NEL、明示 cursor movement により確定した hard boundary は連結しない。{% /req %}

{% req id="FR-005" %}fallible PTY SetSize は commit より前に行い、失敗時は既存状態を公開したまま要求だけを失敗させる。{% /req %}

{% req id="FR-006" %}PTY commit 後に起きるべきでない VT invariant/snapshot failure は内部契約違反である。部分 rollback や誤表示 fallback をせず、session unusable、subscriber sever、structured log/metric により顕在化する。{% /req %}

{% req id="FR-007" %}controller は connection/ownership generation を採否に使い、失効 seed を write せず current desired を reconcile する。{% /req %}

{% req id="FR-008" %}cap は bounded in-memory history を維持する。logical line 途中の eviction は retained head fragment の provenance として保持する。{% /req %}

{% req id="FR-009" %}fork が opaque ANSI を生成し、agent-grid は cell/style/cursor を再解釈しない。fidelity は contract matrix で判定する。{% /req %}

{% req id="FR-010" %}既存 alternate-screen exclusion を継承する。{% /req %}

{% req id="FR-011" %}termvt API は viewer の AttachAtGeometry と internal observer の SubscribeCurrent を分離する。{% /req %}

{% req id="FR-012" %}geometry-bearing inbound subscribe は cols/rows を必須とし、欠落・非整数・0・上限超過を購読前に拒否する。{% /req %}

{% req id="NFR-001" %}seed は commit の linearization point に属する VT state を表し、以後の live event は actor mailbox 順に一度だけ続く。{% /req %}

{% req id="NFR-002" %}semantic model と xterm-compatible ANSI renderer は既存 github.com/takezoh/x/vt fork が所有する。{% /req %}

{% req id="NFR-003" %}inbound subscribe request は geometry-bearing に変わるが outbound output frame の ANSI byte contract は維持する。{% /req %}

{% req id="NFR-004" %}server-side bounded in-memory VT history と raw PTY replay rejection を継承する。{% /req %}

{% req id="NFR-005" %}fake、real fork/PTY、browser xterm の各境界を独立 gate で検証する。{% /req %}

## Data Model

| Owner | Concept | Contract |
|---|---|---|
| x/vt fork | Semantic row/history | 各 row boundary の hard/soft provenance、head-truncated、exact-column pending wrap、cell attributes、cursor anchor を保持する。全 mutation は provenance 保存則に分類される。 |
| x/vt fork | Opaque `ReattachSnapshot` | target geometry に対する xterm-compatible ANSI bytes。hard/soft/cell/cursor の解釈と rendering は fork 内で完結する。 |
| termvt | `AttachGeometry` | bounds 検証済み cols/rows。`AttachAtGeometry` だけが viewer attach size write に使用する。 |
| termvt | subscription mode | `AttachAtGeometry` は size-changing viewer、`SubscribeCurrent` は no-size-change internal observer。 |
| browser controller | desired attach | session id、最新 fitted geometry、ownership epoch、connection epoch。唯一の wire writer が保持する。 |

agent-grid に `SemanticLine`、`CellRun`、wrap inference、ANSI semantic renderer を置かない。termvt の Emulator seam は opaque bytes と typed error/capability の受渡しだけを行う。

## Failure Modes

| Class | Related FR | Detection | Recovery | Observable postcondition |
|---|---|---|---|---|
| invalid geometry | FR-012 | browser/gateway bounds validation | degrade | subscription 不成立、session state 不変、既存 warn/drop posture |
| PTY SetSize failure | FR-005 | prepare phase syscall error | retry | old size/VT/subscribers 不変、desired は retry policy に残る |
| VT invariant / snapshot failure | FR-006 | fork smart constructor / snapshot error | fail_fast | session unusable、subscriber sever、structured internal error。旧 serializer fallback なし |
| seed enqueue/backpressure | FR-006 | bounded queue admission failure | fail_fast | subscriber publish 前なら不成立、publish 後の impossible failure は session unusable |
| disconnect/cancel during attach | FR-007 | connection/ownership generation mismatch | retry | stale seed は transport へ書かず current desired を再 reconcile |
| fork contract drift | FR-001, FR-009 | compile、invariant contract、FakeVsReal | fail_fast | release gate を停止し capability downgrade しない |

## Migration and Legacy Contract

新 semantic-history ADR は `adr-20260624-0066-terminal-scrollback-via-vt-buffer` を supersede する。server-side bounded in-memory VT history、raw PTY replay rejection、alternate-screen exclusion は継承する。newline で区切った physical-row serializer、browser unchanged、resize crossing は構造上存在しないという判断を置換する。

旧 `docs/specs/2026-06-26-terminal-scrollback/spec.md` の FR-004（物理 row 間 newline boundary）と FR-006（wire shape と browser unchanged）は本 spec の FR-001、FR-003、FR-004、FR-009、FR-012 に置換される。移行完了は全 Subscribe consumer の mode 移行、`SerializeScrollback` と旧二段 seed path の削除、fork release pin 更新、全 contract gate 通過で判定する。旧 serializer は fallback として残さない。

## Worked Examples

### soft-to-wide

`cols=5` で `abcdefghij` が `abcde` + soft continuation `fghij` になり、`cols=10` へ attach すると `abcdefghij` 一行になる。その後 `cols=5` に戻すと二行目は xterm の soft wrapped row になる。`abcde\nfghij` を書く実装は不合格。

### hard-break

`abcde\r\nfghij` は `cols=10` でも二行のままである。旧 row が満杯だったことを理由に CR/LF を soft と推測しない。

### pending-wrap-style-cursor

最終列ちょうどの wide/combining glyph、SGR span、EL で消去した trailing cells、cursor を含む snapshot は、attach と再 resize の両方で contract matrix と同じ rows/isWrapped/style/cursor を示す。

## Non-Goals

- daemon restart を越える scrollback 永続化、raw transcript recording/replayは行わない。
- multi-viewer の last-writer-wins arbitration policy は変更しない。AttachAtGeometry を通常 Resize と同格の size write に加えるだけである。
- xterm private buffer API、structured cell wire payload、agent-grid 独自 VT parser は導入しない。
- scrollback cap の operator-facing policy、alternate-screen policy、mobile scroll UX は変更しない。

## Acceptance Trace

{% acceptance id="AC-001" %}Soft wrap is reflowed at wider attach and remains re-resizable.{% /acceptance %}
{% acceptance id="AC-002" %}Application hard break remains hard.{% /acceptance %}
{% acceptance id="AC-003" %}Desired attach waits for fitted geometry.{% /acceptance %}
{% acceptance id="AC-004" %}PTY failure leaves the session unchanged.{% /acceptance %}
{% acceptance id="AC-005" %}Internal semantic failure makes the session unusable without fallback.{% /acceptance %}
{% acceptance id="AC-006" %}Internal tap observes without changing size.{% /acceptance %}
{% acceptance id="AC-007" %}A capped head fragment is not fabricated into a complete line.{% /acceptance %}
{% acceptance id="AC-008" %}ANSI fidelity matrix survives attach and re-resize.{% /acceptance %}
{% acceptance id="AC-009" %}Invalid geometry cannot establish a subscription.{% /acceptance %}


{% transition from="draft" to="approved" date="2026-07-15" %}
User approved implementation
{% /transition %}


{% transition from="approved" to="implemented" date="2026-07-15" %}
Semantic history, geometry-bearing attach, consumer migration, and regression gates implemented
{% /transition %}
