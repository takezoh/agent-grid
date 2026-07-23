// DriverShortcutBar — compact bar of driver-specific shortcuts shown in mobile input mode.
//
// Placement: bottom-left anchor inside `.terminal-fab-layer` (KeyboardFAB /
// JumpToLatestFAB sit bottom-right, so no collision). Carries `data-overlay` so
// useHostPointerInterceptor does not treat taps as outside-taps (same pattern
// as KeyboardFAB). Reads `--terminal-fab-offset` so it lifts above the iOS soft
// keyboard (useVisualViewportLift updates the layer's CSS variable).
//
// Visibility gate:
//   - inputActive (useInputMode().active) — returns null when false (bar is
//     hidden in view mode).
//   - driver present in DRIVER_SHORTCUTS — claude / codex only. shell / gemini /
//     generic fail the table lookup and return null.
//
// tap → sendInput(bytes). Like KeyboardFAB / JumpToLatestFAB it goes through
// the IconButton primitive, so pointerdown.preventDefault keeps focus in the
// textarea and the soft keyboard stays open.

import type { JSX } from "react";
import "../css/driver-shortcut-bar.css";
import { type DriverShortcut, getDriverShortcuts } from "../lib/driverShortcuts";
import { IconButton } from "./primitives/IconButton";

export interface DriverShortcutBarProps {
  /** activeSession.root_driver. Unknown / undefined renders no bar at all. */
  driver: string | null | undefined;
  /** useInputMode().active. The bar is hidden while false. */
  inputActive: boolean;
  /** Raw-byte send closure passed from TerminalPane (`{k:"i", d, sessionId}`). */
  sendInput: (data: string) => void;
}

export function DriverShortcutBar({
  driver,
  inputActive,
  sendInput,
}: DriverShortcutBarProps): JSX.Element | null {
  if (!inputActive) return null;
  const shortcuts = getDriverShortcuts(driver);
  if (shortcuts.length === 0) return null;

  return (
    <div
      className="driver-shortcut-bar"
      data-overlay=""
      data-driver={driver ?? ""}
      role="toolbar"
      aria-label={`${driver} shortcuts`}
    >
      {shortcuts.map((sc: DriverShortcut) => (
        <IconButton
          key={sc.id}
          className="driver-shortcut-bar__btn"
          data-overlay=""
          aria-label={sc.ariaLabel}
          onClick={() => sendInput(sc.bytes)}
        >
          <span aria-hidden="true">{sc.label}</span>
        </IconButton>
      ))}
    </div>
  );
}
