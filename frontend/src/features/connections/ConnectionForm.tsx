import { createSignal, createEffect, Show, type Component } from "solid-js";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Label } from "~/components/ui/primitives";
import { SimpleSelect } from "~/components/ui/select";
import { Spinner } from "~/components/ui/primitives";
import { CheckCircle2, XCircle } from "lucide-solid";
import { ConfigService, type Connection, type TestResult } from "~/lib/api";
import { saveConnection } from "~/stores/connections";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

function blank(): Connection {
  return {
    id: "",
    name: "",
    endpoint: "",
    region: "",
    pathStyle: true,
    accessKey: "",
    secretKey: "",
    createdAt: 0,
  } as Connection;
}

export const ConnectionForm: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  edit?: Connection | null;
}> = (props) => {
  const [form, setForm] = createSignal<Connection>(blank());
  const [testing, setTesting] = createSignal(false);
  const [testRes, setTestRes] = createSignal<TestResult | null>(null);
  const [saving, setSaving] = createSignal(false);

  // Reset the form whenever the dialog opens.
  createEffect(() => {
    if (props.open) {
      setForm(props.edit ? { ...props.edit } : blank());
      setTestRes(null);
    }
  });

  const set = <K extends keyof Connection>(key: K, value: Connection[K]) =>
    setForm((f) => ({ ...f, [key]: value }));

  const valid = () => form().name.trim() && form().endpoint.trim();

  const runTest = async () => {
    setTesting(true);
    setTestRes(null);
    try {
      const res = await ConfigService.Test(form());
      setTestRes(res);
    } catch (e: any) {
      setTestRes({ ok: false, message: String(e?.message ?? e), bucketCount: 0 } as TestResult);
    } finally {
      setTesting(false);
    }
  };

  const submit = async () => {
    if (!valid()) return;
    setSaving(true);
    try {
      await saveConnection(form());
      toast.success(t("common.success"));
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
          <DialogTitle>{props.edit ? t("connections.edit") : t("connections.add")}</DialogTitle>
          <DialogDescription>{t("connections.subtitle")}</DialogDescription>
        </DialogHeader>

        <div class="grid gap-4">
          <div class="grid gap-1.5">
            <Label>{t("connections.displayName")}</Label>
            <Input
              value={form().name}
              placeholder={t("connections.displayNamePlaceholder")}
              onInput={(e) => set("name", e.currentTarget.value)}
            />
          </div>

          <div class="grid gap-1.5">
            <Label>{t("connections.endpoint")}</Label>
            <Input
              value={form().endpoint}
              placeholder={t("connections.endpointPlaceholder")}
              onInput={(e) => set("endpoint", e.currentTarget.value)}
            />
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="grid gap-1.5">
              <Label>{t("connections.region")}</Label>
              <Input
                value={form().region}
                placeholder={t("connections.regionPlaceholder")}
                onInput={(e) => set("region", e.currentTarget.value)}
              />
            </div>
            <div class="grid gap-1.5">
              <Label>{t("connections.addressingStyle")}</Label>
              <SimpleSelect
                value={form().pathStyle ? "path" : "virtual"}
                onChange={(v) => set("pathStyle", v === "path")}
                options={[
                  { value: "path", label: t("connections.pathStyle") },
                  { value: "virtual", label: t("connections.virtualHosted") },
                ]}
              />
            </div>
          </div>

          <div class="grid gap-1.5">
            <Label>{t("connections.accessKey")}</Label>
            <Input
              value={form().accessKey}
              autocomplete="off"
              onInput={(e) => set("accessKey", e.currentTarget.value)}
            />
          </div>

          <div class="grid gap-1.5">
            <Label>{t("connections.secretKey")}</Label>
            <Input
              type="password"
              value={form().secretKey}
              autocomplete="off"
              onInput={(e) => set("secretKey", e.currentTarget.value)}
            />
          </div>

          <Show when={testRes()}>
            <div
              class="flex items-center gap-2 rounded-md border p-2 text-sm"
              classList={{
                "border-success/40 text-success": testRes()!.ok,
                "border-destructive/40 text-destructive": !testRes()!.ok,
              }}
            >
              <Show when={testRes()!.ok} fallback={<XCircle class="h-4 w-4" />}>
                <CheckCircle2 class="h-4 w-4" />
              </Show>
              <span>
                {testRes()!.ok
                  ? t("connections.testOk", { count: testRes()!.bucketCount })
                  : t("connections.testFailed", { message: testRes()!.message })}
              </span>
            </div>
          </Show>
        </div>

        <DialogFooter class="mt-2">
          <Button variant="outline" onClick={runTest} disabled={!valid() || testing()}>
            <Show when={testing()}>
              <Spinner class="h-4 w-4" />
            </Show>
            {testing() ? t("connections.testing") : t("connections.test")}
          </Button>
          <Button onClick={submit} disabled={!valid() || saving()}>
            {saving() ? t("common.saving") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
