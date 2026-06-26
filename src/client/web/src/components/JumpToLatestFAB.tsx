// JumpToLatestFAB — the ↓最新 (jump-to-latest) FAB (FR-MOB-JUMP-001..004,
// ADR 0073 / 0075, WCAG 4.1.2 / 2.5.5).
//
// A thin wrapper over the IconButton primitive (chunk-02): the 44×44 target, the
// real <button>, the non-empty aria-label enforcement, and the
// pointerdown.preventDefault() focus-steal guard all live in IconButton.
//
// The component is *conditionally rendered* on `show` — when the viewport is at
// tail it returns null so the button is fully absent from the DOM and the a11y
// tree (UAC-012). Hiding it with CSS opacity:0 is forbidden: it would leave a
// phantom button readable by screen readers. It carries `data-overlay` so the
// host pointer interceptor (chunk-03) treats a tap as an overlay tap, not an
// outside-tap.

import type { JSX } from "react";
import "../css/jump-fab.css";
import { IconButton } from "./primitives/IconButton";

export interface JumpToLatestFABProps {
  /** From useJumpToLatest.shouldShowFab — true only while in scrollback. */
  show: boolean;
  /** Jump-to-latest action (useJumpToLatest.jumpToBottom). */
  onJump: () => void;
}

export function JumpToLatestFAB({ show, onJump }: JumpToLatestFABProps): JSX.Element | null {
  // Conditional render (not CSS hiding): absent from DOM + a11y tree at tail.
  if (!show) return null;

  return (
    <IconButton
      className="jump-to-latest-fab"
      data-overlay=""
      aria-label="最新へスクロール"
      onClick={onJump}
      icon={<span aria-hidden="true">↓</span>}
    />
  );
}
