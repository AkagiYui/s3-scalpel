import { createResource, Show, Switch, Match, type Component } from "solid-js";
import { ExternalLink, FileQuestion } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Spinner } from "~/components/ui/primitives";
import { PreviewService, S3Service, AppService } from "~/lib/api";
import { keyBasename } from "~/lib/utils";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

export const PreviewDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
  objKey: string;
}> = (props) => {
  const [data] = createResource(
    () => (props.open ? [props.connId, props.bucket, props.objKey] : null),
    async () => PreviewService.GetPreview(props.connId, props.bucket, props.objKey)
  );

  const openExternal = async () => {
    try {
      const url = await S3Service.PresignGet(props.connId, props.bucket, props.objKey, "", 3600);
      await AppService.OpenURL(url);
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="xl">
        <DialogHeader>
          <DialogTitle class="truncate pr-8">{keyBasename(props.objKey)}</DialogTitle>
        </DialogHeader>
        <div class="flex min-h-[200px] items-center justify-center">
          <Show when={!data.loading} fallback={<div class="flex flex-col items-center gap-2 text-muted-foreground"><Spinner class="h-6 w-6" />{t("preview.loading")}</div>}>
            <Show when={data()}>
              {(d) => (
                <Switch>
                  <Match when={d().kind === "image"}>
                    <img src={d().dataUrl} alt="" class="max-h-[70vh] max-w-full rounded object-contain" />
                  </Match>
                  <Match when={d().kind === "text"}>
                    <pre class="selectable max-h-[70vh] w-full overflow-auto rounded-md bg-muted/50 p-3 text-xs">{d().text}</pre>
                  </Match>
                  <Match when={d().kind === "pdf"}>
                    <iframe src={d().dataUrl} class="h-[72vh] w-full rounded border" title="pdf" />
                  </Match>
                  <Match when={d().kind === "media" && d().contentType.startsWith("video")}>
                    <video src={d().url} controls class="max-h-[70vh] max-w-full rounded" />
                  </Match>
                  <Match when={d().kind === "media"}>
                    <audio src={d().url} controls class="w-full" />
                  </Match>
                  <Match when={d().kind === "too-large" || d().kind === "unsupported"}>
                    <div class="flex flex-col items-center gap-4 py-8 text-center text-muted-foreground">
                      <FileQuestion class="h-12 w-12 opacity-40" />
                      <p>{d().kind === "too-large" ? t("preview.tooLarge") : t("preview.unsupported")}</p>
                      <Button variant="outline" onClick={openExternal}>
                        <ExternalLink class="h-4 w-4" />
                        {t("preview.openExternal")}
                      </Button>
                    </div>
                  </Match>
                </Switch>
              )}
            </Show>
          </Show>
        </div>
      </DialogContent>
    </Dialog>
  );
};
