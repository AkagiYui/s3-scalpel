import { createSignal, createRoot, createEffect } from "solid-js";
import { SettingsService, type AppSettings, onEvent } from "~/lib/api";

export const defaultSettings: AppSettings = {
  language: "system",
  theme: "system",
  notifyEnabled: true,
  notifySound: true,
  concurrency: 5,
  partSize: 8 * 1024 * 1024,
  multipartEnabled: true,
  autoConsumeQueue: true,
  previewMaxSize: 10 * 1024 * 1024,
  defaultDownloadDir: "",
} as AppSettings;

const [settings, setSettings] = createSignal<AppSettings>({ ...defaultSettings });
export { settings };

const [systemDark, setSystemDark] = createSignal(
  window.matchMedia("(prefers-color-scheme: dark)").matches
);

// Track the OS color scheme so "system" theme reacts live.
const mql = window.matchMedia("(prefers-color-scheme: dark)");
mql.addEventListener("change", (e) => setSystemDark(e.matches));

/** Effective UI locale, resolving "system" against the browser language. */
export function effectiveLocale(): "en" | "zh" {
  const lang = settings().language;
  if (lang === "zh" || lang === "en") return lang;
  return navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

/** Effective color theme, resolving "system" against the OS preference. */
export function effectiveTheme(): "light" | "dark" {
  const t = settings().theme;
  if (t === "light" || t === "dark") return t;
  return systemDark() ? "dark" : "light";
}

// Apply the theme to the document whenever it changes. createRoot keeps this
// app-global effect alive for the lifetime of the webview.
createRoot(() => {
  createEffect(() => {
    const dark = effectiveTheme() === "dark";
    const root = document.documentElement;
    root.classList.toggle("dark", dark);
    root.setAttribute("data-kb-theme", dark ? "dark" : "light");
    root.style.colorScheme = dark ? "dark" : "light";
  });
});

/** Load settings from the backend. */
export async function loadSettings() {
  try {
    const s = await SettingsService.Get();
    if (s) setSettings(s);
  } catch (e) {
    console.error("loadSettings", e);
  }
}

/** Persist a partial settings update (optimistic). */
export async function updateSettings(patch: Partial<AppSettings>) {
  const next = { ...settings(), ...patch } as AppSettings;
  setSettings(next);
  try {
    const saved = await SettingsService.Update(next);
    if (saved) setSettings(saved);
  } catch (e) {
    console.error("updateSettings", e);
    await loadSettings();
  }
}

// Keep all windows in sync when settings change anywhere.
onEvent<AppSettings>("settings:changed", (s) => {
  if (s) setSettings(s);
});
