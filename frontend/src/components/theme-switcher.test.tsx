import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { themeInitScript, themeStorageKey } from "@/lib/theme";
import { ThemeSwitcher } from "./theme-switcher";

function initializeTheme() {
  // Execute the same static script that the root layout sends before first paint.
  window.eval(themeInitScript);
}

beforeEach(() => {
  // Use jsdom storage rather than Node's experimental global localStorage.
  const browserWindow = (globalThis as typeof globalThis & { jsdom: { window: Window } }).jsdom.window;
  vi.stubGlobal("localStorage", browserWindow.localStorage);
  vi.stubGlobal("sessionStorage", browserWindow.sessionStorage);
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("theme selection", () => {
  it("defaults to system appearance", () => {
    initializeTheme();
    render(<ThemeSwitcher />);
    expect(document.documentElement.dataset.theme).toBe("system");
    expect((screen.getByRole("combobox", { name: "Тема" }) as HTMLSelectElement).value).toBe("system");
  });

  it.each(["light", "dark"])("restores %s before React renders", (theme) => {
    localStorage.setItem(themeStorageKey, theme);
    initializeTheme();
    expect(document.documentElement.dataset.theme).toBe(theme);
    render(<ThemeSwitcher />);
    expect((screen.getByLabelText("Тема") as HTMLSelectElement).value).toBe(theme);
  });

  it("persists changes, restores them on reload and returns to system mode", async () => {
    const user = userEvent.setup();
    render(<ThemeSwitcher />);
    await user.selectOptions(screen.getByLabelText("Тема"), "dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem(themeStorageKey)).toBe("dark");

    cleanup();
    document.documentElement.removeAttribute("data-theme");
    initializeTheme();
    render(<ThemeSwitcher />);
    expect((screen.getByLabelText("Тема") as HTMLSelectElement).value).toBe("dark");

    await user.selectOptions(screen.getByLabelText("Тема"), "light");
    expect(document.documentElement.dataset.theme).toBe("light");
    await user.selectOptions(screen.getByLabelText("Тема"), "system");
    expect(document.documentElement.dataset.theme).toBe("system");
    expect(localStorage.getItem(themeStorageKey)).toBe("system");
  });

  it.each(["invalid", ""])("ignores an invalid stored value: %s", (value) => {
    localStorage.setItem(themeStorageKey, value);
    initializeTheme();
    expect(document.documentElement.dataset.theme).toBe("system");
  });

  it("works when browser storage is unavailable", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("Storage blocked"); });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("Storage blocked"); });
    expect(initializeTheme).not.toThrow();
    render(<ThemeSwitcher />);
    await userEvent.setup().selectOptions(screen.getByLabelText("Тема"), "dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect((screen.getByLabelText("Тема") as HTMLSelectElement).value).toBe("dark");
  });

  it("syncs changes and storage clearing from another tab", () => {
    render(<ThemeSwitcher />);
    act(() => window.dispatchEvent(new StorageEvent("storage", { key: themeStorageKey, newValue: "dark", storageArea: localStorage })));
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect((screen.getByLabelText("Тема") as HTMLSelectElement).value).toBe("dark");

    act(() => window.dispatchEvent(new StorageEvent("storage", { key: null, newValue: null, storageArea: localStorage })));
    expect(document.documentElement.dataset.theme).toBe("system");
    expect((screen.getByLabelText("Тема") as HTMLSelectElement).value).toBe("system");
  });

  it("ignores unrelated storage changes", () => {
    localStorage.setItem(themeStorageKey, "dark");
    initializeTheme();
    render(<ThemeSwitcher />);
    act(() => window.dispatchEvent(new StorageEvent("storage", { key: "other", newValue: "light", storageArea: localStorage })));
    act(() => window.dispatchEvent(new StorageEvent("storage", { key: themeStorageKey, newValue: "light", storageArea: sessionStorage })));
    expect(document.documentElement.dataset.theme).toBe("dark");
  });
});
