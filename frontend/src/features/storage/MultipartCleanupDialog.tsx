import { createSignal, createEffect, For, Show, type Component } from "solid-js";
import { Trash2, RefreshCw } from "lucide-solid";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Checkbox, Spinner } from "~/components/ui/primitives";
import { BucketService, type MultipartUpload } from "~/lib/api";
import { formatBytes, formatDate } from "~/lib/utils";
import { effectiveLocale } from "~/stores/settings";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

/**
 * Lists the multipart uploads a bucket started but never finished, and lets the
 * user abort them. Nothing cleans these up on its own — an interrupted client
 * leaves the parts behind, and providers bill for them until they are aborted.
 */
export const MultipartCleanupDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
}> = (props) => {
  const [uploads, setUploads] = createSignal<MultipartUpload[]>([]);
  const [selected, setSelected] = createSignal<Set<string>>(new Set<string>());
  const [loading, setLoading] = createSignal(false);
  const [aborting, setAborting] = createSignal(false);

  createEffect(() => {
    if (props.open) load();
  });

  const load = async () => {
    setLoading(true);
    setSelected(new Set<string>());
    try {
      const list = await BucketService.ListMultipartUploads(props.connId, props.bucket, "");
      setUploads(list ?? []);
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
      setUploads([]);
    } finally {
      setLoading(false);
    }
  };

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const allSelected = () => uploads().length > 0 && selected().size === uploads().length;

  const toggleAll = () =>
    setSelected(allSelected() ? new Set<string>() : new Set(uploads().map((u) => u.uploadId)));

  const totalWasted = () =>
    uploads()
      .filter((u) => selected().has(u.uploadId))
      .reduce((sum, u) => sum + u.size, 0);

  const abort = async () => {
    const picked = uploads().filter((u) => selected().has(u.uploadId));
    if (!picked.length) return;
    setAborting(true);
    try {
      const n = await BucketService.AbortMultipartUploads(props.connId, props.bucket, picked);
      toast.success(t("multipart.aborted", { count: n }));
      await load();
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setAborting(false);
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent class="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t("multipart.title")}</DialogTitle>
          <DialogDescription>{t("multipart.description")}</DialogDescription>
        </DialogHeader>

        <Show
          when={!loading()}
          fallback={
            <div class="flex justify-center py-12">
              <Spinner class="h-6 w-6" />
            </div>
          }
        >
          <Show
            when={uploads().length}
            fallback={
              <div class="py-12 text-center text-sm text-muted-foreground">
                {t("multipart.empty")}
              </div>
            }
          >
            <div class="max-h-96 overflow-y-auto rounded-md border">
              <div class="grid grid-cols-[2rem_1fr_6rem_5rem_11rem] items-center gap-2 border-b bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                <Checkbox
                  aria-label={t("storage.selectAll")}
                  checked={allSelected()}
                  onChange={toggleAll}
                />
                <span>{t("properties.key")}</span>
                <span class="text-right">{t("properties.size")}</span>
                <span class="text-right">{t("multipart.parts")}</span>
                <span>{t("multipart.initiated")}</span>
              </div>
              <For each={uploads()}>
                {(u) => (
                  <div class="grid grid-cols-[2rem_1fr_6rem_5rem_11rem] items-center gap-2 border-b px-3 py-1.5 text-sm last:border-b-0">
                    <Checkbox
                      aria-label={u.key}
                      checked={selected().has(u.uploadId)}
                      onChange={() => toggle(u.uploadId)}
                    />
                    <span class="truncate" title={u.key}>
                      {u.key}
                    </span>
                    <span class="text-right text-muted-foreground">{formatBytes(u.size)}</span>
                    <span class="text-right text-muted-foreground">{u.partCount}</span>
                    <span class="text-muted-foreground">
                      {formatDate(u.initiated, effectiveLocale())}
                    </span>
                  </div>
                )}
              </For>
            </div>
            <Show when={selected().size > 0}>
              <p class="text-sm text-muted-foreground">
                {t("multipart.selectedSummary", {
                  count: selected().size,
                  size: formatBytes(totalWasted()),
                })}
              </p>
            </Show>
          </Show>
        </Show>

        <DialogFooter>
          <Button variant="outline" onClick={load} disabled={loading()}>
            <RefreshCw class="h-4 w-4" />
            {t("common.refresh")}
          </Button>
          <Button variant="destructive" onClick={abort} disabled={!selected().size || aborting()}>
            <Trash2 class="h-4 w-4" />
            {t("multipart.abortSelected")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
