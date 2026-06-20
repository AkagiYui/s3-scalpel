import { type Component, type JSX } from "solid-js";
import { Tooltip as Primitive } from "@kobalte/core/tooltip";
import { cn } from "~/lib/utils";

export const Tooltip: Component<{
  label: JSX.Element;
  children: JSX.Element;
  placement?: "top" | "bottom" | "left" | "right";
  class?: string;
}> = (props) => (
  <Primitive placement={props.placement ?? "top"} openDelay={400} closeDelay={100}>
    <Primitive.Trigger as="span" class={cn("inline-flex", props.class)}>
      {props.children}
    </Primitive.Trigger>
    <Primitive.Portal>
      <Primitive.Content class="z-50 overflow-hidden rounded-md bg-primary px-2.5 py-1 text-xs text-primary-foreground shadow-md animate-content-show">
        {props.label}
      </Primitive.Content>
    </Primitive.Portal>
  </Primitive>
);
