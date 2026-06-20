import { createResource, Show, For, type Component } from "solid-js";
import { Link, Clock } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Badge, Spinner } from "~/components/ui/primitives";
import { S3Service } from "~/lib/api";
import { formatBytes, formatDate } from "~/lib/utils";
import { effectiveLocale } from "~/stores/settings";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

export const VersionsDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
  objKey: string;
}> = (props) => {
  const [info] = createResource(
    () => (props.open ? [props.connId, props.bucket, props.objKey] : null),
    async () => {
      const enabled = await S3Service.VersioningEnabled(props.connId, props.bucket);
      if (!enabled) return { enabled: false, versions: [] };
      const versions = await S3Service.ListVersions(props.connId, props.bucket, props.objKey);
      return { enabled: true, versions: versions ?? [] };
    }
  );

  const presign = async (versionId: string) => {
    try {
      const url = await S3Service.PresignGet(props.connId, props.bucket, props.objKey, versionId, 3600);
      await navigator.clipboard.writeText(url);
      toast.success(t("common.copied"));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{t("versions.title")}</DialogTitle>
        </DialogHeader>
        <Show when={!info.loading} fallback={<div class="flex justify-center py-8"><Spinner class="h-6 w-6" /></div>}>
          <Show when={info()?.enabled} fallback={<div class="py-6 text-center text-sm text-muted-foreground">{t("versions.notEnabled")}</div>}>
            <Show when={info()!.versions.length} fallback={<div class="py-6 text-center text-sm text-muted-foreground">{t("versions.empty")}</div>}>
              <div class="max-h-[60vh] divide-y overflow-y-auto">
                <For each={info()!.versions}>
                  {(v) => (
                    <div class="flex items-center justify-between gap-3 py-2">
                      <div class="min-w-0">
                        <div class="flex items-center gap-2">
                          <Clock class="h-3.5 w-3.5 text-muted-foreground" />
                          <span class="text-sm">{formatDate(v.lastModified, effectiveLocale())}</span>
                          <Show when={v.isLatest}>
                            <Badge variant="success">{t("versions.current")}</Badge>
                          </Show>
                          <Show when={v.isDeleteMarker}>
                            <Badge variant="destructive">{t("versions.deleteMarker")}</Badge>
                          </Show>
                        </div>
                        <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                          {v.versionId} {v.isDeleteMarker ? "" : `· ${formatBytes(v.size)}`}
                        </div>
                      </div>
                      <Show when={!v.isDeleteMarker}>
                        <Button variant="outline" size="sm" onClick={() => presign(v.versionId)}>
                          <Link class="h-3.5 w-3.5" />
                          {t("storage.presign")}
                        </Button>
                      </Show>
                    </div>
                  )}
                </For>
              </div>
            </Show>
          </Show>
        </Show>
      </DialogContent>
    </Dialog>
  );
};
