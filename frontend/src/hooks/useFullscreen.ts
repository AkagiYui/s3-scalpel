import { Events, Window } from "@wailsio/runtime";
import { createSignal, onCleanup, onMount, type Accessor } from "solid-js";
import { isMac } from "~/lib/platform";

const [isFullscreen, setIsFullscreen] = createSignal(false);
let initialized = false;

/**
 * Tracks the window's fullscreen state. macOS uses dedicated native events;
 * other platforms poll. Used to collapse the traffic-light gap in fullscreen.
 */
export function useFullscreen(): Accessor<boolean> {
  onMount(async () => {
    if (initialized) return;
    initialized = true;
    try {
      setIsFullscreen(await Window.IsFullscreen());
    } catch {
      /* no bridge (preview) */
    }

    if (isMac()) {
      const offEnter = Events.On("mac:WindowDidEnterFullScreen", () => setIsFullscreen(true));
      const offExit = Events.On("mac:WindowDidExitFullScreen", () => setIsFullscreen(false));
      onCleanup(() => {
        offEnter();
        offExit();
        initialized = false;
      });
    } else {
      const id = window.setInterval(async () => {
        try {
          setIsFullscreen(await Window.IsFullscreen());
        } catch {
          /* ignore */
        }
      }, 1000);
      onCleanup(() => {
        clearInterval(id);
        initialized = false;
      });
    }
  });

  return isFullscreen;
}
