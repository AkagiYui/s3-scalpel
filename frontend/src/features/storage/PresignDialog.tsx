import { createSignal, createEffect, Show, type Component } from "solid-js";
import { Copy, Link } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Label, Spinner } from "~/components/ui/primitives";
import { SimpleSelect } from "~/components/ui/select";
import { S3Service } from "~/lib/api";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

export const PresignDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
  objKey: string;
  versionId?: string;
}> = (props) => {
  const [amount, setAmount] = createSignal(1);
  const [unit, setUnit] = createSignal<"minutes" | "hours" | "days">("hours");
  const [method, setMethod] = createSignal<"GET" | "PUT">("GET");
  const [url, setUrl] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  createEffect(() => {
    if (props.open) {
      setUrl("");
      setAmount(1);
      setUnit("hours");
      setMethod("GET");
    }
  });

  const seconds = () => {
    const a = Math.max(1, amount());
    return unit() === "minutes" ? a * 60 : unit() === "hours" ? a * 3600 : a * 86400;
  };

  const generate = async () => {
    setBusy(true);
    try {
      const u =
        method() === "PUT"
          ? await S3Service.PresignPut(props.connId, props.bucket, props.objKey, "", seconds())
          : await S3Service.PresignGet(props.connId, props.bucket, props.objKey, props.versionId ?? "", seconds());
      setUrl(u);
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const copy = async () => {
    await navigator.clipboard.writeText(url());
    toast.success(t("common.copied"));
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{t("presign.title")}</DialogTitle>
          <DialogDescription>{t("presign.note")}</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <Label>{t("presign.method")}</Label>
          <SimpleSelect
            class="w-48"
            value={method()}
            onChange={(v) => setMethod(v as any)}
            options={[
              { value: "GET", label: t("presign.methodGet") },
              { value: "PUT", label: t("presign.methodPut") },
            ]}
          />
          <Label>{t("presign.expiry")}</Label>
          <div class="flex gap-2">
            <Input
              type="number"
              min={1}
              class="w-24"
              value={amount()}
              onInput={(e) => setAmount(Number(e.currentTarget.value) || 1)}
            />
            <SimpleSelect
              class="w-32"
              value={unit()}
              onChange={(v) => setUnit(v as any)}
              options={[
                { value: "minutes", label: t("presign.minutes") },
                { value: "hours", label: t("presign.hours") },
                { value: "days", label: t("presign.days") },
              ]}
            />
            <Button onClick={generate} disabled={busy()}>
              <Show when={busy()} fallback={<Link class="h-4 w-4" />}>
                <Spinner class="h-4 w-4" />
              </Show>
              {t("presign.generate")}
            </Button>
          </div>

          <Show when={url()}>
            <Label>{t("presign.result")}</Label>
            <div class="flex gap-2">
              <Input class="flex-1 font-mono text-xs selectable" readOnly value={url()} />
              <Button variant="outline" size="icon" onClick={copy}>
                <Copy class="h-4 w-4" />
              </Button>
            </div>
          </Show>
        </div>
        <DialogFooter class="mt-2">
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
