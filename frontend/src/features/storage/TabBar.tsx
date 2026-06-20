import { For, Show, type Component } from "solid-js";
import { X, Plus, Database } from "lucide-solid";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel } from "~/components/ui/dropdown-menu";
import { tabs, activeTabId, focusTab, closeTab, openTab, type Tab } from "~/stores/tabs";
import { connections } from "~/stores/connections";
import { cn } from "~/lib/utils";
import { t } from "~/i18n";

const TabChip: Component<{ tab: Tab }> = (props) => {
  const active = () => props.tab.id === activeTabId();
  const sub = () => props.tab.bucket || "";
  return (
    <div
      onClick={() => focusTab(props.tab.id)}
      onAuxClick={(e) => {
        if (e.button === 1) closeTab(props.tab.id); // middle-click closes
      }}
      class={cn(
        "group flex h-8 min-w-0 max-w-52 cursor-pointer items-center gap-2 rounded-t-md border-x border-t px-3 text-sm",
        active()
          ? "border-border bg-background"
          : "border-transparent bg-transparent text-muted-foreground hover:bg-accent/50"
      )}
    >
      <Database class="h-3.5 w-3.5 shrink-0" />
      <span class="min-w-0 flex-1 truncate">
        {props.tab.title}
        <Show when={sub()}>
          <span class="text-muted-foreground"> / {sub()}</span>
        </Show>
      </span>
      <button
        class="shrink-0 rounded p-0.5 opacity-0 hover:bg-accent group-hover:opacity-100"
        classList={{ "opacity-100": active() }}
        onClick={(e) => {
          e.stopPropagation();
          closeTab(props.tab.id);
        }}
      >
        <X class="h-3 w-3" />
      </button>
    </div>
  );
};

export const TabBar: Component = () => {
  return (
    <div class="flex h-10 shrink-0 items-end gap-1 border-b bg-card px-2 pt-2">
      <div class="flex min-w-0 flex-1 items-end gap-1 overflow-x-auto no-scrollbar">
        <For each={tabs()}>{(tab) => <TabChip tab={tab} />}</For>
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger
          as="button"
          class="mb-1 flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          title={t("storage.newTab")}
        >
          <Plus class="h-4 w-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuLabel>{t("storage.pickConnection")}</DropdownMenuLabel>
          <Show
            when={connections().length}
            fallback={<div class="px-2 py-1.5 text-sm text-muted-foreground">{t("connections.none")}</div>}
          >
            <For each={connections()}>
              {(c) => (
                <DropdownMenuItem onSelect={() => openTab(c.id, c.name)}>
                  <Database class="h-4 w-4" />
                  {c.name}
                </DropdownMenuItem>
              )}
            </For>
          </Show>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};
