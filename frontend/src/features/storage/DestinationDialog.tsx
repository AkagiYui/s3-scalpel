import { createSignal, createEffect, type Component } from "solid-js";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Label } from "~/components/ui/primitives";
import { SimpleSelect } from "~/components/ui/select";
import { type BucketInfo } from "~/lib/api";
import { t } from "~/i18n";

/** Choose a destination bucket + prefix for a copy or move operation. */
export const DestinationDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  move: boolean;
  count: number;
  buckets: BucketInfo[];
  defaultBucket: string;
  defaultPrefix: string;
  onConfirm: (destBucket: string, destPrefix: string) => void;
}> = (props) => {
  const [bucket, setBucket] = createSignal("");
  const [prefix, setPrefix] = createSignal("");

  createEffect(() => {
    if (props.open) {
      setBucket(props.defaultBucket);
      setPrefix(props.defaultPrefix);
    }
  });

  const confirm = () => {
    if (!bucket()) return;
    let p = prefix().trim().replace(/^\/+/, "");
    if (p && !p.endsWith("/")) p += "/";
    props.onConfirm(bucket(), p);
    props.onOpenChange(false);
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{props.move ? t("storage.moveTo") : t("storage.copyTo")}</DialogTitle>
          <DialogDescription>
            {props.move
              ? t("storage.batchMoveMessage", { count: props.count, dest: bucket() })
              : t("storage.batchCopyMessage", { count: props.count, dest: bucket() })}
          </DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <div class="grid gap-1.5">
            <Label>{t("storage.chooseDestBucket")}</Label>
            <SimpleSelect
              value={bucket()}
              onChange={setBucket}
              options={props.buckets.map((b) => ({ value: b.name, label: b.name }))}
            />
          </div>
          <div class="grid gap-1.5">
            <Label>{t("storage.chooseDestPrefix")}</Label>
            <Input
              value={prefix()}
              placeholder="path/to/folder/"
              onInput={(e) => setPrefix(e.currentTarget.value)}
            />
          </div>
        </div>
        <DialogFooter class="mt-2">
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={confirm} disabled={!bucket()}>
            {props.move ? t("queue.type.move") : t("queue.type.copy")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
