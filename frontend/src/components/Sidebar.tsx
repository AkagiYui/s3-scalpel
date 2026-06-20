import { type Component, type JSX } from "solid-js";
import { A } from "@solidjs/router";
import { Database, HardDrive, Settings as SettingsIcon, Scissors } from "lucide-solid";
import { t } from "~/i18n";
import { cn } from "~/lib/utils";

const NavItem: Component<{ href: string; icon: JSX.Element; label: string; end?: boolean }> = (
  props
) => (
  <A
    href={props.href}
    end={props.end}
    class={cn(
      "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors no-drag",
      "hover:bg-accent hover:text-accent-foreground"
    )}
    activeClass="!bg-accent !text-accent-foreground"
  >
    {props.icon}
    <span>{props.label}</span>
  </A>
);

export const Sidebar: Component = () => {
  return (
    <aside class="flex w-56 shrink-0 flex-col border-r bg-card">
      {/* Title region doubles as the draggable inset title bar on macOS. */}
      <div class="drag flex h-14 items-center gap-2 px-4">
        <Scissors class="h-5 w-5 text-primary" />
        <span class="font-semibold tracking-tight">{t("app.name")}</span>
      </div>

      <nav class="flex flex-1 flex-col gap-1 p-2">
        <NavItem
          href="/"
          end
          icon={<Database class="h-4 w-4" />}
          label={t("nav.connections")}
        />
        <NavItem href="/storage" icon={<HardDrive class="h-4 w-4" />} label={t("nav.storage")} />
      </nav>

      <div class="border-t p-2">
        <NavItem
          href="/settings"
          icon={<SettingsIcon class="h-4 w-4" />}
          label={t("nav.settings")}
        />
      </div>
    </aside>
  );
};
