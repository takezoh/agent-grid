// KeyboardFAB — explicit 閲覧→入力 mode toggle (FR-MOB-FAB-001 / FR-MOB-FAB-PD-001,
// ADR 0068 / 0075, WCAG 4.1.2 / 2.5.5).
//
// A thin wrapper over the IconButton primitive (chunk-02): the 44×44 target, the
// real <button>, and the pointerdown.preventDefault() focus-steal guard all live
// in IconButton. KeyboardFAB only synchronises the toggle semantics:
//   - aria-pressed mirrors `active`
//   - aria-label is 'キーボードを開く' (closed) / 'キーボードを閉じる' (open)
//   - click → onToggle()
// It carries `data-overlay` so the host pointer interceptor treats a tap on the
// FAB as an overlay tap (not an outside-tap), keeping it from self-triggering
// an exit.

import type { JSX } from "react";
import { IconButton } from "./primitives/IconButton";

export interface KeyboardFABProps {
  /** Current input-mode flag from useInputMode. */
  active: boolean;
  /** Flip the mode (FR-MOB-MODE-003 / 004). */
  onToggle: () => void;
}

export function KeyboardFAB({ active, onToggle }: KeyboardFABProps): JSX.Element {
  return (
    <IconButton
      className="keyboard-fab"
      data-overlay=""
      aria-pressed={active}
      aria-label={active ? "キーボードを閉じる" : "キーボードを開く"}
      onClick={onToggle}
      icon={<span aria-hidden="true">⌨</span>}
    />
  );
}
