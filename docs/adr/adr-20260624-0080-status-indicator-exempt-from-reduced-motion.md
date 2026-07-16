---
id: adr-20260624-0080-status-indicator-exempt-from-reduced-motion
kind: adr
title: ADR 0080 — StatusIndicator は prefers-reduced-motion guard から除外する (機能 motion
  例外)
status: accepted
created: '2026-06-24'
updated: '2026-07-04'
tags:
- adr
- legacy-import
owners: []
relations:
- {type: references, target: adr-20260624-0032-runstate-spinner-additive}
- {type: references, target: adr-20260624-0064-reduced-motion-single-guard}
source_paths:
- Makefile
- src/client/web/src/
decision_makers:
- unknown
summary: 'ADR-0064 で view.css 末尾の @media (prefers-reduced-motion: reduce) ブロックに、.run-state-spinner
  / .session-status-spinner / その後 ADR-0078 で導入された .status-icon--running / --pending
  / --waiting .status-icon__dot / --idle'
---

<!-- migrated_from: docs/adr/0080-status-indicator-exempt-from-reduced-motion.md -->

# ADR 0080 — StatusIndicator は prefers-reduced-motion guard から除外する (機能 motion 例外)

Status: Accepted

Related: [ADR-0032](adr-20260624-0032-runstate-spinner-additive.md), [ADR-0064](../adr/adr-20260624-0064-reduced-motion-single-guard.md), ADR-0078 (legacy link target: `0078-per-status-status-icon.md`, not present in this repository)
Related requirements: FR-MOTION-001 (改定)

## Context

ADR-0064 で view.css 末尾の `@media (prefers-reduced-motion: reduce)` ブロックに、`.run-state-spinner` / `.session-status-spinner` / その後 ADR-0078 で導入された `.status-icon--running` / `--pending` / `--waiting .status-icon__dot` / `--idle .status-icon__filled` をすべて `animation: none !important` で抑制していた。

しかし OS の Reduce Motion がオンの環境では、これらが全部静止する結果、running / pending / waiting / idle が**ほぼ同一の静的円形グリフ**に collapse する。色 (running=青系 / waiting=橙系 / idle=灰系) で辛うじて区別できるが、

- 色弱ユーザーには区別不能
- `aria-label` は読まれるが**目視で進行状態が読み取れない**
- 「同じグレーの円が並んでいて、どのセッションが動いているのかわからない」と感じる UX

WCAG 2.3.3 (Animation from Interactions) は **interaction-triggered な非必須 animation** を抑制対象とし、Apple HIG / W3C "Designing for Reduced Motion" も **progress indicator / loading indicator は機能要素として例外** と明言している。Spinner の motion は status の意味を伝える functional motion であり、ADR-0064 の guard は過剰だった。

2026-06-26 に "Spinner が動かない" 報告 4 連続発生 (fc2bcc9 / e2b1b4a / df49f38 等で CSS / embed / Makefile を順に疑い修正)。5 回目の DevTools プローブで `matchMedia('(prefers-reduced-motion: reduce)').matches === true` が原因と確定。コード側はすべて正常動作していた。

## Decision

(1) StatusIndicator (spinner / pending / waiting / idle の各 animation) を `@media (prefers-reduced-motion: reduce)` guard から**完全に除外**する。具体的には view.css の reduced-motion ブロックから以下 6 selector を削除する:

- `.run-state-spinner`
- `.session-status-spinner`
- `.status-icon--running`
- `.status-icon--pending`
- `.status-icon--waiting .status-icon__dot`
- `.status-icon--idle .status-icon__filled`

(2) ADR-0064 の single-guard rule (view.css 1 箇所集約) は維持する。新規 animation を追加するときの追記先固定ルールも維持。本 ADR は guard の **対象セレクタの定義を変更**する partial revision であり、ADR-0064 を supersede しない。

(3) StatusIcon の motion 設計上の不変条件:
- 周波数: 0.8s 〜 3.6s (低周波)
- 振幅: 1em × 1em (viewport-anchored, large-amplitude motion ではない)
- 位置: 親 (`.session-status-slot` 等) に固定、パララックス無し
- 機能: status の進行をリアルタイムに伝える progress indicator

これら 4 条件を満たさない新規 animation を追加するときは本 exemption の対象外とする。

(4) Reduce Motion ユーザでも、status は依然として **icon shape + color + aria-label の 3 重符号化** (ADR-0032 由来) で読み取れる。本 ADR は **追加で motion も維持**することで、目視判別の信頼性を Reduce Motion 環境でも非 Reduce 環境と同等まで引き上げる。

## Consequences

- **positive**: Reduce Motion ON 環境でも running / pending / waiting / idle が目視で即判別できる (ADR-0032 多重符号化を強化)。
- **positive**: WCAG 2.3.3 / Apple HIG / W3C "Designing for Reduced Motion" の意図と整合 (decorative motion のみ抑制、functional motion は維持)。
- **positive**: "Spinner が動かない" 報告のたびにコード側を疑う誤診 cycle を構造的に断つ。memory `[[feedback-verify-before-fix]]` に再発防止プロトコルを残した。
- **negative**: ごく一部の "motion を一切見たくない" ユーザにとっては想定外。ただし spinner は 1em の低振幅であり、画面酔い / 前庭障害トリガーになる類いの motion ではない。代替策が必要になった場合は将来 ADR で再検討。
- **neutral**: view.css の reduced-motion ブロックの他の suppression (drawer / toast / palette flash / snackbar / tabs / fab) はすべて維持。本 ADR の影響範囲は status-indicator のみ。

## Alternatives Considered

### Reduce Motion 環境では `animation: none` のまま、代わりに opacity breathing (低周波) で進行を示す

- ADR-0078 の 1 ファイル CSS シンプルさが崩れる
- breathing と spinner の二重実装になる
- breathing 自体も結局 animation なので、本質的に同じガード問題が残る

### Reduce Motion 環境では DOM 要素を差し替えて静的アイコン (▶ / ⏸ / ●) にする

- React 側に matchMedia listener を入れる必要があり overengineering
- ADR-0078 の CSS-only 設計を壊す
- アイコン切り替え自体が画面更新になり Reduce Motion の主旨と矛盾

### ユーザに OS の Reduce Motion をオフにしてもらう

- アクセシビリティ機能を OFF にしろと promote するのは UX 上不適切
- そもそも Reduce Motion は decorative motion 抑制の機能で、progress indicator は対象外 (W3C/Apple)
