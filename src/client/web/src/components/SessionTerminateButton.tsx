import * as Tooltip from "@radix-ui/react-tooltip";
import type { JSX, MouseEvent } from "react";
import "../css/session-terminate.css";
import { Icon } from "./icons/Icon";

export interface SessionTerminateButtonProps {
  sessionId: string;
  /** Confirm dialog title (e.g. titleText(card)). */
  sessionLabel: string;
  /** opener is focus-restore target on dialog close (WCAG 2.4.3 / 3.2.1). */
  onRequestTerminate: (sessionId: string, opener: HTMLElement) => void;
  disabled?: boolean;
}

/**
 * FR-024 / UAC-011: icon stop button in HeaderBar with Radix Tooltip.
 * ConfirmDialog flow is owned by App — this only fires onRequestTerminate.
 */
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
    <Tooltip.Provider delayDuration={300}>
      <Tooltip.Root>
        <Tooltip.Trigger asChild>
          <button
            type="button"
            className="icon-button session-terminate-button"
            aria-label={`Stop "${sessionLabel}"`}
            disabled={disabled}
            onClick={handleClick}
          >
            <Icon name="square" size={14} aria-hidden />
          </button>
        </Tooltip.Trigger>
        <Tooltip.Portal>
          <Tooltip.Content className="session-terminate-tooltip" sideOffset={6}>
            Stop session
            <Tooltip.Arrow className="session-terminate-tooltip__arrow" />
          </Tooltip.Content>
        </Tooltip.Portal>
      </Tooltip.Root>
    </Tooltip.Provider>
  );
}
