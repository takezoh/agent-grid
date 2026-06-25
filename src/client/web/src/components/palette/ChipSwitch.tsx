// ChipSwitch.tsx — role='switch' chip for Worktree / Host toggles.
// FR-016 / FR-017 / FR-019 / FR-020 / FR-023 / FR-029 / UAC-008 / UAC-009 / UAC-010
//
// Note — DOM delegation to <SegmentedControl> is intentionally not done here.
// ChipSwitch uses role='switch' (ARIA binary toggle), whereas SegmentedControl
// renders role='radiogroup' + role='radio' (ARIA discrete-choice group). These are
// semantically distinct patterns with different keyboard contracts and screen-reader
// announcements; wrapping one in the other would produce an incorrect ARIA tree.
// Conceptually, a binary ON/OFF switch is the 2-state specialization of the same
// value/label pattern as SegmentedControl, but the DOM structures are separate.
import type { JSX, KeyboardEvent, PointerEvent } from "react";

export interface ChipSwitchProps {
  // hintKey: 'W' (worktree) or 'H' (host) — '[W]' / '[H]' icon hint.
  hintKey: "W" | "H";
  label: string; // 'Worktree' / 'Host (sandbox)' etc.
  checked: boolean; // aria-checked
  onToggle: () => void;
  disabled?: boolean;
  composing: boolean; // FR-023 IME guard
  // testid suffix (data-toggle=worktree|host) — for existing test DOM query compat
  testId?: string;
}

export function ChipSwitch(props: ChipSwitchProps): JSX.Element {
  const { hintKey, label, checked, onToggle, disabled, composing, testId } = props;

  function activate(): void {
    if (disabled) return;
    if (composing) return; // FR-023
    onToggle();
  }

  function onPointerDown(e: PointerEvent<HTMLButtonElement>): void {
    // FR-017: pointerdown + preventDefault prevents stealing focus from the text input.
    e.preventDefault();
    activate();
  }

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>): void {
    if (composing) return;
    if (e.key === " " || e.code === "Space") {
      // FR-019: Space activates the switch.
      e.preventDefault();
      activate();
      return;
    }
    if (e.key === "Enter") {
      // FR-020: Enter activates the switch.
      e.preventDefault();
      activate();
      return;
    }
  }

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      data-toggle={testId}
      data-on={checked ? "on" : "off"}
      className={`palette-chip palette-chip--${checked ? "on" : "off"}`}
      onPointerDown={onPointerDown}
      onKeyDown={onKeyDown}
    >
      <span aria-hidden="true" className="palette-chip__hint">
        [{hintKey}]
      </span>{" "}
      <span className="palette-chip__label">{label}</span>{" "}
      <span aria-hidden="true" className="palette-chip__state">
        {checked ? "ON" : "OFF"}
      </span>
    </button>
  );
}
