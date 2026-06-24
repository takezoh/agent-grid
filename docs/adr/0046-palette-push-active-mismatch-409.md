# ADR 0046 — POST /api/sessions/{id}/push の id mismatch は 409 で返し、照合キーは web gateway が SubscribeEvents から保持する daemon-global ActiveSessionID とする

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-command-palette/spec.md), [plan](../specs/2026-06-24-web-ui-command-palette/plan.md)
Related requirements: FR-025, FR-026

## Context

push は daemon-global active session への操作という意味論 (TUI と同等)。client から path id を明示的に渡すことで race を検出可能にしたいが、照合相手 (daemon-global active か / 各 web client の活動 active か) を明確化する必要がある (MEMORY/web-active-session-ownership で client 独立管理が記録されている)。

## Decision

照合キーは web gateway が既に SubscribeEvents 経由で受信している daemon-global ActiveSessionID を使う。client は palette 起動時の自分の見ている activeSessionID を path に乗せ、その値と server 側 daemon-global active が不一致なら 409 Conflict を返す。Web フロントは送信前に store/daemon snapshot で同じ照合を 1 度行い (FR-023)、二段構えで race を検出する。

## Consequences

- **positive**: race 検出が二段 (client 即時 + server 409) で塞がる
- **positive**: path id を明示するので外部 client / curl 検証も成立
- **positive**: 認可失敗 (401) / 未存在 (404) / 競合 (409) を HTTP status で見分けられる
- **negative**: 複数 web client が異なる activeSessionID を持つ場合、フォアグラウンドでない client からの push は構造的に 409 になる (仕様として明示)

## Alternatives Considered

### POST /api/active-session/push (path に id を載せない)

却下: race 検出ができず外部 client にも不透明

### id 不一致でも強制 push (daemon の active を server 側で書き換え)

却下: UX 計画は push が active 切替を起こさない方針
