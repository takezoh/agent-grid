---
id: adr-20260726-windows-shell-content-driven-session-list
kind: adr
title: Windows Shell の展開幅とセッション状態表示を content-driven 契約にする
status: accepted
created: '2026-07-26'
decision_makers:
- agent-grid-maintainers
tags:
- native-clients
- windows-shell
- vibe-island
owners:
- agent-grid-maintainers
relations: []
source_paths:
- clients/windows-shell/AgentGrid.Shell.Core/SupervisionState/IslandMetrics.cs
- clients/windows-shell/AgentGrid.Shell.WinUI/Panel/PanelWindow.xaml
- clients/ui/src/components/StatusIcon.tsx
summary: 展開 Shell はコンテンツ計測で幅を決め、セッション行は viewport 全幅を使い、状態アイコンは Workspace StatusIcon
  と同じ状態別意味論を持つ
consequences:
  positive:
  - Shell の外枠と一覧内容が同じ measurement contract に従い、空白だけを増やす幅修正を防げる
  negative:
  - 複数セッションは縦方向の full-width row となり、件数が多い場合は縦スクロールが必要になる
  neutral:
  - React/SVG/CSS と WinUI/Composition で同じ意味論と数値契約を個別実装する
confirmation: IslandMetricsTests、PanelPresentationTests、PanelWindowUiE2ETests.Session_rows_fill_the_shell_content_width_when_present
  を実行し、content measurement、状態対応、full-width row と UIA status icon を検証する
updated: '2026-07-26'
---

# Windows Shell の展開幅とセッション状態表示を content-driven 契約にする

## Context

Vibe Island の展開サイズは `RootHost.DesiredSize` を
`IslandMetrics.ExpandedFor` で上下限へ clamp する content-hugging 実装だが、
永続的な設計判断として記録されていなかった。セッション一覧だけは横向き
`StackPanel` の intrinsic-width chip であり、一覧の内容幅と Shell の展開幅が
視覚的に分離していた。

状態表示も Workspace UI の `StatusIcon` が持つ状態別意味論ではなく、Windows
Shell が独自の図形を持っていた。この重複により、running 用リングを全状態へ
適用しても「統一」と誤認できる状態だった。

## Decision

1. 展開 Shell の幅と高さは、表示中コンテンツの desired size を測定し、
   `IslandMetrics` の最小・最大境界へ clamp して決定する。セッション数を理由に
   固定の最小幅へ拡大しない。
2. セッション一覧の各行は content viewport の全幅を使う。ラベルの intrinsic
   width は Shell の content measurement に参加し、最大幅に達した場合だけ省略する。
3. Windows Shell の状態アイコンは Workspace `StatusIcon` の意味論とジオメトリを
   移植する。`Running → running`、`Waiting → waiting`、`Failed → stopped`、
   `Done → idle` とし、色は各 `SessionPhase` の semantic accent を使う。
4. 状態名は可視テキストにせず、AutomationProperties.Name で提供する。

## Consequences

{% consequence kind="positive" %} Shell の外枠と一覧内容が同じ measurement
contract に従い、空白だけを増やす幅修正を防げる。{% /consequence %}

{% consequence kind="negative" %} 複数セッションは横一列の chip ではなく縦方向の
full-width row となり、セッション数が多い場合は縦スクロールが必要になる。
{% /consequence %}

{% consequence kind="neutral" %} Workspace と Windows Shell は描画技術を共有せず、
React/SVG/CSS と WinUI/Composition で同じ意味論と数値契約を個別実装する。
{% /consequence %}

## Alternatives

- **展開最小幅だけを増やす**: 却下。外枠だけが広がり intrinsic-width chip は短いまま
  なので、content-hugging 契約を満たさない。
- **横並び chip を維持して各 chip を均等割りする**: 却下。項目数で幅が変動し、
  長いラベルが Shell の desired width に参加できない。
- **WinUI `ProgressRing` を使う**: 却下。Workspace の waiting、idle、stopped の
  状態別ジオメトリと一致しない。
