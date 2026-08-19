import { createSignal, createEffect, For, Show, type Component } from "solid-js";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Spinner } from "~/components/ui/primitives";
import { S3Service, type PrefixStats } from "~/lib/api";
import { formatBytes } from "~/lib/utils";
import { toast } from "~/components/ui/toast";
import { createCancellableOp } from "~/lib/operation";
import { t } from "~/i18n";

export const StatsDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
  prefix: string;
}> = (props) => {
  const [stats, setStats] = createSignal<PrefixStats | null>(null);
  const op = createCancellableOp();
  const loading = op.running;

  createEffect(() => {
    if (props.open) load();
  });

  const load = async () => {
    setStats(null);
    try {
      const res = await op.run((opID) =>
        S3Service.Stats(opID, props.connId, props.bucket, props.prefix)
      );
      if (res) setStats(res as PrefixStats);
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const classes = () => {
    const s = stats();
    if (!s?.byStorageClass) return [];
    return Object.keys(s.byStorageClass).map((k) => ({
      name: k,
      count: s.byStorageClass[k]?.count ?? 0,
      size: s.byStorageClass[k]?.size ?? 0,
    }));
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{t("stats.title")}</DialogTitle>
        </DialogHeader>
        <div class="truncate font-mono text-xs text-muted-foreground">
          {props.bucket}/{props.prefix || ""}
        </div>
        <Show
          when={!loading()}
          fallback={
            <div class="flex flex-col items-center gap-3 py-10">
              <Spinner class="h-6 w-6" />
              <Button size="sm" variant="outline" onClick={op.cancel}>
                {t("common.cancel")}
              </Button>
            </div>
          }
        >
          <Show when={stats()}>
            <div class="flex flex-col gap-3 py-1">
              <div class="grid grid-cols-2 gap-3">
                <div class="rounded-md border p-3">
                  <div class="text-xs text-muted-foreground">{t("stats.objectCount")}</div>
                  <div class="text-lg font-semibold">{stats()!.objectCount.toLocaleString()}</div>
                </div>
                <div class="rounded-md border p-3">
                  <div class="text-xs text-muted-foreground">{t("stats.totalSize")}</div>
                  <div class="text-lg font-semibold">{formatBytes(stats()!.totalSize)}</div>
                </div>
              </div>
              <Show when={classes().length}>
                <div>
                  <div class="mb-1 text-xs text-muted-foreground">{t("stats.byStorageClass")}</div>
                  <div class="flex flex-col gap-1">
                    <For each={classes()}>
                      {(c) => (
                        <div class="grid grid-cols-[1fr_5rem_6rem] gap-2 rounded border px-2 py-1 text-xs">
                          <span class="font-mono">{c.name}</span>
                          <span class="text-right text-muted-foreground">
                            {c.count.toLocaleString()}
                          </span>
                          <span class="text-right">{formatBytes(c.size)}</span>
                        </div>
                      )}
                    </For>
                  </div>
                </div>
              </Show>
            </div>
          </Show>
        </Show>
        <DialogFooter>
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
