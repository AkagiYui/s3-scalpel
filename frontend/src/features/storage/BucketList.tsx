import { createResource, createSignal, For, Show, type Component } from "solid-js";
import { Plus, RefreshCw, Trash2, FolderOpen, Database, Box } from "lucide-solid";
import { Button } from "~/components/ui/button";
import { Card, Spinner } from "~/components/ui/primitives";
import { PromptDialog } from "./PromptDialog";
import { ConfirmDialog } from "~/components/ConfirmDialog";
import { S3Service, type BucketInfo } from "~/lib/api";
import { formatDate } from "~/lib/utils";
import { effectiveLocale } from "~/stores/settings";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

export const BucketList: Component<{
  connId: string;
  onOpen: (bucket: string) => void;
}> = (props) => {
  const [buckets, { refetch }] = createResource(
    () => props.connId,
    async (connId) => {
      try {
        return (await S3Service.ListBuckets(connId)) ?? [];
      } catch (e: any) {
        toast.error(t("errors.loadBuckets") + " " + String(e?.message ?? e));
        return [] as BucketInfo[];
      }
    }
  );

  const [createOpen, setCreateOpen] = createSignal(false);
  const [delTarget, setDelTarget] = createSignal<string | null>(null);

  const create = async (name: string) => {
    try {
      await S3Service.CreateBucket(props.connId, name);
      toast.success(t("common.success"));
      refetch();
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const doDelete = async () => {
    const name = delTarget();
    if (!name) return;
    try {
      await S3Service.DeleteBucket(props.connId, name);
      toast.success(t("common.success"));
      refetch();
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setDelTarget(null);
    }
  };

  return (
    <div class="flex h-full flex-col">
      <div class="flex items-center justify-between border-b px-4 py-2">
        <div class="flex items-center gap-2 text-sm font-medium">
          <Database class="h-4 w-4" />
          {t("storage.buckets")}
        </div>
        <div class="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => refetch()}>
            <RefreshCw class="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus class="h-3.5 w-3.5" />
            {t("storage.createBucket")}
          </Button>
        </div>
      </div>

      <div class="flex-1 overflow-y-auto p-4">
        <Show when={!buckets.loading} fallback={<div class="flex justify-center py-12"><Spinner class="h-6 w-6" /></div>}>
          <Show
            when={buckets()?.length}
            fallback={
              <div class="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
                <Box class="h-12 w-12 opacity-40" />
                <p>{t("storage.noBuckets")}</p>
              </div>
            }
          >
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
              <For each={buckets()}>
                {(b) => (
                  <Card class="group flex items-center gap-3 p-3 transition-colors hover:border-primary/40">
                    <div
                      class="flex min-w-0 flex-1 cursor-pointer items-center gap-3"
                      onClick={() => props.onOpen(b.name)}
                    >
                      <Box class="h-5 w-5 shrink-0 text-primary" />
                      <div class="min-w-0">
                        <div class="truncate font-medium">{b.name}</div>
                        <Show when={b.createdAt}>
                          <div class="text-xs text-muted-foreground">
                            {formatDate(b.createdAt, effectiveLocale())}
                          </div>
                        </Show>
                      </div>
                    </div>
                    <Button size="icon-sm" variant="ghost" onClick={() => props.onOpen(b.name)}>
                      <FolderOpen class="h-4 w-4" />
                    </Button>
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      class="text-destructive"
                      onClick={() => setDelTarget(b.name)}
                    >
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </Card>
                )}
              </For>
            </div>
          </Show>
        </Show>
      </div>

      <PromptDialog
        open={createOpen()}
        onOpenChange={setCreateOpen}
        title={t("storage.createBucketTitle")}
        placeholder={t("storage.bucketName")}
        onSubmit={create}
      />
      <ConfirmDialog
        open={!!delTarget()}
        onOpenChange={(o) => !o && setDelTarget(null)}
        title={t("storage.deleteBucketTitle")}
        message={t("storage.deleteBucketMessage", { name: delTarget() ?? "" })}
        confirmText={t("common.delete")}
        destructive
        onConfirm={doDelete}
      />
    </div>
  );
};
