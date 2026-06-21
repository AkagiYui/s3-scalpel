import { createSignal, createEffect, For, Show, type Component } from "solid-js";
import { Check, X, Minus, RefreshCw, ShieldCheck } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Spinner } from "~/components/ui/primitives";
import { S3Service, type Capability } from "~/lib/api";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";

export const CapabilitiesDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  /** Empty string probes account-level operations only. */
  bucket?: string;
}> = (props) => {
  const [caps, setCaps] = createSignal<Capability[]>([]);
  const [loading, setLoading] = createSignal(false);

  const run = async () => {
    setLoading(true);
    try {
      const result = await S3Service.CheckCapabilities(props.connId, props.bucket ?? "");
      setCaps(result ?? []);
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    if (props.open) run();
  });

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <ShieldCheck class="h-4 w-4" />
            {t("capabilities.title")}
          </DialogTitle>
          <DialogDescription>{t("capabilities.subtitle")}</DialogDescription>
        </DialogHeader>

        <Show when={!props.bucket}>
          <div class="rounded-md bg-muted/50 px-3 py-2 text-xs text-muted-foreground">
            {t("capabilities.accountScope")}
          </div>
        </Show>

        <Show
          when={!loading()}
          fallback={
            <div class="flex items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
              <Spinner class="h-5 w-5" />
              {t("capabilities.checking")}
            </div>
          }
        >
          <div class="divide-y">
            <For each={caps()}>
              {(c) => (
                <div class="flex items-center justify-between gap-3 py-2">
                  <div class="flex items-center gap-2">
                    <Show
                      when={c.tested}
                      fallback={<Minus class="h-4 w-4 text-muted-foreground" />}
                    >
                      <Show
                        when={c.allowed}
                        fallback={
                          <span class="flex h-5 w-5 items-center justify-center rounded-full bg-destructive/15">
                            <X class="h-3.5 w-3.5 text-destructive" />
                          </span>
                        }
                      >
                        <span class="flex h-5 w-5 items-center justify-center rounded-full bg-success/15">
                          <Check class="h-3.5 w-3.5 text-success" />
                        </span>
                      </Show>
                    </Show>
                    <span class="text-sm">{t(`capabilities.ops.${c.op}`)}</span>
                  </div>
                  <span class="text-xs text-muted-foreground">
                    {!c.tested
                      ? t("capabilities.untested")
                      : c.allowed
                        ? t("capabilities.allowed")
                        : c.detail || t("capabilities.denied")}
                  </span>
                </div>
              )}
            </For>
          </div>
        </Show>

        <div class="flex items-center justify-between gap-2 pt-1">
          <span class="text-xs text-muted-foreground">{t("capabilities.note")}</span>
          <Button variant="outline" size="sm" onClick={run} disabled={loading()}>
            <RefreshCw class={loading() ? "h-3.5 w-3.5 animate-spin" : "h-3.5 w-3.5"} />
            {t("capabilities.recheck")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
};
