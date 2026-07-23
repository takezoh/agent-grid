// ThemeSegmentedControl.tsx — System / Light / Dark theme selector (FR-THEME-007 / ADR-0062)
//
// Wraps the SegmentedControl primitive with the three Theme values.
// ADR-0062: On narrow viewports (< 768px) the control is hidden via CSS
// @media and the palette suggested-action path absorbs the theme entry point.
import type { JSX } from "react";
import { useThemeStore } from "../store/theme";
import type { Theme } from "../store/theme";
import { SegmentedControl } from "./primitives/SegmentedControl";
import type { Segment } from "./primitives/SegmentedControl";

const SEGMENTS: Segment<Theme>[] = [
  { value: "system", label: "System", ariaLabel: "System" },
  { value: "light", label: "Light", ariaLabel: "Light" },
  { value: "dark", label: "Dark", ariaLabel: "Dark" },
];

export function ThemeSegmentedControl(): JSX.Element {
  const theme = useThemeStore((s) => s.theme);
  const setTheme = useThemeStore((s) => s.setTheme);

  return (
    <div className="theme-segmented-control">
      <SegmentedControl<Theme>
        ariaLabel="Theme"
        segments={SEGMENTS}
        value={theme}
        onChange={setTheme}
        idPrefix="theme-sc"
      />
    </div>
  );
}
