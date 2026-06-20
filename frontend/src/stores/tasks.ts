import { createSignal } from "solid-js";
import { QueueService, type Task, windowID, onEvent } from "~/lib/api";

const wid = windowID();

const [tasks, setTasks] = createSignal<Task[]>([]);
const [queueState, setQueueState] = createSignal({ autoConsume: true, concurrency: 5 });
export { tasks, queueState };

/** Register this window's queue and load its tasks + control state. */
export async function initQueue() {
  try {
    const t = await QueueService.RegisterWindow(wid);
    setTasks(t ?? []);
    const st: any = await QueueService.State(wid);
    setQueueState({
      autoConsume: !!st?.autoConsume,
      concurrency: Number(st?.concurrency) || 5,
    });
  } catch (e) {
    console.error("initQueue", e);
  }
}

onEvent<any>("tasks:changed", (d) => {
  if (d?.windowId === wid) setTasks(d.tasks ?? []);
});

onEvent<any>("task:progress", (d) => {
  if (d?.windowId !== wid) return;
  setTasks((prev) =>
    prev.map((t) =>
      t.id === d.id ? ({ ...t, transferred: d.transferred, size: d.size } as Task) : t
    )
  );
});

onEvent<any>("queue:state", (d) => {
  if (d?.windowId === wid)
    setQueueState({
      autoConsume: !!d.state?.autoConsume,
      concurrency: Number(d.state?.concurrency) || 5,
    });
});

/** Queue control wrappers bound to this window. */
export const queue = {
  start: () => QueueService.Start(wid),
  setAutoConsume: (on: boolean) => QueueService.SetAutoConsume(wid, on),
  setConcurrency: (n: number) => QueueService.SetConcurrency(wid, n),
  retry: (id: string) => QueueService.Retry(wid, id),
  retryAll: () => QueueService.RetryAllFailed(wid),
  cancel: (id: string) => QueueService.Cancel(wid, id),
  remove: (id: string) => QueueService.Remove(wid, id),
  clearFinished: () => QueueService.ClearFinished(wid),
  setPriority: (id: string, p: number) => QueueService.SetPriority(wid, id, p),
};

/** Convenience selectors. */
export function activeCount(): number {
  return tasks().filter((t) => t.status === "running").length;
}
export function pendingCount(): number {
  return tasks().filter((t) => t.status === "pending").length;
}
export function failedCount(): number {
  return tasks().filter((t) => t.status === "failed").length;
}
