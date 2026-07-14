import type { JSX, MouseEvent } from "react";
import { usePaletteStore } from "../store/palette";
import { useDaemonSnapshot } from "../store/useDaemonSnapshot";
import { Icon } from "./icons/Icon";

/** FR-008: bottom-anchored New session button — opens palette at new-session. */
export function NewSessionButton(): JSX.Element {
  const daemonSnapshot = useDaemonSnapshot();

  const handleClick = (e: MouseEvent<HTMLButtonElement>) => {
    usePaletteStore.getState().openPalette({
      preselectToolId: "new-session",
      daemonSnapshot,
      opener: e.currentTarget,
    });
  };

  return (
    <button
      type="button"
      className="sidebar-new-session"
      data-role="new-session-button"
      aria-label="New session"
      onClick={handleClick}
    >
      <Icon name="plus" size={16} />
      <span>New session</span>
    </button>
  );
}
