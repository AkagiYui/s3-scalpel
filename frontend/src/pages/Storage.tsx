import { Show, For, onMount, onCleanup, type Component } from "solid-js";
import { HardDrive, Database, Plus } from "lucide-solid";
import { TabBar } from "~/features/storage/TabBar";
import { BucketList } from "~/features/storage/BucketList";
import { FileBrowser } from "~/features/storage/FileBrowser";
import { TaskPanel } from "~/features/storage/TaskPanel";
import { Button } from "~/components/ui/button";
import { tabs, activeTab, activeTabId, openTab, closeTab, openBucket } from "~/stores/tabs";
import { connections } from "~/stores/connections";
import { QueueService, windowID, onEvent } from "~/lib/api";
import { toast } from "~/components/ui/toast";
import * as bus from "~/lib/bus";
import { t } from "~/i18n";

const wid = windowID();

const Storage: Component = () => {
  onMount(() => {
    const offNewTab = bus.on("new-tab", () => {
      const cur = activeTab();
      if (cur) {
        openTab(cur.connectionId, cur.title);
      } else if (connections().length) {
        const c = connections()[0];
        openTab(c.id, c.name);
      } else {
        toast.info(t("connections.none"));
      }
    });
    const offCloseTab = bus.on("close-tab", () => {
      if (activeTabId()) closeTab(activeTabId());
    });

    // Native drag-and-drop uploads land here, addressed to this window.
    const offDrop = onEvent<any>("files:dropped", async (d) => {
      if (d?.wid !== wid) return;
      const paths: string[] = d?.paths ?? [];
      const cur = activeTab();
      if (!cur || !cur.bucket) {
        toast.info(t("storage.pickConnectionHint"));
        return;
      }
      try {
        const n = await QueueService.EnqueueUpload(wid, cur.connectionId, cur.bucket, cur.prefix, paths, 0);
        toast.success(t("storage.enqueued", { count: n }));
      } catch (e: any) {
        toast.error(String(e?.message ?? e));
      }
    });

    onCleanup(() => {
      offNewTab();
      offCloseTab();
      offDrop();
    });
  });

  return (
    <div class="flex h-full flex-col">
      <Show
        when={tabs().length}
        fallback={
          <div class="flex h-full flex-col items-center justify-center gap-5 text-center text-muted-foreground">
            <HardDrive class="h-14 w-14 opacity-30" />
            <div>
              <p class="text-base font-medium text-foreground">{t("storage.pickConnection")}</p>
              <p class="text-sm">{t("storage.pickConnectionHint")}</p>
            </div>
            <Show
              when={connections().length}
              fallback={<p class="text-sm">{t("connections.none")}</p>}
            >
              <div class="flex flex-wrap justify-center gap-2">
                <For each={connections()}>
                  {(c) => (
                    <Button variant="outline" onClick={() => openTab(c.id, c.name)}>
                      <Database class="h-4 w-4" />
                      {c.name}
                    </Button>
                  )}
                </For>
              </div>
            </Show>
          </div>
        }
      >
        <TabBar />
        <div class="min-h-0 flex-1">
          <Show when={activeTab()} keyed>
            {(tab) => (
              <Show
                when={tab.bucket}
                fallback={<BucketList connId={tab.connectionId} onOpen={(b) => openBucket(tab.id, b)} />}
              >
                <FileBrowser tab={tab} />
              </Show>
            )}
          </Show>
        </div>
        <TaskPanel />
      </Show>
    </div>
  );
};

export default Storage;
