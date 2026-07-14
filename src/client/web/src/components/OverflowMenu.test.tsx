import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useThemeStore } from "../store/theme";
import { OverflowMenu } from "./OverflowMenu";
import { ThemeProvider } from "./ThemeProvider";

const STORAGE_KEY = "agent-grid-theme";

function openMenu() {
  const trigger = screen.getByRole("button", { name: "More actions" });
  fireEvent.pointerDown(trigger);
  fireEvent.click(trigger);
}

function renderMenu() {
  return render(
    <ThemeProvider>
      <OverflowMenu />
    </ThemeProvider>,
  );
}

beforeEach(() => {
  useThemeStore.setState({ theme: "system" });
  localStorage.clear();
  delete document.documentElement.dataset.theme;
});

afterEach(() => {
  localStorage.clear();
});

describe("OverflowMenu theme (FR-012 / UAC-005)", () => {
  it("renders overflow trigger with More actions label", () => {
    renderMenu();
    expect(screen.getByRole("button", { name: "More actions" })).toBeTruthy();
  });

  it("selecting Light persists agent-grid-theme=light in localStorage", async () => {
    renderMenu();
    openMenu();
    const light = await screen.findByText("Light");
    act(() => {
      fireEvent.click(light);
    });
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
    expect(useThemeStore.getState().theme).toBe("light");
  });

  it("selecting System removes agent-grid-theme from localStorage", async () => {
    localStorage.setItem(STORAGE_KEY, "dark");
    useThemeStore.setState({ theme: "dark" });
    renderMenu();
    openMenu();
    const system = await screen.findByText("System");
    act(() => {
      fireEvent.click(system);
    });
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
    expect(useThemeStore.getState().theme).toBe("system");
  });
});
