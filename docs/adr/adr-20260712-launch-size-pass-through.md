---
id: adr-20260712-launch-size-pass-through
kind: adr
title: Route spawn size hint through driver pass-through
status: proposed
created: '2026-07-12'
decision_makers:
- Takehito Gondo
tags:
- runtime
- driver
- termvt
- launch
owners: []
relations:
- {type: partOf, target: change-20260712-frame-size-ownership}
- {type: references, target: change-20260712-frame-size-ownership}
source_paths:
- src/client/state/driver_iface.go
- src/client/state/effect.go
- src/client/runtime/interpret_spawn.go
- src/client/drivers
summary: 'size は driver Prepare* を素通しで LaunchOptions/EffSpawnFrame.Options 経由で termvt.Spec
  まで運ぶ。

  Consequences 三極は本文タグに記載 (このリポジトリの docs schema は spec-detail v1 の consequences/confirmation
  frontmatter フィールドを未サポートのため本文のみ)。

  '
---

## Context

{% context %}
web API (`server/web/mux.go`) から termvt.Spec までの spawn 経路は次の 4 hop で構成される: mux.go apiCreateReq → state.LaunchOptions{Cols,Rows uint16} → driver PrepareLaunch/PrepareCreate → EffSpawnFrame.Options → runtime.spawnFrameWindow → PtyBackend.SpawnFrame → termvt.Spec。issue の grep 調査で確認されたとおり、LaunchOptions.Cols/Rows は既に uint16 field として存在し、driver_iface.go にはコメントで「runtime bridges these to termvt.Spec on session launch (β scope)」と書いてあるが、実際には driver 実装 (generic / codex / claude* / shell / gemini) が Options を LaunchPlan/CreateLaunch に verbatim 転写しているかは未検証、そして runtime 側の PtyBackend.SpawnFrame の signature に size を渡していないため 80×24 で spawn する。size は per-pty state (kernel winsize + emulator grid) の SoT であり、driver は size に関心を持たない — worktree / initial_input と同格の launch parameter ではなく、より深い termvt の invariant である。driver を size に関心を持たせると SoT が termvt から driver 側に染み出す。
{% /context %}

## Decision

{% decision %}
size hint の運搬経路は driver を **素通し**する。すなわち driver Prepare* の signature は不変で、driver 実装は現状どおり LaunchOptions を LaunchPlan/CreateLaunch.Options に verbatim 転写する既存契約を利用する。runtime の spawnFrameWindow が EffSpawnFrame.Options.Cols/Rows を読み取り、拡張済み SpawnFrame signature (adr-20260712-spawnframe-inline-size-pair 参照) に cols/rows を渡す。実装フェーズで generic / codex / claude / shell / gemini の Prepare* 実装を grep で verify し、`.Options = req.Options` 相当の pass-through が全て存在することを確認する。写し漏れがあった driver に対しては pass-through を追加する (driver interface には size 責務を追加しない)。
{% /decision %}

## Consequences

{% consequence kind="positive" %}
driver interface に size を露出させないため、size の SoT は「web 境界で入力 → termvt.Spec で消費」の 1 直線に保たれ、driver 側が size を上書き / 参照する構造上の権力集中が発生しない。SoT invariant (design-quality 5 invariant) を守れる。
{% /consequence %}

{% consequence kind="positive" %}
既存の LaunchOptions / EffSpawnFrame.Options / LaunchPlan.Options という transport slot をそのまま再利用するため、新規の transport field / method / event を追加する必要がない。変更行数は SpawnFrame signature 追加と PtyBackend.SpawnFrame 本体の Spec.Cols/Rows 転記に閉じる。
{% /consequence %}

{% consequence kind="negative" %}
approach の中核前提 (driver Prepare* が Options を verbatim 転写する) が実装フェーズで verify されるまで確定しない。写し漏れがある driver が発見された場合、当該 driver 側に 1 行の pass-through 追加が発生する — この検証を怠ると spawn hint が該当 driver 経由のセッションで silent に落ちる (dead hint) 失敗モードが残る。
{% /consequence %}

{% consequence kind="negative" %}
driver に size 責務を持たせないため、将来「特定 driver が size を上書きしたい」ユースケース (例: codex が固定 grid を要求する) が発生した場合、この ADR を supersede する新規 ADR が必要になる。現時点で該当ユースケースは存在しない。
{% /consequence %}

{% consequence kind="neutral" %}
state 層 reducer (reduce_surface.go:134 の pass-through 契約) には手を触れない。size は依然として effect / backend / termvt.Session の pass-through で保持され、reducer は size を保持しない。
{% /consequence %}

## Alternatives

- **B. EffSpawnFrame に SpawnHint 専用 field を新設して runtime が直接読む** — 却下。EffSpawnFrame.Options は既に LaunchOptions を verbatim で運ぶ意味論を持ち、そこに新たな SpawnHint field を並列に追加すると「同じ意味 (spawn 時の size hint) が 2 slot に存在する」drift 源になる。SoT invariant に反する。
- **C. driver の Prepare* に size を明示的に通す (driver 干渉可能にする)** — 却下 (critique verdict=Y for FR-d4 vs DP-d1 asymmetric branch)。size は driver の関心事ではなく termvt の invariant。driver に size 干渉権を与えることは SoT を driver 側に染み出させる構造上の権力集中で、design-quality 5 invariant (SoT / 境界重複禁止) に反する。
