import { createSignal, createEffect, Show, type Component } from "solid-js";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/primitives";
import { t } from "~/i18n";

/** A small single-text-input dialog (bucket name, folder name, jump-to-path…). */
export const PromptDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  placeholder?: string;
  initial?: string;
  confirmText?: string;
  onSubmit: (value: string) => void | Promise<void>;
}> = (props) => {
  const [value, setValue] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  createEffect(() => {
    if (props.open) setValue(props.initial ?? "");
  });

  const submit = async () => {
    const v = value().trim();
    if (!v) return;
    setBusy(true);
    try {
      await props.onSubmit(v);
      props.onOpenChange(false);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="sm">
        <DialogHeader>
          <DialogTitle>{props.title}</DialogTitle>
          <Show when={props.description}>
            <DialogDescription>{props.description}</DialogDescription>
          </Show>
        </DialogHeader>
        <Input
          autofocus
          value={value()}
          placeholder={props.placeholder}
          onInput={(e) => setValue(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && submit()}
        />
        <DialogFooter class="mt-2">
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={submit} disabled={busy() || !value().trim()}>
            {props.confirmText ?? t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
