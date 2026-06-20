import { createSignal, createEffect, type Component } from "solid-js";
import { Folder } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Label, Checkbox } from "~/components/ui/primitives";
import { AppService, windowID } from "~/lib/api";
import { settings } from "~/stores/settings";
import { t } from "~/i18n";

/** Choose a download destination folder, with an option to remember it. */
export const DownloadDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  count: number;
  onConfirm: (destDir: string, setDefault: boolean) => void;
}> = (props) => {
  const [dir, setDir] = createSignal("");
  const [setDefault, setSetDefault] = createSignal(false);

  createEffect(() => {
    if (props.open) {
      setDir(settings().defaultDownloadDir || "");
      setSetDefault(false);
    }
  });

  const choose = async () => {
    const picked = await AppService.PickDirectory(windowID(), t("storage.downloadTo"));
    if (picked) setDir(picked);
  };

  const confirm = () => {
    if (!dir()) return;
    props.onConfirm(dir(), setDefault());
    props.onOpenChange(false);
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{t("storage.downloadTo")}</DialogTitle>
          <DialogDescription>{t("storage.batchDownloadMessage", { count: props.count })}</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <Label>{t("settings.defaultDownloadDir")}</Label>
          <div class="flex gap-2">
            <Input class="flex-1" readOnly value={dir()} placeholder={t("settings.defaultDownloadDirNotSet")} />
            <Button variant="outline" onClick={choose}>
              <Folder class="h-4 w-4" />
              {t("settings.chooseFolder")}
            </Button>
          </div>
          <label class="flex items-center gap-2 text-sm">
            <Checkbox checked={setDefault()} onChange={setSetDefault} />
            {t("storage.setDefaultDownloadDir")}
          </label>
        </div>
        <DialogFooter class="mt-2">
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={confirm} disabled={!dir()}>
            {t("common.download")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
