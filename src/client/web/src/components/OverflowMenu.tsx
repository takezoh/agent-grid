import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import type { JSX } from "react";
import "../css/icon-button.css";
import { useThemeStore } from "../store/theme";
import type { Theme } from "../store/theme";
import { Icon } from "./icons/Icon";

const THEME_OPTIONS: Array<{ value: Theme; label: string }> = [
  { value: "system", label: "System" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

/** FR-012: header overflow menu — theme selection with agent-grid-theme contract. */
export function OverflowMenu(): JSX.Element {
  const theme = useThemeStore((s) => s.theme);
  const setTheme = useThemeStore((s) => s.setTheme);

  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className="icon-button overflow-menu__trigger"
          aria-label="More actions"
        >
          <Icon name="more-horizontal" size={18} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content className="overflow-menu" align="end" sideOffset={6}>
          <DropdownMenu.Label className="overflow-menu__label">Theme</DropdownMenu.Label>
          <DropdownMenu.RadioGroup value={theme} onValueChange={(v) => setTheme(v as Theme)}>
            {THEME_OPTIONS.map((opt) => (
              <DropdownMenu.RadioItem
                key={opt.value}
                className="overflow-menu__item"
                value={opt.value}
              >
                <DropdownMenu.ItemIndicator className="overflow-menu__check">
                  ✓
                </DropdownMenu.ItemIndicator>
                {opt.label}
              </DropdownMenu.RadioItem>
            ))}
          </DropdownMenu.RadioGroup>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
