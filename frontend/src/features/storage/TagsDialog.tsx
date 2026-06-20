import { createSignal, createEffect, For, Show, type Component } from "solid-js";
import { Plus, Trash2 } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "~/components/ui/dialog";
import { DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Spinner } from "~/components/ui/primitives";
import { S3Service, type Tag } from "~/lib/api";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

export const TagsDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
  objKey: string;
}> = (props) => {
  const [tags, setTags] = createSignal<Tag[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);

  createEffect(() => {
    if (props.open) load();
  });

  const load = async () => {
    setLoading(true);
    try {
      const result = await S3Service.GetTags(props.connId, props.bucket, props.objKey);
      setTags((result ?? []).map((x) => ({ key: x.key, value: x.value }) as Tag));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setLoading(false);
    }
  };

  const setRow = (i: number, patch: Partial<Tag>) =>
    setTags((prev) => prev.map((tg, idx) => (idx === i ? ({ ...tg, ...patch } as Tag) : tg)));
  const addRow = () => setTags((prev) => [...prev, { key: "", value: "" } as Tag]);
  const removeRow = (i: number) => setTags((prev) => prev.filter((_, idx) => idx !== i));

  const save = async () => {
    setSaving(true);
    try {
      const clean = tags().filter((tg) => tg.key.trim());
      await S3Service.PutTags(props.connId, props.bucket, props.objKey, clean);
      toast.success(t("tags.saved"));
      props.onOpenChange(false);
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{t("tags.title")}</DialogTitle>
        </DialogHeader>
        <Show when={!loading()} fallback={<div class="flex justify-center py-6"><Spinner class="h-6 w-6" /></div>}>
          <div class="flex flex-col gap-2">
            <Show when={tags().length} fallback={<div class="py-2 text-sm text-muted-foreground">{t("tags.empty")}</div>}>
              <div class="grid grid-cols-[1fr_1fr_auto] gap-2 text-xs text-muted-foreground">
                <span>{t("tags.key")}</span>
                <span>{t("tags.value")}</span>
                <span />
              </div>
              <For each={tags()}>
                {(tg, i) => (
                  <div class="grid grid-cols-[1fr_1fr_auto] gap-2">
                    <Input value={tg.key} onInput={(e) => setRow(i(), { key: e.currentTarget.value })} />
                    <Input value={tg.value} onInput={(e) => setRow(i(), { value: e.currentTarget.value })} />
                    <Button variant="ghost" size="icon" onClick={() => removeRow(i())}>
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                )}
              </For>
            </Show>
            <Button variant="outline" size="sm" class="mt-1 self-start" onClick={addRow}>
              <Plus class="h-4 w-4" />
              {t("tags.add")}
            </Button>
          </div>
        </Show>
        <DialogFooter class="mt-2">
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={save} disabled={saving()}>
            {saving() ? t("common.saving") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
