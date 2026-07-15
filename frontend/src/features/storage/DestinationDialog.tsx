import { createSignal, createEffect, Show, type Component } from "solid-js";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Label } from "~/components/ui/primitives";
import { SimpleSelect } from "~/components/ui/select";
import { S3Service, type BucketInfo } from "~/lib/api";
import { connections } from "~/stores/connections";
import { t } from "~/i18n";

/** Choose a destination connection + bucket + prefix for a copy or move. */
export const DestinationDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  move: boolean;
  count: number;
  buckets: BucketInfo[];
  currentConnId: string;
  defaultBucket: string;
  defaultPrefix: string;
  onConfirm: (destConnId: string, destBucket: string, destPrefix: string) => void;
}> = (props) => {
  const [connId, setConnId] = createSignal("");
  const [bucket, setBucket] = createSignal("");
  const [prefix, setPrefix] = createSignal("");
  const [buckets, setBuckets] = createSignal<BucketInfo[]>([]);

  createEffect(() => {
    if (props.open) {
      setConnId(props.currentConnId);
      setBuckets(props.buckets);
      setBucket(props.defaultBucket);
      setPrefix(props.defaultPrefix);
    }
  });

  // When the destination connection changes to a different one, load its buckets.
  const onConnChange = async (id: string) => {
    setConnId(id);
    if (id === props.currentConnId) {
      setBuckets(props.buckets);
      setBucket(props.defaultBucket);
      return;
    }
    try {
      const list = (await S3Service.ListBuckets(id)) ?? [];
      setBuckets(list);
      setBucket(list[0]?.name ?? "");
    } catch {
      setBuckets([]);
      setBucket("");
    }
  };

  const confirm = () => {
    if (!bucket()) return;
    let p = prefix().trim().replace(/^\/+/, "");
    if (p && !p.endsWith("/")) p += "/";
    props.onConfirm(connId(), bucket(), p);
    props.onOpenChange(false);
  };

  const crossConn = () => connId() !== props.currentConnId;

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
            <Label>{t("storage.chooseDestConn")}</Label>
            <SimpleSelect
              value={connId()}
              onChange={onConnChange}
              options={connections().map((c) => ({ value: c.id, label: c.name }))}
            />
            <Show when={crossConn() && props.move}>
              <p class="text-xs text-amber-500">{t("storage.crossConnMoveHint")}</p>
            </Show>
          </div>
          <div class="grid gap-1.5">
            <Label>{t("storage.chooseDestBucket")}</Label>
            <SimpleSelect
              value={bucket()}
              onChange={setBucket}
              options={buckets().map((b) => ({ value: b.name, label: b.name }))}
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
