import { createSignal, For, type Component } from "solid-js";
import { Portal } from "solid-js/web";
import { CheckCircle2, XCircle, Info, X } from "lucide-solid";
import { cn } from "~/lib/utils";

export type ToastKind = "success" | "error" | "info";
type ToastItem = { id: number; kind: ToastKind; message: string };

const [items, setItems] = createSignal<ToastItem[]>([]);
let counter = 0;

function push(kind: ToastKind, message: string, timeout = 4000) {
  const id = ++counter;
  setItems((prev) => [...prev, { id, kind, message }]);
  window.setTimeout(() => dismiss(id), timeout);
}

function dismiss(id: number) {
  setItems((prev) => prev.filter((t) => t.id !== id));
}

/** Global toast API. */
export const toast = {
  success: (m: string) => push("success", m),
  error: (m: string) => push("error", m, 6000),
  info: (m: string) => push("info", m),
};

const icons = {
  success: CheckCircle2,
  error: XCircle,
  info: Info,
};

/** Renders the toast stack; mount once near the app root. */
export const Toaster: Component = () => {
  return (
    <Portal>
      <div class="pointer-events-none fixed bottom-4 right-4 z-[100] flex w-80 flex-col gap-2">
        <For each={items()}>
          {(t) => {
            const Icon = icons[t.kind];
            return (
              <div
                class={cn(
                  "pointer-events-auto flex items-start gap-3 rounded-lg border bg-popover p-3 text-sm text-popover-foreground shadow-lg animate-content-show"
                )}
              >
                <Icon
                  class={cn(
                    "mt-0.5 h-4 w-4 shrink-0",
                    t.kind === "success" && "text-success",
                    t.kind === "error" && "text-destructive",
                    t.kind === "info" && "text-muted-foreground"
                  )}
                />
                <span class="flex-1 break-words">{t.message}</span>
                <button
                  class="text-muted-foreground hover:text-foreground"
                  onClick={() => dismiss(t.id)}
                >
                  <X class="h-3.5 w-3.5" />
                </button>
              </div>
            );
          }}
        </For>
      </div>
    </Portal>
  );
};
