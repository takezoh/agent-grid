// DriverShortcutBar — mobile 入力モード時に出る driver 固有 shortcut の compact bar.
//
// 配置: `.terminal-fab-layer` 内の bottom-left anchor (既存 KeyboardFAB /
// JumpToLatestFAB は bottom-right なので衝突しない). `data-overlay` を付けて
// useHostPointerInterceptor が outside-tap 扱いしないようにする (KeyboardFAB
// と同じ pattern). `--terminal-fab-offset` を読んで iOS soft keyboard 上に
// lift される (useVisualViewportLift が同 layer の CSS 変数を更新).
//
// 可視性 gate:
//   - inputActive (useInputMode().active) — false なら null return (閲覧モード
//     では bar 非表示).
//   - driver が DRIVER_SHORTCUTS に居る — claude / codex のみ. shell / gemini /
//     generic では table lookup 失敗で null return.
//
// tap → sendInput(bytes). 既存の KeyboardFAB / JumpToLatestFAB と同じく
// IconButton primitive 経由なので pointerdown.preventDefault で textarea から
// focus を奪わず soft keyboard を閉じない.

import type { JSX } from "react";
import "../css/driver-shortcut-bar.css";
import { type DriverShortcut, getDriverShortcuts } from "../lib/driverShortcuts";
import { IconButton } from "./primitives/IconButton";

export interface DriverShortcutBarProps {
  /** activeSession.root_driver. 不明 / undefined なら bar 自体が描画されない. */
  driver: string | null | undefined;
  /** useInputMode().active. false の間は bar を隠す. */
  inputActive: boolean;
  /** TerminalPane から渡される raw byte 送信 closure (`{k:"i", d, sessionId}`). */
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
      aria-label={`${driver} の shortcut`}
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
