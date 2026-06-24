# ADR 0042 — new-session 送信は host?:bool ではなく既存 wire (sandbox?:'host') に揃える

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-command-palette/spec.md), [plan](../specs/2026-06-24-web-ui-command-palette/plan.md)
Related requirements: FR-014, FR-015, FR-021

## Context

本機能の host トグル ON は sandbox bypass を意味する。一方、既存 POST /api/sessions の apiCreateReq は sandbox?: 'auto' | 'host' という string enum を採用している (TUI と共通)。本 spec で host?:bool を新設すると wire 形式が二重化する。

## Decision

ToolDef new-session の送信ペイロードでは host トグル ON → sandbox: 'host'、OFF → sandbox 省略 (= 'auto' 扱い) にマッピングする。worktree?:bool は既存形式のまま使う。

## Consequences

- **positive**: mux.go の parseSandbox / apiCreateReq に手を入れずに済む
- **positive**: TUI と Web で wire vocabulary が揃う
- neutral: TS 側で host (UI 名) → sandbox (wire 名) のマッピング 1 関数を持つ

## Alternatives Considered

### host?:bool を新設し apiCreateReq を拡張

却下: wire 形式の二重化と TUI とのズレ
