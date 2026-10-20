export type Theme = "system" | "light" | "dark";

export const themeStorageKey = "uptime-theme";
const themeChangeEvent = "uptime-theme-change";

function normalizeTheme(value: string | null | undefined): Theme {
  return value === "light" || value === "dark" ? value : "system";
}

// Run in <head> before paint; system colors are resolved by CSS.
export const themeInitScript = `(() => {
  try {
    const theme = localStorage.getItem(${JSON.stringify(themeStorageKey)});
    document.documentElement.dataset.theme = theme === "light" || theme === "dark" ? theme : "system";
  } catch {}
})();`;

export function getTheme(): Theme {
  return normalizeTheme(document.documentElement.dataset.theme);
}

export function getServerTheme(): Theme {
  return "system";
}

export function setTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem(themeStorageKey, theme);
  } catch {
    // The selection still works when browser storage is unavailable.
  }
  window.dispatchEvent(new Event(themeChangeEvent));
}

export function subscribeToTheme(onChange: () => void) {
  function handleStorage(event: StorageEvent) {
    if (event.storageArea !== localStorage || (event.key !== themeStorageKey && event.key !== null)) return;
    document.documentElement.dataset.theme = normalizeTheme(event.newValue);
    onChange();
  }

  window.addEventListener(themeChangeEvent, onChange);
  window.addEventListener("storage", handleStorage);
  return () => {
    window.removeEventListener(themeChangeEvent, onChange);
    window.removeEventListener("storage", handleStorage);
  };
}
