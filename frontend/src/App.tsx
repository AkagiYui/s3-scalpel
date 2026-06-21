import { onMount, onCleanup, type Component } from "solid-js";
import { useNavigate, type RouteSectionProps } from "@solidjs/router";
import { Sidebar } from "~/components/Sidebar";
import { TitleBar } from "~/components/TitleBar";
import { Toaster } from "~/components/ui/toast";
import { loadSettings } from "~/stores/settings";
import { loadConnections } from "~/stores/connections";
import { initQueue } from "~/stores/tasks";
import { windowID, onEvent } from "~/lib/api";
import { refreshPlatform } from "~/lib/platform";
import * as bus from "~/lib/bus";

export const App: Component<RouteSectionProps> = (props) => {
  const navigate = useNavigate();
  const wid = windowID();

  onMount(() => {
    refreshPlatform();
    loadSettings();
    loadConnections();
    initQueue();
  });

  // Menu / global shortcut actions are routed from the Go menu to the focused
  // window. Act only on events addressed to this window.
  const offMenu = onEvent<any>("menu:action", (d) => {
    if (d?.wid && d.wid !== wid) return;
    switch (d?.action) {
      case "settings":
        navigate("/settings");
        break;
      case "about":
        navigate("/settings");
        setTimeout(() => bus.emit("scroll-about"), 50);
        break;
      case "new-connection":
        navigate("/");
        setTimeout(() => bus.emit("new-connection"), 0);
        break;
      case "new-tab":
        navigate("/storage");
        setTimeout(() => bus.emit("new-tab"), 0);
        break;
      case "close-tab":
        bus.emit("close-tab");
        break;
      case "refresh":
        bus.emit("refresh");
        break;
      case "find":
        bus.emit("find");
        break;
    }
  });

  const offSound = onEvent<any>("notify:sound", (d) => bus.playBeep(!!d?.error));

  onCleanup(() => {
    offMenu();
    offSound();
  });

  return (
    <div class="flex h-full w-full flex-col overflow-hidden bg-background text-foreground">
      <TitleBar />
      <div class="flex min-h-0 flex-1 overflow-hidden">
        <Sidebar />
        <main class="flex min-w-0 flex-1 flex-col overflow-hidden">{props.children}</main>
      </div>
      <Toaster />
    </div>
  );
};
