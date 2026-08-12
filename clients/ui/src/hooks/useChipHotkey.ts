import { useEffect } from "react";
import { usePaletteStore } from "../store/palette";

// useChipHotkey — document capture-phase listener for the Alt+W chip toggle.
// FR-018: Alt+W toggles the worktree chip.
// FR-023: IME composition suppresses the hotkey (composing guard).
// FR-030: worktree toggle in footer — Alt+W active whenever palette is open.
// ADR-0037: capture phase is load-bearing — same rationale as useGlobalHotkey.

export interface ChipHotkeyOptions {
  worktreeChipVisible: boolean;
}

export function useChipHotkey(opts: ChipHotkeyOptions): void {
  const { worktreeChipVisible } = opts;

  useEffect(() => {
    const listener = (e: KeyboardEvent): void => {
      const s = usePaletteStore.getState();
      if (!s.open) return;
      if (s.composing) return;
      if (!e.altKey) return;

      if (e.code === "KeyW" && worktreeChipVisible) {
        e.preventDefault();
        s.toggleWorktree();
      }
    };
    document.addEventListener("keydown", listener, true);
    return () => document.removeEventListener("keydown", listener, true);
  }, [worktreeChipVisible]);
}
