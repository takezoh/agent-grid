// DriverShortcut — driver 別の compact shortcut 定義 (モバイル shortcut bar 用).
//
// `bytes` は WS `i` フレーム (`{k:"i", d, sessionId}`) で pty stdin に直接流れる
// 生バイト列。xterm がキーボード入力で吐く ANSI と同じ表現にしてある:
//   - Shift+Tab → CSI Z = terminfo kcbt = `\x1b[Z`
//   - Esc → `\x1b`
//   - Ctrl-C → `\x03`
//
// driver-name → shortcut list は Web 側 hardcode. shell/gemini/generic は
// table に居ないので DriverShortcutBar が render されない (正しい挙動).

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
      ariaLabel: "Claude mode を切り替え (Shift+Tab)",
      bytes: BYTES_SHIFT_TAB,
    },
    { id: "esc", label: "Esc", ariaLabel: "Escape を送信", bytes: BYTES_ESC },
    { id: "ctrlc", label: "Ctrl-C", ariaLabel: "中断 (Ctrl+C)", bytes: BYTES_CTRL_C },
  ],
  codex: [
    {
      id: "mode",
      label: "Mode",
      ariaLabel: "Codex approval mode を切り替え (Shift+Tab)",
      bytes: BYTES_SHIFT_TAB,
    },
    {
      id: "esc",
      label: "Esc",
      ariaLabel: "ターンをキャンセル (Escape)",
      bytes: BYTES_ESC,
    },
    { id: "ctrlc", label: "Ctrl-C", ariaLabel: "中断 (Ctrl+C)", bytes: BYTES_CTRL_C },
  ],
};

export function getDriverShortcuts(driver: string | null | undefined): readonly DriverShortcut[] {
  if (!driver) return [];
  return DRIVER_SHORTCUTS[driver] ?? [];
}
