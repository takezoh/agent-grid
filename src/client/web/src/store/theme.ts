import { create } from "zustand";

export type Theme = "system" | "light" | "dark";

export type ThemeState = {
  theme: Theme;
};

export type ThemeActions = {
  setTheme: (value: Theme) => void;
};

const VALID_THEMES: readonly Theme[] = ["system", "light", "dark"];

export const useThemeStore = create<ThemeState & ThemeActions>()((set) => ({
  theme: "system",
  setTheme: (value) => {
    if (!VALID_THEMES.includes(value)) return;
    set({ theme: value });
  },
}));
