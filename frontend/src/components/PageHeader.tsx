import { type Component, type JSX, Show } from "solid-js";

export const PageHeader: Component<{
  title: string;
  subtitle?: string;
  children?: JSX.Element;
}> = (props) => {
  return (
    <header class="flex h-14 shrink-0 items-center justify-between gap-4 border-b px-6">
      <div class="min-w-0">
        <h1 class="truncate text-base font-semibold leading-tight">{props.title}</h1>
        <Show when={props.subtitle}>
          <p class="truncate text-xs text-muted-foreground">{props.subtitle}</p>
        </Show>
      </div>
      <div class="flex shrink-0 items-center gap-2">{props.children}</div>
    </header>
  );
};
