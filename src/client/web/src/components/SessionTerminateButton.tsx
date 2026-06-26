// SessionTerminateButton — SessionRow 内の ✕ (terminate) ボタン.
//
// IconButton primitive を wrap して以下を担保:
//   - 44x44 touch target (IconButton 既定 + CSS で PC 表示時のみ縮小)
//   - aria-label に session title を埋め込み (誤操作防止 / SR 親切)
//   - 親 UnifiedListbox の onPointerDown が "行 activate" を発火するので
//     pointerdown と click の両方で stopPropagation する (click では IconButton
//     primitive が pointerdown.preventDefault するが、伝播は止めない)
//
// 親 SessionRow から渡された onRequestTerminate(id) を呼ぶだけ. dialog 表示
// は AppShell 側で持つ.

import type { JSX, MouseEvent, PointerEvent } from "react";
import "../css/session-terminate.css";
import { IconButton } from "./primitives/IconButton";

export interface SessionTerminateButtonProps {
  sessionId: string;
  /** Confirm dialog 用の title (例: titleText(card)). */
  sessionLabel: string;
  /** opener は dialog close 時の focus 戻し先 (a11y; WCAG 2.4.3 / 3.2.1). */
  onRequestTerminate: (sessionId: string, opener: HTMLElement) => void;
  disabled?: boolean;
}

export function SessionTerminateButton({
  sessionId,
  sessionLabel,
  onRequestTerminate,
  disabled,
}: SessionTerminateButtonProps): JSX.Element {
  const handlePointerDown = (e: PointerEvent<HTMLButtonElement>): void => {
    // UnifiedListbox の onPointerDown (activate row) を抑制. IconButton
    // primitive 側も pointerdown を listen して preventDefault するが、
    // 親への伝播は別軸なので stopPropagation を明示する.
    e.stopPropagation();
  };
  const handleClick = (e: MouseEvent<HTMLButtonElement>): void => {
    e.stopPropagation();
    onRequestTerminate(sessionId, e.currentTarget);
  };
  return (
    <IconButton
      className="session-terminate-button"
      aria-label={`「${sessionLabel}」を終了`}
      disabled={disabled}
      onPointerDown={handlePointerDown}
      onClick={handleClick}
      icon={<span aria-hidden="true">×</span>}
    />
  );
}
