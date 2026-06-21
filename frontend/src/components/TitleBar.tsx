import { Show, createSignal, onMount, type Component } from "solid-js";
import { Window } from "@wailsio/runtime";
import { Scissors, Minus, Square, Copy, X } from "lucide-solid";
import { isMac } from "~/lib/platform";
import { useFullscreen } from "~/hooks/useFullscreen";
import { t } from "~/i18n";

/**
 * Full-width draggable title bar. macOS keeps the native traffic lights (a gap
 * is reserved on the left); Windows/Linux get custom min/max/close controls.
 */
export const TitleBar: Component = () => {
  const fullscreen = useFullscreen();
  const [maximised, setMaximised] = createSignal(false);

  onMount(() => {
    if (!isMac) Window.IsMaximised().then(setMaximised).catch(() => {});
  });

  const toggleMax = async () => {
    try {
      await Window.ToggleMaximise();
      setMaximised(await Window.IsMaximised());
    } catch {
      /* no bridge */
    }
  };

  return (
    <div
      class="drag flex h-10 shrink-0 select-none items-center border-b bg-card text-sm"
      onDblClick={() => {
        if (!isMac) toggleMax();
      }}
    >
      {/* Reserve space for the macOS traffic lights (hidden in fullscreen). */}
      <Show when={isMac && !fullscreen()}>
        <div class="h-full w-[78px] shrink-0" />
      </Show>

      {/* App identity */}
      <div class="flex items-center gap-2 px-3" classList={{ "pl-2": isMac }}>
        <Scissors class="h-4 w-4 text-primary" />
        <span class="font-semibold tracking-tight">{t("app.name")}</span>
      </div>

      {/* Draggable filler */}
      <div class="h-full flex-1" />

      {/* Windows / Linux window controls */}
      <Show when={!isMac}>
        <div class="no-drag flex h-full items-center">
          <button class="winctrl-btn" title={t("window.minimize")} onClick={() => Window.Minimise()}>
            <Minus class="h-4 w-4" />
          </button>
          <button
            class="winctrl-btn"
            title={maximised() ? t("window.restore") : t("window.maximize")}
            onClick={toggleMax}
          >
            <Show when={maximised()} fallback={<Square class="h-3.5 w-3.5" />}>
              <Copy class="h-3.5 w-3.5" />
            </Show>
          </button>
          <button class="winctrl-btn winctrl-close" title={t("window.close")} onClick={() => Window.Close()}>
            <X class="h-4 w-4" />
          </button>
        </div>
      </Show>
    </div>
  );
};
