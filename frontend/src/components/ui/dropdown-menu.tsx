import { splitProps, type Component, type ComponentProps } from "solid-js";
import { DropdownMenu as Primitive } from "@kobalte/core/dropdown-menu";
import { cn } from "~/lib/utils";

export const DropdownMenu = Primitive;
export const DropdownMenuTrigger = Primitive.Trigger;
export const DropdownMenuGroup = Primitive.Group;

export const DropdownMenuContent: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <Primitive.Portal>
      <Primitive.Content
        class={cn(
          "z-50 min-w-[10rem] overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md animate-content-show",
          local.class
        )}
        {...rest}
      />
    </Primitive.Portal>
  );
};

export const DropdownMenuItem: Component<
  ComponentProps<"div"> & { destructive?: boolean; disabled?: boolean; onSelect?: () => void }
> = (props) => {
  const [local, rest] = splitProps(props, ["class", "destructive"]);
  return (
    <Primitive.Item
      class={cn(
        "relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none transition-colors data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        local.destructive &&
          "text-destructive data-[highlighted]:bg-destructive data-[highlighted]:text-destructive-foreground",
        local.class
      )}
      {...rest}
    />
  );
};

export const DropdownMenuSeparator: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return <Primitive.Separator class={cn("-mx-1 my-1 h-px bg-muted", local.class)} {...rest} />;
};

export const DropdownMenuLabel: Component<{ class?: string; children?: any }> = (props) => {
  return (
    <Primitive.GroupLabel class={cn("px-2 py-1.5 text-xs font-semibold text-muted-foreground", props.class)}>
      {props.children}
    </Primitive.GroupLabel>
  );
};
