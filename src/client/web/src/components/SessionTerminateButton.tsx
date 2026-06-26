// SessionTerminateButton — DriverViewPanel header に置くアクティブセッション終了ボタン.
//
// IconButton primitive を wrap して以下を担保:
//   - text + stop glyph の outlined danger button (ghost at rest / filled on hover)
//   - aria-label に session title を埋め込み (誤操作防止 / SR 親切)
//   - mobile (<768px && pointer:coarse) では 44x44 touch target
//
// 親 DriverViewPanel から渡された onRequestTerminate(id, opener) を呼ぶだけ.
// dialog 表示は AppShell 側で持つ (opener は dialog close 時の focus 戻し先).

import type { JSX, MouseEvent } from "react";
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
  const handleClick = (e: MouseEvent<HTMLButtonElement>): void => {
    onRequestTerminate(sessionId, e.currentTarget);
  };
  return (
    <IconButton
      className="session-terminate-button"
      aria-label={`「${sessionLabel}」を終了`}
      disabled={disabled}
      onClick={handleClick}
      icon={
        // Stop-square glyph: font に依存しないクリスプ描画 + currentColor 追従.
        <svg
          width="10"
          height="10"
          viewBox="0 0 10 10"
          aria-hidden="true"
          focusable="false"
          className="session-terminate-button__glyph"
        >
          <rect x="0" y="0" width="10" height="10" rx="1.5" fill="currentColor" />
        </svg>
      }
    >
      <span className="session-terminate-button__label">終了</span>
    </IconButton>
  );
}
