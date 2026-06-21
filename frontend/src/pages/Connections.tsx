import { createSignal, For, Show, onMount, onCleanup, type Component } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { Plus, Pencil, Trash2, Database, ExternalLink, Globe, ShieldCheck } from "lucide-solid";
import { PageHeader } from "~/components/PageHeader";
import { Button } from "~/components/ui/button";
import { Card } from "~/components/ui/primitives";
import { Badge } from "~/components/ui/primitives";
import { ConnectionForm } from "~/features/connections/ConnectionForm";
import { CapabilitiesDialog } from "~/features/storage/CapabilitiesDialog";
import { ConfirmDialog } from "~/components/ConfirmDialog";
import { connections, deleteConnection } from "~/stores/connections";
import { openTab } from "~/stores/tabs";
import { type Connection } from "~/lib/api";
import { toast } from "~/components/ui/toast";
import { formatDate } from "~/lib/utils";
import { effectiveLocale } from "~/stores/settings";
import { t } from "~/i18n";
import * as bus from "~/lib/bus";

const Connections: Component = () => {
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = createSignal(false);
  const [editing, setEditing] = createSignal<Connection | null>(null);
  const [confirmDel, setConfirmDel] = createSignal<Connection | null>(null);
  const [capsConn, setCapsConn] = createSignal<Connection | null>(null);

  const add = () => {
    setEditing(null);
    setFormOpen(true);
  };
  const edit = (c: Connection) => {
    setEditing(c);
    setFormOpen(true);
  };

  const open = (c: Connection) => {
    openTab(c.id, c.name);
    navigate("/storage");
  };

  const doDelete = async () => {
    const c = confirmDel();
    if (!c) return;
    try {
      await deleteConnection(c.id);
      toast.success(t("common.success"));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setConfirmDel(null);
    }
  };

  // "New connection" menu shortcut.
  onMount(() => {
    const off = bus.on("new-connection", add);
    onCleanup(off);
  });

  return (
    <div class="flex h-full flex-col">
      <PageHeader title={t("connections.title")} subtitle={t("connections.subtitle")}>
        <Button onClick={add}>
          <Plus class="h-4 w-4" />
          {t("connections.add")}
        </Button>
      </PageHeader>

      <div class="flex-1 overflow-y-auto p-6">
        <Show
          when={connections().length}
          fallback={
            <div class="flex h-full flex-col items-center justify-center gap-4 text-center text-muted-foreground">
              <Database class="h-12 w-12 opacity-40" />
              <p>{t("connections.none")}</p>
              <Button onClick={add} variant="outline">
                <Plus class="h-4 w-4" />
                {t("connections.add")}
              </Button>
            </div>
          }
        >
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <For each={connections()}>
              {(c) => (
                <Card class="group flex flex-col gap-3 p-4 transition-colors hover:border-primary/40">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <div class="flex items-center gap-2">
                        <span class="truncate font-medium">{c.name}</span>
                        <Badge variant="secondary">
                          {c.pathStyle ? t("connections.pathStyle") : t("connections.virtualHosted")}
                        </Badge>
                      </div>
                      <div class="mt-1 flex items-center gap-1.5 truncate text-xs text-muted-foreground">
                        <Globe class="h-3 w-3 shrink-0" />
                        <span class="truncate">{c.endpoint}</span>
                      </div>
                    </div>
                  </div>

                  <div class="text-xs text-muted-foreground">
                    {c.region ? `${c.region} · ` : ""}
                    {formatDate(c.createdAt, effectiveLocale())}
                  </div>

                  <div class="mt-auto flex items-center gap-2 pt-2">
                    <Button size="sm" onClick={() => open(c)} class="flex-1">
                      <ExternalLink class="h-3.5 w-3.5" />
                      {t("connections.open")}
                    </Button>
                    <Button
                      size="icon-sm"
                      variant="outline"
                      onClick={() => setCapsConn(c)}
                      title={t("capabilities.title")}
                    >
                      <ShieldCheck class="h-3.5 w-3.5" />
                    </Button>
                    <Button size="icon-sm" variant="outline" onClick={() => edit(c)}>
                      <Pencil class="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="icon-sm"
                      variant="outline"
                      onClick={() => setConfirmDel(c)}
                      class="text-destructive hover:text-destructive"
                    >
                      <Trash2 class="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </Card>
              )}
            </For>
          </div>
        </Show>
      </div>

      <ConnectionForm open={formOpen()} onOpenChange={setFormOpen} edit={editing()} />
      <Show when={capsConn()}>
        <CapabilitiesDialog
          open
          onOpenChange={(o) => !o && setCapsConn(null)}
          connId={capsConn()!.id}
          bucket=""
        />
      </Show>
      <ConfirmDialog
        open={!!confirmDel()}
        onOpenChange={(o) => !o && setConfirmDel(null)}
        title={t("connections.deleteTitle")}
        message={t("connections.deleteMessage", { name: confirmDel()?.name ?? "" })}
        confirmText={t("common.delete")}
        destructive
        onConfirm={doDelete}
      />
    </div>
  );
};

export default Connections;
