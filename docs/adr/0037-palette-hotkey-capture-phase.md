# ADR 0037 — Cmd/Ctrl+K は document の capture phase で listen し、常設ボタンを保険として併設する

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-command-palette/spec.md), [plan](../specs/2026-06-24-web-ui-command-palette/plan.md)
Related requirements: FR-001, FR-002, FR-029

## Context

xterm.js は textarea 上で keydown を消費するため bubble phase では hotkey が奪われる。一方 Firefox の Ctrl+K (検索バー) など preventDefault 不能なブラウザ実装が残る。

## Decision

document.addEventListener('keydown', handler, {capture: true}) で先取りし preventDefault する。同時に Header に常設『Command (⌘K / Ctrl+K)』ボタンを露出してキー奪われ環境の保険とする。useGlobalHotkey は App.tsx トップで 1 回 mount し、既に open なら検索 input に focus を戻すだけ (phase/入力保持)。

## Consequences

- **positive**: xterm focus 中でも palette 起動が成立する
- **positive**: ADR-0030 の subscribe 唯一所有を破らない (key 観測のみ、subscribe を作らない)
- **positive**: 連打冪等性が hooks 構造で保証される
- **negative**: capture phase の挙動は xterm textarea が document 子孫であることに依存するため、jsdom テストでその経路を担保する必要がある

## Alternatives Considered

### window bubble phase で listen

却下: xterm が key を奪い起動できないケースが残る

### xterm.attachCustomKeyEventHandler で内部から検出

却下: xterm 依存が深くなり、palette が xterm 初期化順序に影響される
