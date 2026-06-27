# ADR 0079 — セッションカード Title slot に「AI title → user-prompt summary → placeholder」のフォールバックチェーンを導入する

Status: Accepted

Supersedes: [ADR-0076](0076-session-card-title-subtitle-two-slot.md) (Title slot に関する部分のみ)
Related code: `src/client/driver/view_builder.go` (`resolveCardTitleSubtitle`), `src/client/driver/{claude_view.go,codex_view.go,gemini_view.go,generic_view.go,shell_view.go}`, `src/client/lib/claude/transcript/transcript_event.go` (`parseTitleEntry`), `src/client/tui/view.go`, `src/client/web/src/components/SessionList.tsx`

## Context

ADR-0076 で Title slot は「AI 由来の `Card.Title` のみ、空なら `TITLE_PLACEHOLDER = "New Session"`」と決めた。Subtitle に user-prompt summary を入れる構造はうまく機能したが、Title slot 周りで 2 つの運用問題が発覚した:

1. **Claude Code が transcript の title 行を `custom-title` → `ai-title` (`aiTitle` field) にリネーム** していた。agent-reactor の parser は legacy `customTitle` しか拾わないため、Claude セッションの `cs.Title` は永続的に空。結果、全 Claude セッションのカードが永遠に "New Session" のまま並び、識別不能になった (タイトルとしての機能を果たしていない状態)。
2. **AI title が出るのは LLM が生成してから** (新規セッションでは数 turn 後、ものによっては全く出ない)。出るまでの間 Title slot が "New Session" で埋まり続けるのは ADR-0076 の "Title slot は常に何かを描画する" 原則に矛盾しないが、user-prompt summary という「より具体的な人間が読める文字列」が既に Subtitle slot に出ているのに Title slot だけ placeholder に張り付くのは情報損失。

## Decision

### 1. transcript parser を `ai-title` 対応に拡張

`parseCustomTitleEntry` を `parseTitleEntry` にリネームし、`"type": "custom-title"` と `"type": "ai-title"` の両方を受け付け、`customTitle` または `aiTitle` field の non-empty 値を `KindCustomTitle` Entry に変換する。Kind enum 自体は変更せず、Claude Code の wire shape 変更を parser 側で吸収する。

### 2. Title slot に 3 段フォールバックチェーンを実装

`src/client/driver/view_builder.go` に `resolveCardTitleSubtitle(aiTitle, summary, lastPrompt string) (string, string)` を新設:

- **Title** = `firstNonEmpty(aiTitle, summary)` — AI 生成 title → user-prompt summary → "" (Web client が `TITLE_PLACEHOLDER` で埋める)
- **Subtitle** = `firstNonEmpty(summary, lastPrompt)` — user-prompt summary → 直近 raw prompt → ""

`LastPrompt` は Title 候補ではない (要約済みではない raw 文字列なので)。チェーンは driver 共通なので 5 つの view.go (claude/codex/gemini/generic/shell) すべてが同じヘルパを通すように統一。

### 3. Subtitle dedup は data 層ではなく UI 層で行う

`resolveCardTitleSubtitle` 自体は dedup しない。Title と Subtitle が同じ文字列になる可能性を data 層で許容する理由:

- `src/client/state/reduce_peer.go:254` の peer-summary fallback は `drv.View(...).Card.Subtitle` を読む
- `src/client/tools/builtin.go:176` の send-to-session palette は同じく `Card.Subtitle` を palette label に組み込む

これらの非描画コンシューマは "そのセッションを人間が識別するための文字列" を 1 つ欲しいだけで、Title slot と被るかどうかは関心ない。data 層で Subtitle を空にすると、Summary 由来の情報がまるごと消えてこれらの consumer が機能不全に陥る (本 PR の初期実装で実際に regressed)。代わりに:

- `src/client/web/src/components/SessionList.tsx` の `subtitleText(card)` で `sub === card.title?.trim()` なら "" を返す
- `src/client/tui/view.go` の `sessionCardLines` で `subtitle != title` を subtitle 行描画の前提条件にする

両 UI 層で「同じ文字列を 2 行に並べない」原則は維持される。

### 4. ADR-0076 の "Title slot は AI 由来のみ" 条項は撤回

ADR-0076 §Decision 1 の「Title slot は常に何かを描画する。`TITLE_PLACEHOLDER` で埋める」は維持。撤回するのは ADR-0076 §Alternatives で却下した「Subtitle が空のときに Title slot に Subtitle を上げる」案。本 ADR で *user-prompt summary* に限ってこの promotion を採用する (理由: 要約済みなので Title slot に置いても文長制約を破らない / Subtitle 側ポリシー (user-prompt-only) を継承するので意味論も一貫)。

`LastPrompt` の promotion は依然として却下する (raw / 長文 / 改行混じり / Title としての semantics に合わない)。

## Consequences

- Claude Code 2026-Q2 以降の `ai-title` レコードを正しく拾えるようになり、Claude セッションのカードが識別可能になる
- AI title 未到着でも Summary が出た瞬間に Title slot が人間が読める文字列に変わる
- Title slot のフォールバックチェーンが全 driver で統一されるため、6 つめの driver を追加するときの仕様も明確 (`resolveCardTitleSubtitle` を呼ぶだけ)
- `Card.Subtitle` を読む既存 consumer (peer fallback, palette label) は ADR-0076 時点の挙動のまま動き続ける
- UI 層 (Web SessionList) は title/subtitle 完全一致を hide する dedup を持つ。これが本 ADR の追加コスト

## Alternatives Considered

### Subtitle dedup を data 層で行う (`Card.Subtitle = ""` を返す)

却下: 上記 §Decision 3 の理由。`reduce_peer.go` / `tools/builtin.go` が `Card.Subtitle` を "human label source" として読んでいるため、data 層で空にすると下流が壊れる。code-review で実際に CONFIRMED された regression であり、UI 層 dedup に移すことで両立できる。

### `LastPrompt` も Title 候補に含める (4 段チェーン)

却下: ユーザの明示要求が「(1) AI title (2) user-prompt の **要約** (3) New Session」だったため、要約されていない raw prompt を Title slot に上げるのは仕様外。LastPrompt は Subtitle slot に残り、UI 側で見える経路は確保される。

### `Kind` enum を `KindCustomTitle` → `KindSessionTitle` などにリネーム

却下: 内部 enum 名と transcript wire 上の `type` 文字列は別レイヤ。downstream code (transcript_render / tracker / insight) は Kind だけ見ていて wire type を覚えていないので、リネームしてもバグ防止効果がない一方で diff churn が大きい。Provenance を残したい場合は別 ADR で `Source` field を追加する形が筋がいい。
