// DriverShortcut — per-driver compact shortcut definitions (mobile shortcut bar).
//
// `bytes` are raw byte sequences that flow straight to pty stdin via the WS
// `i` frame (`{k:"i", d, sessionId}`), matching the ANSI xterm emits for keys:
//   - Shift+Tab → CSI Z = terminfo kcbt = `\x1b[Z`
//   - Esc → `\x1b`
//   - Ctrl-C → `\x03`
//
// The driver-name → shortcut list is hardcoded web-side. shell/gemini/generic
// are absent from the table so DriverShortcutBar renders nothing (correct).

export const BYTES_SHIFT_TAB = "\x1b[Z";
export const BYTES_ESC = "\x1b";
export const BYTES_CTRL_C = "\x03";

export interface DriverShortcut {
  id: string;
  label: string;
  ariaLabel: string;
  bytes: string;
}

export const DRIVER_SHORTCUTS: Readonly<Record<string, readonly DriverShortcut[]>> = {
  claude: [
    {
      id: "mode",
      label: "Mode",
      ariaLabel: "Toggle Claude mode (Shift+Tab)",
      bytes: BYTES_SHIFT_TAB,
    },
    { id: "esc", label: "Esc", ariaLabel: "Send Escape", bytes: BYTES_ESC },
    { id: "ctrlc", label: "Ctrl-C", ariaLabel: "Interrupt (Ctrl+C)", bytes: BYTES_CTRL_C },
  ],
  codex: [
    {
      id: "mode",
      label: "Mode",
      ariaLabel: "Toggle Codex approval mode (Shift+Tab)",
      bytes: BYTES_SHIFT_TAB,
    },
    {
      id: "esc",
      label: "Esc",
      ariaLabel: "Cancel turn (Escape)",
      bytes: BYTES_ESC,
    },
    { id: "ctrlc", label: "Ctrl-C", ariaLabel: "Interrupt (Ctrl+C)", bytes: BYTES_CTRL_C },
  ],
};

export function getDriverShortcuts(driver: string | null | undefined): readonly DriverShortcut[] {
  if (!driver) return [];
  return DRIVER_SHORTCUTS[driver] ?? [];
}
