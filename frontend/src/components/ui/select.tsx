import { type Component, Show } from "solid-js";
import { Select as SelectPrimitive } from "@kobalte/core/select";
import { Check, ChevronsUpDown } from "lucide-solid";
import { cn } from "~/lib/utils";

export type SelectOption = { value: string; label: string; disabled?: boolean };

/**
 * A simple single-value select over string options, styled like solid-ui and
 * rendered in a portal so it escapes overflow containers.
 */
export const SimpleSelect: Component<{
  options: SelectOption[];
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  class?: string;
  disabled?: boolean;
}> = (props) => {
  const selected = () => props.options.find((o) => o.value === props.value) ?? null;

  return (
    <SelectPrimitive<SelectOption>
      options={props.options}
      optionValue="value"
      optionTextValue="label"
      optionDisabled="disabled"
      value={selected()}
      onChange={(opt) => opt && props.onChange?.(opt.value)}
      disabled={props.disabled}
      placeholder={props.placeholder}
      itemComponent={(itemProps) => (
        <SelectPrimitive.Item
          item={itemProps.item}
          class="relative flex w-full cursor-pointer select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm outline-none data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50"
        >
          <span class="absolute left-2 flex h-3.5 w-3.5 items-center justify-center">
            <SelectPrimitive.ItemIndicator>
              <Check class="h-4 w-4" />
            </SelectPrimitive.ItemIndicator>
          </span>
          <SelectPrimitive.ItemLabel>{itemProps.item.rawValue.label}</SelectPrimitive.ItemLabel>
        </SelectPrimitive.Item>
      )}
    >
      <SelectPrimitive.Trigger
        class={cn(
          "flex h-9 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
          props.class
        )}
      >
        <SelectPrimitive.Value<SelectOption> class="truncate">
          {(state) => (
            <Show when={state.selectedOption()} fallback={props.placeholder}>
              {state.selectedOption()?.label}
            </Show>
          )}
        </SelectPrimitive.Value>
        <ChevronsUpDown class="h-4 w-4 opacity-50" />
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content class="relative z-50 min-w-[8rem] overflow-hidden rounded-md border bg-popover text-popover-foreground shadow-md animate-content-show">
          <SelectPrimitive.Listbox class="max-h-72 overflow-y-auto p-1" />
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive>
  );
};
