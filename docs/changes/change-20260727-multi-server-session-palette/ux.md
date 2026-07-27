---
change: change-20260727-multi-server-session-palette
role: ux
technology_candidates:
- id: candidate-declarative-typed-parameters
  candidate: command-owned declarative typed parameter schema
  source: explicit_user_input
  proposed_for: [任意個parameterの描画, model/effort固有分岐の除去, 安全なinvocation materialization]
  required_capabilities: [stable parameter idと順序, default/required/validation, typed valueの検証とmapping]
  ux_constraints_relevant_to_evaluation: [一括form, default・明示値・未指定の区別, stale schemaからの回復]
  design_questions: [initial renderer type range, argument materialization owner]
  disqualifiers: [client shell interpolation, alias parsing, 不定順描画]
- id: candidate-fleet-config-aggregation
  candidate: server-qualified fleet config snapshot with per-server status
  source: repository_context
  proposed_for: [fleet catalog, partial failure isolation, server-qualified routing]
  required_capabilities: [stable server identity, display name, server別status]
  ux_constraints_relevant_to_evaluation: [一部障害で健康な範囲を止めない, 選択から作成後まで同じidentityを示す]
  design_questions: [freshness, aggregation owner, stale selection invalidation]
  disqualifiers: [単一server snapshotの流用, server identity欠落]
design_handoffs:
- id: handoff-parameter-contract
  required_outcome: 任意個parameterを順序、既定値、必須性、検証規則とともに宣言し、表示と実行で同じ意味を保つ。
  design_obligations: [initial renderer型範囲, ordering/grouping, wire semantics, materialization権威, schema drift処理]
  verification_obligations: [一括form, invalid拒否と回復, default/明示値/未指定の一致, shell interpolationとalias parsingの排除]
  provenance: Exploreのdesign-owned判断をproduct choiceへ昇格させずhandoffとして保持する。
---

<!-- lifecycle is owned by change.md -->

# Multi-server session palette UX

## Goal

New Session の最初の候補面から全接続 server の project を横断検索し、stable server identity、project-scoped command、任意個 parameter を確認して、表示どおりの宛先へ session を作成する。既存 palette の統合検索、disabled reason、明示的 Confirm、keyboard/pointer/支援技術の操作規律と opaque command 実行互換を継承する。

## User Goals

- server を先に切り替えず、全接続 server の同名 project を区別して選ぶ。
- 宛先で有効な command と parameter を設定し、default・明示値・未指定を確認して起動する。
- session/project 一覧の走査中と選択後に所属 server を確認する。
- 一部または下流取得だけが失敗しても、健康な範囲と有効な入力を保って回復する。

## Primary Flows and Acceptance Scenarios

### F-001 Fleet project discovery

Entry: New Session を開くと全接続 server の project と所属 server が同じ候補面に現れる。project 名または server 表示で絞り込み、解除すると元の fleet catalog に戻る。選択後と戻る操作でも stable server identity と横断候補を維持する。

- UAC-001: 同名 project を stable server identity で区別して選べる。検索対象が project 名だけ、server gate、hover-only label、解除後に全候補へ戻らない実装は fail。`vs_legacy: must-fail`
- UAC-002: command 選択中も宛先が見え、戻ると server-local に狭まらない。server identity を捨てる実装は fail。`vs_legacy: must-fail`

### F-002 Contextual command, parameter form, and creation

Entry: 選択した server + project に有効な command だけが現れる。全 parameter は一つの form に明示順序で現れ、未編集 default は実際の適用値、明示選択値、optional 未指定を区別する。

- UAC-003: 別 project 専用 command は選択候補にならず、対象 project に適用される typed profile だけを発見・選択できる。server 共通一覧の流用や旧 opaque command への fallback は fail。`vs_legacy: must-fail`
- UAC-004: 任意個 parameter を一括 form で任意順に編集できる。default の省略、未指定との混同、alias 直積、不定順は fail。`vs_legacy: must-fail`
- UAC-005: required/invalid 値では作成せず、理由と修正対象へ各 modality から到達でき、有効な context と値を保つ。汎用エラーや全選択消失は fail。`vs_legacy: must-fail`
- UAC-006: schema drift 時は残存・適合値だけを保持し、削除値を破棄、新規・不適合値を要修正として示す。再取得失敗では理由、retry、context、入力、focus を保ち、成功後 form へ戻る。全値の無言初期化や古い値の実行は fail。`vs_legacy: must-fail`
- UAC-007: 確認面の stable server identity、project、command、defaultを含む適用値・明示値・未指定と、作成後の実宛先が一致する。zero-parameter profile は空 form を完了状態として作成できる。別 server routing、行き止まり、架空入力、旧 command fallback は fail。`vs_legacy: must-fail`

Exit: 明示的 Confirm 後、確認した stable server identity の server に一つの session が作成され、同じ所属 server を active context で確認できる。

### F-003 Server-qualified lists

- UAC-008: project 行は既存 grouping を保ちつつ server label を常時示す。衝突時だけ、hover-only、server section への regroup は fail。`vs_legacy: must-fail`
- UAC-009: session は走査中と active context で所属 server を常時示す。session ID 単独 identity は fail。`vs_legacy: must-fail`
- UAC-010: display name 重複時は stable server identity の補助表示で区別できる。同一文字列だけの表示は fail。`vs_legacy: must-fail`

### F-004 Partial failure and recovery

- UAC-011: project catalog、command catalog、parameter schema の一部取得失敗では、失敗対象・理由・retry を示し、健康な範囲と選択済み宛先を保つ。fleet 全体停止、無言欠落、宛先消失は fail。`vs_legacy: must-fail`
- UAC-012: catalog 再取得時、編集中の宛先が有効なら選択・入力・focus を保持する。無効なら理由と再選択を示す。無言の初期化または無効 context の黙認は fail。`vs_legacy: must-fail`

Exit: retry 成功後は同じ flow で候補または form へ戻り、別 scope への移動や palette の閉じ直しなしに続行できる。

## Experience Closure

4 flow、12 scenarios が entry、progress、success/exit、failure、recovery、keyboard/pointer/支援技術、identity collision、single/multi-server、partial failure、parameter validation/schema drift を覆う。観察を変える product decision は解決済みで、open product choice はない。

## Design Handoff

`candidate-declarative-typed-parameters`、`candidate-fleet-config-aggregation`、`handoff-parameter-contract` は非規範の design 入力である。initial renderer type range と argument materialization owner は design が決める実装境界であり、UX product choice や acceptance blocker ではない。

## Open Questions

なし。
