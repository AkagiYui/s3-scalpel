import { System } from "@wailsio/runtime";

// Platform flags, resolved once. Outside the Wails webview (e.g. a plain-browser
// preview) the runtime has no bridge and these safely report false.
export const isMac = safe(() => System.IsMac());
export const isWindows = safe(() => System.IsWindows());
export const isLinux = safe(() => System.IsLinux());

function safe(fn: () => boolean): boolean {
  try {
    return fn();
  } catch {
    return false;
  }
}
