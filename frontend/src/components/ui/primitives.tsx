import { splitProps, type Component, type ComponentProps, type JSX } from "solid-js";
import { cn } from "~/lib/utils";

/* ---------------------------------- Card ---------------------------------- */

export const Card: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <div
      class={cn("rounded-lg border bg-card text-card-foreground shadow-sm", local.class)}
      {...rest}
    />
  );
};

export const CardHeader: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return <div class={cn("flex flex-col space-y-1.5 p-5", local.class)} {...rest} />;
};

export const CardTitle: Component<ComponentProps<"h3">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return <h3 class={cn("font-semibold leading-none tracking-tight", local.class)} {...rest} />;
};

export const CardDescription: Component<ComponentProps<"p">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return <p class={cn("text-sm text-muted-foreground", local.class)} {...rest} />;
};

export const CardContent: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return <div class={cn("p-5 pt-0", local.class)} {...rest} />;
};

export const CardFooter: Component<ComponentProps<"div">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return <div class={cn("flex items-center p-5 pt-0", local.class)} {...rest} />;
};

/* --------------------------------- Input ---------------------------------- */

export const Input: Component<ComponentProps<"input">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <input
      class={cn(
        "flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
        local.class
      )}
      {...rest}
    />
  );
};

export const Textarea: Component<ComponentProps<"textarea">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <textarea
      class={cn(
        "flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
        local.class
      )}
      {...rest}
    />
  );
};

export const Label: Component<ComponentProps<"label">> = (props) => {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <label
      class={cn("text-sm font-medium leading-none text-foreground", local.class)}
      {...rest}
    />
  );
};

/* --------------------------------- Badge ---------------------------------- */

type BadgeVariant = "default" | "secondary" | "destructive" | "outline" | "success" | "warning";

export const Badge: Component<ComponentProps<"span"> & { variant?: BadgeVariant }> = (props) => {
  const [local, rest] = splitProps(props, ["class", "variant"]);
  const variants: Record<BadgeVariant, string> = {
    default: "border-transparent bg-primary text-primary-foreground",
    secondary: "border-transparent bg-secondary text-secondary-foreground",
    destructive: "border-transparent bg-destructive text-destructive-foreground",
    success: "border-transparent bg-success text-success-foreground",
    warning: "border-transparent bg-warning text-warning-foreground",
    outline: "text-foreground",
  };
  return (
    <span
      class={cn(
        "inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium",
        variants[local.variant ?? "default"],
        local.class
      )}
      {...rest}
    />
  );
};

/* ------------------------------- Separator -------------------------------- */

export const Separator: Component<ComponentProps<"div"> & { orientation?: "horizontal" | "vertical" }> = (
  props
) => {
  const [local, rest] = splitProps(props, ["class", "orientation"]);
  return (
    <div
      role="separator"
      class={cn(
        "shrink-0 bg-border",
        local.orientation === "vertical" ? "h-full w-px" : "h-px w-full",
        local.class
      )}
      {...rest}
    />
  );
};

/* -------------------------------- Progress -------------------------------- */

export const Progress: Component<{ value?: number; class?: string; indeterminate?: boolean }> = (
  props
) => {
  return (
    <div class={cn("relative h-2 w-full overflow-hidden rounded-full bg-secondary", props.class)}>
      <div
        class={cn(
          "h-full bg-primary transition-[width] duration-200",
          props.indeterminate && "animate-pulse"
        )}
        style={{ width: `${Math.max(0, Math.min(100, props.value ?? 0))}%` }}
      />
    </div>
  );
};

/* -------------------------------- Spinner --------------------------------- */

export const Spinner: Component<{ class?: string }> = (props) => (
  <svg
    class={cn("animate-spin", props.class)}
    width="16"
    height="16"
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
  >
    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
    <path
      class="opacity-75"
      fill="currentColor"
      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
    />
  </svg>
);

/* -------------------------------- Checkbox -------------------------------- */

export const Checkbox: Component<{
  checked?: boolean;
  indeterminate?: boolean;
  onChange?: (checked: boolean) => void;
  disabled?: boolean;
  class?: string;
  "aria-label"?: string;
}> = (props) => {
  return (
    <input
      type="checkbox"
      aria-label={props["aria-label"]}
      checked={props.checked}
      disabled={props.disabled}
      ref={(el) => el && (el.indeterminate = !!props.indeterminate)}
      onChange={(e) => props.onChange?.(e.currentTarget.checked)}
      class={cn(
        "h-4 w-4 shrink-0 cursor-pointer rounded border-input text-primary accent-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
        props.class
      )}
    />
  );
};

/* ---------------------------------- Kbd ----------------------------------- */

export const Kbd: Component<{ children: JSX.Element; class?: string }> = (props) => (
  <kbd
    class={cn(
      "pointer-events-none inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground",
      props.class
    )}
  >
    {props.children}
  </kbd>
);
