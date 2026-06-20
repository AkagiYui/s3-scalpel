import { type Component } from "solid-js";
import { Switch as SwitchPrimitive } from "@kobalte/core/switch";
import { cn } from "~/lib/utils";

export const Switch: Component<{
  checked?: boolean;
  onChange?: (checked: boolean) => void;
  disabled?: boolean;
  class?: string;
}> = (props) => {
  return (
    <SwitchPrimitive
      checked={props.checked}
      onChange={props.onChange}
      disabled={props.disabled}
      class={cn("inline-flex items-center", props.class)}
    >
      <SwitchPrimitive.Input class="peer" />
      <SwitchPrimitive.Control
        class={cn(
          "inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors",
          "data-[checked]:bg-primary data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50",
          "bg-input focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background"
        )}
      >
        <SwitchPrimitive.Thumb class="pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform data-[checked]:translate-x-4 translate-x-0" />
      </SwitchPrimitive.Control>
    </SwitchPrimitive>
  );
};
