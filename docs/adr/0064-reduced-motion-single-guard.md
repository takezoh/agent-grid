# ADR 0064 — prefers-reduced-motion: reduce の一元 guard と新規 animation の追記先固定

Status: Proposed

Related: [spec](../specs/2026-06-25-web-ui-redesign/spec.md), [plan](../specs/2026-06-25-web-ui-redesign/plan.md), [ux](../specs/2026-06-25-web-ui-redesign/ux.md)
Related requirements: FR-MOTION-001, FR-MOTION-002

## Context

既存 view.css の .run-state-spinner / .session-status-spinner は animation: run-state-spin 0.8s linear infinite を無条件適用しており、prefers-reduced-motion: reduce 環境でも回り続ける (UAC-016 vs_legacy: must-fail)。palette 系には .palette-active-context--flash / .palette-listbox__row--flash 等の flash アニメも存在し、ADR-0052/0057 系で導入されている。否定役は『palette 既存 flash も同 guard 配下に入るかが書かれていない』を指摘した。

## Decision

(1) view.css 末尾に @media (prefers-reduced-motion: reduce) ブロックを 1 つ置き、その内側に .run-state-spinner / .session-status-spinner / .drawer-slide / .undo-snackbar / .palette-active-context--flash / .palette-listbox__row--flash / 各種 transition を集約する。
(2) 新規 animation を追加するときの追記先を本ブロックに固定し、PR description / コードコメントで明示する。
(3) reduced-motion でも running 状態は icon + text + 色で読み取れる (ADR-0032 多重符号化を維持)。

## Consequences

- positive: UAC-016 counterexample (無条件 animation) を 1 ブロック guard で構造的に解消。
- positive: 新規 animation 追加時の追記先が固定され retrospective レビューが容易。
- negative: palette 系 flash の挙動が reduced-motion で変わるため、既存 palette テスト (ADR-0052/0057 系) で flash 観察 assertion がある場合は test-setup の matchMedia mock で reduce=false を明示する必要がある。
- neutral: spinner の DOM 構造 (ADR-0032 加法的 spinner + aria-hidden) は維持。

## Alternatives Considered

### 各セレクタごとに inline で @media guard

追記先が分散して新規 animation 追加時の漏れが起きやすい。

### JS で prefers-reduced-motion を読み animation を出し分け

DOM 操作が増え CSS-only で達成できる責務に対して overengineering。

### reduced-motion で running を icon+text+色のうち色だけ残す (= spinner と text を両方消す)

ADR-0032 多重符号化 (icon+text+色) を侵害し screen-reader 利用者の状態把握を弱める。
