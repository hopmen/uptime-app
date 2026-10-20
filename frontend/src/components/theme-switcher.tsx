"use client";

import { useSyncExternalStore } from "react";
import { getServerTheme, getTheme, setTheme, subscribeToTheme, type Theme } from "@/lib/theme";

export function ThemeSwitcher() {
  const theme = useSyncExternalStore(subscribeToTheme, getTheme, getServerTheme);

  return (
    <label className="theme-switcher">
      <span>Тема</span>
      <select value={theme} onChange={(event) => setTheme(event.target.value as Theme)}>
        <option value="system">Системная</option>
        <option value="light">Светлая</option>
        <option value="dark">Тёмная</option>
      </select>
    </label>
  );
}
