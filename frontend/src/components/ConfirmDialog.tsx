import { type Component, Show } from "solid-js";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { t } from "~/i18n";

export const ConfirmDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  destructive?: boolean;
  onConfirm: () => void;
}> = (props) => {
  const confirm = () => {
    props.onConfirm();
    props.onOpenChange(false);
  };

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="sm">
        <DialogHeader>
          <DialogTitle>{props.title}</DialogTitle>
          <DialogDescription>{props.message}</DialogDescription>
        </DialogHeader>
        <DialogFooter class="mt-2">
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>
            {props.cancelText ?? t("common.cancel")}
          </Button>
          <Button variant={props.destructive ? "destructive" : "default"} onClick={confirm}>
            <Show when={props.confirmText} fallback={t("common.confirm")}>
              {props.confirmText}
            </Show>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
