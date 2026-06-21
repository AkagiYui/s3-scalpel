import { System } from "@wailsio/runtime";
import { createSignal } from "solid-js";

export type OS = "mac" | "windows" | "linux" | "unknown";

/** Reliable synchronous guess from the webview user-agent (desktop webviews
 *  report their real OS). Available immediately, unlike the Wails environment. */
function fromUserAgent(): OS {
  const ua = (typeof navigator !== "undefined" && navigator.userAgent) || "";
  if (/Mac OS X|Macintosh/i.test(ua)) return "mac";
  if (/Windows/i.test(ua)) return "windows";
  if (/Linux|X11|CrOS/i.test(ua)) return "linux";
  return "unknown";
}

/** Authoritative Wails value, or null if the environment isn't injected yet.
 *  IsMac/IsWindows read window._wails.environment, which the native side
 *  populates after the page loads — so this can be null at import time. */
function fromWailsSync(): OS | null {
  try {
    if (System.IsMac()) return "mac";
    if (System.IsWindows()) return "windows";
    if (System.IsLinux()) return "linux";
  } catch {
    /* runtime not ready */
  }
  return null;
}

const [os, setOs] = createSignal<OS>(fromWailsSync() ?? fromUserAgent());

export const platformOS = os;
export const isMac = () => os() === "mac";
export const isWindows = () => os() === "windows";
export const isLinux = () => os() === "linux";

/**
 * Reconcile the platform with the authoritative Wails environment once the
 * runtime is ready (call from an onMount). Safe to call repeatedly; falls back
 * to the async Environment() query when the synchronous value isn't available.
 */
export function refreshPlatform() {
  const sync = fromWailsSync();
  if (sync) {
    setOs(sync);
    return;
  }
  try {
    System.Environment()
      .then((env: any) => {
        const o = env?.OS;
        if (o === "darwin") setOs("mac");
        else if (o === "windows") setOs("windows");
        else if (o === "linux") setOs("linux");
      })
      .catch(() => {});
  } catch {
    /* ignore */
  }
}
