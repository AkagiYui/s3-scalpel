import { createResource, Show, For, type Component } from "solid-js";
import { Copy } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "~/components/ui/dialog";
import { Spinner } from "~/components/ui/primitives";
import { S3Service } from "~/lib/api";
import { formatBytes, formatDate } from "~/lib/utils";
import { effectiveLocale } from "~/stores/settings";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(t("common.copied"));
  } catch {
    /* ignore */
  }
}

const Field: Component<{ label: string; value?: string; mono?: boolean; copyable?: boolean }> = (props) => (
  <Show when={props.value}>
    <div class="grid grid-cols-[8rem_1fr] gap-2 py-1.5 text-sm">
      <span class="text-muted-foreground">{props.label}</span>
      <div class="flex items-start gap-1.5">
        <span class={`selectable break-all ${props.mono ? "font-mono text-xs" : ""}`}>{props.value}</span>
        <Show when={props.copyable}>
          <button class="shrink-0 text-muted-foreground hover:text-foreground" onClick={() => copy(props.value!)}>
            <Copy class="h-3 w-3" />
          </button>
        </Show>
      </div>
    </div>
  </Show>
);

export const PropertiesDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
  objKey: string;
}> = (props) => {
  const [data] = createResource(
    () => (props.open ? [props.connId, props.bucket, props.objKey] : null),
    async () => S3Service.Properties(props.connId, props.bucket, props.objKey, "")
  );

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{t("properties.title")}</DialogTitle>
        </DialogHeader>
        <Show when={!data.loading} fallback={<div class="flex justify-center py-8"><Spinner class="h-6 w-6" /></div>}>
          <Show when={data()}>
            {(p) => (
              <div class="divide-y">
                <Field label={t("properties.key")} value={p().key} mono copyable />
                <Field label={t("properties.size")} value={`${formatBytes(p().size)} (${p().size} B)`} />
                <Field label={t("properties.contentType")} value={p().contentType} />
                <Field label={t("properties.etag")} value={p().etag} mono copyable />
                <Field label={t("properties.lastModified")} value={formatDate(p().lastModified, effectiveLocale())} />
                <Field label={t("properties.storageClass")} value={p().storageClass} />
                <Field label={t("properties.versionId")} value={p().versionId} mono />
                <Field label={t("properties.cacheControl")} value={p().cacheControl} />
                <Field label={t("properties.contentEncoding")} value={p().contentEncoding} />
                <div class="py-2">
                  <div class="mb-1 text-sm text-muted-foreground">{t("properties.metadata")}</div>
                  <Show
                    when={p().metadata && Object.keys(p().metadata).length}
                    fallback={<div class="text-xs text-muted-foreground">{t("properties.noMetadata")}</div>}
                  >
                    <div class="rounded-md bg-muted/50 p-2 font-mono text-xs">
                      <For each={Object.entries(p().metadata)}>
                        {([k, v]) => (
                          <div class="selectable">
                            {k}: {v}
                          </div>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>
              </div>
            )}
          </Show>
        </Show>
      </DialogContent>
    </Dialog>
  );
};
