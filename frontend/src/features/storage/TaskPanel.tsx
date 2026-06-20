import { createSignal, For, Show, type Component } from "solid-js";
import {
  ChevronUp,
  ChevronDown,
  Upload,
  Download,
  Trash2,
  Copy,
  FolderInput,
  X,
  RotateCcw,
  Play,
  ArrowUp,
  ListChecks,
} from "lucide-solid";
import { Button } from "~/components/ui/button";
import { Badge, Progress, Input } from "~/components/ui/primitives";
import { Switch } from "~/components/ui/switch";
import { Tooltip } from "~/components/ui/tooltip";
import { tasks, queueState, queue, activeCount, pendingCount, failedCount } from "~/stores/tasks";
import { type Task } from "~/lib/api";
import { formatBytes, keyBasename } from "~/lib/utils";
import { cn } from "~/lib/utils";
import { t } from "~/i18n";

const typeIcon = (type: string) => {
  switch (type) {
    case "upload":
      return Upload;
    case "download":
      return Download;
    case "delete":
      return Trash2;
    case "copy":
      return Copy;
    case "move":
      return FolderInput;
    default:
      return Upload;
  }
};

const statusVariant = (s: string): "default" | "secondary" | "destructive" | "success" | "warning" => {
  switch (s) {
    case "completed":
      return "success";
    case "failed":
      return "destructive";
    case "running":
      return "default";
    case "canceled":
      return "warning";
    default:
      return "secondary";
  }
};

const TaskRow: Component<{ task: Task }> = (props) => {
  const Icon = typeIcon(props.task.type);
  const pct = () =>
    props.task.size > 0 ? Math.round((props.task.transferred / props.task.size) * 100) : 0;
  const finished = () =>
    props.task.status === "completed" || props.task.status === "failed" || props.task.status === "canceled";
  const name = () =>
    props.task.type === "copy" || props.task.type === "move"
      ? keyBasename(props.task.destKey || props.task.key)
      : keyBasename(props.task.key);

  return (
    <div class="flex items-center gap-3 border-b px-4 py-2 text-sm last:border-b-0">
      <Icon class="h-4 w-4 shrink-0 text-muted-foreground" />
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <span class="truncate font-medium" title={props.task.key}>
            {name()}
          </span>
          <Badge variant={statusVariant(props.task.status)} class="shrink-0">
            {t(`queue.status.${props.task.status}`)}
          </Badge>
          <Show when={props.task.retries > 0}>
            <span class="shrink-0 text-xs text-muted-foreground">×{props.task.retries}</span>
          </Show>
        </div>
        <Show when={props.task.status === "running" && props.task.size > 0}>
          <div class="mt-1 flex items-center gap-2">
            <Progress value={pct()} class="h-1.5 flex-1" />
            <span class="shrink-0 text-xs text-muted-foreground">
              {formatBytes(props.task.transferred)} / {formatBytes(props.task.size)}
            </span>
          </div>
        </Show>
        <Show when={props.task.status === "failed" && props.task.error}>
          <div class="mt-0.5 truncate text-xs text-destructive" title={props.task.error}>
            {props.task.error}
          </div>
        </Show>
        <Show when={props.task.status === "running" && props.task.size === 0}>
          <div class="mt-1">
            <Progress indeterminate value={100} class="h-1.5" />
          </div>
        </Show>
      </div>

      <div class="flex shrink-0 items-center gap-1">
        <Show when={props.task.status === "pending"}>
          <Tooltip label={t("queue.raisePriority")}>
            <Button size="icon-sm" variant="ghost" onClick={() => queue.setPriority(props.task.id, props.task.priority + 1)}>
              <ArrowUp class="h-3.5 w-3.5" />
            </Button>
          </Tooltip>
        </Show>
        <Show when={props.task.status === "failed" || props.task.status === "canceled"}>
          <Tooltip label={t("common.retry")}>
            <Button size="icon-sm" variant="ghost" onClick={() => queue.retry(props.task.id)}>
              <RotateCcw class="h-3.5 w-3.5" />
            </Button>
          </Tooltip>
        </Show>
        <Show when={props.task.status === "running" || props.task.status === "pending"}>
          <Tooltip label={t("queue.cancel")}>
            <Button size="icon-sm" variant="ghost" onClick={() => queue.cancel(props.task.id)}>
              <X class="h-3.5 w-3.5" />
            </Button>
          </Tooltip>
        </Show>
        <Show when={finished()}>
          <Button size="icon-sm" variant="ghost" onClick={() => queue.remove(props.task.id)}>
            <X class="h-3.5 w-3.5" />
          </Button>
        </Show>
      </div>
    </div>
  );
};

export const TaskPanel: Component = () => {
  const [expanded, setExpanded] = createSignal(false);

  return (
    <div class="shrink-0 border-t bg-card">
      {/* Control bar */}
      <div class="flex h-11 items-center gap-3 px-4">
        <button
          class="flex items-center gap-2 text-sm font-medium"
          onClick={() => setExpanded((e) => !e)}
        >
          <ListChecks class="h-4 w-4" />
          {t("queue.title")}
          <Show when={expanded()} fallback={<ChevronUp class="h-4 w-4" />}>
            <ChevronDown class="h-4 w-4" />
          </Show>
        </button>

        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <Show when={activeCount()}>
            <span class="text-primary">{t("queue.active", { count: activeCount() })}</span>
          </Show>
          <Show when={pendingCount()}>
            <span>{t("queue.waiting", { count: pendingCount() })}</span>
          </Show>
          <Show when={failedCount()}>
            <span class="text-destructive">{failedCount()} ✕</span>
          </Show>
        </div>

        <div class="ml-auto flex items-center gap-3">
          <label class="flex items-center gap-2 text-xs">
            {t("queue.autoConsume")}
            <Switch
              checked={queueState().autoConsume}
              onChange={(v) => queue.setAutoConsume(v)}
            />
          </label>
          <Show when={!queueState().autoConsume}>
            <Button size="sm" variant="outline" onClick={() => queue.start()} disabled={!pendingCount()}>
              <Play class="h-3.5 w-3.5" />
              {t("queue.start")}
            </Button>
          </Show>
          <label class="flex items-center gap-1.5 text-xs">
            {t("queue.concurrency")}
            <Input
              type="number"
              min={1}
              max={32}
              class="h-7 w-16"
              value={queueState().concurrency}
              onChange={(e) => queue.setConcurrency(Math.max(1, Number(e.currentTarget.value) || 1))}
            />
          </label>
          <Show when={failedCount()}>
            <Button size="sm" variant="ghost" onClick={() => queue.retryAll()}>
              {t("queue.retryAll")}
            </Button>
          </Show>
          <Button size="sm" variant="ghost" onClick={() => queue.clearFinished()}>
            {t("queue.clearFinished")}
          </Button>
        </div>
      </div>

      {/* Task list */}
      <Show when={expanded()}>
        <div class={cn("max-h-72 overflow-y-auto border-t")}>
          <Show
            when={tasks().length}
            fallback={<div class="py-8 text-center text-sm text-muted-foreground">{t("queue.noTasks")}</div>}
          >
            <For each={tasks()}>{(task) => <TaskRow task={task} />}</For>
          </Show>
        </div>
      </Show>
    </div>
  );
};
