import { splitProps, type Component, type ComponentProps, type JSX } from "solid-js";
import { Dialog as DialogPrimitive } from "@kobalte/core/dialog";
import { X } from "lucide-solid";
import { cn } from "~/lib/utils";

export const Dialog: Component<{
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  children: JSX.Element;
}> = (props) => (
  <DialogPrimitive open={props.open} onOpenChange={props.onOpenChange}>
    {props.children}
  </DialogPrimitive>
);

export const DialogContent: Component<
  ComponentProps<"div"> & { hideClose?: boolean; size?: "sm" | "md" | "lg" | "xl" }
> = (props) => {
  const [local, rest] = splitProps(props, ["class", "children", "hideClose", "size"]);
  const sizes = { sm: "max-w-sm", md: "max-w-lg", lg: "max-w-2xl", xl: "max-w-4xl" };
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay class="fixed inset-0 z-50 bg-black/50 data-[expanded]:animate-in data-[closed]:animate-out" />
      <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <DialogPrimitive.Content
          class={cn(
            "relative z-50 grid w-full gap-4 rounded-lg border bg-background p-6 shadow-lg animate-content-show",
            sizes[local.size ?? "md"],
            "max-h-[85vh] overflow-y-auto",
            local.class
          )}
          {...rest}
        >
          {local.children}
          {!local.hideClose && (
            <DialogPrimitive.CloseButton class="absolute right-4 top-4 rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring">
              <X class="h-4 w-4" />
            </DialogPrimitive.CloseButton>
          )}
        </DialogPrimitive.Content>
      </div>
    </DialogPrimitive.Portal>
  );
};

export const DialogHeader: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return <div class={cn("flex flex-col space-y-1.5 text-left", local.class)} {...rest} />;
};

export const DialogFooter: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <div class={cn("flex flex-col-reverse gap-2 sm:flex-row sm:justify-end", local.class)} {...rest} />
  );
};

export const DialogTitle: Component<ComponentProps<"h2">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <DialogPrimitive.Title
      class={cn("text-lg font-semibold leading-none tracking-tight", local.class)}
      {...rest}
    />
  );
};

export const DialogDescription: Component<ComponentProps<"p">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <DialogPrimitive.Description class={cn("text-sm text-muted-foreground", local.class)} {...rest} />
  );
};

export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogCloseButton = DialogPrimitive.CloseButton;
